package interceptor

import "strings"

func Password(pattern string, password string, showOut ...bool) Interceptor {
	return Pattern(pattern, appendLF(password), strings.TrimSpace, showOut...)
}

func LastLinePassword(pattern string, input string, showOut ...bool) Interceptor {
	return Pattern(pattern, appendLF(input), lastLine, showOut...)
}
