package core

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// TestKeepAlive_DeadDetection 连续失败达到阈值时触发 OnDead
func TestKeepAlive_DeadDetection(t *testing.T) {
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, inReader) }()
	go func() { _, _ = io.Copy(io.Discard, outReader) }()
	_ = outWriter

	var deadErr atomic.Value
	var deadCount int32
	cfg := Config{
		KeepAlive: &KeepAliveConfig{
			Interval:    20 * time.Millisecond,
			MaxFailures: 2,
			Send:        func() error { return errTestSendFailed },
			OnDead:      func(err error) { atomic.AddInt32(&deadCount, 1); deadErr.Store(err) },
		},
	}
	r := NewReader(inWriter, outReader, nil, cfg)
	defer r.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&deadCount) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, atomic.LoadInt32(&deadCount) >= 1, "应触发OnDead")
	if v := deadErr.Load(); v != nil {
		assert.ErrorIs(t, v.(error), errTestSendFailed)
	}
}

// TestKeepAlive_Success 保活成功不触发 OnDead
func TestKeepAlive_Success(t *testing.T) {
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, inReader) }()
	go func() { _, _ = io.Copy(io.Discard, outReader) }()
	_ = outWriter

	var calls int32
	cfg := Config{
		KeepAlive: &KeepAliveConfig{
			Interval:    20 * time.Millisecond,
			MaxFailures: 2,
			Send:        func() error { atomic.AddInt32(&calls, 1); return nil },
			OnDead:      func(err error) { t.Error("不应触发OnDead") },
		},
	}
	r := NewReader(inWriter, outReader, nil, cfg)
	defer r.Close()

	time.Sleep(150 * time.Millisecond)
	assert.True(t, atomic.LoadInt32(&calls) >= 3, "保活应周期执行")
}

var errTestSendFailed = errors.New("send failed")
