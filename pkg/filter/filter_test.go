package filter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	strArr = [][]string{
		{"abcc\b", "abc"},
		{"\babcc\bcc\b\b", "abc"},
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
		//{"\x1b[31;47;<r>;<g>;<b>mabc\x1b[0m", "abc"},// not support
		{"\u001B[80;1HCD_DA11F_MX01#\x1b[80;1H\x1b[80;17H", "CD_DA11F_MX01#"},
		{"\u001B[1;13r\u001B[1;1H\u001B[80;1HPress any key to continue\n\u001B[13;1H\u001B[?25h\u001B[80;27H\u001B[?6l\u001B[1;80r\u001B[?7h\u001B[2J\u001B[1;1H\u001B[1920;1920H\u001B[6n\u001B[1;1HYour previous successful login (as manager) was on 2022-08-25 01:30:16 from 172.26.66.11 \u001B[1;80r\u001B[80;1H\u001B[80;1H\u001B[2K\u001B[80;1H\u001B[?25h\u001B[80;1H\u001B[80;1HCD_OA_11F_MX01#\u001B[80;1H\u001B[80;17H\u001B[80;1H\u001B[?25h\u001B[80;17H", "Press any key to continue\nYour previous successful login (as manager) was on 2022-08-25 01:30:16 from 172.26.66.11 CD_OA_11F_MX01#"},
		{"abc$\b\b\b\r\n\r", "abc$\n"},
	}
)

func TestDefaultFilter(t *testing.T) {
	for _, val := range strArr {
		assert.Equal(t, []byte(val[1]), DefaultFilter([]byte(val[0])))
	}
}

func BenchmarkDefaultFilter(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, val := range strArr {
			DefaultFilter([]byte(val[0]))
		}
	}
}
