package injector

import (
	"regexp"
	"strings"
)

type InputInjector func(str string) (match bool, showOut bool, input string)

func Default() []InputInjector {
	return append(Continue(), More())
}

func More() InputInjector {
	return Pattern(`(?i)^\s*-+\s*more\s*-+`, " ", lastLine, false)
}

func Continue() []InputInjector {
	return []InputInjector{
		Pattern(`(?i)^press\s+any\s+key\s+to\s+continue`, " ", lastLine, false),
		Pattern(`(?i)y[es]/n[o]`, "n", lastLine, false),
	}
}

func Password(pattern string, password string, showOut ...bool) InputInjector {
	return Pattern(pattern, appendLF(password), strings.TrimSpace, showOut...)
}

func LastLinePassword(pattern string, input string, showOut ...bool) InputInjector {
	return Pattern(pattern, appendLF(input), lastLine, showOut...)
}

func Regexp(regex *regexp.Regexp, input string, format func(string) string, showOut ...bool) InputInjector {
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

func Pattern(pattern string, input string, format func(string) string, showOut ...bool) InputInjector {
	return Regexp(regexp.MustCompile(pattern), input, format, showOut...)
}
