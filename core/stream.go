package core

import (
	"bytes"
	"github.com/3th1nk/easygo/charset"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/v2/filter"
	"io"
	"sync"
)

// newFilterChain 构建过滤管道：自定义 Filter(可选) 串联在内置管道之前。
//
//	每个 stream 使用独立的内置管道实例，避免状态串扰
func newFilterChain(cfg Config) filter.Filter {
	var opts filter.Options
	if cfg.FilterOptions != nil {
		opts = *cfg.FilterOptions
	} else {
		opts = filter.DefaultOptions()
	}
	builtin := filter.NewFilter(opts)
	if isNil(cfg.Filter) {
		return builtin
	}
	return filter.Chain(cfg.Filter, builtin)
}

// stream 单 goroutine 驱动的输出流：读取原始字节 → 录制 → 过滤 → 拆行 → 逐行解码。
//
// 只解码完整行：不完整的尾行由过滤器保持(Pending)，直到遇到 \n 才解码，
//
//	因此多字节字符跨网络分包不会被截断(前提：编码的多字节字符不含 0x0A，见 Config.Decoder 契约)。
type stream struct {
	src     io.Reader
	rawOut  io.Writer
	flt     filter.Filter
	decoder func(b []byte) ([]byte, error)
	strip   bool // 是否剔除解码后的 U+FFFD 替换字符

	fltMu  sync.Mutex    // 保护 flt：Push 在读 goroutine，DropPending 可能在 Reader goroutine
	mu     sync.Mutex    // 保护以下字段
	lines  []string      // 已完成行(待消费)
	remain string        // 未完成行(Pending)的解码缓存
	err    error         // 读错误
	notify chan struct{} // 数据通知(容量1，合并通知)
}

func newStream(src io.Reader, cfg Config) *stream {
	s := &stream{
		src:     src,
		rawOut:  cfg.RawOut,
		flt:     newFilterChain(cfg),
		decoder: cfg.Decoder,
		strip:   cfg.FilterOptions == nil || cfg.FilterOptions.ReplaceChar,
		notify:  make(chan struct{}, 1),
	}
	if cfg.FilterOptions != nil {
		s.strip = cfg.FilterOptions.ReplaceChar
	}
	go s.run()
	return s
}

// Notify 返回数据通知 channel，读取到新数据时收到信号，用于事件驱动读取
func (s *stream) Notify() <-chan struct{} { return s.notify }

// PopLines 消费缓冲的行；回调返回 true 表示丢弃当前未完成行。
//
//	数据耗尽时返回流错误(如 EOF)，让上层感知流结束(ReadAll 依赖此退出)
func (s *stream) PopLines(f func(lines []string, remaining string) (dropRemaining bool)) (popped int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.lines) == 0 && s.remain == "" {
		return 0, s.err
	}

	droppedRemaining := 0
	if f(s.lines, s.remain) {
		s.remain = ""
		droppedRemaining = 1
	}

	droppedLines := len(s.lines)
	s.lines = s.lines[:0]
	return droppedLines + droppedRemaining, s.err
}

// dropPending 丢弃当前未完成行(输出拦截器命中该行后调用，避免其后续内容残留)
func (s *stream) dropPending() {
	s.mu.Lock()
	s.remain = ""
	s.mu.Unlock()

	s.fltMu.Lock()
	s.flt.DropPending()
	s.fltMu.Unlock()
}

func (s *stream) run() {
	buf := make([]byte, 4096)
	for {
		n, err := s.src.Read(buf)
		if n > 0 {
			// 注意：Read 可能同时返回 n>0 与 err!=nil(如连接关闭前的最后一读)，必须先处理数据再处理错误
			s.ingest(buf[:n])
		}
		if err != nil {
			s.mu.Lock()
			s.err = err
			s.mu.Unlock()
			// 流结束也通知一次，让 ReadAll 及时退出而不是等下一个确认周期
			select {
			case s.notify <- struct{}{}:
			default:
			}
			return
		}
	}
}

func (s *stream) ingest(chunk []byte) {
	if !isNil(s.rawOut) {
		if _, err := s.rawOut.Write(chunk); err != nil {
			util.PrintErrln("write raw out failed: %s", err)
		}
	}

	s.fltMu.Lock()
	out := s.flt.Push(chunk)
	pending := append([]byte(nil), s.flt.Pending()...)
	s.fltMu.Unlock()

	var lines []string
	if len(out) > 0 {
		lines = make([]string, 0, bytes.Count(out, []byte{'\n'}))
		for _, line := range bytes.Split(out[:len(out)-1], []byte{'\n'}) {
			lines = append(lines, s.decodeLine(line))
		}
	}

	s.mu.Lock()
	s.lines = append(s.lines, lines...)
	s.remain = s.decodeLine(pending)
	s.mu.Unlock()

	// 通知有新数据(非阻塞，多次通知合并)
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// decodeLine 解码一行完整内容(仅在行边界上解码，保证多字节字符完整)
func (s *stream) decodeLine(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var str string
	if s.decoder != nil {
		if d, err := s.decoder(b); err == nil {
			str = string(d)
		} else {
			str = string(b)
		}
	} else {
		str = charset.ToUTF8(string(b))
	}
	if s.strip {
		str = filter.StripReplaceChars(str)
	}
	return str
}
