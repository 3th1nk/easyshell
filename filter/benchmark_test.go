package filter

import (
	"bytes"
	"strings"
	"testing"
)

// benchInput 构造接近真实设备输出的混合内容：文本行 + 转义序列 + CRLF
func benchInput(lines int) []byte {
	var buf bytes.Buffer
	for i := 0; i < lines; i++ {
		buf.WriteString("  version 7.1.070, Release 3506P10, config line number ")
		buf.WriteString(strings.Repeat("x", 40))
		buf.WriteString("\x1b[31m") // SGR
		buf.WriteString("\r\r\n")
		if i%32 == 0 {
			buf.WriteString("\x1b]0;device title\x07") // OSC
		}
	}
	return buf.Bytes()
}

// BenchmarkFilter_Default 默认配置：真实场景的吞吐基线
func BenchmarkFilter_Default(b *testing.B) {
	input := benchInput(256)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewFilter(DefaultOptions())
		for len(input) > 0 {
			n := 4096
			if n > len(input) {
				n = len(input)
			}
			_ = f.Push(input[:n])
			input = input[n:]
		}
	}
}

// BenchmarkFilter_Passthrough 关闭全部过滤：快速路径上限
func BenchmarkFilter_Passthrough(b *testing.B) {
	input := benchInput(256)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewNoop()
		for len(input) > 0 {
			n := 4096
			if n > len(input) {
				n = len(input)
			}
			_ = f.Push(input[:n])
			input = input[n:]
		}
	}
}
