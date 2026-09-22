package core

import (
	"context"
	"errors"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var defaultInterceptors = []interceptor.Interceptor{
	interceptor.More(),
	interceptor.Continue(),
}

// promptMatchWindow 提示符/拦截器匹配只扫描行的尾部窗口(从行边界截断)。
//
//	匹配规则均为尾部锚定，超大行(如防火墙的超长配置行，可达 16MB+)只需匹配尾部，
//	避免每个数据块都对整行做正则匹配的 O(N²) 开销
const promptMatchWindow = 8 * 1024

// Reader 交互式 shell 的读写器：驱动输出流的消费、提示符判定与拦截器调度。
//
// 使用约束：
//   - 同一时刻只允许一个 Read(并发 Read 会返回 ErrConcurrentRead)；
//   - 读取过程中可并发调用 Prompt/IsPrompt；
//   - 生命周期为 Close(幂等)，底层连接的关闭由上层 Shell 负责。
type Reader struct {
	cfg Config
	in  io.Writer
	out *stream
	err *stream // stderr 流，可能为 nil
	lo  *lazyOut

	// mu 保护 prompt、promptRegex、lastCmd：Read 过程中会写入，而 Prompt、IsPrompt 可能被其他 goroutine 调用
	mu          sync.Mutex
	prompt      string
	promptRegex []*regexp.Regexp
	lastCmd     string // 最后一次 Write 的命令(错误检测报告中引用)

	// errorPatterns 错误检测规则(len==0 表示关闭检测)
	errorPatterns []*ErrorPattern

	readMu    sync.Mutex
	closed    atomic.Bool
	closeOnce sync.Once
}

// NewReader 创建读写器。in 为命令写入端；out 为输出流；errStream 为 stderr 流(可为 nil)。
//
//	cfg 以值传入并使用内部副本，不会修改调用方传入的结构体。
func NewReader(in io.Writer, out, errStream io.Reader, cfg Config) *Reader {
	cfg = cfg.normalize()

	r := &Reader{
		cfg:         cfg,
		in:          in,
		out:         newStream(out, cfg),
		promptRegex: append([]*regexp.Regexp(nil), cfg.PromptRegex...),
	}
	if cfg.ErrorPatterns == nil {
		r.errorPatterns = DefaultErrorPatterns()
	} else {
		r.errorPatterns = cfg.ErrorPatterns
	}
	if !isNil(errStream) {
		r.err = newStream(errStream, cfg)
	}
	if cfg.LazyOutInterval > 0 || cfg.LazyOutSize > 0 {
		r.lo = newLazyOut(cfg.LazyOutInterval, cfg.LazyOutSize)
	}
	if cfg.KeepAlive != nil {
		r.startKeepAlive(cfg.KeepAlive)
	}
	return r
}

// Close 标记关闭(幂等)，冲刷延迟输出。
//
//	底层连接/进程的关闭由上层 Shell 负责，关闭后 Write 返回 ErrClosed，Read 返回 ErrClosed
func (r *Reader) Close() error {
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		if r.lo != nil {
			r.lo.Stop()
			r.lo = nil
		}
	})
	return nil
}

// Write 写入一行命令(自动在末尾补充 \n；空串等价于写入单个换行)
func (r *Reader) Write(cmd string) error {
	if cmd == "" {
		cmd = "\n"
	} else if cmd[len(cmd)-1] != '\n' {
		cmd += "\n"
	}
	// 记录命令(错误检测报告引用)；注意拦截器的应答走 WriteRaw，不覆盖
	r.mu.Lock()
	r.lastCmd = strings.TrimRight(cmd, "\n")
	r.mu.Unlock()
	return r.WriteRaw([]byte(cmd))
}

// WriteRaw 原样写入(不自动补充换行)，供密码输入、翻页应答等场景
func (r *Reader) WriteRaw(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if r.closed.Load() || isNil(r.in) {
		return &Error{Op: OpWrite, Err: ErrClosed}
	}
	if !isNil(r.cfg.RawIn) {
		// 录制输入方向(拦截器的自动应答也会经过这里)；录制失败不影响主流程
		_, _ = r.cfg.RawIn.Write(b)
	}
	if _, err := r.in.Write(b); err != nil {
		r.logf(slog.LevelWarn, "shell.write failed", "err", err)
		return &Error{Op: OpWrite, Err: err}
	}
	return nil
}

// Prompt 返回最近一次匹配到的提示符(并发安全)
func (r *Reader) Prompt() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prompt
}

// IsPrompt 判断给定内容是否命中提示符规则(并发安全，等价于 v1 的 IsEndLine)
func (r *Reader) IsPrompt(s string) bool {
	return r.isPrompt(s, nil)
}

// isPrompt 提示符判定；promptOverride 非 nil 时严格模式：仅以该规则匹配
//
//	(默认规则与其误报排除规则均不参与——调用方明确指定的规则无需兜底，也避免宽松规则误匹配输出)
func (r *Reader) isPrompt(s string, promptOverride *regexp.Regexp) bool {
	if promptOverride != nil {
		return promptOverride.MatchString(s)
	}

	r.mu.Lock()
	regexArr := r.promptRegex
	r.mu.Unlock()

	var matched bool
	for _, v := range regexArr {
		if v != nil && v.MatchString(s) {
			matched = true
		}
	}
	if !matched && DefaultPromptRegex.MatchString(s) {
		matched = true
	}

	// 尾缀特征字符可能误匹配，把已知的误匹配场景排除(如 "[user@host]$ Password:")
	if matched && (UsernameRegex.MatchString(s) || PasswordRegex.MatchString(s) || FlexibleOptionPromptRegex.MatchString(s)) {
		matched = false
	}
	return matched
}

// firstOptOf 取第一个可选项(最多一个，多余忽略)
func firstOptOf(opts []RunOptions) RunOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return RunOptions{}
}

// ReadUntilPrompt 读取输出直到提示符(等价于 Read(ctx, true, ...))。
//
//	onOut 在读取流程中同步调用：阻塞它会推迟读取与提示符判定
//	(设备可能在等待拦截器应答导致输出停住)，需要重处理时请在回调内自行异步化。
func (r *Reader) ReadUntilPrompt(ctx context.Context, onOut func(lines []string), opts ...RunOptions) error {
	return r.read(ctx, true, firstOptOf(opts).Prompt, onOut, firstOptOf(opts).Interceptors)
}

// ReadAll 读取全部输出直到流结束(等价于 Read(ctx, false, ...))
func (r *Reader) ReadAll(ctx context.Context, onOut func(lines []string), opts ...RunOptions) error {
	return r.read(ctx, false, firstOptOf(opts).Prompt, onOut, firstOptOf(opts).Interceptors)
}

// Run 写入命令并读取输出直到提示符，等价于 Write + ReadUntilPrompt。
//
//	提示符会变化的场景(进入配置模式、su/sudo 等)使用 RunOptions.Prompt 指定本次命令的结束提示符。
func (r *Reader) Run(ctx context.Context, cmd string, onOut func(lines []string), opts ...RunOptions) error {
	if err := r.Write(cmd); err != nil {
		return err
	}
	opt := firstOptOf(opts)
	return r.read(ctx, true, opt.Prompt, onOut, opt.Interceptors)
}

// InConfigMode 基于最近匹配的提示符推断是否处于配置模式(启发式)：
//
//   - H3C/华为：方括号系统视图样式 [name] / [name-subview] → true(与用户视图 <name> 区分)
//   - Cisco：提示符包含 "(config" → true
//   - Linux/其他：false
//
// 注意这是启发式判断，特殊定制的提示符可能不准确。
func (r *Reader) InConfigMode() bool {
	p := strings.TrimSpace(r.Prompt())
	if p == "" {
		return false
	}
	// Cisco 系：SW01(config)# / SW01(config-if)#
	if strings.Contains(p, "(config") {
		return true
	}
	// H3C/华为系：[SW03] / [SW03-vlan1](系统视图与子视图)，排除 Linux 的 [root@host ~] 样式
	return configModeBracketRegex.MatchString(p)
}

// configModeBracketRegex H3C/华为配置模式提示符：方括号包裹、不含空格/@/~(排除Linux的 [root@host ~])
var configModeBracketRegex = regexp.MustCompile(`^\[[^@~\s\]]{1,64}(-[^@~\s\]]+)*\]`)

// RunAll 写入命令并读取全部输出直到流结束，等价于 Write + ReadAll
func (r *Reader) RunAll(ctx context.Context, cmd string, onOut func(lines []string), opts ...RunOptions) error {
	if err := r.Write(cmd); err != nil {
		return err
	}
	opt := firstOptOf(opts)
	return r.read(ctx, false, opt.Prompt, onOut, opt.Interceptors)
}

func (r *Reader) Read(ctx context.Context, stopOnPrompt bool, onOut func(lines []string), opts ...RunOptions) (err error) {
	opt := firstOptOf(opts)
	return r.read(ctx, stopOnPrompt, opt.Prompt, onOut, opt.Interceptors)
}

// logf 日志钩子(nil 安全)，keyValues 为 slog 属性对
func (r *Reader) logf(level slog.Level, msg string, keyValues ...any) {
	if r.cfg.Logger != nil {
		r.cfg.Logger.Log(context.Background(), level, msg, keyValues...)
	}
}

// read 读取输出；promptOverride 非 nil 时仅以该规则判定命令结束(严格模式，默认规则不参与)
func (r *Reader) read(ctx context.Context, stopOnPrompt bool, promptOverride *regexp.Regexp, onOut func(lines []string), interceptors []interceptor.Interceptor) (err error) {
	// 单读者守卫：并发 Read 会互相争抢输出导致串流错乱
	if !r.readMu.TryLock() {
		return &Error{Op: OpRead, Err: ErrConcurrentRead}
	}
	defer r.readMu.Unlock()
	if r.closed.Load() {
		return &Error{Op: OpRead, Err: ErrClosed}
	}
	if r.cfg.BeforeRead != nil {
		if err = r.cfg.BeforeRead(); err != nil {
			return err
		}
	}

	r.mu.Lock()
	lastCmd := r.lastCmd
	r.mu.Unlock()
	start := time.Now()
	r.logf(slog.LevelDebug, "shell.read start", "cmd", lastCmd)

	if r.lo != nil {
		r.lo.SetOut(onOut)
		onOut = r.lo.Add
	}

	ticker := time.NewTicker(r.cfg.ReadConfirmWait)
	defer ticker.Stop()

	// pop 读取并处理输出，返回 true 表示本次读取结束(读到流结尾 或 已确认命令执行完成)
	var outBuf strings.Builder
	var stop bool
	var confirm int
	var ctxDone bool
	var devErr *DeviceError
	var devErrs []*DeviceError
	pop := func() bool {
		_, e := r.out.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
			stop = false
			if len(lines) != 0 && onOut != nil {
				onOut(lines)
			}

			// 命令输出错误检测(仅扫描已交付的完整行；登录横幅在 Shell 创建阶段已消费，不参与)
			if len(r.errorPatterns) > 0 {
				r.mu.Lock()
				cmd := r.lastCmd
				r.mu.Unlock()
				if de := detectErrors(r.errorPatterns, cmd, lines); de != nil {
					r.logf(slog.LevelWarn, "shell.device_error", "pattern", de.PatternName, "cmd", de.Cmd, "line", de.Line)
					if r.cfg.ErrorPolicy == ErrorCollect {
						devErrs = append(devErrs, de)
					} else { // ErrorFail
						devErr = de
						return true
					}
				}
			}

			// 匹配优先级：指定的拦截器规则 > 默认拦截器规则 > 命令结束提示符规则
			//	匹配内容只取尾部窗口(超大行的行内容与提示符规则均为尾部有效)
			if len(interceptors) > 0 {
				if outBuf.Len() > 0 {
					outBuf.WriteString("\n")
				}
				outBuf.WriteString(strings.Join(lines, "\n"))
				if remaining != "" {
					outBuf.WriteString("\n")
					outBuf.WriteString(remaining)
				}
				window := tailWindow(outBuf.String(), promptMatchWindow)
				for _, f := range interceptors {
					if match, showOut, input := f(window); match {
						outBuf.Reset()
						// 先丢弃过滤器的未完成行再写入应答：
						//	应答会触发设备回写新数据，必须避免其拼进即将作废的旧行
						r.out.dropFilterPending()
						// 已知限制：多行拦截器命中时，匹配窗口内的前缀行不会作为输出返回
						_ = r.Write(input) // 自动补 \n
						return !showOut
					}
				}
				// 无匹配时仅保留尾部有限窗口，避免大输出场景下内存和正则匹配开销持续增长
				trimOutBuf(&outBuf, promptMatchWindow)
			}

			if remaining == "" {
				return false
			}

			// 默认拦截器规则(More 翻页 / Continue 继续)
			for _, f := range defaultInterceptors {
				if match, showOut, input := f(remaining); match {
					outBuf.Reset()
					r.out.dropFilterPending()
					if showOut && onOut != nil {
						onOut([]string{remaining})
					}
					_ = r.WriteRaw([]byte(input))
					return !showOut
				}
			}

			// 命令输出结束(提示符命中)
			if r.isPrompt(tailWindow(remaining, promptMatchWindow), promptOverride) {
				r.mu.Lock()
				// 当未指定提示符规则 且 AutoPrompt=true 时，尝试自动纠正提示符匹配规则
				//	(指定了覆盖规则时，AutoPrompt 不生效——覆盖规则由调用方负责)
				if promptOverride == nil && len(r.promptRegex) == 0 && r.cfg.AutoPrompt {
					if re := findPromptRegex(remaining); re != nil {
						r.promptRegex = append(r.promptRegex, re)
					}
				}
				r.prompt = remaining
				r.mu.Unlock()
				stop = stopOnPrompt
				if !r.cfg.ShowPrompt {
					r.out.dropFilterPending()
				}
				return !r.cfg.ShowPrompt
			}

			return false
		})
		if e != nil {
			// 保留 err 后退出循环，后续继续处理 stderr
			if e != io.EOF && !errors.Is(e, io.ErrClosedPipe) && !errors.Is(e, io.ErrNoProgress) && !errors.Is(e, io.ErrUnexpectedEOF) {
				err = &Error{Op: OpRead, Err: e}
			}
			return true
		}
		if devErr != nil { // ErrorFail：命中设备错误立即退出
			return true
		}

		if stop {
			if confirm >= r.cfg.ReadConfirm {
				return true
			}
			confirm++
		} else {
			confirm = 0
		}
		return false
	}

	// 事件驱动 + 轮询兜底：有新数据时立即处理(降低交互延迟)，无数据时靠 ticker 递增确认次数
	outNotify := r.out.Notify()
loop:
	for {
		select {
		case <-ctx.Done():
			ctxDone = true
			switch e := ctx.Err(); {
			default:
				err = &Error{Op: OpRead, Err: e}
			case errors.Is(e, context.DeadlineExceeded):
				err = &Error{Op: OpTimeout, Err: e}
			case errors.Is(e, context.Canceled):
				err = &Error{Op: OpCanceled, Err: e}
			}
			break loop

		case <-ticker.C:
			if pop() {
				break loop
			}

		case <-outNotify:
			if pop() {
				break loop
			}
		}
	}

	// stderr 按策略处理；ctx 超时/取消的错误优先，不被 stderr 内容覆盖
	if errSt := r.drainStderr(onOut); errSt != nil && !ctxDone && err == nil {
		err = errSt
	}

	// 设备错误汇总：Fail 策略优先返回首个命中；Collect 策略聚合全部命中
	if devErr != nil && err == nil && !ctxDone {
		err = devErr
	}
	if len(devErrs) > 0 && err == nil && !ctxDone {
		errs := make([]error, len(devErrs))
		for i, de := range devErrs {
			errs[i] = de
		}
		err = &Error{Op: OpRead, Err: errors.Join(errs...)}
	}

	if err != nil {
		r.logf(slog.LevelWarn, "shell.read done", "cmd", lastCmd, "duration", time.Since(start), "err", err)
	} else {
		r.logf(slog.LevelDebug, "shell.read done", "cmd", lastCmd, "duration", time.Since(start))
	}

	if r.lo != nil {
		r.lo.Out()
	}
	return err
}

// drainStderr 读取并按策略处理 stderr
func (r *Reader) drainStderr(onOut func(lines []string)) (err error) {
	if r.err == nil {
		return nil
	}

	confirm := 0
	var buf strings.Builder
	for {
		popped, e := r.err.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(strings.Join(lines, "\n"))
			if remaining != "" {
				buf.WriteString("\n")
				buf.WriteString(remaining)
			}
			return true
		})
		if e != nil {
			if e != io.EOF && !errors.Is(e, io.ErrClosedPipe) && !errors.Is(e, io.ErrNoProgress) && !errors.Is(e, io.ErrUnexpectedEOF) {
				return &Error{Op: OpRead, Err: e}
			}
			break
		}
		if popped == 0 {
			if confirm >= r.cfg.ReadConfirm {
				break
			}
			confirm++
		} else {
			confirm = 0
		}
		time.Sleep(r.cfg.ReadConfirmWait)
	}

	if buf.Len() == 0 {
		return nil
	}
	switch r.cfg.Stderr {
	case StderrIgnore:
		return nil
	case StderrOutput:
		if onOut != nil {
			onOut(strings.Split(buf.String(), "\n"))
		}
		return nil
	default: // StderrError
		return &Error{Op: OpRead, Err: errors.New(buf.String())}
	}
}

// trimOutBuf 将拦截器匹配缓冲裁剪为尾部 limit 字节的内容(从行边界截断)
func trimOutBuf(buf *strings.Builder, limit int) {
	if buf.Len() <= limit {
		return
	}
	s := buf.String()
	tail := s[len(s)-limit:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	buf.Reset()
	buf.WriteString(tail)
}

// findPromptRegex 基于提示符内容自动生成更精确的匹配规则。
//
//	由于提示符的格式非常自由，自动识别有可能错误，应视情况使用(AutoPrompt)。
func findPromptRegex(remaining string) *regexp.Regexp {
	// 提示符在交互过程中可能变化(进入子模式、切换用户、主备切换等)，先提取主机名再通配尾部
	hostname := findHostname(remaining)
	if hostname == "" {
		return nil
	}

	// 主机名可能被设备缩写，取前10个字符作为前缀匹配
	var prefix string
	if runStr := []rune(hostname); len(runStr) > 10 {
		prefix = string(runStr[:10])
	}

	var pattern string
	if prefix != "" {
		pattern = `(?i)(` + regexp.QuoteMeta(hostname) + `|` + regexp.QuoteMeta(prefix) + `)` + DefaultPromptSuffixPattern
	} else {
		pattern = `(?i)` + regexp.QuoteMeta(hostname) + DefaultPromptSuffixPattern
	}
	if re, err := regexp.Compile(pattern); err == nil {
		return re
	}
	return nil
}

// findHostname 从提示符内容中提取主机名。
//
//	提示符格式因设备类型、厂商、用户配置而异，可能包含中文与特殊字符，这里只能尽量匹配：
//	  Linux: [root@localhost ~]# / [root@192.168.1.24 /home/admin]$
//	  网络设备: hostname# / hostname(config)# / <HUAWEI> / HRP_M[HUAWEI-diagnose]
//	  山石防火墙缩写: S-ABC-D1-EFG-~(M)#
func findHostname(remaining string) string {
	if remaining == "" {
		return ""
	}

	// 移除结束符以及前后空格
	hostname := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(remaining), DefaultPromptTailChars))
	// 如果包含@，取@后面的内容作为主机名
	if idx := strings.IndexByte(hostname, '@'); idx != -1 {
		hostname = hostname[idx+1:]
	}
	// 如果包含空格、波浪号，取空格、波浪号前面内容作为主机名
	if idx := strings.IndexAny(hostname, " ~"); idx != -1 {
		hostname = hostname[:idx]
	}
	// 如果包含左括号，取左括号后面的内容作为主机名
	if idx := strings.IndexAny(hostname, "<(["); idx != -1 {
		hostname = hostname[idx+1:]
	}
	// 如果包含右括号，取右括号前面的内容作为主机名
	if idx := strings.IndexAny(hostname, ">)]"); idx != -1 {
		hostname = hostname[:idx]
	}

	return hostname
}
