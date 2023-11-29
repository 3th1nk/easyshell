package interceptor

import "strings"

func Password(pattern string, password string, showOut ...bool) Interceptor {
	return Pattern(pattern, AppendLF(password), strings.TrimSpace, showOut...)
}

func LastLinePassword(pattern string, input string, showOut ...bool) Interceptor {
	return Pattern(pattern, AppendLF(input), LastLine, showOut...)
}
