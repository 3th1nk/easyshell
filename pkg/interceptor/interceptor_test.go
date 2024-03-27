package interceptor

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestPattern(t *testing.T) {
	pattern := "\\[Y/N\\]:"
	i := Pattern(pattern, "y", strings.TrimSpace, false)
	matched, show, input := i("want to continue [Y/N]:")
	assert.Equal(t, true, matched)
	assert.Equal(t, "y", input)
	assert.Equal(t, false, show)
}
