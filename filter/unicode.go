package filter

import "strings"

// replaceChar U+FFFD 替换字符
const replaceChar = "�"

// StripReplaceChars 剔除文本中的 U+FFFD 替换字符。
//
// 该函数属于过滤管道的"解码后"阶段：字节级过滤(转义/退格/换行)在解码前完成，
// 而替换字符是解码器(如 GB18030→UTF8)对无法解码字节的产物，只能在解码后剔除。
// 由调用方(core)在解码完成后调用。
func StripReplaceChars(s string) string {
	if !strings.Contains(s, replaceChar) {
		return s
	}
	return strings.ReplaceAll(s, replaceChar, "")
}
