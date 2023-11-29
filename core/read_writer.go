package core

import (
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/lazyOut"
	"github.com/3th1nk/easyshell/internal/lineReader"
	"github.com/3th1nk/easyshell/pkg/interceptor"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	promptTailChars = `#$>)\]:~%`
	promptSuffix    = `[\s\S]*[` + promptTailChars + `]+\s*$`
)

var (
	defaultInterceptors = []interceptor.Interceptor{
		interceptor.More(),
		interceptor.Continue(),
	}
	defaultPromptRegex = regexp.MustCompile(`\S+` + promptSuffix)
)

func New(in io.Writer, out, err io.Reader, cfg Config) *ReadWriter {
	if cfg.ReadConfirmWait <= 0 {
		cfg.ReadConfirmWait = 20 * time.Millisecond
	}

	if cfg.ReadConfirm <= 0 {
		cfg.ReadConfirm = 3
	}

	var opts []lineReader.Option
	if cfg.Filter != nil {
		opts = append(opts, lineReader.WithFilter(cfg.Filter))
	}
	if cfg.Decoder != nil {
		opts = append(opts, lineReader.WithDecoder(cfg.Decoder))
	}

	r := &ReadWriter{
		in:  in,
		out: lineReader.New(out, opts...),
		err: lineReader.New(err, opts...),
		cfg: cfg,
	}
	if cfg.LazyOutInterval > 0 || cfg.LazyOutSize > 0 {
		r.lo = lazyOut.New(cfg.LazyOutInterval, cfg.LazyOutSize)
	}
	return r
}

type ReadWriter struct {
	cfg      Config
	in       io.Writer
	out, err *lineReader.LineReader
	lo       *lazyOut.LazyOut
	prompt   string
}

func (r *ReadWriter) Stop() {
	if r.lo != nil {
		r.lo.Stop()
		r.lo = nil
	}
	r.in, r.out, r.err = nil, nil, nil
}

// Write 写入一个命令（自动在末尾补充 \n 换行符）。
func (r *ReadWriter) Write(cmd string) (err error) {
	if cmd == "" {
		cmd = "\n"
	} else if cmd[len(cmd)-1] != '\n' {
		cmd += "\n"
	}
	return r.WriteRaw([]byte(cmd))
}

// WriteRaw 向输入流写入指定内容，并等待指定时间（默认 10 毫秒）。
func (r *ReadWriter) WriteRaw(b []byte) (err error) {
	if len(b) != 0 {
		_, err = r.in.Write(b)
	}
	return nil
}

// Prompt 命令交互过程中提示符可能发生变化，该方法获取最新的提示符
func (r *ReadWriter) Prompt() string {
	return r.prompt
}

func (r *ReadWriter) ReadToEndLine(timeout time.Duration, onOut func(lines []string), interceptors ...interceptor.Interceptor) (err error) {
	return r.Read(true, timeout, onOut, interceptors...)
}

func (r *ReadWriter) ReadAll(timeout time.Duration, onOut func(lines []string), interceptors ...interceptor.Interceptor) (err error) {
	return r.Read(false, timeout, onOut, interceptors...)
}

func (r *ReadWriter) Read(stopOnEndLine bool, timeout time.Duration, onOut func(lines []string), interceptors ...interceptor.Interceptor) (err error) {
	if r.cfg.BeforeRead != nil {
		if err = r.cfg.BeforeRead(); err != nil {
			return err
		}
	}

	if r.lo != nil {
		r.lo.SetOut(onOut)
		onOut = r.lo.Add
	}

	timeoutAt := time.Now().Add(timeout)
	var lineBuf string
	var stop bool
	var confirm int
	for {
		_, e := r.out.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
			if len(lines) != 0 && onOut != nil {
				onOut(lines)
			}

			// 命令行结束提示符
			if remaining != "" && r.IsEndLine(remaining) {
				r.prompt = remaining
				// 仅当默认的提示符匹配规则匹配上 且 AutoPrompt=true 时，尝试自动纠正提示符匹配规则
				if len(r.cfg.PromptRegex) == 0 && r.cfg.AutoPrompt {
					if re := findPromptRegex(remaining); re != nil {
						r.cfg.PromptRegex = append(r.cfg.PromptRegex, re)
						util.PrintTimeLn("correct end prompt regex:" + re.String())
					}
				}
				stop = stopOnEndLine
				return !r.cfg.ShowPrompt
			}
			stop = false

			// 缓存区所有内容
			if len(interceptors) != 0 {
				if lineBuf == "" {
					lineBuf = strings.Join(lines, "\n")
				} else {
					lineBuf += "\n" + strings.Join(lines, "\n")
				}
				if remaining != "" {
					lineBuf += "\n" + remaining
				}
				for _, f := range interceptors {
					if match, showOut, input := f(lineBuf); match {
						lineBuf = ""
						if showOut && remaining != "" && onOut != nil {
							onOut([]string{remaining})
						}
						_, _ = r.in.Write([]byte(input))
						return true
					}
				}
			}

			// 最后一行内容
			if remaining != "" {
				for _, f := range defaultInterceptors {
					if match, showOut, input := f(remaining); match {
						lineBuf = ""
						if showOut && onOut != nil {
							onOut([]string{remaining})
						}
						_, _ = r.in.Write([]byte(input))
						return true
					}
				}
			}
			return false
		})
		if e != nil {
			// 保留 err 后退出循环，继续后续的 err.PopLines
			if e != io.EOF && e != io.ErrClosedPipe && e != io.ErrNoProgress && e != io.ErrUnexpectedEOF {
				err = &Error{Op: "read", Err: e}
			}
			break
		}
		if !time.Now().Before(timeoutAt) {
			return &Error{Op: "timeout"}
		}
		// util.PrintTimeLn("--> stop=%v, confirm=%v", stop, confirm)
		if stop {
			if confirm >= r.cfg.ReadConfirm {
				// util.PrintTimeLn("--> stop read out")
				break
			} else {
				confirm++
			}
		} else {
			confirm = 0
		}
		time.Sleep(r.cfg.ReadConfirmWait)
	}

	if r.err != nil {
		confirm = 0
		var errMsg string
		for {
			popped, e := r.err.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
				if errMsg == "" {
					errMsg = strings.Join(lines, "\n")
				} else {
					errMsg += "\n" + strings.Join(lines, "\n")
				}
				if remaining != "" {
					errMsg += "\n" + remaining
				}
				return true
			})
			if e != nil && err == nil {
				// 保留 err 后退出循环，继续后续的 err.PopLines
				if e != io.EOF && e != io.ErrClosedPipe && e != io.ErrNoProgress && e != io.ErrUnexpectedEOF {
					err = e
				}
				break
			}
			if popped == 0 {
				if confirm >= r.cfg.ReadConfirm {
					// util.PrintTimeLn("--> stop read err")
					break
				} else {
					confirm++
				}
			} else {
				confirm = 0
			}
			time.Sleep(r.cfg.ReadConfirmWait)
		}
		if errMsg != "" {
			err = &Error{Op: "read", Err: fmt.Errorf(errMsg)}
		}
	}

	if r.lo != nil {
		r.lo.Out()
	}

	return
}

func (r *ReadWriter) IsEndLine(s string) bool {
	if len(r.cfg.PromptRegex) != 0 {
		for _, v := range r.cfg.PromptRegex {
			if v != nil && v.MatchString(s) {
				//util.PrintTimeLn("prompt matched:" + s)
				return true
			}
		}
		return false
	}

	if defaultPromptRegex.MatchString(s) {
		//util.PrintTimeLn("default prompt matched:" + s)
		return true
	}
	return false
}

// findPromptRegex
//	！！！由于提示符的格式非常自由，自动识别有可能错误，应视情况使用 ！！！
func findPromptRegex(remaining string) *regexp.Regexp {
	// 由于提示符在交互过程中可能会变化，这里先提取一下主机名，再通配一下尾部
	//  场景：
	//	1.网络设备配置进入模式
	//		hostname# => hostname(config)#
	//	2.主机切换用户
	//		user1@hostname# => user2@hostname#
	// 	3.华为防火墙开启双机热备，主：HRP_M（旧版本：HRP-A）、备：HRP_S
	//		[USG6000V1]hrp enable
	//		HRP_M[USG6000V1]
	hostname := findHostname(remaining)
	if hostname == "" {
		return nil
	}

	// 存在提示符被省略的情况，虽然findHostname处理过山石防火墙的情况，但是还是有可能出现其他情况，这里再通配一下
	//	山石防火墙：S-ABC-D1-EFG-~(M)#
	runStr := []rune(hostname)
	if len(runStr) > 10 {
		hostname = fmt.Sprintf(`(%v|%v\S+)`, hostname, string(runStr[:10]))
	}
	prompt := `(?i)` + hostname + promptSuffix
	return regexp.MustCompile(prompt)
}

func findHostname(remaining string) string {
	if remaining == "" {
		return ""
	}

	// 提示符格式非常自由，设备类型、厂商、用户配置不同，提示符格式也不同，可能包含中文、特殊字符，这里只能尽量匹配
	//	1.Linux主机：
	//		[root@localhost ~]#
	//		[localhost.localdomain ~]$
	//  2.网络设备配置模式：
	//		hostname#
	//		hostname(config)#
	//  3.中文主机名：
	//		中文主机名 #
	//	4.华为防火墙：
	//		<HUAWEI>hrp enable
	//		HRP_M<HUAWEI> system-view
	//		HRP_M[HUAWEI] diagnose
	//		HRP_M[HUAWEI-diagnose] display firewall cpu-table
	//	5.山石防火墙，主机名超过长度缩写：
	//		S-ABC-D1-EFG-~(M)#
	//	6.提示符的结束字符可能有多种情况：
	//		# $ > ) ] : ~ %

	// 移除结束符以及前后空格
	hostname := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(remaining), promptTailChars))
	// 如果包含@，取@后面的内容作为主机名
	if idx := strings.IndexByte(hostname, '@'); idx != -1 {
		hostname = hostname[idx+1:]
	}
	// 如果包含.，取.前面的内容作为主机名
	if idx := strings.IndexByte(hostname, '.'); idx != -1 {
		hostname = hostname[:idx]
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
