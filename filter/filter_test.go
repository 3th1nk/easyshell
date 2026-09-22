package filter

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"strings"
	"testing"
)

// result 一次喂入测试的输出结果
type result struct {
	Lines   []string // 交付的完整行(不含\n)
	Pending string   // 未完成行
}

func (r result) String() string {
	return "{lines:" + strings.Join(r.Lines, "|") + ", pending:" + r.Pending + "}"
}

// feed 将多个 chunk 依次喂入过滤器，收集交付的行
func feed(f Filter, chunks ...[]byte) result {
	var lines []string
	for _, c := range chunks {
		lines = append(lines, splitDelivered(f.Push(c))...)
	}
	return result{Lines: lines, Pending: string(f.Pending())}
}

// feedWhole 整段一次性喂入
func feedWhole(f Filter, input string) result {
	return feed(f, []byte(input))
}

// splitDelivered 拆分 Push 交付的字节(契约保证以\n分隔的完整行)
func splitDelivered(out []byte) []string {
	if len(out) == 0 {
		return nil
	}
	out = out[:len(out)-1] // 去掉最后一个\n
	return strings.Split(string(out), "\n")
}

// chunkSplits 生成多种切分方式：整段、1字节、2字节、3字节、随机点
func chunkSplits(input string, seed int64) [][]string {
	splits := [][]string{
		{input},
		strSplitEvery(input, 1),
		strSplitEvery(input, 2),
		strSplitEvery(input, 3),
	}
	rnd := rand.New(rand.NewSource(seed))
	for _, n := range []int{5, 17} {
		var chunks []string
		for s := input; len(s) > 0; {
			k := n + rnd.Intn(n)
			if k > len(s) {
				k = len(s)
			}
			chunks = append(chunks, s[:k])
			s = s[k:]
		}
		splits = append(splits, chunks)
	}
	return splits
}

func strSplitEvery(s string, n int) []string {
	var arr []string
	for len(s) > 0 {
		k := n
		if k > len(s) {
			k = len(s)
		}
		arr = append(arr, s[:k])
		s = s[k:]
	}
	return arr
}

// optsOf 基于 DefaultOptions 覆盖部分字段
func optsOf(f func(*Options)) *Options {
	o := DefaultOptions()
	f(&o)
	return &o
}

// TestFSM 状态机表驱动测试：输入 → 期望交付的行与未完成行
func TestFSM(t *testing.T) {
	def := DefaultOptions()
	for _, obj := range []struct {
		name    string
		opts    *Options // nil 则使用 DefaultOptions；非 nil 以传入为准(不做字段合并)
		input   string
		lines   []string
		pending string
	}{
		// 纯文本
		{"plain", nil, "hello\nworld\n", []string{"hello", "world"}, ""},
		{"plain_no_lf", nil, "hello", nil, "hello"},

		// CSI 序列
		{"csi_cursor", nil, "abc\x1b[2;5Hdef\n", []string{"abcdef"}, ""},
		{"csi_private", nil, "a\x1b[?25lb\n", []string{"ab"}, ""},
		{"csi_split_params", nil, "a\x1b[1;10;20;30;40Hb\n", []string{"ab"}, ""},

		// SGR 颜色：默认剔除
		{"sgr_default", nil, "a\x1b[31mred\x1b[0m!\n", []string{"ared!"}, ""},
		// SGR 颜色：KeepColor 保留(含 24bit/256 色/冒号参数形式)
		{"sgr_keep", optsOf(func(o *Options) { o.KeepColor = true }), "a\x1b[31mred\x1b[0m!\n", []string{"a\x1b[31mred\x1b[0m!"}, ""},
		{"sgr_24bit", optsOf(func(o *Options) { o.KeepColor = true }), "x\x1b[38;2;255;10;20my\n", []string{"x\x1b[38;2;255;10;20my"}, ""},
		{"sgr_256", optsOf(func(o *Options) { o.KeepColor = true }), "x\x1b[38;5;196my\n", []string{"x\x1b[38;5;196my"}, ""},
		{"sgr_colon_form", optsOf(func(o *Options) { o.KeepColor = true }), "x\x1b[38:2::1:2:3my\n", []string{"x\x1b[38:2::1:2:3my"}, ""},
		{"sgr_colon_form_default", nil, "x\x1b[38:2::1:2:3my\n", []string{"xy"}, ""},

		// EL/ED 擦除：作用于当前行
		{"el_erase", nil, "abc\x1b[Kdef\n", []string{"def"}, ""},
		{"el_erase_0", nil, "abc\x1b[0Kdef\n", []string{"def"}, ""},
		{"ed_erase", nil, "abc\x1b[2Jdef\n", []string{"def"}, ""},
		{"el_no_erase", optsOf(func(o *Options) { o.Erase = false }), "abc\x1b[Kdef\n", []string{"abcdef"}, ""},
		// 重绘场景：\r 清行 + EL + 重写(v1 金标准)
		{"redraw", nil, "旧内容\r\x1b[K新内容\n", []string{"新内容"}, ""},

		// OSC：BEL 与 ST 终止
		{"osc_bel", nil, "\x1b]0;title\x07hello\n", []string{"hello"}, ""},
		{"osc_st", nil, "\x1b]0;title\x1b\\hello\n", []string{"hello"}, ""},
		{"osc_body_multiline", nil, "\x1b]0;a\nb\x07c\n", []string{"c"}, ""},

		// DCS/SOS/PM/APC：ST 终止
		{"dcs", nil, "\x1bP1;2|data\x1b\\ok\n", []string{"ok"}, ""},
		{"sos", nil, "\x1bXdata\x1b\\ok\n", []string{"ok"}, ""},
		{"pm", nil, "\x1b^data\x1b\\ok\n", []string{"ok"}, ""},
		{"apc", nil, "\x1b_data\x1b\\ok\n", []string{"ok"}, ""},
		{"dcs_bel_not_terminator", nil, "\x1bPdata\x07ok\n", nil, ""}, // BEL 不终止 DCS，整段吞掉至 EOF
		{"dcs_bel_terminator", optsOf(func(o *Options) { o.TerminatorBEL = true }), "\x1bPdata\x07ok\n", []string{"ok"}, ""},

		// 两字节 ESC 序列与字符集指定
		{"esc_charset", nil, "a\x1b(Bb\n", []string{"ab"}, ""},
		{"esc_ris", nil, "a\x1bcb\n", []string{"ab"}, ""},
		{"esc_can_abort", nil, "a\x1b\x18b\n", []string{"ab"}, ""},
		{"esc_c0_execute", nil, "ab\x1b\ncd\n", []string{"ab", "cd"}, ""},

		// Escape=false 原样保留
		{"escape_off", optsOf(func(o *Options) { o.Escape = false }), "a\x1b[31mb\n", []string{"a\x1b[31mb"}, ""},

		// 退格
		{"backspace", nil, "abc\b\bX\n", []string{"aX"}, ""},
		{"backspace_at_bol", nil, "\bX\n", []string{"X"}, ""},
		{"backspace_all", nil, "abc\b\b\b\n", []string{""}, ""},
		{"backspace_off", optsOf(func(o *Options) { o.Backspace = false }), "abc\bX\n", []string{"abc\bX"}, ""},
		{"backspace_utf8", nil, "中文\bAB\n", []string{"中AB"}, ""},
		{"backspace_utf8_full", nil, "中文\b\bAB\n", []string{"AB"}, ""},

		// ARRAY APV 特殊退格：$ + 退格 + \r\n\r + 内容，$ 保留、内容同行
		{"array_apv", nil, "$\b\r\n\rcontent\n", []string{"$content"}, ""},

		// CRLF 与 H3C \r\r\n
		{"crlf", nil, "a\r\nb\n", []string{"a", "b"}, ""},
		{"crcr_lf", nil, "a\r\r\nb\n", []string{"a", "b"}, ""},
		{"crcr_lf_nul", nil, "a\r\r\n\x00b\n", []string{"a", "b"}, ""},
		{"lf_only", nil, "a\nb\n", []string{"a", "b"}, ""},
		{"crlf_off", optsOf(func(o *Options) { o.CRLF = false }), "a\r\nb\n", []string{"a\r", "b"}, ""},

		// 单独 \r 的三种模式
		{"cr_begin_of_line", nil, "abc\rXY\n", []string{"XY"}, ""},
		{"cr_begin_of_line_multi", nil, "abc\rXY\rZ\n", []string{"Z"}, ""},
		{"cr_drop", optsOf(func(o *Options) { o.CRTrim = CRTrimDropCR }), "abc\rXY\n", []string{"abcXY"}, ""},
		{"cr_keep", optsOf(func(o *Options) { o.CRTrim = CRTrimKeep }), "abc\rXY\n", []string{"abc\rXY"}, ""},
		{"cr_off_passthrough", optsOf(func(o *Options) { o.CRLF = false }), "abc\rXY\n", []string{"abc\rXY"}, ""},

		// NUL
		{"nul_drop", nil, "a\x00b\n", []string{"ab"}, ""},
		{"nul_keep", optsOf(func(o *Options) { o.DropNUL = false }), "a\x00b\n", []string{"a\x00b"}, ""},
		{"cr_lf_nul", nil, "a\r\n\x00b\n", []string{"a", "b"}, ""},

		// 显式零值 Options = 关闭全部过滤(仅保留行组装)
		{"all_off", &Options{}, "a\x1b[31m\r\nb\b\n", []string{"a\x1b[31m\r", "b\b"}, ""},
	} {
		t.Run(obj.name, func(t *testing.T) {
			opts := def
			if obj.opts != nil {
				opts = *obj.opts
			}
			got := feedWhole(NewFilter(opts), obj.input)
			assert.Equal(t, result{obj.lines, obj.pending}, got, "input=%q", obj.input)
		})
	}
}

// TestFSM_ChunkSplit 跨 chunk 一致性：同一输入按不同方式切分喂入，输出必须与整段喂入完全一致。
//
//	这是有状态过滤器相对 v1 无状态实现的核心价值验证。
func TestFSM_ChunkSplit(t *testing.T) {
	inputs := []string{
		"hello\nworld\n",
		"abc\x1b[2;5Hdef\x1b[31m!\x1b[0m\n",
		"\x1b]0;title\x07hello\n",
		"\x1b]0;title\x1b\\hello\n",
		"\x1bP1;2|data\x1b\\ok\n",
		"a\r\r\n\x00b\r\n\n",
		"中文\b\bAB\r\n",
		"旧内容\r\x1b[K新内容\n",
		"$\b\r\n\rcontent\n",
		"prompt# ", // 无换行的未完成行
	}
	for i, input := range inputs {
		var want result
		for j, chunks := range chunkSplits(input, int64(i+1)) {
			got := feed(NewFilter(DefaultOptions()), toChunks(chunks)...)
			if j == 0 {
				want = got
				continue
			}
			assert.Equal(t, want, got, "input=%q, chunks=%q", input, chunks)
		}
	}
}

func toChunks(arr []string) [][]byte {
	out := make([][]byte, len(arr))
	for i, s := range arr {
		out[i] = []byte(s)
	}
	return out
}

// TestFSM_Pending 验证 Pending 的可见性与 DropPending
func TestFSM_Pending(t *testing.T) {
	f := NewFilter(DefaultOptions())

	assert.Empty(t, f.Pending())
	_ = f.Push([]byte("prom"))
	assert.Equal(t, "prom", string(f.Pending()))

	f.DropPending()
	assert.Empty(t, f.Pending())

	_ = f.Push([]byte("pt# "))
	assert.Equal(t, "pt# ", string(f.Pending()))

	// 残余行通过 Flush 交付
	out := f.Flush()
	assert.Equal(t, []string{"pt# "}, splitDelivered(out))
	assert.Empty(t, f.Pending())
}

// TestFSM_MaxLineBuffer 超限直通：停止行编辑仿真，内容原样保留
func TestFSM_MaxLineBuffer(t *testing.T) {
	f := NewFilter(*optsOf(func(o *Options) { o.MaxLineBuffer = 4 }))
	got := feedWhole(f, "abcdef\bX\n")
	// 'e' 追加时超限 → 直通，后续退格原样保留
	assert.Equal(t, []string{"abcdef\bX"}, got.Lines)

	// 行交付后直通模式复位
	got = feedWhole(f, "ab\bZ\n")
	assert.Equal(t, []string{"aZ"}, got.Lines)
}

// TestFSM_Flush_Incomplete 序列在流结束时未完成的行为
func TestFSM_Flush_Incomplete(t *testing.T) {
	// 默认：丢弃不完整序列，交付残余行
	f := NewFilter(DefaultOptions())
	_ = f.Push([]byte("text\x1b[12"))
	out := f.Flush()
	assert.Equal(t, []string{"text"}, splitDelivered(out))

	// DropIncomplete=false：已累积的原始字节原样输出
	f = NewFilter(Options{})
	_ = f.Push([]byte("text\x1b[12"))
	out = f.Flush()
	assert.Equal(t, []string{"text\x1b[12"}, splitDelivered(out))

	// 字符串序列体截断：内容未缓存，总是丢弃
	f = NewFilter(DefaultOptions())
	_ = f.Push([]byte("\x1b]0;title"))
	assert.Empty(t, f.Flush())
}

// TestFilterFunc 无状态适配器的契约
func TestFilterFunc(t *testing.T) {
	var f Filter = FilterFunc(func(p []byte) []byte {
		out := make([]byte, 0, len(p))
		for _, b := range p {
			if b != '\r' {
				out = append(out, b)
			}
		}
		return out
	})
	out := f.Push([]byte("a\rb\n"))
	assert.Equal(t, []byte("ab\n"), out)
	assert.Empty(t, f.Pending())
	f.DropPending()
	assert.Empty(t, f.Flush())
}

// TestStripReplaceChars 解码后阶段的替换字符剔除
func TestStripReplaceChars(t *testing.T) {
	assert.Equal(t, "abc", StripReplaceChars("abc"))
	assert.Equal(t, "abc", StripReplaceChars("a�b��c"))
	assert.Equal(t, "", StripReplaceChars("��"))
}
