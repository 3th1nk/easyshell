package record

import (
	"context"
	"io"
	"time"
)

// Clock 时钟接口(测试注入，避免真实等待)
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// PlayOptions 回放的可选参数(可省略)
type PlayOptions struct {
	// Speed 回放速度：0=原速(默认)；>0 为倍速(如 2=两倍速)；<0 为尽快(不等待)
	Speed float64
	// Directions 要回放的帧方向，nil 表示全部(DirOut 与 DirEvent；输入帧默认不回放)
	Directions []Direction
	// OnFrame 每帧回调(回放时调用，返回错误则中止)
	OnFrame func(Frame) error
	// Clock 时钟(nil 使用真实时钟)
	Clock Clock
}

func (opt PlayOptions) clock() Clock {
	if opt.Clock != nil {
		return opt.Clock
	}
	return realClock{}
}

func (opt PlayOptions) want(dir Direction) bool {
	if len(opt.Directions) == 0 {
		return dir != DirIn
	}
	for _, d := range opt.Directions {
		if d == dir {
			return true
		}
	}
	return false
}

// Play 按原始时序回放到 out(默认仅输出方向帧)。
//
//	回放使用绝对目标时间补偿而非逐帧 Sleep，长时间阻塞后能自动追赶。
func (p *Player) Play(ctx context.Context, out io.Writer, opts ...PlayOptions) error {
	var opt PlayOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	clock := opt.clock()
	start := clock.Now()

	for _, frame := range p.frames {
		if !opt.want(frame.Dir) {
			continue
		}
		if opt.OnFrame != nil {
			if err := opt.OnFrame(frame); err != nil {
				return err
			}
		}
		if frame.Dir != DirOut {
			continue
		}

		// 等待到目标时间(考虑倍速)，负速直接输出
		target := frame.Delta
		if opt.Speed > 0 {
			target = time.Duration(float64(target) / opt.Speed)
		}
		if opt.Speed >= 0 {
			elapsed := clock.Now().Sub(start)
			if wait := target - elapsed; wait > 0 {
				clock.Sleep(wait)
			}
		}

		if out != nil {
			if _, err := out.Write(frame.Payload); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// AsReader 返回按时序输出 DirOut 帧内容的 Reader，
//
//	可直接交给 core.NewReader 实现交互式重放(复用提示符检测与拦截器)。
func (p *Player) AsReader(opts ...PlayOptions) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		_ = p.Play(context.Background(), pw, opts...)
		_ = pw.Close()
	}()
	return pr
}
