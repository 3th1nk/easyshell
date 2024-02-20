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
		{" --- more --- \r\r    \r\r", true},
	} {
		result, _, _ := More()(unit.str)
		t.Logf("%d, %02x, %v", len(unit.str), unit.str, unit.expected)
		assert.Equal(t, unit.expected, result)
	}
}
