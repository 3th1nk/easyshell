package interceptor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAlwaysNo(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"[yes/no]:", true},
		{"(yes/no):", true},
		{"(yes/no)", false}, // 尾部没有提示符
		{"[y/n]?", true},
		{"(y/n):", true},
		{"[yn]?", true},
		{"(yn)?", true},
		{"[y|n]?", true},
		{"(y|n)?", true},
		{"Save system config?[Y/N]:", true},
		{"(ye/no)", false},             // 单词不完整
		{"yes|no)", false},             // 缺少成对的括号
		{"y|n", false},                 // 缺少括号
		{"Is this ok [y/d/N]:", false}, // 非通用形式，可自定义正则
		{"Are you sure you want to continue connecting (yes/no/[fingerprint])?", false}, // 非通用形式，可自定义正则
	} {
		result, _, _ := AlwaysNo()(unit.str)
		t.Logf("%s, %v", unit.str, unit.expected)
		assert.Equal(t, unit.expected, result)
	}
}
