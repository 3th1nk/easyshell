package interceptor

import "regexp"

func More() Interceptor {
	return Regexp(regexp.MustCompile(`(?i)^\s*-+\s*more\s*-+`), " ", LastLine, false)
}
