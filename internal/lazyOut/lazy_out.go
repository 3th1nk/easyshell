package lazyOut

import (
	"github.com/3th1nk/easygo/util/timeUtil"
	"sync"
	"time"
)

func New(interval time.Duration, size int) *LazyOut {
	return &LazyOut{interval: interval, size: size}
}

type LazyOut struct {
	onOut    func(lines []string)
	size     int
	lineSize int
	lines    []string
	mu       sync.Mutex
	ticker   *timeUtil.Ticker
	nextTick time.Time
	interval time.Duration
}

func (l *LazyOut) Stop() {
	if l.ticker != nil {
		l.ticker.Stop(0)
	}
	l.Out()
}

func (l *LazyOut) Out() {
	l.mu.Lock()
	onOut := l.onOut
	lines := l.lines
	l.lines, l.lineSize = nil, 0
	l.mu.Unlock()

	if onOut != nil && len(lines) != 0 {
		onOut(lines)
	}
}

func (l *LazyOut) SetOut(f func(lines []string)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.onOut = f

	if l.ticker == nil && l.interval > 0 {
		d := l.interval / 5
		if d < 10*time.Millisecond {
			d = 10 * time.Millisecond
		}
		l.ticker = timeUtil.NewTicker(d, d, func(t time.Time) {
			var lines []string
			l.mu.Lock()
			onOut := l.onOut
			if len(l.lines) != 0 && !t.Before(l.nextTick) {
				lines = l.lines
				l.lines, l.lineSize = nil, 0
				l.nextTick = t.Add(l.interval)
			}
			l.mu.Unlock()
			if onOut != nil && lines != nil {
				onOut(lines)
			}
		})
		l.nextTick = time.Now().Add(l.interval)
	}
}

func (l *LazyOut) Add(lines []string) {
	if l.onOut == nil {
		return
	}

	l.mu.Lock()
	onOut := l.onOut
	var outLines []string
	l.lines = append(l.lines, lines...)
	for _, s := range lines {
		l.lineSize += len(s)
	}
	if l.size > 0 && l.lineSize >= l.size {
		// fmt.Println(fmt.Sprintf("out by size: %v, %v", l.n, l.size))
		outLines = l.lines
		l.lines, l.lineSize = nil, 0
		if l.interval > 0 {
			l.nextTick = time.Now().Add(l.interval)
		}
	}
	l.mu.Unlock()

	if onOut != nil && outLines != nil {
		onOut(outLines)
	}
}
