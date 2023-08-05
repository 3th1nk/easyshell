package interceptor

import (
	"strings"
	"unicode"
)

func lastLine(str string) string {
	str = strings.TrimRightFunc(str, unicode.IsSpace)
	if i := strings.LastIndexByte(str, '\n'); i != -1 {
		str = str[i+1:]
	}
	return str
}

func appendLF(s string) string {
	if len(s) == 0 {
		return "\n"
	}
	if s[len(s)-1] != '\n' {
		return s + "\n"
	}
	return s
}
