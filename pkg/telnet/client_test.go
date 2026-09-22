package telnet

import (
	"bufio"
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/testutil"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

var (
	client *Client
)

func TestMain(m *testing.M) {
	// 设备凭据从环境变量注入(格式: [user:password@]host[:port])，未设置时跳过依赖真实设备的用例
	cred := testutil.FromEnv("CISCO")
	if cred == nil {
		fmt.Println("skip device tests: set EASYSHELL_TEST_CISCO to run")
		client = nil
		m.Run()
		return
	}

	var err error
	client, err = NewClient(&ClientConfig{
		Addr:     fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 23)),
		User:     cred.User,
		Password: cred.Password,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		// 设备不可达时跳过依赖真实设备的用例，避免离线环境下测试直接失败
		fmt.Printf("skip device tests, connect failed: %v\n", err)
		client = nil
	}
	m.Run()
}

func TestClient_Read(t *testing.T) {
	if client == nil {
		t.Skip("device unreachable")
	}
	defer func() {
		assert.NoError(t, client.Close())
	}()
	t.Log(client.welcomeStr)
	t.Log(client.promptStr)

	cmd := "show version"
	_, err := client.Write([]byte(cmd + "\n"))
	assert.NoError(t, err)

	data, _, err := client.ReadUtil2(cmd)
	assert.NoError(t, err)
	t.Log(data.String())

	data, _, err = client.doReadUtilPrompt(15 * time.Second)
	assert.NoError(t, err)
	t.Log(data.String())
}

// TestClient_Write 测试 Write 的 IAC 转义和 LF 转换
//
//	重点：数据中的无效 UTF-8 字节（如 GB18030 编码）不能被误判为 IAC
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
				cfg: &ClientConfig{WriteTimeout: 5 * time.Second, UnixWriteMode: obj.unixWriteMode},
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
			// n 的语义与原实现一致：返回实际写入网络的字节数（LF 转换为 CR LF 会计入 2 字节）
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
