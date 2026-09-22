package easyshell

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// ExitCodeOptions ExitCode 的可选参数(可省略)
type ExitCodeOptions struct {
	// Command 获取退出码的命令，零值默认 "echo $?"(仅类Unix shell 有效)。
	//	网络设备等无 echo 语义的环境不适用
	Command string
	// Timeout 超时时间，零值默认 5 秒
	Timeout time.Duration
}

// ExitCode 获取上一条命令的退出码(通过执行 `echo $?` 并解析输出实现)。
//
//	解析规则：取最后一行非空内容，去除回显与提示符后转 int；解析失败返回错误。
func ExitCode(ctx context.Context, s Shell, opts ...ExitCodeOptions) (int, error) {
	opt := ExitCodeOptions{Command: "echo $?", Timeout: 5 * time.Second}
	if len(opts) > 0 {
		if opts[0].Command != "" {
			opt.Command = opts[0].Command
		}
		if opts[0].Timeout > 0 {
			opt.Timeout = opts[0].Timeout
		}
	}

	var lines []string
	if err := s.Run(ctx, opt.Command, func(arr []string) {
		lines = append(lines, arr...)
	}); err != nil {
		return 0, err
	}

	// 从后往前找最后一个非空行，剔除命令回显后解析
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		s = strings.TrimPrefix(s, opt.Command)
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// 去除可能粘连的提示符(如 "$ 0" / "#0" 之外的情况)
		code, err := strconv.Atoi(lastIntToken(s))
		if err != nil {
			continue
		}
		return code, nil
	}
	return 0, errExitCodeNotFound
}

var errExitCodeNotFound = exitCodeError("exit code not found in output")

type exitCodeError string

func (e exitCodeError) Error() string { return string(e) }

// lastIntToken 取字符串中最后一段纯数字
func lastIntToken(s string) string {
	fields := strings.Fields(s)
	for i := len(fields) - 1; i >= 0; i-- {
		if isAllDigits(fields[i]) {
			return fields[i]
		}
	}
	return s
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
