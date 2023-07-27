package injector

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSayNo(t *testing.T) {
	for _, unit := range []struct {
		str      string
		expected bool
	}{
		{"[yes/no]", true},
		{"(yes/no)", true},
		{"?[y/n]", true},
		{"?(y/n)", true},
		{"[yn]?", true},
		{"(yn)?", true},
		{"[y|n]", true},
		{"(y|n)", true},
		{"(ye/no)", false},
		{"yes|no", false},
		{"y|n", false},
		{"Is this ok [y/d/N]:", false},
		{"Are you sure you want to continue connecting (yes/no/[fingerprint])?", false},
	} {
		result, _, _ := AlwaysNo()(unit.str)
		t.Logf("%s, %v", unit.str, unit.expected)
		assert.Equal(t, unit.expected, result)
	}
}
