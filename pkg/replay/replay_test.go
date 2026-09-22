package replay

import (
	"context"
	"github.com/3th1nk/easyshell/core"
	"github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestNewReplay(t *testing.T) {
	// 测试数据由真实设备测试(TestSshShell_NetDevice_H3C)本地录制生成，已被.gitignore忽略；
	// 不存在时(如CI环境)跳过
	if _, err := os.Stat("./testdata/WorkSW03_ssh.txt"); err != nil {
		t.Skip("testdata not found, run TestSshShell_NetDevice_H3C to generate it")
	}

	player := NewReplay("./testdata/WorkSW03_ssh.txt", &Config{
		Config: core.Config{
			PromptRegex: []*regexp.Regexp{
				regexp.MustCompile(`[\s\S]*[$#%>\]:]+\s*$`),
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
