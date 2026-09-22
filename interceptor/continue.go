package interceptor

import "regexp"

func Continue() Interceptor {
	return Regexp(regexp.MustCompile(`(?i)^\s*press\s+any\s+key\s+to\s+continue(\s\r)?`), " ", LastLine, false)
}
