package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"github.com/3th1nk/easyshell/v2/internal/testutil"
	"github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"testing"
	"time"
)

// 真实设备测试(需配置 .env 或环境变量，见 test_cred_test.go 顶部说明)，离线环境自动跳过。
//
// 用例分层：TestMock*_* 为离线测试(默认执行)；TestDevice_* 为真机测试(需凭据)。

func deviceCtx(t *testing.T) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Minute)
}

// TestDevice_SshShell_Run 真机：执行命令并读取输出
func TestDevice_SshShell_Run(t *testing.T) {
	cred, ok := sshCredFromEnv("LINUX", false)
	if !ok {
		t.Skip("set EASYSHELL_TEST_LINUX to run this test")
	}
	s, err := NewSshShell(SshConfig{Credential: cred})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := deviceCtx(t)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "echo $((40+2))", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "42"))
}

// TestDevice_SshShell_More 真机：获取长配置时More翻页被自动处理
func TestDevice_SshShell_More(t *testing.T) {
	cred, ok := sshCredFromEnv("H3C", true)
	if !ok {
		t.Skip("set EASYSHELL_TEST_H3C to run this test")
	}

	var rawOut recordBuffer
	s, err := NewSshShell(SshConfig{
		Credential: cred,
		Config:     core.Config{AutoPrompt: true, RawOut: &rawOut},
		TermHeight: 24,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "display saved-configuration", func(arr []string) {
		lines = append(lines, arr...)
	}))

	assert.True(t, rawOut.contains("More"), "原始输出中应出现More翻页提示")
	assert.False(t, hasLine(lines, "---- More"), "过滤后的输出中不应残留More提示")
	lines = trimEmptyLines(lines)
	assert.Greater(t, len(lines), 100, "配置行数过少")
	assert.Equal(t, "return", lines[len(lines)-1], "配置应以return结尾")
}

// recordBuffer 收集原始输出的缓冲
type recordBuffer struct {
	b []byte
}

func (r *recordBuffer) Write(p []byte) (int, error) {
	r.b = append(r.b, p...)
	return len(p), nil
}

func (r *recordBuffer) contains(s string) bool {
	return len(r.b) > 0 && regexp.MustCompile(s).Match(r.b)
}

// TestDevice_TelnetShell_Run 真机：Telnet 登录与命令交互
func TestDevice_TelnetShell_Run(t *testing.T) {
	cred, ok := telnetCredFromEnv("CISCO")
	if !ok {
		t.Skip("set EASYSHELL_TEST_CISCO to run this test")
	}
	s, err := NewTelnetShell(TelnetConfig{Credential: cred})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := deviceCtx(t)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "show version", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "show version"))
	assert.NotEqual(t, "", s.Prompt())
}

// TestDevice_SshShell_Sftp 真机：SFTP 上传下载
func TestDevice_SshShell_Sftp(t *testing.T) {
	cred, ok := sshCredFromEnv("LINUX", false)
	if !ok {
		t.Skip("set EASYSHELL_TEST_LINUX to run this test")
	}
	s, err := NewSshShell(SshConfig{Credential: cred})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	localFile, err := os.CreateTemp(t.TempDir(), "easyshell-sftp-test")
	assert.NoError(t, err)
	_, _ = localFile.WriteString("sftp test content")
	_ = localFile.Close()

	assert.NoError(t, s.Upload(context.Background(), localFile.Name(), "/tmp/easyshell-sftp-test"))
	assert.NoError(t, s.Delete(context.Background(), "/tmp/easyshell-sftp-test"))
}

// TestDevice_SshShell_Password 真机：su root 密码交互
func TestDevice_SshShell_Password(t *testing.T) {
	cred, ok := sshCredFromEnv("LINUX", false)
	if !ok {
		t.Skip("set EASYSHELL_TEST_LINUX to run this test")
	}
	rootPwd := testutil.Getenv("LINUX_ROOT_PASSWORD")
	if rootPwd == "" {
		t.Skip("set EASYSHELL_TEST_LINUX_ROOT_PASSWORD to run this test")
	}
	s, err := NewSshShell(SshConfig{Credential: cred})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := deviceCtx(t)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "su root", func(arr []string) {
		lines = append(lines, arr...)
	}, RunOptions{Interceptors: []interceptor.Interceptor{
		interceptor.Password(core.PasswordRegex.String(), rootPwd),
	}}))
	assert.NoError(t, s.Run(ctx, "whoami", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.True(t, hasLine(lines, "root"))
}
