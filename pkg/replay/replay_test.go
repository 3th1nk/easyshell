package replay

import (
	"context"
	"github.com/3th1nk/easyshell/core"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
	"time"
)

func TestNewReplay(t *testing.T) {
	player := NewReplay("./testdata/WorkSW03_ssh.txt", &Config{
		Config: core.Config{
			PromptRegex: []*regexp.Regexp{
				regexp.MustCompile(`WorkSW03[\s\S]*[$#%>\]:]+\s*$`),
			},
			AutoPrompt:      true,
			LazyOutInterval: 500 * time.Millisecond,
			LazyOutSize:     8192,
		},
	})
	assert.NotNil(t, player)
	defer player.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := player.Play(ctx); err != nil {
		t.Error(err)
	}
}
