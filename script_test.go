package easyshell

import (
	"context"
	"errors"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// TestMockRunScript 离线验证多命令脚本执行
func TestMockRunScript(t *testing.T) {
	srv := testsrv.NewSshServer(t)
	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	// 全部成功
	var executed []string
	err = RunScript(context.Background(), s, func(cmd string, lines []string) {
		executed = append(executed, cmd)
	}, "cmd1", "cmd2", "cmd3")
	assert.NoError(t, err)
	assert.Equal(t, []string{"cmd1", "cmd2", "cmd3"}, executed)

	// context 取消 → ScriptError
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = RunScript(ctx, s, nil, "cmd")
	assert.Error(t, err)
	var se *ScriptError
	assert.True(t, errors.As(err, &se))
}

// TestMockRunScript_DeviceError 设备错误命中时中止脚本并定位命令
func TestMockRunScript_DeviceError(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	// 自定义会话：对 "bad" 命令返回设备解析错误
	srv.Session = func(ss *testsrv.SshSession) {
		write := func(str string) { _, _ = ss.Stdout.Write([]byte(str)) }
		write(srv.Prompt)
		buf := make([]byte, 4096)
		for {
			if _, err := ss.Stdin.Read(buf); err != nil {
				return
			}
			cmd := string(buf)
			if len(cmd) >= 3 && cmd[:3] == "bad" {
				write("% Unrecognized command found at '^' position.\n" + srv.Prompt)
				continue
			}
			write("out:" + trimCmd(cmd) + "\n" + srv.Prompt)
		}
	}

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	var executed []string
	err = RunScript(context.Background(), s, func(cmd string, lines []string) {
		executed = append(executed, cmd)
	}, "cmd1", "bad command", "cmd3")
	assert.Error(t, err)

	var se *ScriptError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, "bad command", se.Cmd)
	assert.Equal(t, 1, se.Index)
	// cmd1 已执行；bad 的错误输出行在检测前已流式交付(流式语义)，cmd3 不再执行
	assert.Equal(t, []string{"cmd1", "bad command"}, executed)

	var de *core.DeviceError
	assert.True(t, errors.As(err, &de), "ScriptError 应解包出 DeviceError, got=%v", err)
}

func trimCmd(s string) string {
	out := make([]byte, 0, len(s))
	for _, b := range []byte(s) {
		if b != '\r' && b != '\n' {
			out = append(out, b)
		}
	}
	return string(out)
}
