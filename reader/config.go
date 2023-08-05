package reader

import (
	"regexp"
	"time"
)

type Config struct {
	// 从 io.Reader 中读取到数据后，用来过滤特殊字符的自定义函数，在 Decoder 前执行
	Filter func(b []byte) []byte

	// 从 io.Reader 中读取到数据后，用来解码的自定义函数
	Decoder func(b []byte) ([]byte, error)

	// 命令行提示符的匹配规则
	EndPrompt []*regexp.Regexp

	// 是否自动纠正命令行提示符，仅当未指定 EndPrompt 时有效
	//	该参数为true时，会在默认规则第一次匹配到结束符时尝试修正匹配规则，某些情况下可能修正后的规则不如默认规则灵活，慎用
	AutoEndPrompt bool

	// 是否输出命令行提示符
	ShowEndPrompt bool

	// 调用 ReadToEndLine 时的确认次数
	ReadConfirm int
	// 调用 ReadToEndLine 时的确认间隔
	ReadConfirmWait time.Duration

	// 调用 ReadXXX 函数前的自定义回调函数
	BeforeRead func() error

	// 延迟触发 OnOut 的时间间隔
	//   如果需要在超过指定间隔或输出内容超过指定长度后再触发 OnOut、而不是实时触发 OnOut，可以指定 LazyOutInterval 和 LazyOutSize
	LazyOutInterval time.Duration
	// 延迟触发 OnOut 的缓冲区大小
	//   如果需要在超过指定间隔或输出内容超过指定长度后再触发 OnOut、而不是实时触发 OnOut，可以指定 LazyOutInterval 和 LazyOutSize
	LazyOutSize int
}
