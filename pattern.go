package easyshell

import "github.com/3th1nk/easyshell/v2/internal/core"

// 内置提示符/认证提示正则(供拦截器等场景引用)
var (
	// DefaultPromptRegex 默认命令行提示符匹配规则
	DefaultPromptRegex = core.DefaultPromptRegex
	// UsernameRegex 登录用户名输入提示
	UsernameRegex = core.UsernameRegex
	// PasswordRegex 登录密码输入提示
	PasswordRegex = core.PasswordRegex
)
