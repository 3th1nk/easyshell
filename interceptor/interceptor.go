package interceptor

import (
	"regexp"
)

type Interceptor func(str string) (match bool, showOut bool, input string)

func invalidInterceptor(str string) (bool, bool, string) {
	return false, false, ""
}

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

// Pattern 需要调用方保证 pattern 是合法的正则表达式
func Pattern(pattern string, input string, format func(string) string, showOut ...bool) Interceptor {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return invalidInterceptor
	}
	return Regexp(re, input, format, showOut...)
}

// LastLinePattern 需要调用方保证 pattern 是合法的正则表达式
func LastLinePattern(pattern string, input string, showOut ...bool) Interceptor {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return invalidInterceptor
	}
	return Regexp(re, input, LastLine, showOut...)
}
