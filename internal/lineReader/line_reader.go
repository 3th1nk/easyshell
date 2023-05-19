package lineReader

import (
	"bytes"
	"github.com/3th1nk/easygo/charset"
	"github.com/3th1nk/easyshell/internal/filter"
	"io"
	"strings"
	"sync"
)

func New(r io.Reader, opts ...Option) *LineReader {
	if r != nil {
		obj := &LineReader{
			r:      r,
			filter: filter.CtrlFilter,
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

type LineReader struct {
	r               io.Reader                      //
	decoder         func(b []byte) ([]byte, error) // 二进制解码函数
	filter          func(b []byte) []byte          // 字符过滤器
	lines           []string                       // 缓冲区中已经读取到的行
	remaining       string                         // 缓冲区中最后一个换行符后面的部分
	remainingOffset int                            // 缓冲区中最后一个换行符后面的部分的长度（在缓冲区中的原始长度）
	mu              sync.Mutex                     //
	err             error                          //
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

		lr.mu.Lock()

		// 获取有效的缓冲区内容（并移除掉已经丢弃的 remaining）
		var realBuf []byte
		if lr.remaining == "" && lr.remainingOffset != 0 {
			// 如果 remaining 为空、但 remainingOffset 不为 0 ，表示 remaining 在 popLine 函数中已经被丢弃了
			realBuf = lr.filter(buf[lr.remainingOffset:size])
			lr.remainingOffset = 0
		} else {
			realBuf = lr.filter(buf[:size])
		}

		// 从缓冲区最后一个换行符的位置，将缓冲区拆分为两部分
		//   如果有换行符，则缓冲区中只留下最后一个换行符后面的内容；
		//   如果没有换行符，缓冲区保留（同时要判断缓冲区自动扩容）。
		if i := bytes.LastIndexByte(realBuf, '\n'); i != -1 {
			// lines
			var linesStr string
			if lr.decode(&linesStr, realBuf[:i]); linesStr != "" {
				arr := strings.Split(linesStr, "\n")
				for i, s := range arr {
					if len(s) != 0 && s[len(s)-1] == '\r' {
						arr[i] = s[:len(s)-1]
					} else {
						arr[i] = s
					}
				}
				lr.lines = append(lr.lines, arr...)
			}

			// remaining
			tmp := realBuf[i+1:]
			lr.decode(&lr.remaining, tmp)
			copy(buf, tmp)
			offset = len(tmp)
		} else {
			// remaining
			lr.decode(&lr.remaining, realBuf)
			if size == len(buf) {
				buf = make([]byte, size*2)
				copy(buf, realBuf)
			}
			offset = len(realBuf)
		}
		lr.remainingOffset = offset

		lr.mu.Unlock()
	}
}

func (lr *LineReader) decode(s *string, b []byte) {
	if len(b) == 0 {
		return
	}

	if lr.decoder != nil {
		if b2, err := lr.decoder(b); err == nil {
			*s = string(b2)
			return
		}
		*s = string(b)
	} else {
		// 如果没有指定自定义的解码方法，则默认尝试转换UTF8
		*s = charset.ToUTF8(string(b))
	}
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