package interceptor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAlwaysYesOrNo(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"[yes/no]:", true},
		{"(yes/no):", true},
		{"(yes/no)", false}, // 尾部没有问号或冒号
		{"[y/n]?", true},
		{"(y/n):", true},
		{"[yn]?", false}, // 缺少分隔符
		{"(yn)?", false}, // 缺少分隔符
		{"[y|n]?", true},
		{"(y|n)?", true},
		{"Save system config?[Y/N]:", true},
		{"(ye/no)", false},             // 单词不完整，尾部没有问号或冒号
		{"yes|no)", false},             // 缺少成对的括号
		{"y|n", false},                 // 缺少括号
		{"Is this ok [y/d/N]:", false}, // 非通用形式，可使用 AlwaysOption
		{"Are you sure you want to continue connecting (yes/no/[fingerprint])?", false}, // 非通用形式，可使用 AlwaysOption
	} {
		t.Logf("%s, %v", unit.str, unit.expected)
		result, _, _ := AlwaysYes()(unit.str)
		assert.Equal(t, unit.expected, result)

		result, _, _ = AlwaysNo()(unit.str)
		assert.Equal(t, unit.expected, result)
	}
}

func TestAlwaysOption(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"[yes/no]:", true},
		{"(yes/no):", true},
		{"(yes/no)", false}, // 尾部没有问号或冒号
		{"[y/n]?", true},
		{"(y/n):", true},
		{"[yn]?", false}, // 缺少分隔符
		{"(yn)?", false}, // 缺少分隔符
		{"[y|n]?", true},
		{"(y|n)?", true},
		{"Save system config?[Y/N]:", true},
		{"(ye/no)", false}, // 尾部没有问号或冒号
		{"yes|no)", false}, // 缺少成对的括号
		{"y|n", false},     // 缺少括号
		{"Is this ok [y/d/N]:", true},
		{"Are you sure you want to continue connecting (yes/no/[fingerprint])?", true},
	} {
		t.Logf("%s, %v", unit.str, unit.expected)
		result, _, _ := AlwaysOption(0)(unit.str)
		assert.Equal(t, unit.expected, result)
	}
}
