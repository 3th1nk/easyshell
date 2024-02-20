package interceptor

import "regexp"

func More() Interceptor {
	// 部分设备遇到 more 时，会在其后返回回车符来清除 more 提示，这里将回车符一起匹配(清除)，避免干扰正常内容
	return Regexp(regexp.MustCompile(`(?i)^\s*-+\s*more\s*-+(\s\r)?`), " ", LastLine, false)
}
