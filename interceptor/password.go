package interceptor

import "strings"

// Password 需要调用方保证 pattern 是合法的正则表达式
func Password(pattern string, password string, showOut ...bool) Interceptor {
	return Pattern(pattern, AppendLF(password), strings.TrimSpace, showOut...)
}

// LastLinePassword 需要调用方保证 pattern 是合法的正则表达式
func LastLinePassword(pattern string, input string, showOut ...bool) Interceptor {
	return Pattern(pattern, AppendLF(input), LastLine, showOut...)
}
