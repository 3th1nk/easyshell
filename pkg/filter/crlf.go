package filter

import (
	"bytes"
)

// CrlfFilter 处理回车换行
// 	为了避免未读取完整但最后一个字符是 \r 导致最后一行内容被清除的情况，这里只处理最后一个\n之前的内容
func CrlfFilter(s []byte) []byte {
	var remaining []byte
	if idx := bytes.LastIndexByte(s, '\n'); idx >= 0 {
		s, remaining = s[:idx+1], s[idx+1:]
	} else {
		return s
	}
	length := len(s)
	for pos := 0; pos < length; {
		if s[pos] != '\r' {
			pos++
			continue
		}

		if pos+1 < length {
			// \r的下一个字符
			switch s[pos+1] {
			case '\n':
				// \r\n 统一替换为 \n
				length -= dropBytes(s, pos, pos+1)
				pos++
				continue

			case '\r':
				// !!! 部分H3C设备特殊情况处理，可能有副作用 !!!
				//	\r\r\n 	   统一替换为 \n
				//  \r\r\n\NUL 统一替换为 \n
				if pos+2 < length && s[pos+2] == '\n' {
					length -= dropBytes(s, pos, pos+2)
					pos++
					if pos < length && s[pos] == '\x00' {
						length -= dropBytes(s, pos, pos+1)
					}
					continue
				}
			}
		}

		// 清除\r及其左侧的当前行内容
		if index := bytes.LastIndexByte(s[:pos], '\n'); index >= 0 {
			length -= dropBytes(s, index+1, pos+1)
			pos = index + 1
		} else {
			s = s[pos+1:]
			length -= pos + 1
			pos = 0
		}
	}
	return append(s[:length], remaining...)
}
