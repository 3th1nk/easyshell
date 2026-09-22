package interceptor

import (
	"regexp"
)

// Interceptor 输出拦截器：由读取循环在每批输出上同步调用。
//
//	返回值：
//	  match   是否命中；命中后读取循环将 input 自动写入设备(等价人工输入)，
//	          并按 showOut 决定命中行的去留
//	  showOut 命中行是否仍交付给 onOut(典型：密码/翻页提示为 false 隐藏)
//	  input   自动应答内容(可为空串)
//
//	注意：阻塞本函数会推迟读取与提示符判定(设备可能在等待应答)，
//	需要重处理时请在返回前自行异步化。
type Interceptor func(str string) (match bool, showOut bool, input string)

func invalidInterceptor(str string) (bool, bool, string) {
	return false, false, ""
}

// Regexp 基于已编译正则创建拦截器。
//
//	format 在匹配前对整批输出做变换(如 LastLine 取最后一行、strings.TrimSpace 归一)；
//	showOut 可省略：省略时命中行隐藏，传入时以传入值为准。
func Regexp(regex *regexp.Regexp, input string, format func(string) string, showOut ...bool) Interceptor {
	return func(str string) (bool, bool, string) {
		if format != nil {
			str = format(str)
		}
		if regex.MatchString(str) {
			hide := len(showOut) != 0 && !showOut[0]
			return true, !hide, input
		}
		return false, false, ""
	}
}

// Pattern 基于正则表达式字符串创建拦截器(pattern 非法时返回永远不命中的拦截器)。
//	参数语义同 Regexp。
func Pattern(pattern string, input string, format func(string) string, showOut ...bool) Interceptor {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return invalidInterceptor
	}
	return Regexp(re, input, format, showOut...)
}
