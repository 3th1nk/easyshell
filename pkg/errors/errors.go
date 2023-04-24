package errors

import (
	"github.com/3th1nk/easygo/util/strUtil"
)

func isOpError(err error, op string) bool {
	if v, _ := err.(*Error); v != nil {
		return v.Op == op
	}
	return false
}

func IsTimeout(err error) bool { return isOpError(err, "timeout") }

func IsDial(err error) bool { return isOpError(err, "dial") }

func IsAuth(err error) bool { return isOpError(err, "auth") }

type Error struct {
	// Op is the operation which caused the error, such as "dial" or "auth".
	Op string
	// For operations involving a remote network connection.
	// like Dial, Read, or Write, Addr is the remote address of that connection.
	Addr string
	// Err is the error that occurred during the operation.
	Err error
}

// 是否是超时错误
func (e *Error) Timeout() bool { return e.Op == "timeout" }

// 是否是连接错误
func (e *Error) Dial() bool { return e.Op == "dial" }

// 是否是身份认证错误
func (e *Error) Auth() bool { return e.Op == "auth" }

func (e *Error) Name() string {
	return "Shell" + strUtil.UcFirst(e.Op) + "Error"
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	switch e.Op {
	case "timeout":
		return "context deadline exceeded"
	default:
		s := e.Op + " error"
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
