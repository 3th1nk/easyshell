package lineReader

import (
	"bytes"
	"github.com/3th1nk/easygo/charset"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/filter"
	"io"
	"strings"
	"sync"
)

func New(r io.Reader, opts ...Option) *LineReader {
	if misc.IsNil(r) {
		return nil
	}

	obj := &LineReader{
		r:      r,
		filter: filter.NewDefaultFilter(),
		lines:  make([]string, 0, 4),
	}
	for _, opt := range opts {
		opt(obj)
	}
	go obj.read()
	return obj
}

type Option func(*LineReader)

func WithDecoder(decoder func(b []byte) ([]byte, error)) Option {
	return func(reader *LineReader) {
		reader.decoder = decoder
	}
}

func WithFilter(filter filter.IFilter) Option {
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
	rawOut          io.Writer                      // 原始数据输出
	filter          filter.IFilter                 // 字符过滤器
	decoder         func(b []byte) ([]byte, error) // 二进制解码函数
	lines           []string                       // 缓冲区中已经读取到的行
	remaining       string                         // 缓冲区中最后一个换行符后面的部分
	remainingOffset int                            // 缓冲区中最后一个换行符后面的部分的长度（在缓冲区中的原始长度）
	mu              sync.Mutex                     //
	err             error                          //
}

func (lr *LineReader) read() {
	// 缓冲区、缓冲区写入位置
	// ！！！Read()最多读取一个缓冲区大小的内容，有可能读取不完整，导致字符过滤时可能出现问题
	//	这里基于经验设置大小为4096，只是尽量降低发生概率，无法完全避免
	buf, offset := make([]byte, 4096), 0
	for {
		n, err := lr.r.Read(buf[offset:])
		if err != nil {
			lr.err = err
			return
		}
		size := offset + n
		if err = lr.doRawOut(buf[offset:size]); err != nil {
			util.PrintErrln("write raw out failed: %s", err)
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

func (lr *LineReader) doRawOut(s []byte) error {
	if !misc.IsNil(lr.rawOut) {
		if _, err := lr.rawOut.Write(s); err != nil {
			return err
		}
	}
	return nil
}

func (lr *LineReader) doFilter(s []byte) []byte {
	if !misc.IsNil(lr.filter) {
		return lr.filter.Do(s)
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

	var droppedRemaining int
	if f(lr.lines, lr.remaining) {
		lr.remaining = ""
		droppedRemaining = 1
	}

	droppedLines := len(lr.lines)
	if droppedLines > 0 {
		lr.lines = lr.lines[:0]
	}

	return droppedLines + droppedRemaining, lr.err
}
