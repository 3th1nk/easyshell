package filter

import (
	"bytes"
	"unicode/utf8"
)

// 转义序列状态机的状态
type state uint8

const (
	stGround state = iota // 文本态
	stCR                  // 已收到 \r，等待下一字节(\r\n / \r\r\n / 单独\r)
	stCRCR                // 已收到 \r\r(H3C 设备)，等待 \n
	stEsc                 // 已收到 ESC
	stEscInt              // ESC + 中间字节(0x20-0x2F)，如 ESC ( B 字符集指定
	stCSI                 // ESC [ 之后(CSI 序列)
	stStr                 // 字符串序列体(OSC/DCS/SOS/PM/APC)
	stStrEsc              // 字符串体中收到 ESC，等待 '\' 组成 ST
)

// fsm 内置过滤器：有状态的转义序列状态机 + 当前行缓冲。
//
// 与 v1 的无状态过滤函数相比的关键差异：
//   - 转义序列跨网络分包截断时仍能正确识别(状态与已累积字节跨 Push 保留)；
//   - 退格/擦除/单独\r清行实时作用于尚未交付的当前行，而非事后补救；
//   - 只交付完整行，不完整尾行保留在 Pending 中，多字节字符不会被分包截断后解码出错。
type fsm struct {
	opt     Options
	state   state
	seq     []byte // 当前转义序列的原始字节累积(从 ESC 开始；用于 Flush 时还原与 SGR 保留)
	strType byte   // stStr 的起始类型字符(']'/'P'/'X'/'^'/'_')
	line    []byte // 当前未完成行缓冲
	dropped bool   // 直通模式：行超过 MaxLineBuffer 后停止行编辑仿真(退格/擦除/清行)
	apv     int    // ARRAY APV 特殊退格(`$`+退格)后需要吞掉的 \r\n\r 字节计数
}

func (f *fsm) Push(p []byte) (out []byte) {
	for len(p) > 0 {
		// 快速路径：文本态下批量拷贝到下一个需要特殊处理的字节
		if f.state == stGround && !f.dropped && f.apv == 0 {
			i := bytes.IndexAny(p, "\x1b\r\n\b\x00")
			if i == -1 {
				f.appendLine(p...)
				return out
			}
			if i > 0 {
				f.appendLine(p[:i]...)
				p = p[i:]
			}
		}
		out = f.step(p[0], out)
		p = p[1:]
	}
	return out
}

func (f *fsm) Pending() []byte { return f.line }

func (f *fsm) DropPending() {
	f.line = f.line[:0]
	f.dropped = false
	f.apv = 0
	f.state = stGround
	f.seq = f.seq[:0]
}

func (f *fsm) Flush() (out []byte) {
	switch f.state {
	case stEsc, stEscInt, stCSI:
		// 未完成的转义序列：按配置丢弃或原样输出已累积的原始字节
		if !f.opt.DropIncomplete {
			f.appendLine(f.seq...)
		}
	case stCR:
		f.appendLine('\r')
	case stCRCR:
		f.appendLine('\r', '\r')
	}
	f.state = stGround
	f.seq = f.seq[:0]
	if len(f.line) > 0 {
		out = f.commit(out)
	}
	return out
}

func (f *fsm) step(b byte, out []byte) []byte {
	switch f.state {
	case stGround:
		return f.ground(b, out)
	case stCR:
		return f.crState(b, out)
	case stCRCR:
		return f.crcrState(b, out)
	case stEsc:
		return f.esc(b, out)
	case stEscInt:
		return f.escInt(b, out)
	case stCSI:
		return f.csi(b, out)
	case stStr:
		return f.strBody(b, out)
	case stStrEsc:
		return f.strEsc(b, out)
	}
	f.state = stGround
	return f.ground(b, out)
}

// ground 文本态的字节处理
func (f *fsm) ground(b byte, out []byte) []byte {
	// ARRAY APV 特殊退格：`$`+退格后吞掉紧随的 \r\n\r
	if f.apv > 0 {
		if b == '\r' || b == '\n' {
			f.apv--
			return out
		}
		f.apv = 0
	}

	switch {
	case b == 0x1B && f.opt.Escape:
		f.beginEsc()
	case b == '\b' && f.opt.Backspace && !f.dropped:
		f.backspace()
	case b == '\n':
		out = f.commit(out)
	case b == '\r' && f.opt.CRLF && !f.dropped:
		f.state = stCR
	case b == 0x00 && f.opt.DropNUL:
		// 丢弃
	default:
		f.appendLine(b)
	}
	return out
}

// crState 已收到单独的 \r
func (f *fsm) crState(b byte, out []byte) []byte {
	switch b {
	case '\n': // \r\n → \n
		f.state = stGround
		return f.commit(out)
	case '\r': // \r\r，等待 \n(H3C)
		f.state = stCRCR
		return out
	default:
		return f.endCR(out, b)
	}
}

// crcrState 已收到 \r\r(H3C 设备的 \r\r\n 前缀)
func (f *fsm) crcrState(b byte, out []byte) []byte {
	switch b {
	case '\n': // \r\r\n → 单个 \n
		f.state = stGround
		return f.commit(out)
	case '\r':
		return out
	case 0x00:
		if f.opt.DropNUL {
			return out
		}
	}
	// 非换行：两个挂起的 \r 各自按 CRTrim 处理后，以文本态处理当前字节
	f.state = stGround
	f.appendCRs()
	return f.ground(b, out)
}

// endCR 处理挂起的单独 \r 后，以文本态语义重新处理当前字节
func (f *fsm) endCR(out []byte, next byte) []byte {
	f.state = stGround
	f.appendCRs()
	return f.ground(next, out)
}

// appendCRs 按照当前 CRTrim 配置补回挂起的 \r
func (f *fsm) appendCRs() {
	if f.dropped {
		f.appendLine('\r')
		return
	}
	switch f.opt.CRTrim {
	case CRTrimBeginOfLine:
		f.line = f.line[:0]
	case CRTrimDropCR:
		// 仅丢弃 \r
	default: // CRTrimKeep
		f.appendLine('\r')
	}
}

// esc ESC 之后的字节
func (f *fsm) esc(b byte, out []byte) []byte {
	switch {
	case b == '[': // CSI
		f.seq = append(f.seq, b)
		f.state = stCSI
	case b == ']' || b == 'P' || b == 'X' || b == '^' || b == '_':
		// OSC/DCS/SOS/PM/APC 字符串序列
		f.strType = b
		f.state = stStr
	case b >= 0x20 && b <= 0x2F: // 中间字节
		f.seq = append(f.seq, b)
		f.state = stEscInt
	case b == 0x1B: // 悬挂 ESC，重新开始
		f.seq = f.seq[:1]
	case b == 0x18 || b == 0x1A: // CAN/SUB 中止
		f.state = stGround
	case b < 0x20: // C0 控制字符：中止序列并立即执行
		f.state = stGround
		return f.ground(b, out)
	default: // 0x30-0x7E：两字节序列(ESC 7/8/=/>, ESC c 等)，丢弃
		f.state = stGround
	}
	return out
}

// escInt ESC + 中间字节之后
func (f *fsm) escInt(b byte, out []byte) []byte {
	switch {
	case b >= 0x20 && b <= 0x2F:
		f.seq = append(f.seq, b)
	case b == 0x1B:
		f.beginEsc()
	case b == 0x18 || b == 0x1A:
		f.state = stGround
	case b < 0x20:
		f.state = stGround
		return f.ground(b, out)
	default: // final byte(0x30-0x7E)，如 ESC ( B，整段丢弃
		f.state = stGround
	}
	return out
}

// csi CSI 序列体(ESC [ 之后)
func (f *fsm) csi(b byte, out []byte) []byte {
	switch {
	case (b >= 0x30 && b <= 0x3F) || (b >= 0x20 && b <= 0x2F):
		// 参数字节(0-9:;<=>?)与中间字节(space !"#$%&'()*+,-./)
		f.seq = append(f.seq, b)
	case b == 0x1B:
		f.beginEsc()
	case b == 0x18 || b == 0x1A:
		f.state = stGround
	case b < 0x20:
		f.state = stGround
		return f.ground(b, out)
	case b >= 0x40 && b <= 0x7E:
		f.state = stGround
		return f.csiDispatch(b, out)
	default: // 非法字节，中止丢弃
		f.state = stGround
	}
	return out
}

// csiDispatch CSI 终止字节派发
func (f *fsm) csiDispatch(final byte, out []byte) []byte {
	switch final {
	case 'm': // SGR(含 24bit/256 色与冒号参数形式，不解析参数故天然支持)
		if f.opt.KeepColor {
			f.seq = append(f.seq, final)
			f.appendLine(f.seq...)
		}
	case 'K', 'J': // EL/ED 擦除：清空当前未完成行(无列位置信息，保守按整行处理)
		if f.opt.Erase && !f.dropped {
			f.line = f.line[:0]
		}
	}
	// 其余(光标移动/模式设置等)丢弃
	return out
}

// strBody 字符串序列体(OSC/DCS/SOS/PM/APC)，内容全部吞掉
func (f *fsm) strBody(b byte, out []byte) []byte {
	switch {
	case b == 0x07 && (f.strType == ']' || f.opt.TerminatorBEL): // BEL 终止(标准仅 OSC 接受)
		f.state = stGround
	case b == 0x1B:
		f.state = stStrEsc
	case b == 0x18 || b == 0x1A:
		f.state = stGround
	}
	return out
}

// strEsc 字符串体中收到 ESC，等待 '\' 组成 ST
func (f *fsm) strEsc(b byte, out []byte) []byte {
	switch {
	case b == '\\': // ST 终止
		f.state = stGround
	case b == 0x07 && (f.strType == ']' || f.opt.TerminatorBEL): // 容错：BEL 终止
		f.state = stGround
	case b == 0x1B:
		// 继续等待 '\'
	case b == 0x18 || b == 0x1A:
		f.state = stGround
	default:
		// 容错：该 ESC 并非 ST 前缀，视为新序列的开始
		f.state = stGround
		f.beginEsc()
		return f.esc(b, out)
	}
	return out
}

// backspace 退格：从当前行末尾删除一个字符。
//
//	UTF-8 按 rune 回退；非 UTF-8 尾字节(如 GBK 的第二字节)按 1 字节处理(与 v1 行为一致)。
//	特殊场景：ARRAY APV 负载均衡设备输入超长回缩时发送 `$`+退格+\r\n\r+内容，
//	此时保留 `$`，仅吞掉退格与紧随的 \r\n\r。
func (f *fsm) backspace() {
	if n := len(f.line); n > 0 && f.line[n-1] == '$' {
		f.apv = 3
		return
	}
	n := len(f.line)
	if n == 0 {
		return
	}
	_, size := utf8.DecodeLastRune(f.line)
	if size < 1 {
		size = 1
	}
	f.line = f.line[:n-size]
}

// commit 交付当前行(追加 \n 作为行结束)并复位行缓冲
func (f *fsm) commit(out []byte) []byte {
	out = append(out, f.line...)
	out = append(out, '\n')
	f.line = f.line[:0]
	f.dropped = false
	return out
}

// appendLine 向当前行缓冲追加内容，并检查直通模式阈值
func (f *fsm) appendLine(bs ...byte) {
	if f.opt.MaxLineBuffer > 0 && !f.dropped && len(f.line)+len(bs) > f.opt.MaxLineBuffer {
		f.dropped = true
	}
	f.line = append(f.line, bs...)
}

// beginEsc 开始一个新的转义序列(累积原始字节，供 Flush 还原)
func (f *fsm) beginEsc() {
	f.state = stEsc
	f.seq = append(f.seq[:0], 0x1B)
}
