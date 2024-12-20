package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/lazyOut"
	"github.com/3th1nk/easyshell/internal/lineReader"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/interceptor"
	"io"
	"regexp"
	"strings"
	"time"
)

var defaultInterceptors = []interceptor.Interceptor{
	interceptor.More(),
	interceptor.Continue(),
}

func New(in io.Writer, out, err io.Reader, cfg Config) *ReadWriter {
	if misc.IsNil(in) {
		panic("in is nil")
	}
	if misc.IsNil(out) {
		panic("out is nil")
	}

	if cfg.ReadConfirmWait <= 0 {
		cfg.ReadConfirmWait = 100 * time.Millisecond
	}
	if cfg.ReadConfirm <= 0 {
		cfg.ReadConfirm = 3
	}

	var opts []lineReader.Option
	if !misc.IsNil(cfg.RawOut) {
		opts = append(opts, lineReader.WithRawOut(cfg.RawOut))
	}
	if !misc.IsNil(cfg.Filter) {
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.Read(ctx, true, onOut, interceptors...)
}

func (r *ReadWriter) ReadAll(timeout time.Duration, onOut func(lines []string), interceptors ...interceptor.Interceptor) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.Read(ctx, false, onOut, interceptors...)
}

func (r *ReadWriter) Read(ctx context.Context, stopOnEndLine bool, onOut func(lines []string), interceptors ...interceptor.Interceptor) (err error) {
	if r.cfg.BeforeRead != nil {
		if err = r.cfg.BeforeRead(); err != nil {
			return err
		}
	}

	if r.lo != nil {
		r.lo.SetOut(onOut)
		onOut = r.lo.Add
	}

	ticker := time.NewTicker(r.cfg.ReadConfirmWait)
	defer ticker.Stop()

	var outBuf strings.Builder
	var stop bool
	var confirm int
	for {
		select {
		case <-ctx.Done():
			switch err = ctx.Err(); {
			default:
				return &Error{Op: "read", Err: err}
			case errors.Is(err, context.DeadlineExceeded):
				return &Error{Op: "timeout", Err: err}
			case errors.Is(err, context.Canceled):
				return &Error{Op: "canceled", Err: err}
			}

		case <-ticker.C:
			_, e := r.out.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
				stop = false
				if len(lines) != 0 && onOut != nil {
					onOut(lines)
				}

				// 匹配优先级：指定的拦截器规则 > 默认拦截器规则 > 命令结束提示符规则
				if len(interceptors) > 0 {
					if outBuf.Len() > 0 {
						outBuf.WriteString("\n")
					}
					outBuf.WriteString(strings.Join(lines, "\n"))
					if remaining != "" {
						outBuf.WriteString("\n")
						outBuf.WriteString(remaining)
					}
					for _, f := range interceptors {
						if match, showOut, input := f(outBuf.String()); match {
							//util.PrintTimeLn("interceptor matched: %v => %v", outBuf.String(), input)
							outBuf.Reset()
							// TODO 如果是匹配多行内容的拦截器，前面行的内容总是被返回了，后续优化
							_ = r.Write(input) // 这里自动加了 \n
							return !showOut
						}
					}
				}

				if remaining == "" {
					return false
				}

				// 默认拦截器规则
				for _, f := range defaultInterceptors {
					if match, showOut, input := f(remaining); match {
						outBuf.Reset()
						if showOut && onOut != nil {
							onOut([]string{remaining})
						}
						_ = r.WriteRaw([]byte(input))
						return !showOut
					}
				}

				// 命令输出结束
				if r.IsEndLine(remaining) {
					//  当未指定提示符规则 且 AutoPrompt=true时，尝试自动纠正提示符匹配规则
					if len(r.cfg.PromptRegex) == 0 && r.cfg.AutoPrompt {
						if re := findPromptRegex(remaining); re != nil {
							r.cfg.PromptRegex = append(r.cfg.PromptRegex, re)
							util.PrintTimeLn("prompt:" + remaining + ", correct prompt regex:" + re.String())
						}
					}
					r.prompt = remaining
					stop = stopOnEndLine
					return !r.cfg.ShowPrompt
				}

				return false
			})
			if e != nil {
				// 保留 err 后退出循环，继续后续的 err.PopLines
				if e != io.EOF && !errors.Is(e, io.ErrClosedPipe) && !errors.Is(e, io.ErrNoProgress) && !errors.Is(e, io.ErrUnexpectedEOF) {
					err = &Error{Op: "read", Err: e}
				}
				goto exit
			}

			// util.PrintTimeLn("--> stop=%v, confirm=%v", stop, confirm)
			if stop {
				if confirm >= r.cfg.ReadConfirm {
					// util.PrintTimeLn("--> stop read out")
					goto exit
				}
				confirm++
			} else {
				confirm = 0
			}
		}
	}

exit:
	if r.err != nil {
		confirm = 0
		var errBuf strings.Builder
		for {
			popped, e := r.err.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
				if errBuf.Len() > 0 {
					errBuf.WriteString("\n")
				}
				errBuf.WriteString(strings.Join(lines, "\n"))
				if remaining != "" {
					errBuf.WriteString("\n")
					errBuf.WriteString(remaining)
				}
				return true
			})
			if e != nil && err == nil {
				if e != io.EOF && !errors.Is(e, io.ErrClosedPipe) && !errors.Is(e, io.ErrNoProgress) && !errors.Is(e, io.ErrUnexpectedEOF) {
					err = e
				}
				break
			}
			if popped == 0 {
				if confirm >= r.cfg.ReadConfirm {
					break
				} else {
					confirm++
				}
			} else {
				confirm = 0
			}
			time.Sleep(r.cfg.ReadConfirmWait)
		}
		if errBuf.Len() > 0 {
			err = &Error{Op: "read", Err: fmt.Errorf(errBuf.String())}
		}
	}

	if r.lo != nil {
		r.lo.Out()
	}

	return
}

func (r *ReadWriter) IsEndLine(s string) bool {
	var matched bool
	if len(r.cfg.PromptRegex) != 0 {
		for _, v := range r.cfg.PromptRegex {
			if v != nil && v.MatchString(s) {
				// util.PrintTimeLn("prompt matched:" + s)
				matched = true
			}
		}
	}

	if !matched && DefaultPromptRegex.MatchString(s) {
		// util.PrintTimeLn("default prompt matched:" + s)
		matched = true
	}

	// 由于尾缀特征字符的缘故，可能误匹配，但目前没有更优的规则，先把已知的误匹配场景排除
	//	如：
	//		[testuser@localhost ~]$ Username:
	//		[testuser@localhost ~]$ Password:
	if matched && (UsernameRegex.MatchString(s) || PasswordRegex.MatchString(s) || FlexibleOptionPromptRegex.MatchString(s)) {
		matched = false
	}

	return matched
}

// findPromptRegex
//
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

	// 存在提示符被省略、子命令模式等情况，这里取前10个字符作为前缀匹配
	//	山石防火墙提示符超长被省略：S-ABC-D1-EFG-~(M)#
	var prefix string
	runStr := []rune(hostname)
	if len(runStr) > 10 {
		prefix = string(runStr[:10])
	}

	var pattern string
	if prefix != "" {
		pattern = fmt.Sprintf(`(?i)(%v|%v)%v`, hostname, prefix, DefaultPromptSuffixPattern)
	} else {
		pattern = fmt.Sprintf(`(?i)%v%v`, hostname, DefaultPromptSuffixPattern)
	}
	if re, err := regexp.Compile(pattern); err == nil {
		return re
	}

	// 主机名中可能包含特殊字符，如果正则编译失败，尝试转义后再次编译
	if prefix != "" {
		pattern = fmt.Sprintf(`(?i)(%v|%v)%v`, regexp.QuoteMeta(hostname), regexp.QuoteMeta(prefix), DefaultPromptSuffixPattern)
	} else {
		pattern = fmt.Sprintf(`(?i)%v%v`, regexp.QuoteMeta(hostname), DefaultPromptSuffixPattern)
	}
	if re, err := regexp.Compile(pattern); err == nil {
		return re
	}

	util.Println("findPromptRegex fail, remaining: %v", remaining)
	return nil
}

func findHostname(remaining string) string {
	if remaining == "" {
		return ""
	}

	// 提示符格式非常自由，设备类型、厂商、用户配置不同，提示符格式也不同，可能包含中文、特殊字符，这里只能尽量匹配
	//	1.Linux主机：
	//		[root@localhost ~]#
	//		[root@localhost.localdomain ~]$
	//	    [root@192.168.1.24 /home/admin]$
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
