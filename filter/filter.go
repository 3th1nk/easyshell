// Package filter 提供终端输出的字节级过滤：剔除 ANSI/ECMA-48 转义序列、处理退格与换行归一化。
//
// 与常见的无状态过滤函数不同，Filter 是有状态的：
//   - 转义序列即使被网络分包截断在任意字节边界，也能正确识别并剔除；
//   - 退格、擦除(EL/ED)、单独 \r 清行等"行编辑"操作实时作用于尚未交付的当前行；
//   - 只交付完整行(以 \n 结尾)，不完整的尾行保留在内部，避免多字节字符被分包截断后解码出乱码。
package filter

// Filter 有状态的字节级过滤器。
//
// 契约：
//   - Push 返回截至最近一个 \n 的已清理字节(即只交付完整行)；
//   - 未完成行保留在内部，通过 Pending() 只读访问(提示符匹配需要它)；
//   - Push 的返回值与 Pending() 的内容在下次 Push 前有效，调用方不应修改。
//
// Filter 实例非并发安全：设计为由单一读 goroutine 驱动(core.stream)。
type Filter interface {
	// Push 喂入一段原始字节，返回本次可交付的完整行(可能为空)
	Push(p []byte) (out []byte)
	// Pending 返回当前未完成行的只读视图(不含 \n；可能为空)
	Pending() []byte
	// DropPending 丢弃当前未完成行(输出拦截器命中该行后由调用方调用)
	DropPending()
	// Flush 流结束时调用：返回残余的未完成行(连同 \n 以便调用方按行处理)；
	//	未完成的转义序列按 DropIncomplete 配置丢弃或原样输出
	Flush() (out []byte)
}

// FilterFunc 无状态过滤函数适配器，等价于 v1 的 IFilter，用于简单场景的 1:1 迁移：
//
//	f := filter.FilterFunc(func(p []byte) []byte { ... })
//
// 注意：FilterFunc 不具备行编辑语义(退格/擦除作用于完整行)与跨包截断恢复能力，
// Push 返回的内容可以包含不完整行，由调用方(core.stream)负责行组装。
type FilterFunc func(p []byte) []byte

func (f FilterFunc) Push(p []byte) []byte { return f(p) }
func (FilterFunc) Pending() []byte        { return nil }
func (FilterFunc) DropPending()           {}
func (FilterFunc) Flush() []byte          { return nil }

// NewFilter 创建内置的转义序列状态机过滤器。
//
//	opts 不传时使用 DefaultOptions()；传入时以传入值为准，不做字段级合并
//	(显式传零值 Options{} 即关闭全部过滤，等价于不做字符清理的行组装器)。
func NewFilter(opts ...Options) Filter {
	if len(opts) == 0 {
		opts = []Options{DefaultOptions()}
	}
	return &fsm{opt: opts[0]}
}

// NewNoop 创建不做任何字符清理的过滤器(仅保留行组装语义)。
func NewNoop() Filter {
	return &fsm{}
}

type chainFilter struct{ first, second Filter }

// Chain 组合两个过滤器：数据先经过 first 再经过 second。
//	Pending/DropPending/Flush 同时作用于两者。
func Chain(first, second Filter) Filter {
	return &chainFilter{first: first, second: second}
}

func (c *chainFilter) Push(p []byte) []byte  { return c.second.Push(c.first.Push(p)) }
func (c *chainFilter) Pending() []byte       { return c.second.Pending() }
func (c *chainFilter) DropPending()          { c.first.DropPending(); c.second.DropPending() }
func (c *chainFilter) Flush() []byte {
	if rest := c.first.Flush(); len(rest) > 0 {
		c.second.Push(rest)
	}
	return c.second.Flush()
}
