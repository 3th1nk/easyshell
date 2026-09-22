package record

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

// TestWriter_HeaderGolden 锁死文件头布局(64字节逐字段)
func TestWriter_HeaderGolden(t *testing.T) {
	startedAt := time.Unix(1700000000, 0).UTC()
	sk := &realSeeker{}
	w, err := NewWriter(sk, Meta{
		Host:      "192.0.2.1",
		Port:      22,
		Protocol:  "ssh",
		User:      "admin",
		Comment:   "test",
		StartedAt: startedAt,
	})
	assert.NoError(t, err)
	assert.NoError(t, w.Close())

	head := sk.Bytes()[:64]
	assert.Equal(t, "ESHREC", string(head[0x00:0x06]))
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(head[0x06:]))
	assert.Equal(t, uint32(128), binary.LittleEndian.Uint32(head[0x08:]))
	assert.Equal(t, uint32(0), binary.LittleEndian.Uint32(head[0x0C:]), "flags应为0(未开输入录制)")
	assert.Equal(t, int64(1700000000000000), int64(binary.LittleEndian.Uint64(head[0x10:])))
	// TLV: host/port/protocol/user/comment
	assert.Equal(t, byte(1), head[0x20]) // host type
}

// realSeeker 支持 Seek 的可读写缓冲(定位写)
type realSeeker struct {
	data []byte
	pos  int64
}

func (s *realSeeker) Write(p []byte) (int, error) {
	if s.pos+int64(len(p)) > int64(len(s.data)) {
		s.data = append(s.data, make([]byte, s.pos+int64(len(p))-int64(len(s.data)))...)
	}
	copy(s.data[s.pos:], p)
	s.pos += int64(len(p))
	return len(p), nil
}

func (s *realSeeker) Bytes() []byte { return s.data }

func (s *realSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.pos = offset
	case io.SeekCurrent:
		s.pos += offset
	case io.SeekEnd:
		s.pos = int64(len(s.data)) + offset
	}
	return s.pos, nil
}

// TestRoundTrip 写读回环：帧内容与顺序完全一致
func TestRoundTrip(t *testing.T) {
	sk := &realSeeker{}
	w, err := NewWriter(sk, Meta{Host: "h", Protocol: "ssh", User: "u"}, Options{CaptureInput: true})
	assert.NoError(t, err)

	_, _ = w.Write([]byte("banner\n"))
	_, _ = w.Write([]byte("prompt# "))
	_, _ = w.Input().Write([]byte("show version\n"))
	assert.NoError(t, w.Event("closed"))
	assert.NoError(t, w.Close())

	// Close 后继续写应报错
	_, err = w.Write([]byte("x"))
	assert.Error(t, err)

	p, err := OpenBytes(sk.Bytes())
	assert.NoError(t, err)
	defer p.Close()

	frames := p.Frames()
	assert.Len(t, frames, 4)
	assert.Equal(t, DirOut, frames[0].Dir)
	assert.Equal(t, []byte("banner\n"), frames[0].Payload)
	assert.Equal(t, DirOut, frames[1].Dir)
	assert.Equal(t, DirIn, frames[2].Dir)
	assert.Equal(t, []byte("show version\n"), frames[2].Payload)
	assert.Equal(t, DirEvent, frames[3].Dir)
	assert.Equal(t, "closed", string(frames[3].Payload))

	// 单调递增
	for i := 1; i < len(frames); i++ {
		assert.GreaterOrEqual(t, frames[i].Delta, frames[i-1].Delta)
	}
}

// TestOpen_BadMagic 损坏文件应返回错误
func TestOpen_BadMagic(t *testing.T) {
	_, err := OpenBytes([]byte("not an easyshell recording"))
	assert.Error(t, err)
}

// TestPlay_FastSpeed 回放：负速(尽快)输出全部输出方向帧
func TestPlay_FastSpeed(t *testing.T) {
	sk := &realSeeker{}
	w, err := NewWriter(sk, Meta{Protocol: "ssh"})
	assert.NoError(t, err)
	_, _ = w.Write([]byte("a"))
	time.Sleep(5 * time.Millisecond)
	_, _ = w.Write([]byte("b"))
	time.Sleep(5 * time.Millisecond)
	_, _ = w.Write([]byte("c"))
	assert.NoError(t, w.Close())

	p, err := OpenBytes(sk.Bytes())
	assert.NoError(t, err)
	defer p.Close()

	var got bytes.Buffer
	start := time.Now()
	assert.NoError(t, p.Play(context.Background(), &got, PlayOptions{Speed: -1}))
	assert.Less(t, time.Since(start), time.Second, "负速回放不应真实等待")
	assert.Equal(t, "abc", got.String())

	// OnFrame 回调
	count := 0
	assert.NoError(t, p.Play(context.Background(), nil, PlayOptions{Speed: -1, OnFrame: func(f Frame) error {
		count++
		return nil
	}}))
	assert.Equal(t, 3, count)
}

// fakeClock 测试注入时钟
type fakeClock struct {
	now   time.Time
	slept time.Duration
}

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.slept += d; c.now = c.now.Add(d) }

// TestPlay_OriginalSpeed 原速回放按 Delta 等待(注入时钟，零真实等待)
func TestPlay_OriginalSpeed(t *testing.T) {
	sk := &realSeeker{}
	w, err := NewWriter(sk, Meta{Protocol: "ssh"})
	assert.NoError(t, err)
	_, _ = w.Write([]byte("1"))
	time.Sleep(20 * time.Millisecond)
	_, _ = w.Write([]byte("2"))
	assert.NoError(t, w.Close())

	p, err := OpenBytes(sk.Bytes())
	assert.NoError(t, err)
	defer p.Close()

	clock := &fakeClock{now: time.Now()}
	var got bytes.Buffer
	_ = p.Play(context.Background(), &got, PlayOptions{Clock: clock})
	assert.Equal(t, "12", got.String())
	assert.Greater(t, clock.slept, time.Duration(0), "原速回放应产生等待")
	assert.Less(t, clock.slept, time.Second, "不应真实长等待")
}
