package filter

import (
	"bytes"
)

func backspaceFilter(s []byte) []byte {
	var pos, bsCnt int
	length := len(s)
	for ; pos < length; pos++ {
		if s[pos] == '\b' {
			bsCnt++
			continue
		}

		if bsCnt > 0 {
			// ARRAY APV负载均衡设备输入内容过长触发收缩时的特殊退格
			// $ + 退格 + \r\n\r + 内容
			var isArrayApvIndent bool
			if pos-bsCnt > 0 && s[pos-bsCnt-1] == '$' &&
				pos+2 < length && s[pos] == '\r' && s[pos+1] == '\n' && s[pos+2] == '\r' {
				isArrayApvIndent = true
			}

			if isArrayApvIndent {
				// 删除退格
				length -= dropBytes(s, pos-bsCnt, pos)
				// 跳过\r\n\r
				pos = pos - bsCnt + 3
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
				length -= dropBytes(s, start, pos)
				pos = start
			}
			bsCnt = 0
		}
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
		length -= dropBytes(s, start, pos)
	}

	return s[:length]
}
