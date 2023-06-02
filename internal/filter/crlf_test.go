package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_carriageReturnFilter(t *testing.T) {
	src := []byte{0x32, 0x33, 0x35, 0x0D, 0x0D, 0x0A}
	expect := []byte{0x0A}
	dst := crlfFilter(src)
	assert.Equal(t, expect, dst)
}
