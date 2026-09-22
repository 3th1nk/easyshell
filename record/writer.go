package record

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrRecordClosed 录制器已关闭
var ErrRecordClosed = errors.New("record: writer is closed")

// Options 录制的可选参数(可省略)。
//
//	注意：开启 CaptureInput 后输入方向(命令、密码、拦截器应答)都会被录制；
//	包含密码的帧会自动标记 Secret，Dump 时默认打码，但录制文件本身仍是明文，请妥善保管。
type Options struct {
	// CaptureInput 是否录制输入方向(默认不录制)
	CaptureInput bool
}

// Writer 录制器：Write 写入输出方向帧(可直接赋给 core.Config.RawOut)，
//
//	Input() 返回输入方向捕获 writer(可赋给 core.Config.RawIn)，Event 记录事件。
type Writer struct {
	mu        sync.Mutex
	w         io.WriteSeeker
	startedAt time.Time
	closed    bool
	input     *inputWriter
}

// NewWriter 创建录制器(写入到 w，需支持 Seek 以便 Close 时回写时长)
func NewWriter(w io.WriteSeeker, meta Meta, opts ...Options) (*Writer, error) {
	var flags uint32
	if len(opts) > 0 && opts[0].CaptureInput {
		flags |= flagHasInput
	}
	if err := writeHeader(w, meta, flags); err != nil {
		return nil, err
	}
	writer := &Writer{w: w, startedAt: meta.StartedAt}
	if writer.startedAt.IsZero() {
		writer.startedAt = time.Now()
	}
	if flags&flagHasInput != 0 {
		writer.input = &inputWriter{w: writer}
	}
	return writer, nil
}

// NewFileWriter 创建录制器并写入文件(自动创建目录)。
func NewFileWriter(path string, meta Meta, opts ...Options) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return NewWriter(f, meta, opts...)
}

// Write 实现 io.Writer，记录输出方向帧
func (w *Writer) Write(p []byte) (int, error) {
	if err := w.frame(DirOut, false, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Input 返回输入方向捕获 writer；未开启 CaptureInput 时返回 Discard(静默丢弃，便于无条件接入)
func (w *Writer) Input() io.Writer {
	if w.input == nil {
		return io.Discard
	}
	return w.input
}

// Event 记录一条事件(如连接关闭原因)
func (w *Writer) Event(msg string) error {
	return w.frame(DirEvent, false, []byte(msg))
}

func (w *Writer) frame(dir Direction, secret bool, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return ErrRecordClosed
	}
	delta := time.Since(w.startedAt).Microseconds()
	return writeFrame(w.w, delta, dir, boolToFlags(secret), payload)
}

// Close 回写录制时长并落盘；之后继续 Write 返回 ErrClosed
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true

	duration := time.Since(w.startedAt).Microseconds()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(duration))
	if _, err := w.w.Seek(0x18, io.SeekStart); err != nil {
		return err
	}
	if _, err := w.w.Write(buf[:]); err != nil {
		return err
	}
	if f, ok := w.w.(*os.File); ok {
		return f.Sync()
	}
	return nil
}

// inputWriter 输入方向捕获 writer
type inputWriter struct {
	w *Writer
}

func (i *inputWriter) Write(p []byte) (int, error) {
	if err := i.w.frame(DirIn, false, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func boolToFlags(secret bool) byte {
	if secret {
		return frameSecret
	}
	return 0
}

// durationAt 读取文件头中的录制时长
func durationAt(r io.ReadSeeker) (time.Duration, error) {
	var buf [8]byte
	if _, err := r.Seek(0x18, io.SeekStart); err != nil {
		return 0, err
	}
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	_, err := r.Seek(0, io.SeekStart)
	return time.Duration(int64(binary.LittleEndian.Uint64(buf[:]))), err
}
