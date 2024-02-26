package lineReader

import (
	"bytes"
	"github.com/3th1nk/easygo/charset"
	"github.com/3th1nk/easyshell/pkg/filter"
	"io"
	"strings"
	"sync"
)

func New(r io.Reader, opts ...Option) *LineReader {
	if r != nil {
		obj := &LineReader{
			r:      r,
			filter: filter.DefaultFilter,
			lines:  make([]string, 0, 4),
		}
		for _, opt := range opts {
			opt(obj)
		}
		go obj.read()
		return obj
	}
	return nil
}

type Option func(*LineReader)

func WithDecoder(decoder func(b []byte) ([]byte, error)) Option {
	return func(reader *LineReader) {
		reader.decoder = decoder
	}
}

func WithFilter(filter func(b []byte) []byte) Option {
	return func(reader *LineReader) {
		reader.filter = filter
	}
}

func WithRawOut(w io.Writer) Option {
	return func(reader *LineReader) {
		reader.rawOut = w
	}
}

type LineReader struct {
	r               io.Reader                      //
	decoder         func(b []byte) ([]byte, error) // 二进制解码函数
	filter          func(b []byte) []byte          // 字符过滤器
	lines           []string                       // 缓冲区中已经读取到的行
	remaining       string                         // 缓冲区中最后一个换行符后面的部分
	remainingOffset int                            // 缓冲区中最后一个换行符后面的部分的长度（在缓冲区中的原始长度）
	mu              sync.Mutex                     //
	err             error                          //
	rawOut          io.Writer                      // 原始数据输出
}

func (lr *LineReader) read() {
	// 缓冲区、缓冲区写入位置
	buf, offset := make([]byte, 1024), 0
	for {
		n, err := lr.r.Read(buf[offset:])
		if err != nil {
			lr.err = err
			return
		}
		size := offset + n
		if lr.rawOut != nil {
			_, _ = lr.rawOut.Write(buf[offset:size])
		}

		lr.mu.Lock()

		// 获取有效的缓冲区内容（并移除掉已经丢弃的 remaining）
		var realBuf []byte
		if lr.remaining == "" && lr.remainingOffset != 0 {
			// 如果 remaining 为空、但 remainingOffset 不为 0 ，表示 remaining 在 popLine 函数中已经被丢弃了
			realBuf = lr.doFilter(buf[lr.remainingOffset:size])
			lr.remainingOffset = 0
		} else {
			realBuf = lr.doFilter(buf[:size])
		}

		// 从缓冲区最后一个换行符的位置，将缓冲区拆分为两部分
		//   如果有换行符，则缓冲区中只留下最后一个换行符后面的内容；
		//   如果没有换行符，缓冲区保留（同时要判断缓冲区自动扩容）。
		if i := bytes.LastIndexByte(realBuf, '\n'); i >= 0 {
			if linesStr := lr.decode(realBuf[:i]); linesStr != "" {
				arr := strings.Split(linesStr, "\n")
				lr.lines = append(lr.lines, arr...)
			}

			// remaining
			lr.remaining = lr.decode(realBuf[i+1:])
			copy(buf, realBuf[i+1:])
			offset = len(realBuf[i+1:])

		} else {
			// 自动扩容
			if size == len(buf) {
				buf = make([]byte, size*2)
			}

			// remaining
			lr.remaining = lr.decode(realBuf)
			copy(buf, realBuf)
			offset = len(realBuf)
		}
		lr.remainingOffset = offset

		lr.mu.Unlock()
	}
}

func (lr *LineReader) doFilter(s []byte) []byte {
	if lr.filter != nil {
		return lr.filter(s)
	}
	return s
}

func (lr *LineReader) decode(s []byte) string {
	if len(s) == 0 {
		return ""
	}

	if lr.decoder != nil {
		if s2, err := lr.decoder(s); err == nil {
			return string(s2)
		}
		return string(s)
	}
	// 如果没有指定自定义的解码方法，则默认尝试转换UTF8
	return charset.ToUTF8(string(s))
}

func (lr *LineReader) PopLines(f func(lines []string, remaining string) (dropRemaining bool)) (popped int, err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if len(lr.lines) == 0 && lr.remaining == "" {
		return 0, nil
	}

	droppedLines, droppedRemaining := len(lr.lines), 0
	if f(lr.lines, lr.remaining) {
		lr.remaining = ""
		droppedRemaining = 1
	}

	if droppedLines != 0 {
		lr.lines = make([]string, 0, 4)
	}

	return droppedLines + droppedRemaining, lr.err
}
