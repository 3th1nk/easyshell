// Package testsrv 提供离线测试用的 mock 服务(SSH/Telnet/SFTP)，不引入任何外部依赖。
package testsrv

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// TelnetServer 模拟网络设备的 telnet 服务：IAC 协商 + 用户名/密码认证 + 命令回显。
type TelnetServer struct {
	Addr     string
	Welcome  string
	User     string
	Password string
	Prompt   string

	ln net.Listener
	t  *testing.T
}

// NewTelnetServer 启动 mock telnet 服务；测试结束后自动关闭。
//
//	authMode: "password" 仅密码认证(直接提示 Password:)；"user" 先提示用户名再提示密码。
//	密码错误时重新提示(最多2次)，用于验证客户端的认证失败错误。
func NewTelnetServer(t *testing.T, authMode string) *TelnetServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	s := &TelnetServer{
		Addr:     ln.Addr().String(),
		Welcome:  "Welcome to mock device\n",
		User:     "admin",
		Password: "passw0rd",
		Prompt:   "<MOCK>",
		ln:       ln,
		t:        t,
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn, authMode)
		}
	}()
	return s
}

func (s *TelnetServer) handle(conn net.Conn, authMode string) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	write := func(str string) {
		_, _ = conn.Write([]byte(str))
	}

	// 模拟服务端 IAC 协商：WILL ECHO + WILL SGA
	_, _ = conn.Write([]byte{255, 251, 1, 255, 251, 3})
	write(s.Welcome)

	// 认证(密码错误时重新提示，最多2次)
	switch authMode {
	case "none": // 无认证，直接进入命令交互
		goto authed
	case "password":
		for i := 0; i < 2; i++ {
			write("Password:")
			if s.expect(br, s.Password) {
				goto authed
			}
		}
		return
	default: // "user"
		for i := 0; i < 2; i++ {
			write("Login:")
			if !s.expect(br, s.User) {
				return
			}
			write("Password:")
			if s.expect(br, s.Password) {
				goto authed
			}
		}
		return
	}

authed:
	write(s.Prompt)
	for {
		cmd, err := s.readLine(br)
		if err != nil {
			return
		}
		if cmd == "" {
			// 回显空行(客户端认证完成后的 scroll-to-new-line 会发送 LF)
			write("\n")
			continue
		}
		if cmd == "quit" {
			return
		}
		write("out:" + cmd + "\n" + s.Prompt)
	}
}

// readLine 读取一行(跳过 IAC 协议序列，忽略 \r)
func (s *TelnetServer) readLine(br *bufio.Reader) (string, error) {
	var line []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 255 { // IAC: 255 + command (+ option，若 command != IAC)
			b2, err := br.ReadByte()
			if err != nil {
				return "", err
			}
			if b2 != 255 {
				_, _ = br.ReadByte()
			}
			continue
		}
		if b == '\n' {
			return strings.TrimSpace(string(line)), nil
		}
		if b == '\r' {
			continue
		}
		line = append(line, b)
	}
}

// expect 读取一行并比较内容(跳过 IAC 协议序列)，匹配返回 true；不匹配返回 false
func (s *TelnetServer) expect(br *bufio.Reader, want string) bool {
	line, err := s.readLine(br)
	return err == nil && line == want
}
