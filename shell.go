package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
)

// Shell 是 CmdShell/SshShell/TelnetShell 的统一抽象，面向"命令交互"场景。
//
// 使用约束：
//   - 同一时刻只允许一个读操作在执行(并发 Read/Run 会返回错误)；
//   - 所有阻塞方法都接受 context，取消/超时通过 context 控制；
//   - Close 幂等，调用后所有操作返回 ErrClosed。
type Shell interface {
	// Write 写入一行命令，自动在末尾补充 \n；空串等价于写入单个换行
	Write(cmd string) error
	// WriteRaw 原样写入(不自动补充换行)，供密码输入、翻页应答等场景
	WriteRaw(p []byte) error

	// Read 读取输出；stopOnPrompt=true 时读到提示符即返回(提示符行默认不作为输出返回)
	Read(ctx context.Context, stopOnPrompt bool, onOut func(lines []string),
		interceptors ...interceptor.Interceptor) error
	// ReadUntilPrompt 读取输出直到提示符
	ReadUntilPrompt(ctx context.Context, onOut func(lines []string),
		interceptors ...interceptor.Interceptor) error
	// ReadAll 读取全部输出直到流结束
	ReadAll(ctx context.Context, onOut func(lines []string),
		interceptors ...interceptor.Interceptor) error

	// Run 写入命令并读取输出直到提示符，等价于 Write + ReadUntilPrompt
	Run(ctx context.Context, cmd string, onOut func(lines []string),
		interceptors ...interceptor.Interceptor) error
	// RunAll 写入命令并读取全部输出直到流结束，等价于 Write + ReadAll
	RunAll(ctx context.Context, cmd string, onOut func(lines []string),
		interceptors ...interceptor.Interceptor) error

	// Prompt 返回最近一次匹配到的提示符(读取过程中可并发调用)
	Prompt() string
	// IsPrompt 判断给定内容是否命中提示符规则(并发安全)
	IsPrompt(s string) bool

	// Close 关闭(幂等)
	Close() error
}

// shellBase 非导出委托层：三个 Shell 实现共用，避免样板代码，
// 同时不把 core.Reader 暴露到公共 API。
type shellBase struct{ rw *core.Reader }

func (b shellBase) Write(cmd string) error { return b.rw.Write(cmd) }
func (b shellBase) WriteRaw(p []byte) error {
	return b.rw.WriteRaw(p)
}
func (b shellBase) Read(ctx context.Context, stopOnPrompt bool, onOut func(lines []string),
	interceptors ...interceptor.Interceptor) error {
	return b.rw.Read(ctx, stopOnPrompt, onOut, interceptors...)
}
func (b shellBase) ReadUntilPrompt(ctx context.Context, onOut func(lines []string),
	interceptors ...interceptor.Interceptor) error {
	return b.rw.ReadUntilPrompt(ctx, onOut, interceptors...)
}
func (b shellBase) ReadAll(ctx context.Context, onOut func(lines []string),
	interceptors ...interceptor.Interceptor) error {
	return b.rw.ReadAll(ctx, onOut, interceptors...)
}
func (b shellBase) Run(ctx context.Context, cmd string, onOut func(lines []string),
	interceptors ...interceptor.Interceptor) error {
	return b.rw.Run(ctx, cmd, onOut, interceptors...)
}
func (b shellBase) RunAll(ctx context.Context, cmd string, onOut func(lines []string),
	interceptors ...interceptor.Interceptor) error {
	return b.rw.RunAll(ctx, cmd, onOut, interceptors...)
}
func (b shellBase) Prompt() string     { return b.rw.Prompt() }
func (b shellBase) IsPrompt(s string) bool { return b.rw.IsPrompt(s) }
func (b shellBase) Close() error       { return b.rw.Close() }
