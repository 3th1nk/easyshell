package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	ctrlStrArr = [][]string{
		{"abcc\b", "abc"},
		{"\babcc\bc\b", "abc"},
		{"\x1b[Kabc", "abc"},
		{"\x1b[2habc", "abc"},
		{"\x1b[?25labc", "abc"},
		{"\x1b[;1fabc", "abc"},
		{"\x1b[1;1fabc", "abc"},
		{"\x1b[1@abc", "abc"},
		{"\x1b[1Aabc", "abc"},
		{"\x1b[31mabc\x1b[0m", "abc"},
		{"\x1b[31;47mabc\x1b[0m", "abc"},
		{"\x1b[31:47mabc\x1b[0m", "abc"},
		{"\x1b[31;47;<r>;<g>;<b>mabc\x1b[0m", "abc"},
		{"\u001B[80;1HCD_DA11F_MX01#\x1b[80;1H\x1b[80;17H", "CD_DA11F_MX01#"},
	}
)

func TestCtrlFilter(t *testing.T) {
	for _, val := range ctrlStrArr {
		assert.Equal(t, CtrlFilter([]byte(val[0])), []byte(val[1]))
	}
}

func BenchmarkCtrlFilter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, val := range ctrlStrArr {
			CtrlFilter([]byte(val[0]))
		}
	}
}
