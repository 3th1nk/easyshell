package easyshell

import (
	"context"
	"errors"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMockSshShell_Run 离线(mock)验证 SSH Shell 的登录与命令交互
func TestMockSshShell_Run(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	assert.True(t, hasLine(s.HeadLine(), "Welcome"), "登录横幅: %v", s.HeadLine())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "show version", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.Equal(t, []string{"out:show version"}, lines)
	assert.Equal(t, srv.Prompt, s.Prompt())

	// 连续执行第二条命令
	lines = nil
	assert.NoError(t, s.Run(ctx, "display clock", func(arr []string) {
		lines = append(lines, arr...)
	}))
	assert.Equal(t, []string{"out:display clock"}, lines)
}

// TestMockSshShell_More 离线验证 More 翻页拦截
func TestMockSshShell_More(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	// 自定义会话：分页输出，等待空格应答后继续
	srv.Session = func(ss *testsrv.SshSession) {
		write := func(str string) {
			_, _ = ss.Stdout.Write([]byte(str))
		}
		write(srv.Prompt)
		for {
			buf := make([]byte, 4096)
			n, err := ss.Stdin.Read(buf)
			if err != nil {
				return
			}
			cmd := string(buf[:n])
			if n > 0 && cmd[0] == ' ' {
				continue // 翻页应答(由下方分页逻辑消耗)
			}
			write("page1\n")
			write("---- More ----")
			// 等待空格应答
			for {
				if _, err := ss.Stdin.Read(buf); err != nil {
					return
				}
				break
			}
			write("\npage2\n")
			write(srv.Prompt)
			_ = cmd
		}
	}

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, s.Run(ctx, "display all", func(arr []string) {
		lines = append(lines, arr...)
	}, interceptor.More()))
	assert.True(t, hasLine(lines, "page1"))
	assert.True(t, hasLine(lines, "page2"))
	assert.False(t, hasLine(lines, "More"), "More提示不应出现在输出中: %v", lines)
}

// TestMockSshShell_ExitCode 离线验证退出码获取
func TestMockSshShell_ExitCode(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// mock 会话回显 "out:echo $?"，退出码解析不到数字时返回错误(验证解析逻辑本身)
	_, err = ExitCode(ctx, s)
	assert.Error(t, err)
}

// TestMockSshShell_AuthError 验证认证失败错误分类
func TestMockSshShell_AuthError(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	_, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: "wrong", Timeout: 3 * time.Second},
	})
	assert.Error(t, err)
	assert.True(t, core.IsAuth(err), "got=%v", err)
}

// TestMockSshShell_Sftp 离线验证 SFTP 上传(原子写)/下载/递归删除
func TestMockSshShell_Sftp(t *testing.T) {
	srv := testsrv.NewSshServer(t)
	rootDir := t.TempDir()
	srv.RootDir = rootDir

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	// 本地准备上传文件与目录
	localDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(localDir, "sub"), 0755)
	content1 := []byte("hello sftp")
	content2 := []byte("nested file")
	assert.NoError(t, os.WriteFile(filepath.Join(localDir, "a.txt"), content1, 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(localDir, "sub", "b.txt"), content2, 0644))

	// 上传目录
	assert.NoError(t, s.SftpUpload(localDir, "upload"))

	// 校验远端内容
	cli, err := s.SftpClient()
	assert.NoError(t, err)
	got, err := readRemoteFile(cli, "upload/a.txt")
	assert.NoError(t, err)
	assert.Equal(t, content1, got)
	got, err = readRemoteFile(cli, "upload/sub/b.txt")
	assert.NoError(t, err)
	assert.Equal(t, content2, got)

	// 已存在且不覆盖 → ErrExist；覆盖 → 成功且内容更新
	err = s.SftpUpload(filepath.Join(localDir, "a.txt"), "upload/a.txt")
	assert.True(t, errors.Is(err, os.ErrExist), "got=%v", err)

	content3 := []byte("updated")
	assert.NoError(t, os.WriteFile(filepath.Join(localDir, "a.txt"), content3, 0644))
	assert.NoError(t, s.SftpUpload(filepath.Join(localDir, "a.txt"), "upload/a.txt", SftpOptions{Force: true}))
	got, err = readRemoteFile(cli, "upload/a.txt")
	assert.NoError(t, err)
	assert.Equal(t, content3, got)

	// 下载
	downDir := t.TempDir()
	assert.NoError(t, s.SftpDown("upload/a.txt", filepath.Join(downDir, "a.txt")))
	got, err = os.ReadFile(filepath.Join(downDir, "a.txt"))
	assert.NoError(t, err)
	assert.Equal(t, content3, got)

	// 递归删除
	assert.NoError(t, s.SftpRemove("upload"))
	_, err = cli.Stat("upload")
	assert.Error(t, err)
}

// readRemoteFile 读取远端文件内容
func readRemoteFile(cli *sftp.Client, p string) ([]byte, error) {
	f, err := cli.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func hostOf(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

func portOf(addr string) int {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			n := 0
			for _, c := range addr[i+1:] {
				n = n*10 + int(c-'0')
			}
			return n
		}
	}
	return 0
}
