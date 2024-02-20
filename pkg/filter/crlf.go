package filter

import (
	"bytes"
)

// CrlfFilter 处理回车换行
func CrlfFilter(s []byte) []byte {
	length := len(s)
	for pos := 0; pos < length; {
		if s[pos] != '\r' {
			pos++
			continue
		}

		// \r\n 统一替换为 \n
		if pos+1 < length && s[pos+1] == '\n' {
			length -= dropBytes(s, pos, pos+1)
			pos++
			continue
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
	return s[:length]
}
