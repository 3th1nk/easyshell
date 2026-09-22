package misc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLine(t *testing.T) {
	out := []string{
		"aaa",
		"bbb",
		"ccc",
		"aaa",
		"aaa bbb",
	}
	assert.Equal(t, true, HasLine(out, "aaa"))
	assert.Equal(t, true, HasLine(out, "aaa", "bbb"))
	assert.Equal(t, false, HasLine(out, "aaa", "ccc"))
	assert.Equal(t, true, Contains(out, "aaa"))
	assert.Equal(t, true, Contains(out, "aaa", "bbb"))
	assert.Equal(t, false, Contains(out, "aaa", "ccc"))
	assert.Equal(t, 3, LineCount(out, "aaa"))
	assert.Equal(t, 1, LineCount(out, "aaa", "bbb"))
	assert.Equal(t, 0, LineCount(out, "aaa", "ccc"))
}

func TestTrimEmptyLine(t *testing.T) {
	assert.Equal(t, []string{"aaa", "bbb"}, TrimEmptyLine([]string{"", "aaa", "bbb", ""}))
	assert.Equal(t, []string{"aaa", "bbb"}, TrimEmptyLine([]string{"aaa", "bbb"}))
	assert.Equal(t, []string{"aaa"}, TrimEmptyLine([]string{"", "", "aaa"}))
	assert.Equal(t, []string{"aaa"}, TrimEmptyLine([]string{"aaa", "", ""}))
	// 全部为空行时应返回空
	assert.Empty(t, TrimEmptyLine([]string{"", "", ""}))
	assert.Empty(t, TrimEmptyLine([]string{""}))
	assert.Empty(t, TrimEmptyLine([]string{}))
}
