package filter

import "bytes"

// crlfFilter 处理回车换行
func crlfFilter(s []byte) []byte {
	for pos := 0; pos < len(s); {
		if s[pos] != '\r' {
			pos++
			continue
		}

		// \r\n 统一替换为 \n
		if pos+1 < len(s) && s[pos+1] == '\n' {
			s = append(s[:pos], s[pos+1:]...)
			pos++
			continue
		}

		// 清除\r及其左侧的当前行内容
		if index := bytes.LastIndexByte(s[:pos], '\n'); index >= 0 {
			s = append(s[:index+1], s[pos+1:]...)
			pos = index + 1
		} else {
			s = s[pos+1:]
			pos = 0
		}
	}
	return s
}
