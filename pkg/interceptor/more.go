package interceptor

import "regexp"

func More() Interceptor {
	// 部分设备遇到 more 时，会在其后返回回车符来清除 more 提示，这里将回车符一起匹配(清除)，避免干扰正常内容
	//	例如: ---- More ----\r\r
	// 这里尾部可能会有控制字符等，所以不做严格的尾部匹配，由此带来的问题是可能会误匹配到一些非 more 提示的内容
	return Regexp(regexp.MustCompile(`(?i)^\s*-{2,}\s*(\(?\s*)?more(\s+\d+%\s*\)?)?\s*-{2,}(\s\r)?`), " ", LastLine, false)
}
