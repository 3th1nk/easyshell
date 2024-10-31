package replay

import (
	"context"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/core"
	"os"
)

type Config struct {
	core.Config
}

type Replay struct {
	cfg *Config
	cr  *core.ReadWriter
	r   *Reader
}

func NewReplay(path string, cfg *Config) *Replay {
	r := NewReader(path)
	if r == nil {
		return nil
	}

	return &Replay{
		cfg: cfg,
		cr:  core.New(os.Stdin, r, nil, cfg.Config),
		r:   r,
	}
}

func (this *Replay) Close() error {
	this.cr.Stop()
	return this.r.Close()
}

func (this *Replay) Play(ctx context.Context) error {
	return this.cr.Read(ctx, false, func(lines []string) {
		for _, line := range lines {
			util.Println(line)
		}
	})
}
