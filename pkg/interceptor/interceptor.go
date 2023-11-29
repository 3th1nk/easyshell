package interceptor

import (
	"regexp"
)

type Interceptor func(str string) (match bool, showOut bool, input string)

func Regexp(regex *regexp.Regexp, input string, format func(string) string, showOut ...bool) Interceptor {
	return func(str string) (bool, bool, string) {
		if format != nil {
			str = format(str)
		}
		if regex.MatchString(str) {
			if len(showOut) != 0 {
				return true, showOut[0], input
			}
			return true, true, input
		}
		return false, false, ""
	}
}

func LastLineRegex(regex *regexp.Regexp, input string, showOut ...bool) Interceptor {
	return Regexp(regex, input, LastLine, showOut...)
}

func Pattern(pattern string, input string, format func(string) string, showOut ...bool) Interceptor {
	return Regexp(regexp.MustCompile(pattern), input, format, showOut...)
}

func LastLinePattern(pattern string, input string, showOut ...bool) Interceptor {
	return Regexp(regexp.MustCompile(pattern), input, LastLine, showOut...)
}
