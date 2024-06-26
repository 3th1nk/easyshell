package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/core"
	"golang.org/x/text/encoding/simplifiedchinese"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"unicode"
)

type CmdShellConfig struct {
	core.Config
	Context context.Context
	Prepare func(c *exec.Cmd)
}

func (c *CmdShellConfig) EnsureInit() {
	switch runtime.GOOS {
	case "windows":
		if c.Decoder == nil {
			c.Decoder = simplifiedchinese.GB18030.NewDecoder().Bytes
		}

		if len(c.PromptRegex) == 0 {
			//  "C:\\Users\\Administrator>"
			//	"PS C:\\Users\\Administrator>"
			c.PromptRegex = []*regexp.Regexp{regexp.MustCompile(`\S+>\s*$`)}
		}
	}
}

func NewCmdShell(cmdAndArgs string, config ...*CmdShellConfig) *CmdShell {
	var cfg *CmdShellConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = &CmdShellConfig{}
	}
	cfg.EnsureInit()

	arr := splitCmd(cmdAndArgs)
	var cmd *exec.Cmd
	if cfg.Context == nil {
		cmd = exec.Command(arr[0], arr[1:]...)
	} else {
		cmd = exec.CommandContext(cfg.Context, arr[0], arr[1:]...)
	}

	if cfg.Prepare != nil {
		cfg.Prepare(cmd)
	}

	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if f := cfg.BeforeRead; f != nil {
		cfg.BeforeRead = func() error {
			if cmd.Process == nil {
				return cmd.Start()
			}
			return f()
		}
	} else {
		cfg.BeforeRead = func() error {
			if cmd.Process == nil {
				return cmd.Start()
			}
			return nil
		}
	}

	return &CmdShell{
		ReadWriter: core.New(in, out, err, cfg.Config),
		c:          cmd,
	}
}

type CmdShell struct {
	*core.ReadWriter
	c *exec.Cmd
}

func (s *CmdShell) Cmd() *exec.Cmd {
	return s.c
}

func (s *CmdShell) Close() error {
	if s.c.Process != nil {
		return s.c.Process.Kill()
	}
	return nil
}

// 将 命令行字符串 解析为数组。
func splitCmd(cmd string) []string {
	mark := rune(0)       // 左引号。0 表示没有引号。
	start := -1           // 当前节的起始索引
	isLastEscape := false // 上个字符是否是转义符
	arr := make([]string, 0, 8)
	for pos, char := range cmd {
		func() {
			isSpace := unicode.IsSpace(char)
			if isSpace && start == -1 {
				return
			}

			// start == -1 表示这里到了下一个元素的起始位置
			if start == -1 {
				if char == '"' || char == '\'' {
					start = pos + 1
					mark = char
				} else {
					start = pos
					mark = 0
				}
				return
			}

			switch char {
			case mark:
				// 有左引号、而且现在找到了右引号
				if !isLastEscape {
					s := cmd[start:pos]
					if s2, err := strconv.Unquote(string(mark) + s + string(mark)); err == nil {
						arr = append(arr, s2)
					} else {
						arr = append(arr, s)
					}
					start = -1
				}
			default:
				if start != -1 && isSpace && mark == 0 {
					s := cmd[start:pos]
					arr = append(arr, s)
					start = -1
				}
			}
		}()
		if char == '\\' {
			isLastEscape = !isLastEscape
		} else {
			isLastEscape = false
		}
	}
	if start != -1 {
		arr = append(arr, cmd[start:])
	}
	return arr
}
