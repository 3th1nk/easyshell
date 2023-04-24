package injector

import (
	"regexp"
	"strings"
)

var (
	moreRegex     = regexp.MustCompile(`(?i)^\s*-+\s*more\s*-+`)
	continueRegex = regexp.MustCompile(`(?i)^press any key to continue`)
)

type InputInjector func(str string) (match bool, showOut bool, input string)

func More() InputInjector {
	return Regexp(moreRegex, " ", lastLine, false)
}

func Continue() InputInjector {
	return Regexp(continueRegex, " ", lastLine, false)
}

func Password(pattern string, password string, showOut ...bool) (InputInjector, error) {
	return Pattern(pattern, appendLF(password), strings.TrimSpace, showOut...)
}

func PasswordList(pattern []string, password string, showOut ...bool) ([]InputInjector, error) {
	arr := make([]InputInjector, len(pattern))
	for i, p := range pattern {
		if v, err := Pattern(p, appendLF(password), strings.TrimSpace, showOut...); err != nil {
			return nil, err
		} else {
			arr[i] = v
		}
	}
	return arr, nil
}

func LastLinePassword(pattern string, input string, showOut ...bool) (InputInjector, error) {
	return Pattern(pattern, appendLF(input), lastLine, showOut...)
}

func LastLinePasswordList(pattern []string, password string, showOut ...bool) ([]InputInjector, error) {
	arr := make([]InputInjector, len(pattern))
	for i, p := range pattern {
		if v, err := Pattern(p, appendLF(password), lastLine, showOut...); err != nil {
			return nil, err
		} else {
			arr[i] = v
		}
	}
	return arr, nil
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

func Pattern(pattern string, input string, format func(string) string, showOut ...bool) (InputInjector, error) {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return Regexp(r, input, format, showOut...), nil
}

func PatternList(pattern []string, input string, format func(string) string, showOut ...bool) ([]InputInjector, error) {
	arr := make([]InputInjector, len(pattern))
	for i, p := range pattern {
		if v, err := Pattern(p, input, format, showOut...); err != nil {
			return nil, err
		} else {
			arr[i] = v
		}
	}
	return arr, nil
}
