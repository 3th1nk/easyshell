package filter

import (
	"bytes"
)

func backspaceFilter(s []byte) []byte {
	var bsCnt int
	var pos int
	for pos < len(s) {
		if s[pos] == '\b' {
			bsCnt++
			pos++
			continue
		}

		if bsCnt > 0 {
			// ARRAY APV负载均衡设备输入内容过长触发收缩时的特殊退格
			// $ + 退格 + \r\n\r + 内容
			var isArrayApvIndent bool
			if pos-bsCnt > 0 && s[pos-bsCnt-1] == '$' &&
				pos+2 < len(s) && s[pos] == '\r' && s[pos+1] == '\n' && s[pos+2] == '\r' {
				isArrayApvIndent = true
			}

			if isArrayApvIndent {
				// 删除退格
				s = append(s[:pos-bsCnt], s[pos:]...)
				// 跳过\r\n\r
				pos = pos - bsCnt + 4
			} else {
				// 删除退格以及前面等长的内容
				start := pos - bsCnt<<1
				if start < 0 {
					start = 0
				}
				// 退格只处理所在行内的字符
				if index := bytes.LastIndexByte(s[:pos], '\n'); index >= start {
					start = index + 1
				}
				s = append(s[:start], s[pos:]...)
				pos = start + 1
			}

			bsCnt = 0
			continue
		}

		pos++
	}

	// 处理尾部的退格
	if bsCnt > 0 {
		start := pos - bsCnt<<1
		if start < 0 {
			start = 0
		}
		// 退格只处理所在行内的字符
		if index := bytes.LastIndexByte(s[:pos], '\n'); index >= start {
			start = index + 1
		}
		s = append(s[:start], s[pos:]...)
	}

	return s
}
