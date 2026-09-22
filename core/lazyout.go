package core

import (
	"sync"
	"time"
)

// lazyOut 延迟输出聚合：按时间间隔或累计大小批量触发 onOut，削峰高频输出。
//
// 非 goroutine 安全，仅在持有 Reader 读锁的上下文中使用；ticker 回调通过 mu 保护。
type lazyOut struct {
	onOut    func([]string)
	size     int
	lineSize int
	lines    []string
	mu       sync.Mutex
	ticker   *time.Ticker
	nextTick time.Time
	interval time.Duration
}

func newLazyOut(interval time.Duration, size int) *lazyOut {
	return &lazyOut{interval: interval, size: size}
}

// Stop 停止定时器并冲刷剩余输出
func (l *lazyOut) Stop() {
	if l.ticker != nil {
		l.ticker.Stop()
		l.ticker = nil
	}
	l.Out()
}

// Out 立即冲刷缓冲的输出
func (l *lazyOut) Out() {
	l.mu.Lock()
	onOut := l.onOut
	lines := l.lines
	l.lines, l.lineSize = nil, 0
	l.mu.Unlock()

	if onOut != nil && len(lines) != 0 {
		onOut(lines)
	}
}

// SetOut 设置输出回调并启动定时器
func (l *lazyOut) SetOut(f func([]string)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.onOut = f
	if l.ticker == nil && l.interval > 0 {
		d := l.interval / 5
		if d < 10*time.Millisecond {
			d = 10 * time.Millisecond
		}
		l.ticker = time.NewTicker(d)
		go func(ticker *time.Ticker) {
			for range ticker.C {
				var lines []string
				l.mu.Lock()
				onOut := l.onOut
				now := time.Now()
				if len(l.lines) != 0 && !now.Before(l.nextTick) {
					lines = l.lines
					l.lines, l.lineSize = nil, 0
					l.nextTick = now.Add(l.interval)
				}
				l.mu.Unlock()
				if onOut != nil && lines != nil {
					onOut(lines)
				}
			}
		}(l.ticker)
		l.nextTick = time.Now().Add(l.interval)
	}
}

// Add 追加输出，达到累计大小阈值时立即触发
func (l *lazyOut) Add(lines []string) {
	l.mu.Lock()
	onOut := l.onOut
	if onOut == nil {
		l.mu.Unlock()
		return
	}

	var outLines []string
	l.lines = append(l.lines, lines...)
	for _, s := range lines {
		l.lineSize += len(s)
	}
	if l.size > 0 && l.lineSize >= l.size {
		outLines = l.lines
		l.lines, l.lineSize = nil, 0
		if l.interval > 0 {
			l.nextTick = time.Now().Add(l.interval)
		}
	}
	l.mu.Unlock()

	if outLines != nil {
		onOut(outLines)
	}
}
