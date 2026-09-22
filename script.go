package easyshell

import (
	"context"
	"fmt"
)

// ScriptError 脚本执行错误：标识失败发生在哪条命令上
type ScriptError struct {
	// Cmd 失败的命令
	Cmd string
	// Index 命令在脚本中的下标(从0开始)
	Index int
	// Err 底层错误(含设备错误检测命中的 *core.DeviceError)
	Err error
}

func (e *ScriptError) Error() string {
	return fmt.Sprintf("script failed at command[%d] %q: %v", e.Index, e.Cmd, e.Err)
}

func (e *ScriptError) Unwrap() error { return e.Err }

// RunScript 顺序执行多条命令；任何一条失败(连接错误、超时、设备错误检测命中等)
//	立即中止并返回 *ScriptError。onOut 可为 nil，其 cmd 参数为产生该输出的命令。
func RunScript(ctx context.Context, s Shell, onOut func(cmd string, lines []string), cmds ...string) error {
	for i, cmd := range cmds {
		if err := ctx.Err(); err != nil {
			return &ScriptError{Cmd: cmd, Index: i, Err: err}
		}
		err := s.Run(ctx, cmd, func(lines []string) {
			if onOut != nil {
				onOut(cmd, lines)
			}
		})
		if err != nil {
			return &ScriptError{Cmd: cmd, Index: i, Err: err}
		}
	}
	return nil
}
