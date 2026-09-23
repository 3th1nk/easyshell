package core

import (
	"github.com/3th1nk/easyshell/v2/interceptor"
	"regexp"
	"time"
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
	// Timeout 本次调用的超时(独立于 ctx，零值不限、跟随 ctx)。
	//
	//	从写入命令/开始读取起算的总时长上限：Run=单条命令(每命令超时)，
	//	ReadUntilPrompt/ReadAll/RunAll=本次调用整体。
	//	用途是给慢命令(ping/copy/长配置回显)单独放宽、或给交互类命令单独收紧，
	//	避免为局部需求放大整个会话 ctx 的超时。
	//	超时返回的错误可经 IsTimeout 判断(OpTimeout)，与 ctx 超时同一语义。
	Timeout time.Duration
}
