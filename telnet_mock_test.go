package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
	"time"
)

// TestMockTelnetShell_LoginRegex 非标登录提示符场景：
//
//	内置规则匹配不到时登录失败；通过 LoginUserRegex/LoginPassRegex 自定义后可正常登录交互
func TestMockTelnetShell_LoginRegex(t *testing.T) {
	srv := testsrv.NewTelnetServer(t, "user")
	srv.UserPrompt, srv.PassPrompt = "Account:", "Secret:" // 模拟非标设备提示符

	cred := TelnetCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second}

	// 内置规则不认识非标提示符 → 登录失败
	_, err := NewTelnetShell(TelnetConfig{Credential: cred})
	assert.Error(t, err, "非标提示符内置规则应登录失败: %v", err)

	// 自定义登录正则 → 登录成功
	s, err := NewTelnetShell(TelnetConfig{
		Credential:     cred,
		LoginUserRegex: regexp.MustCompile(`(?i).*account:\s*$`),
		LoginPassRegex: regexp.MustCompile(`(?i).*secret:\s*$`),
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "show version", func(arr []string) { lines = append(lines, arr...) }))
	assert.True(t, hasLine(lines, "out:show version"), "命令输出: %v", lines)
}
