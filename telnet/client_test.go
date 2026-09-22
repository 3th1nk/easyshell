package telnet

import (
	"bufio"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/stretchr/testify/assert"
	"net"
	"strings"
	"testing"
	"time"
)

// TestClient_Write 测试 Write 的 IAC 转义和 LF 转换
//
//	重点：数据中的无效 UTF-8 字节(如 GB18030 编码)不能被误判为 IAC
func TestClient_Write(t *testing.T) {
	for _, obj := range []struct {
		name          string
		unixWriteMode bool
		input         []byte
		expect        []byte
	}{
		{"iac_escape", false, []byte("abc\xffd"), []byte("abc\xff\xffd")},
		{"invalid_utf8_not_iac", false, []byte("a\xC4\xE3b\xFFc\nd"), []byte("a\xC4\xE3b\xFF\xFFc\nd")},
		{"lf_no_convert", false, []byte("a\nb"), []byte("a\nb")},
		{"lf_convert", true, []byte("a\nb"), []byte("a\r\nb")},
		{"lf_convert_and_iac_escape", true, []byte("a\nb\xff"), []byte("a\r\nb\xff\xff")},
	} {
		t.Run(obj.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			defer func() {
				_ = c1.Close()
				_ = c2.Close()
			}()
			cli := &Client{
				c:   c1,
				r:   bufio.NewReaderSize(c1, 4096),
				cfg: Config{WriteTimeout: 5 * time.Second, UnixWriteMode: obj.unixWriteMode},
			}

			var got []byte
			done := make(chan struct{})
			go func() {
				defer close(done)
				buf := make([]byte, 512)
				for len(got) < len(obj.expect) {
					n, err := c2.Read(buf)
					if err != nil {
						return
					}
					got = append(got, buf[:n]...)
				}
			}()

			n, err := cli.Write(obj.input)
			assert.NoError(t, err)
			// n 的语义：实际写入网络的字节数(LF 转换为 CR LF 会计入 2 字节)
			assert.Equal(t, len(obj.expect), n)

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("read timeout")
			}
			assert.Equal(t, obj.expect, got)
		})
	}
}

// TestNewClient_Errors 验证错误分类
func TestNewClient_Errors(t *testing.T) {
	// 连接拒绝 → dial 错误
	_, err := NewClient(Config{Addr: "127.0.0.1:1", Timeout: 2 * time.Second})
	assert.Error(t, err)
	assert.True(t, core.IsDial(err), "got=%v", err)

	// 认证失败 → auth 错误
	srv := testsrv.NewTelnetServer(t, "password")
	_, err = NewClient(Config{Addr: srv.Addr, User: "admin", Password: "wrong", Timeout: 3 * time.Second})
	assert.Error(t, err)
	assert.True(t, core.IsAuth(err), "got=%v", err)
}

// TestNewClient_MockServer 通过 mock 服务器验证两种认证模式的登录流程
func TestNewClient_MockServer(t *testing.T) {
	for _, mode := range []string{"password", "user"} {
		t.Run(mode, func(t *testing.T) {
			srv := testsrv.NewTelnetServer(t, mode)

			client, err := NewClient(Config{
				Addr:     srv.Addr,
				User:     srv.User,
				Password: srv.Password,
				Timeout:  3 * time.Second,
			})
			if !assert.NoError(t, err) {
				return
			}
			defer client.Close()

			assert.Equal(t, strings.TrimRight(srv.Welcome, "\n"), client.Welcome())
			assert.Equal(t, srv.Prompt, client.Prompt())

			// 命令交互
			_, err = client.Write([]byte("show version\n"))
			assert.NoError(t, err)
			buf := make([]byte, 128)
			n, err := client.Read(buf)
			assert.NoError(t, err)
			assert.Contains(t, string(buf[:n]), "out:show version")
		})
	}
}

// TestNewClient_NoAuth 未配置用户名密码时直接返回(设备无需认证的场景)
func TestNewClient_NoAuth(t *testing.T) {
	srv := testsrv.NewTelnetServer(t, "none")
	client, err := NewClient(Config{Addr: srv.Addr, Timeout: 3 * time.Second})
	if !assert.NoError(t, err) {
		return
	}
	defer client.Close()
	assert.Contains(t, client.Welcome(), "Welcome")
}
