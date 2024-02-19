package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_carriageReturnFilter(t *testing.T) {
	src := []byte{0x32, 0x0A, 0x33, 0x35, 0x0D, 0x0D, 0x36, 0x0D, 0x0A}
	expect := []byte{0x32, 0x0A, 0x36, 0x0A}
	dst := CrlfFilter(src)
	assert.Equal(t, expect, dst)
}
