package interceptor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestContinue(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"press any key to continue", true},
		{"Press any key to continue", true},
	} {
		result, _, _ := Continue()(unit.str)
		t.Logf("%s, %v", unit.str, unit.expected)
		assert.Equal(t, unit.expected, result)
	}
}
