package core

import (
	"github.com/3th1nk/easyshell/pkg/interceptor"
	"regexp"
)

const (
	DefaultPromptTailChars     = `$#%>\]:`
	DefaultPromptSuffixPattern = `.*[` + DefaultPromptTailChars + `]\s*$`
)

var (
	DefaultPromptRegex        = regexp.MustCompile(`\S+` + DefaultPromptSuffixPattern)
	FlexibleOptionPromptRegex = regexp.MustCompile(interceptor.FlexibleOptionPromptPattern)
	UsernameRegex             = regexp.MustCompile(`(?i).*(login|user(name)?):\s*$`)
	PasswordRegex             = regexp.MustCompile(`(?i).*pass(word)?:\s*$`)
)
