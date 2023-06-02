package reader

import (
	"fmt"
	"github.com/3th1nk/easyshell/internal/lazyOut"
	"github.com/3th1nk/easyshell/internal/lineReader"
	"github.com/3th1nk/easyshell/pkg/errors"
	"github.com/3th1nk/easyshell/pkg/injector"
	"io"
	"regexp"
	"strings"
	"time"
)

var (
	promptSuffix = `[\s\S]*[#$>:~%\]]+\s*$`

	DefaultEndPrompt = regexp.MustCompile(`\S+` + promptSuffix)

	defaultRemainingInjector = []injector.InputInjector{
		injector.More(),
		injector.Continue(),
	}
)

func New(in io.Writer, out, err io.Reader, cfg Config) *Reader {
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

	r := &Reader{
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

type Reader struct {
	cfg      Config
	in       io.Writer
	out, err *lineReader.LineReader
	lo       *lazyOut.LazyOut
}

func (r *Reader) Stop() {
	if r.lo != nil {
		r.lo.Stop()
		r.lo = nil
	}
	r.in, r.out, r.err = nil, nil, nil
}

// Write 写入一个命令（自动在末尾补充 \n 换行符）。
func (r *Reader) Write(cmd string) (err error) {
	if cmd == "" {
		cmd = "\n"
	} else if cmd[len(cmd)-1] != '\n' {
		cmd += "\n"
	}
	return r.WriteRaw([]byte(cmd))
}

// WriteRaw 向输入流写入指定内容，并等待指定时间（默认 10 毫秒）。
func (r *Reader) WriteRaw(b []byte) (err error) {
	if len(b) != 0 {
		_, err = r.in.Write(b)
	}
	return nil
}

func (r *Reader) ReadToEndLine(timeout time.Duration, onOut func(lines []string), injector ...injector.InputInjector) (err error) {
	return r.Read(true, timeout, onOut, injector...)
}

func (r *Reader) ReadAll(timeout time.Duration, onOut func(lines []string), injector ...injector.InputInjector) (err error) {
	return r.Read(false, timeout, onOut, injector...)
}

func (r *Reader) Read(stopOnEndLine bool, timeout time.Duration, onOut func(lines []string), injector ...injector.InputInjector) (err error) {
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
	var injectStr string
	var stop bool
	var confirm int
	for {
		_, e := r.out.PopLines(func(lines []string, remaining string) (dropRemaining bool) {
			if len(lines) != 0 && onOut != nil {
				onOut(lines)
			}

			// 命令行结束提示符
			if remaining != "" && r.IsEndLine(remaining) {
				stop = stopOnEndLine
				return !r.cfg.ShowEndPrompt
			}
			stop = false

			// 缓存区所有内容
			if len(injector) != 0 {
				if injectStr == "" {
					injectStr = strings.Join(lines, "\n")
				} else {
					injectStr += "\n" + strings.Join(lines, "\n")
				}
				if remaining != "" {
					injectStr += "\n" + remaining
				}
				for _, f := range injector {
					if match, showOut, input := f(injectStr); match {
						// 重置 injectStr
						injectStr = ""
						// 如果匹配到了 Injector，则将 line 当作新行输出
						if showOut && remaining != "" && onOut != nil {
							onOut([]string{remaining})
						}
						// input
						_, _ = r.in.Write([]byte(input))
						return true
					}
				}
			}

			// 最后一行内容
			if remaining != "" {
				for _, f := range defaultRemainingInjector {
					if match, showOut, input := f(remaining); match {
						// 重置 injectStr
						injectStr = ""
						// 如果匹配到了 Injector，则将 line 当作新行输出
						if showOut && onOut != nil {
							onOut([]string{remaining})
						}
						// input
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
				err = &errors.Error{Op: "read", Err: e}
			}
			break
		}
		if !time.Now().Before(timeoutAt) {
			return &errors.Error{Op: "timeout"}
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
			err = &errors.Error{Op: "read", Err: fmt.Errorf(errMsg)}
		}
	}

	if r.lo != nil {
		r.lo.Out()
	}

	return
}

func (r *Reader) IsEndLine(s string) bool {
	if len(r.cfg.EndPrompt) != 0 {
		for _, v := range r.cfg.EndPrompt {
			if v != nil && v.MatchString(s) {
				//util.PrintTimeLn("end prompt matched:" + s)
				return true
			}
		}
		return false
	}

	if DefaultEndPrompt.MatchString(s) {
		//util.PrintTimeLn("default end prompt matched:" + s)
		// 当默认的提示符匹配规则匹配上 且 AutoEndPrompt=true 时，自动纠正提示符匹配规则
		//	！！！由于提示符的格式非常自由，特别是主机上，自动纠正有可能反而导致无法匹配，应视情况使用 ！！！
		if r.cfg.AutoEndPrompt {
			// 由于提示符在交互过程中可能会变化，这里先提取一下主机名，再通配一下尾部
			//	网络设备进入配置模式：hostname# => hostname(config)#
			//  主机切换用户：user1@hostname# => user2@hostname#
			regex := regexp.MustCompile(`[a-zA-Z0-9_.@&\-]+`)
			if str := regex.FindString(s); str != "" {
				// 如果包含（常用的）分隔符，默认取分隔符之后的内容作为主机名
				hostname := str
				for _, sep := range []byte{'@', '.'} {
					if index := strings.IndexByte(str, sep); index != -1 {
						hostname = str[index+1:]
						break
					}
				}
				r.tryCorrectEndPrompt(hostname)
			}
		}
		return true
	}
	return false
}

func (r *Reader) tryCorrectEndPrompt(hostname string) {
	if hostname == "" {
		return
	}

	// 存在提示符被省略的情况，如 山石防火墙：S-ABC-D1-EFG-~(M)#
	if len(hostname) > 10 {
		hostname = fmt.Sprintf(`(%v|%v\S+)`, hostname, hostname[:10])
	}
	prompt := `(?i)` + hostname + promptSuffix
	r.cfg.EndPrompt = []*regexp.Regexp{regexp.MustCompile(prompt)}
	//util.PrintTimeLn("correct end prompt regex:" + prompt)
}
