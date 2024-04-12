package interceptor

import (
	"github.com/3th1nk/easygo/util/strUtil"
	"regexp"
	"strings"
)

const (
	// DefaultOptionPromptPattern  默认选项的匹配规则
	DefaultOptionPromptPattern = `(?i)[\[(]y(es)?[/|]no?[\])][?:]\s*$`
	// FlexibleOptionPromptPattern 灵活选项的匹配规则
	FlexibleOptionPromptPattern = `(?i)[\[(][a-z]+([/|][a-z\[\]]+)+[\])][?:]\s*$`
)

func AlwaysYes(showOut ...bool) Interceptor {
	showOut = append(showOut, false)
	return Regexp(regexp.MustCompile(DefaultOptionPromptPattern), AppendLF("y"), LastLine, showOut...)
}

func AlwaysNo(showOut ...bool) Interceptor {
	showOut = append(showOut, false)
	return Regexp(regexp.MustCompile(DefaultOptionPromptPattern), AppendLF("n"), LastLine, showOut...)
}

func AlwaysOption(optionIndex int, showOut ...bool) Interceptor {
	showOut = append(showOut, false)
	return func(str string) (bool, bool, string) {
		str = LastLine(str)
		if re := regexp.MustCompile(FlexibleOptionPromptPattern); re.MatchString(str) {
			// 截取选项部分，并去掉前后的括号，例如 [yes/no] -> yes/no
			if idx := strings.IndexAny(str, "[("); idx != -1 {
				str = str[idx+1:]
			}
			if idx := strings.IndexAny(str, ")]"); idx != -1 {
				str = str[:idx]
			}
			// 获取所有选项，这里options一定不为空（由匹配正则保证）
			options := strUtil.Split(str, "/|", false, func(s string) string {
				// 有的选项是可选的，需要去掉括号，例如 yes/no/[fingerprint]
				s = strings.TrimPrefix(s, "[")
				s = strings.TrimSuffix(s, "]")
				return s
			})

			// 选择的选项不能超出范围
			var input string
			if optionIndex >= 0 && optionIndex < len(options) {
				input = strings.TrimSpace(options[optionIndex])
			}
			return true, showOut[0], AppendLF(input)
		}

		return false, false, ""
	}
}
