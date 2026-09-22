package easyshell

import (
	"bytes"
	"context"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/3th1nk/easyshell/v2/record"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordAndReplay 端到端：mock SSH 会话录制(RawOut/RawIn) → Dump 查看 → AsReader 交互式重放
func TestRecordAndReplay(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	// 1. 录制会话
	recBuf := &recBuffer{}
	rec, err := record.NewWriter(recBuf, record.Meta{
		Host: "127.0.0.1", Protocol: "ssh", User: "admin",
	}, record.Options{CaptureInput: true})
	if !assert.NoError(t, err) {
		return
	}

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
		Config:     Config{RawOut: rec, RawIn: rec.Input()},
	})
	if !assert.NoError(t, err) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var lines []string
	assert.NoError(t, s.Run(ctx, "show version", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.NoError(t, rec.Event("session closed"))
	assert.NoError(t, s.Close())
	assert.NoError(t, rec.Close())

	// 录制内容包含输入帧(命令)与输出帧(回显)
	player, err := record.OpenBytes(recBuf.bytes())
	if !assert.NoError(t, err) {
		return
	}
	defer player.Close()

	dirs := map[record.Direction]bool{}
	for _, f := range player.Frames() {
		dirs[f.Dir] = true
	}
	assert.True(t, dirs[record.DirOut], "应有输出帧")
	assert.True(t, dirs[record.DirIn], "应捕获输入帧(CaptureInput开启)")
	assert.True(t, dirs[record.DirEvent], "应有事件帧")

	// 2. Dump 可读转储
	var dumpBuf bytes.Buffer
	assert.NoError(t, record.Dump(bytes.NewReader(recBuf.bytes()), &dumpBuf))
	assert.Contains(t, dumpBuf.String(), "ESHREC")
	assert.Contains(t, dumpBuf.String(), "out")
	assert.Contains(t, dumpBuf.String(), "in ")

	// 3. AsReader 交互式重放：输出帧喂给 core.Reader，复用提示符检测
	replayPlayer, err := record.OpenBytes(recBuf.bytes())
	if !assert.NoError(t, err) {
		return
	}
	defer replayPlayer.Close()

	r := core.NewReader(io.Discard, replayPlayer.AsReader(record.PlayOptions{Speed: -1}), nil, core.Config{})
	var replayLines []string
	assert.NoError(t, r.ReadUntilPrompt(ctx, func(arr []string) {
		replayLines = append(replayLines, arr...)
	}))
	assert.True(t, hasLine(replayLines, "out:show version"), "重放输出: %v", replayLines)
}

// recBuffer 录制缓冲(支持 record.Writer 需要的定位写)
type recBuffer struct {
	data []byte
	pos  int64
}

func (r *recBuffer) Write(p []byte) (int, error) {
	if r.pos+int64(len(p)) > int64(len(r.data)) {
		r.data = append(r.data, make([]byte, r.pos+int64(len(p))-int64(len(r.data)))...)
	}
	copy(r.data[r.pos:], p)
	r.pos += int64(len(p))
	return len(p), nil
}

func (r *recBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.pos = offset
	case io.SeekCurrent:
		r.pos += offset
	case io.SeekEnd:
		r.pos = int64(len(r.data)) + offset
	}
	return r.pos, nil
}

func (r *recBuffer) bytes() []byte { return r.data }

// TestMockSshShell_RecordSugar 验证 Record 配置糖：填路径即可完成录制(元数据自动填充、随Close收尾)
func TestMockSshShell_RecordSugar(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	fixture := filepath.Join(t.TempDir(), "rec", "session.eshrec")
	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
		Record:     &RecordConfig{Path: fixture, CaptureInput: true, Comment: "sugar test"},
	})
	if !assert.NoError(t, err) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	assert.NoError(t, s.Run(ctx, "show version", func([]string) {}))
	assert.NoError(t, s.Close()) // Close 应自动收尾录制

	// 文件存在且内容完整(元数据自动填充 + 命令输出帧)
	player, err := record.Open(fixture)
	if !assert.NoError(t, err) {
		return
	}
	defer player.Close()
	meta := player.Meta()
	assert.Equal(t, "ssh", meta.Protocol)
	assert.Equal(t, srv.User, meta.User)
	assert.Equal(t, "sugar test", meta.Comment)
	assert.Equal(t, hostOf(srv.Addr), meta.Host)

	var hasOut bool
	for _, f := range player.Frames() {
		if f.Dir == record.DirOut && bytes.Contains(f.Payload, []byte("out:show version")) {
			hasOut = true
		}
	}
	assert.True(t, hasOut, "录制应包含命令输出帧")

	// DumpAsciinema 可直接导出
	f, _ := os.Open(fixture)
	var cast bytes.Buffer
	assert.NoError(t, record.DumpAsciinema(f, &cast))
	_ = f.Close()
}
