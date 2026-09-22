package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/record"
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

	// ReadUntilPrompt 读取输出直到提示符(提示符行默认不作为输出返回)
	ReadUntilPrompt(ctx context.Context, onOut func(lines []string),
		opts ...RunOptions) error
	// ReadAll 读取全部输出直到流结束
	ReadAll(ctx context.Context, onOut func(lines []string),
		opts ...RunOptions) error

	// Run 写入命令并读取输出直到提示符，等价于 Write + ReadUntilPrompt
	Run(ctx context.Context, cmd string, onOut func(lines []string),
		opts ...RunOptions) error
	// RunAll 写入命令并读取全部输出直到流结束，等价于 Write + ReadAll
	RunAll(ctx context.Context, cmd string, onOut func(lines []string),
		opts ...RunOptions) error

	// Prompt 返回最近一次匹配到的提示符(读取过程中可并发调用)
	Prompt() string
	// IsPrompt 判断给定内容是否命中提示符规则(并发安全)
	IsPrompt(s string) bool
	// InConfigMode 基于最近匹配的提示符推断是否处于配置模式(启发式，
	//	覆盖 H3C/华为方括号视图与 Cisco (config) 样式；Linux 等返回 false)
	InConfigMode() bool

	// Close 关闭(幂等)
	Close() error
}

// shellBase 非导出委托层：三个 Shell 实现共用，避免样板代码，
// 同时不把 core.Reader 暴露到公共 API。
type shellBase struct {
	rw       *core.Reader
	recorder *record.Writer // 会话录制器(配置了 Record 时非nil，随Close自动收尾)
}

func (b shellBase) Write(cmd string) error { return b.rw.Write(cmd) }
func (b shellBase) WriteRaw(p []byte) error {
	return b.rw.WriteRaw(p)
}
func (b shellBase) ReadUntilPrompt(ctx context.Context, onOut func(lines []string),
	opts ...RunOptions) error {
	return b.rw.ReadUntilPrompt(ctx, onOut, opts...)
}
func (b shellBase) ReadAll(ctx context.Context, onOut func(lines []string),
	opts ...RunOptions) error {
	return b.rw.ReadAll(ctx, onOut, opts...)
}
func (b shellBase) Run(ctx context.Context, cmd string, onOut func(lines []string),
	opts ...RunOptions) error {
	return b.rw.Run(ctx, cmd, onOut, opts...)
}
func (b shellBase) RunAll(ctx context.Context, cmd string, onOut func(lines []string),
	opts ...RunOptions) error {
	return b.rw.RunAll(ctx, cmd, onOut, opts...)
}
func (b shellBase) Prompt() string         { return b.rw.Prompt() }
func (b shellBase) IsPrompt(s string) bool { return b.rw.IsPrompt(s) }
func (b shellBase) InConfigMode() bool     { return b.rw.InConfigMode() }
func (b shellBase) Close() error           { return b.rw.Close() }
