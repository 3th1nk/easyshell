package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/record"
	"golang.org/x/text/encoding/simplifiedchinese"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

// CmdConfig 本地命令 Shell 配置(零值合法)
type CmdConfig struct {
	Config
	// Command 要执行的命令(经 splitCommand 解析为程序与参数)
	Command string
	// Dir 命令的工作目录
	Dir string
	// Env 命令的环境变量(nil 继承当前进程)
	Env []string
	// Prepare 命令启动前的自定义回调(可修改 exec.Cmd)
	Prepare func(c *exec.Cmd)
	// Record 会话录制配置，nil 时不录制
	Record *RecordConfig
}

// NewCmdShell 创建本地命令 Shell。
//
//	命令进程在首次 Read 时懒启动；Command 为空返回 core.ErrEmptyCommand。
func NewCmdShell(ctx context.Context, cfg CmdConfig) (*CmdShell, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, &core.Error{Op: core.OpShell, Err: core.ErrEmptyCommand}
	}

	if runtime.GOOS == "windows" && cfg.Decoder == nil {
		cfg.Decoder = simplifiedchinese.GB18030.NewDecoder().Bytes
	}

	// Windows 下命令提示符(可能带 PowerShell 前缀)，其他平台使用内置默认规则
	if runtime.GOOS == "windows" && len(cfg.PromptRegex) == 0 {
		cfg.PromptRegex = []*regexp.Regexp{regexp.MustCompile(`\S+>\s*$`)}
	}

	arr := splitCommand(cfg.Command)
	cmd := exec.CommandContext(ctx, arr[0], arr[1:]...)
	cmd.Dir = cfg.Dir
	cmd.Env = cfg.Env
	if cfg.Prepare != nil {
		cfg.Prepare(cmd)
	}

	rec, err := wireRecord(cfg.Record, &cfg.Config, record.Meta{Protocol: "cmd"})
	if err != nil {
		return nil, err
	}

	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// 进程懒启动：首次 Read 时启动(保证输出管道在进程输出前就位，不丢失开头的数据)
	var startOnce bool
	cfg.BeforeRead = func() error {
		if !startOnce {
			startOnce = true
			return cmd.Start()
		}
		return nil
	}

	return &CmdShell{
		shellBase: shellBase{rw: core.NewReader(in, out, stderr, cfg.Config), recorder: rec},
		c:         cmd,
	}, nil
}

// CmdShell 本地命令 Shell
type CmdShell struct {
	shellBase
	c *exec.Cmd
}

func (s *CmdShell) Cmd() *exec.Cmd {
	return s.c
}

// Close 终止命令进程(幂等)
func (s *CmdShell) Close() error {
	if s.c.Process != nil {
		_ = s.c.Process.Kill()
	}
	return s.shellBase.Close()
}

// splitCommand 将命令行字符串解析为程序与参数数组(支持单引号、双引号)。
func splitCommand(cmd string) []string {
	mark := rune(0)       // 左引号。0 表示没有引号。
	start := -1           // 当前节的起始索引
	isLastEscape := false // 上个字符是否是转义符
	arr := make([]string, 0, 8)
	for pos, char := range cmd {
		isSpace := unicode.IsSpace(char)
		if isSpace && start == -1 {
			continue
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
			continue
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
				arr = append(arr, cmd[start:pos])
				start = -1
			}
		}

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
