package filter

// CRTrimMode 单独 \r 的处理模式
type CRTrimMode uint8

const (
	// CRTrimKeep 原样保留 \r
	CRTrimKeep CRTrimMode = iota
	// CRTrimDropCR 仅丢弃 \r 本身
	CRTrimDropCR
	// CRTrimBeginOfLine 丢弃 \r 及其左侧内容直到行首(设备重绘行的场景)
	CRTrimBeginOfLine
)

// DefaultOptions 返回推荐的默认过滤配置
func DefaultOptions() Options {
	return Options{
		Escape:         true,
		Erase:          true,
		Backspace:      true,
		CRLF:           true,
		CRTrim:         CRTrimBeginOfLine,
		DropNUL:        true,
		ReplaceChar:    true,
		DropIncomplete: true,
		MaxLineBuffer:  0, // 不限制
	}
}

// Options 过滤器的可选配置。
//
// 通过 NewFilter(opts ...Options) 传入：
//   - 不传参数时使用 DefaultOptions()；
//   - 传入时以传入值为准，不做字段级合并(显式传零值 Options{} 即关闭全部过滤)。
type Options struct {
	// Escape 是否处理转义序列(ESC 开头的 ANSI/ECMA-48 序列：CSI、OSC、DCS、SOS、PM、APC 等)
	Escape bool
	// KeepColor 是否保留 SGR 颜色序列(ESC[...m，含 24bit/256 色)，仅 Escape=true 时有效
	KeepColor bool
	// Erase 是否让擦除序列(ESC[K、ESC[J)作用于当前未完成行(修复重绘行的残留内容)。
	//	无列位置信息，EL/ED 一律按"清空当前行"的保守语义处理；设备用 ESC[K 做局部重绘时
	//	如出现内容丢失，可将此项关闭
	Erase bool
	// Backspace 是否处理退格(\b 从当前行删除一个字符，UTF-8 按 rune 回退)
	Backspace bool
	// CRLF 是否做换行归一化(\r\n 与 H3C 设备的 \r\r\n 统一为 \n)以及单独 \r 的处理
	CRLF bool
	// CRTrim 单独 \r 的处理模式，仅 CRLF=true 时有效
	CRTrim CRTrimMode
	// DropNUL 是否丢弃 NUL(\x00) 字节
	DropNUL bool
	// ReplaceChar 是否剔除解码后文本中的 U+FFFD 替换字符。
	//	注意该阶段发生在解码之后，由调用方(core)在 decode 完成后调用 StripReplaceChars
	ReplaceChar bool
	// DropIncomplete 流结束(Flush)时是否丢弃未完成的转义序列，false 时将已累积的原始字节原样输出
	DropIncomplete bool
	// TerminatorBEL DCS/SOS/PM/APC 字符串序列是否也接受 BEL(\x07) 终止(标准仅 OSC 接受 BEL)
	TerminatorBEL bool
	// MaxLineBuffer 当前未完成行的字节缓冲上限；<=0 表示不限制(默认)。
	//	少数防火墙设备的超大配置单行可超过 16MB，默认不设限以保证编辑仿真正常工作；
	//	仅作为调用方自我保护的保险丝，超限后该行进入直通模式(停止退格/擦除/清行仿真，
	//	转义序列处理照常)，直到本行交付
	MaxLineBuffer int
}
