package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

// TestMockCmdShell 离线验证本地命令 Shell(go 命令跨平台可用)
func TestMockCmdShell(t *testing.T) {
	s, err := NewCmdShell(context.Background(), CmdConfig{Command: "go version"})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, s.ReadAll(ctx, func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "go version"), "got=%v", lines)
}

// TestMockCmdShell_Empty 空命令返回错误(修复v1的panic)
func TestMockCmdShell_Empty(t *testing.T) {
	_, err := NewCmdShell(context.Background(), CmdConfig{Command: ""})
	assert.Error(t, err)
	assert.True(t, errorsIs(err, core.ErrEmptyCommand))
}

// TestMockCmdShell_Run 交互式本地 shell 场景由平台差异较大，这里只验证写入接口
func TestMockCmdShell_Run(t *testing.T) {
	s, err := NewCmdShell(context.Background(), CmdConfig{Command: "go version"})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assert.NoError(t, s.Write("go env GOVERSION"))

	// 等待进程退出(ReadAll 到 EOF)
	var lines []string
	assert.NoError(t, s.ReadAll(ctx, func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, len(lines) > 0)
}

func errorsIs(err error, target error) bool {
	return err != nil && strings.Contains(err.Error(), target.Error())
}
