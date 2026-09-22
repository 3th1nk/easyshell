package core

import (
	"reflect"
	"strings"
)

// isNil 判断值是否为 nil(含接口内携带 nil 指针的场景)
func isNil[T any](v T) bool {
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Invalid: // nil 接口
		return true
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return val.IsNil()
	}
	return false
}

// trimEmptyLines 移除前后的空行
func trimEmptyLines(a []string) []string {
	start := 0
	for ; start < len(a); start++ {
		if a[start] != "" {
			break
		}
	}
	a = a[start:]

	end := len(a)
	for end > 0 && a[end-1] == "" {
		end--
	}
	return a[:end]
}

// tailWindow 返回字符串尾部窗口内的内容(从行边界开始截断)。
//	提示符/拦截器的匹配规则均为尾部锚定，对超大行(如防火墙的超长配置行)只需匹配尾部窗口，
//	避免每个数据块都对整行做正则匹配的 O(N²) 开销
func tailWindow(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	tail := s[len(s)-limit:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	return tail
}
