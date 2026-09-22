package easyshell

import (
	"github.com/3th1nk/easyshell/v2/internal/core"
)

// 错误类型与判断函数(实现在 internal/core，此处别名导出)。
//
//	用法：
//	  var de *easyshell.DeviceError
//	  if errors.As(err, &de) { ... }
//	  if easyshell.IsTimeout(err) { ... }

// Op 错误的操作类型
type Op = core.Op

const (
	OpDial     = core.OpDial     // 建立连接失败
	OpAuth     = core.OpAuth     // 身份认证失败
	OpSession  = core.OpSession  // 创建会话失败
	OpTerm     = core.OpTerm     // 请求伪终端失败
	OpShell    = core.OpShell    // 请求交互式 shell 失败
	OpRead     = core.OpRead     // 读取输出失败
	OpWrite    = core.OpWrite    // 写入失败
	OpSftp     = core.OpSftp     // SFTP/SCP 操作失败
	OpTimeout  = core.OpTimeout  // 超时
	OpCanceled = core.OpCanceled // 取消
)

// Error 带操作类型与远端地址的错误，支持 errors.As/Is 解包
type Error = core.Error

// DeviceError 设备命令解析错误(错误检测命中)
type DeviceError = core.DeviceError

// ErrClosed shell 已关闭
var ErrClosed = core.ErrClosed

// ErrConcurrentRead 同一时刻只允许一个读操作
var ErrConcurrentRead = core.ErrConcurrentRead

// ErrEmptyCommand 命令为空
var ErrEmptyCommand = core.ErrEmptyCommand

// IsTimeout 是否超时错误(兼容 context.DeadlineExceeded)
func IsTimeout(err error) bool { return core.IsTimeout(err) }

// IsCanceled 是否取消错误(兼容 context.Canceled)
func IsCanceled(err error) bool { return core.IsCanceled(err) }

// IsDial 是否建立连接失败
func IsDial(err error) bool { return core.IsDial(err) }

// IsAuth 是否身份认证失败
func IsAuth(err error) bool { return core.IsAuth(err) }

// IsClosed 是否已关闭错误
func IsClosed(err error) bool { return core.IsClosed(err) }

// OpOf 返回错误的操作类型
func OpOf(err error) (Op, bool) { return core.OpOf(err) }
