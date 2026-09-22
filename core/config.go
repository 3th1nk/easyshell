package core

import (
	"github.com/3th1nk/easyshell/v2/filter"
	"io"
	"regexp"
	"time"
)

// StderrPolicy stderr 输出的处理策略
type StderrPolicy uint8

const (
	// StderrError 默认：stderr 内容聚合后作为读取错误返回(不会覆盖超时/取消错误)。
	//	适用于"stderr 输出即错误信号"的场景；命令向 stderr 输出告警信息的场景请改用其他策略
	StderrError StderrPolicy = iota
	// StderrOutput stderr 行合并进 onOut 输出回调
	StderrOutput
	// StderrIgnore 丢弃 stderr 内容
	StderrIgnore
)

// ErrorPolicy 命令输出错误检测的命中处理策略
type ErrorPolicy uint8

const (
	// ErrorFail 默认：命中后立即中止本次 Read 并返回 *DeviceError
	//	(命中行之前已交付给 onOut 的内容保持不变)。
	//	登录横幅在 Shell 创建阶段已被单独消费，不参与检测，
	//	因此登录时自动输出错误样式信息的设备不会被误报
	ErrorFail ErrorPolicy = iota
	// ErrorCollect 命中后继续读取(适合异步日志中夹带错误样式行的设备)，
	//	Read 结束时以 errors.Join 聚合所有 *DeviceError 返回
	ErrorCollect
)

// Config 读取过程的配置(零值合法，未设置的字段使用库默认值)。
//
// 配置以值传入，库内部使用副本，不会修改调用方传入的结构体。
type Config struct {
	// RawOut 原始输出的捕获 writer(位于过滤器之前，即未过滤、未解码的原始字节)。
	//	可挂接 record.Writer 用于录制
	RawOut io.Writer
	// RawIn 输入方向的捕获 writer(Write/WriteRaw 写入连接前捕获，拦截器的自动应答也会被捕获)。
	//	可挂接 record.Writer.Input()；注意捕获的内容可能包含密码，请配合录制文件的 secret 打码机制使用
	RawIn io.Writer
	// Filter 自定义过滤器，串联在内置过滤管道之前(其输出仍会经过内置管道)。
	//	传入 filter.NewNoop() 仅关闭内置字符清理；nil 时使用内置管道
	Filter filter.Filter
	// FilterOptions 内置过滤管道的配置，nil 时使用 filter.DefaultOptions()；仅 Filter==nil 时生效
	FilterOptions *filter.Options
	// Decoder 自定义解码函数(作用于完整行)。
	//	契约：目标编码的多字节字符中不会出现 0x0A 字节(GBK/GB18030/UTF-8 均满足，UTF-16 不满足)，
	//	因为本库先按 \n 拆分完整行、再逐行解码，以保证多字节字符跨网络分包不被截断
	Decoder func(b []byte) ([]byte, error)
	// PromptRegex 命令行提示符的匹配规则；nil 时使用内置默认规则
	PromptRegex []*regexp.Regexp
	// AutoPrompt 是否自动纠正提示符规则：仅当未设置 PromptRegex 时生效，
	//	默认规则首次命中提示符后，基于主机名生成更精确的规则(部分场景可能不如默认规则灵活，慎用)
	AutoPrompt bool
	// ShowPrompt 是否把提示符行作为输出返回
	ShowPrompt bool
	// ReadConfirm 判定命令执行完成的确认次数(提示符命中后，连续 N 个确认周期无新数据才返回)，默认 3
	ReadConfirm int
	// ReadConfirmWait 确认周期，默认 100ms
	ReadConfirmWait time.Duration
	// BeforeRead 每次 Read 前的回调(如 CmdShell 的进程懒启动)；返回错误则中止本次读取
	BeforeRead func() error
	// LazyOutInterval 延迟触发 onOut 的时间间隔(0 表示关闭延迟)
	LazyOutInterval time.Duration
	// LazyOutSize 延迟触发 onOut 的累计字节数(0 表示不按大小触发)
	LazyOutSize int
	// Stderr stderr 处理策略，默认 StderrError
	Stderr StderrPolicy
	// ErrorPatterns 命令输出的错误检测规则。
	//	nil 时使用内置默认规则(仅命令解析器错误，见 DefaultErrorPatterns；
	//	登录横幅不参与检测)；
	//	显式传空切片关闭检测；传自定义规则则以传入为准(可基于 DefaultErrorPatterns() 追加)
	ErrorPatterns []*ErrorPattern
	// ErrorPolicy 错误命中的处理策略，默认 ErrorFail
	ErrorPolicy ErrorPolicy
}

func (cfg Config) normalize() Config {
	if cfg.ReadConfirmWait <= 0 {
		cfg.ReadConfirmWait = 100 * time.Millisecond
	}
	if cfg.ReadConfirm <= 0 {
		cfg.ReadConfirm = 3
	}
	return cfg
}
