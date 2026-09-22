package core

import (
	"context"
	"errors"
)

// Op 错误的操作类型
type Op string

const (
	OpDial     Op = "dial"     // 建立连接失败
	OpAuth     Op = "auth"     // 身份认证失败
	OpSession  Op = "session"  // 创建会话失败
	OpTerm     Op = "term"     // 请求伪终端失败
	OpShell    Op = "shell"    // 请求交互式 shell 失败
	OpRead     Op = "read"     // 读取输出失败
	OpWrite    Op = "write"    // 写入失败
	OpSftp     Op = "sftp"     // SFTP 操作失败
	OpTimeout  Op = "timeout"  // 超时
	OpCanceled Op = "canceled" // 取消
)

// Error 带操作类型与远端地址的错误，支持 errors.As/Is 解包
type Error struct {
	// Op 发生错误的操作
	Op Op
	// Addr 远端地址(涉及网络连接的操作)
	Addr string
	// Err 底层错误
	Err error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch e.Op {
	case OpTimeout:
		return context.DeadlineExceeded.Error()
	case OpCanceled:
		return context.Canceled.Error()
	default:
		s := string(e.Op) + " error"
		if e.Err != nil {
			s += ": " + e.Err.Error()
		}
		if e.Addr != "" {
			s += ", addr=" + e.Addr
		}
		return s
	}
}

func (e *Error) Unwrap() error { return e.Err }

func isOpError(err error, op Op) bool {
	var e *Error
	return errors.As(err, &e) && e.Op == op
}

// IsTimeout 是否超时错误(同时兼容标准库 context.DeadlineExceeded)
func IsTimeout(err error) bool {
	return isOpError(err, OpTimeout) || errors.Is(err, context.DeadlineExceeded)
}

// IsCanceled 是否取消错误(同时兼容标准库 context.Canceled)
func IsCanceled(err error) bool {
	return isOpError(err, OpCanceled) || errors.Is(err, context.Canceled)
}

// IsDial 是否建立连接失败
func IsDial(err error) bool { return isOpError(err, OpDial) }

// IsAuth 是否身份认证失败
func IsAuth(err error) bool { return isOpError(err, OpAuth) }

// IsClosed 是否已关闭错误
func IsClosed(err error) bool { return errors.Is(err, ErrClosed) }

// OpOf 返回错误的操作类型(覆盖 dial/auth/session/term/read/write/sftp 等全部类型)
func OpOf(err error) (Op, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Op, true
	}
	return "", false
}

var (
	// ErrClosed shell 已关闭
	ErrClosed = errors.New("easyshell: shell is closed")
	// ErrConcurrentRead 同一时刻只允许一个读操作
	ErrConcurrentRead = errors.New("easyshell: concurrent read not allowed")
	// ErrEmptyCommand 命令为空
	ErrEmptyCommand = errors.New("easyshell: empty command")
)
