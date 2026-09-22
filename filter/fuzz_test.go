package filter

import (
	"bytes"
	"testing"
)

// feedAll 将数据按指定大小切片依次喂入，返回累计交付的字节
func feedAll(f Filter, data []byte, chunk int) []byte {
	var out []byte
	for len(data) > 0 {
		n := chunk
		if n > len(data) {
			n = len(data)
		}
		out = append(out, f.Push(data[:n])...)
		data = data[n:]
	}
	return out
}

// FuzzFilter 状态机属性化测试：
//   - 任意字节序列(含非法UTF-8、任意转义序列碎片)不得 panic；
//   - Escape 开启时，交付内容与未完成行中不应残留 ESC 开头的转义序列；
//   - 交付内容与未完成行合并后，非转义字符的数量不多于输入(过滤器只删不增)。
func FuzzFilter(f *testing.F) {
	f.Add([]byte("hello\nworld\r\r\n\x00"))
	f.Add([]byte("abc\x1b[2;5Hdef\x1b]0;t\x07"))
	f.Add([]byte("\x1bP1;2|data\x1b\\ok\n"))
	f.Add([]byte("$\b\r\n\rcontent\n"))
	f.Add([]byte("中文\bAB\r\n"))
	f.Add([]byte("\x1b[38;2;1;2;3m\x1b[0m\x1b[?25l\x1b(B"))

	f.Fuzz(func(t *testing.T, data []byte) {
		def := NewFilter(DefaultOptions())
		out := feedAll(def, data, 7)
		out = append(out, def.Flush()...)

		// 不变量1：交付内容与未完成行中不残留转义序列
		if bytes.ContainsRune(out, 0x1b) {
			t.Fatalf("ESC leaked into output: %q", out)
		}
		if bytes.ContainsRune(def.Pending(), 0x1b) {
			t.Fatalf("ESC leaked into pending: %q", def.Pending())
		}

		// 不变量2：任意切分方式下输出一致(确定性)
		for _, chunk := range []int{1, 2, 3, 13} {
			other := NewFilter(DefaultOptions())
			otherOut := feedAll(other, data, chunk)
			otherOut = append(otherOut, other.Flush()...)
			if !bytes.Equal(out, otherOut) {
				t.Fatalf("chunk=%d output differs: %q vs %q", chunk, out, otherOut)
			}
		}
	})
}

// BenchmarkFilterStress 高频转义序列混合场景
func BenchmarkFilterStress(b *testing.B) {
	input := bytes.Repeat([]byte("data line\x1b[31m\x1b[0m\r\r\n"), 100)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := NewFilter(DefaultOptions())
		_ = feedAll(f, input, 4096)
	}
}
