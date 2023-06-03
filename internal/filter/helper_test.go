package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_dropBytes(t *testing.T) {
	src := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39}
	expect := []byte{0x30, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39}
	dropCnt := dropBytes(src, 1, 2)
	assert.Equal(t, 1, dropCnt)
	assert.Equal(t, expect, src[:len(src)-dropCnt])
}

func Test_dropMultiBytes(t *testing.T) {
	src := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39}
	dropArr := [][2]int{
		{0, 1}, // 0x30
		{4, 6}, // 0x34, 0x35
		{7, 8}, // 0x37
	}
	expect := []byte{0x31, 0x32, 0x33, 0x36, 0x38, 0x39}
	actual := dropMultiBytes(src, dropArr)
	assert.Equal(t, expect, actual)
}
