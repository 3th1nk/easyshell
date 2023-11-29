package interceptor

import "regexp"

func AlwaysYes() Interceptor {
	return Regexp(regexp.MustCompile(`(?i)[\[(](y|yes)[/|]?(n|no)[])][:?]\s*$`), "y", LastLine, false)
}

func AlwaysNo() Interceptor {
	return Regexp(regexp.MustCompile(`(?i)[\[(](y|yes)[/|]?(n|no)[])][:?]\s*$`), "n", LastLine, false)
}
