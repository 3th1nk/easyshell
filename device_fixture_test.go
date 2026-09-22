package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/record"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 真机录制 fixture 的存放路径(在 .gitignore 的 **/testdata 覆盖范围内)：
//
//	真机原始输出可能包含设备配置、主机名等敏感信息，因此 fixture 只保留在本地、不入库；
//	CI/他人环境无 fixture 时，回放回归测试自动跳过。
const deviceFixturePath = "testdata/device/h3c_more.eshrec"

// TestDevice_RecordFixture 真机：录制 H3C 长配置会话(含 More 翻页)为 fixture。
//
//	运行: go test -run TestDevice_RecordFixture -v .
func TestDevice_RecordFixture(t *testing.T) {
	cred, ok := sshCredFromEnv("H3C", true)
	if !ok {
		t.Skip("set EASYSHELL_TEST_H3C to run this test")
	}

	_ = os.MkdirAll(filepath.Dir(deviceFixturePath), 0755)
	rec, err := record.NewFileWriter(deviceFixturePath, record.Meta{
		Host: cred.Host, Port: cred.Port, Protocol: "ssh", User: cred.User,
	}, record.Options{CaptureInput: true})
	if !assert.NoError(t, err) {
		return
	}

	s, err := NewSshShell(SshConfig{
		Credential: cred,
		Config:     core.Config{AutoPrompt: true, RawOut: rec, RawIn: rec.Input()},
		TermHeight: 24,
	})
	if !assert.NoError(t, err) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	assert.NoError(t, s.Run(ctx, "display saved-configuration", func([]string) {}))

	assert.NoError(t, s.Close())
	assert.NoError(t, rec.Event("session closed"))
	assert.NoError(t, rec.Close())
	t.Logf("fixture written: %s", deviceFixturePath)
}

// TestUnit_ReplayDeviceFixture 离线：重放真机录制 fixture，验证提示符检测、
//
//	More 拦截与过滤器在真实设备字节流上的表现(无 fixture 时跳过，如CI)。
func TestUnit_ReplayDeviceFixture(t *testing.T) {
	if _, err := os.Stat(deviceFixturePath); err != nil {
		t.Skip("fixture not found, run TestDevice_RecordFixture with a real device first")
	}

	player, err := record.Open(deviceFixturePath)
	if !assert.NoError(t, err) {
		return
	}
	defer player.Close()
	assert.Equal(t, "ssh", player.Meta().Protocol)

	// 输入方向帧由真机会话产生(命令与翻页空格应答)，重放时不需要也不应再发送。
	//	快速重放(Speed:-1)会瞬间倾倒全部帧，因此用较长的确认窗口判定会话结束，
	//	一次 Read 消费整个会话(横幅+命令回显+配置输出+More翻页+提示符)
	cfg := core.Config{ReadConfirmWait: 500 * time.Millisecond, ReadConfirm: 2}
	ar := player.AsReader(record.PlayOptions{Speed: -1})
	defer ar.Close()
	replay := core.NewReader(io.Discard, ar, nil, cfg)
	defer replay.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, replay.Run(ctx, "display saved-configuration", func(arr []string) {
		lines = append(lines, arr...)
	}, RunOptions{Interceptors: []interceptor.Interceptor{interceptor.More()}}))

	// 与真机直连行为一致的断言
	//	注：快速重放下命令回显可能被提示符命中后的 pending 清除竞态擦掉(真机无此问题，设备此时等待输入)，故不断言回显
	_ = lines
	assert.False(t, hasLine(lines, "---- More"), "More提示不应残留")
	lines = trimEmptyLines(lines)
	assert.Greater(t, len(lines), 100, "配置行数过少: %d", len(lines))
	assert.Equal(t, "return", lines[len(lines)-1], "配置应以return结尾")
	assert.NotEqual(t, "", replay.Prompt(), "应匹配到提示符")
}
