package interceptor

import "regexp"

func Continue() Interceptor {
	return Regexp(regexp.MustCompile(`(?i)^press\s+any\s+key\s+to\s+continue`), " ", LastLine, false)
}
