package filter

// checkAnsiEscape 检查控制字符
//
//	参考资料：
//	https://vt100.net/docs/vt100-ug/chapter3.html#S3.3
//	https://en.wikipedia.org/wiki/ANSI_escape_code#CSI_(Control_Sequence_Introducer)_sequences
//
// TODO 对于修改内容的控制符，应该同时处理对应的内容
//
//	由于当前函数是在每次读操作之后调用，涉及修改的内容可能还未被完整的读取到缓冲区，可能导致异常
func checkAnsiEscape(s []byte, pos int) (bool, int, [2]int) {
	length := len(s)
	if pos >= length || s[pos] != '\x1b' {
		return false, pos, [2]int{}
	}
	start := pos
	// skip '\x1b'
	pos++

	if pos < length {
		switch s[pos] {
		case 'N':
			// SS2 (Single Shift Two)
		case 'O':
			// SS3 (Single Shift Three)
		case 'P':
			// DCS (Device Control String)
		case '[':
			// CSI (Control Sequence Introducer)
			// TODO: support 24-bit colors, such as `ESC[38;2;⟨r⟩;⟨g⟩;⟨b⟩ m`

			// skip '['
			pos++

			//	any number (including none) of "parameter bytes" in the range 0x30–0x3F (ASCII 0–9:;<=>?)
			for pos < length {
				if s[pos] < '\x30' || s[pos] > '\x3F' {
					break
				}
				pos++
			}

			//	then any number (including none) of "intermediate bytes" in the range 0x20–0x2F (ASCII space and !"#$%&'()*+,-./)
			for pos < length {
				if s[pos] < '\x20' || s[pos] > '\x2F' {
					break
				}
				pos++
			}

			//	finally by a single "final byte" in the range 0x40–0x7E (ASCII @A–Z[\]^_`a–z{|}~)
			if pos < length && s[pos] >= '\x40' && s[pos] <= '\x7E' {
				pos++
				return true, pos, [2]int{start, pos}
			}

		case '\\':
			// ST (String Terminator)
		case ']':
			// OSC	(Operating System Command)
		case 'X':
			// SOS	(Start of String)
		case '^':
			// PM (Privacy Message)
		case '_':
			// APC (Application Program Command)
		case 'c':
			// RIS (Reset to Initial State)
		}
	}

	return false, pos, [2]int{}
}
