package core

import (
	"github.com/3th1nk/easyshell/v2/interceptor"
	"regexp"
)

// RunOptions 命令执行/读取的可选参数。
//
//	按库约定以可变参数传入(opts ...RunOptions)：可省略(使用默认行为)；
//	最多传一个，传入则以传入为准(不做字段合并)。
type RunOptions struct {
	// Prompt 指定本次命令的结束提示符(严格模式)。
	//
	//	设置后本次读取仅以该规则匹配结束——默认宽松规则与其误报排除规则均不参与，
	//	用于提示符会变化的命令(网络设备进入/退出配置模式、主机 su/sudo、子命令环境等)：
	//	既避免宽松规则误匹配输出内容，也避免变化后的提示符匹配不到而超时。
	//	命令结束后通过 Prompt() 获取新的提示符。
	Prompt *regexp.Regexp
	// Interceptors 输出拦截器(密码交互、翻页应答、选项应答、错误检测应答等)
	Interceptors []interceptor.Interceptor
}
