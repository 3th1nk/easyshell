package interceptor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMore(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"---- More ----", true},
		{" --- more --- ", true},
		{"-- more --", true},
		{"- more -", false}, // 翻页More至少有连续两个 -
		{"---- more ---- \r\r    \r\r", true},
		{"---- more 80% ----", true},
		{"----(more 80%)----", true},
		{"---- (more 80%) ----", true},
		{"---- ( more 80% ) ----", true},
		{"abc--more--", false}, // 前缀不匹配
		{"--more--abc", true},  // 由于不能做严格的尾部匹配，这种会误匹配
	} {
		result, _, _ := More()(unit.str)
		t.Logf("%d, %02x, %v", len(unit.str), unit.str, unit.expected)
		assert.Equal(t, unit.expected, result)
	}
}
