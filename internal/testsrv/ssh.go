package testsrv

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

// SshServer 模拟 SSH 服务端(基于 golang.org/x/crypto/ssh 服务端实现，零外部依赖)：
//
//	密码认证 + PTY + 交互式 shell + SFTP 子系统。
type SshServer struct {
	Addr     string
	User     string
	Password string
	Banner   string
	Prompt   string
	// Session 自定义会话行为(stdin 读取命令、stdout 写入输出)；nil 时使用默认回显行为
	Session func(s *SshSession)
	// RootDir SFTP 子系统的根目录(为空时不启用 SFTP)
	RootDir string

	ln net.Listener
	t  *testing.T
}

// SshSession 会话的输入输出
type SshSession struct {
	Stdin  io.Reader
	Stdout io.Writer
}

// NewSshServer 启动 mock SSH 服务；测试结束后自动关闭。
func NewSshServer(t *testing.T) *SshServer {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}

	s := &SshServer{
		User:     "admin",
		Password: "passw0rd",
		Banner:   "Welcome to mock ssh",
		Prompt:   "<mock># ",
		t:        t,
	}

	config := &ssh.ServerConfig{}
	config.PasswordCallback = func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if meta.User() == s.User && string(password) == s.Password {
			return nil, nil
		}
		return nil, fmt.Errorf("password rejected for %q", meta.User())
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.ln = ln
	s.Addr = ln.Addr().String()
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn, config)
		}
	}()
	return s
}

func (s *SshServer) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		// direct-tcpip：跳板转发(本服务作为跳板机时)
		if newChannel.ChannelType() == "direct-tcpip" {
			var p struct {
				Addr     string
				Port     uint32
				OrigIP   string
				OrigPort uint32
			}
			if err := ssh.Unmarshal(newChannel.ExtraData(), &p); err != nil {
				_ = newChannel.Reject(ssh.ConnectionFailed, "bad payload")
				continue
			}
			upstream, err := net.Dial("tcp", net.JoinHostPort(p.Addr, strconv.Itoa(int(p.Port))))
			if err != nil {
				_ = newChannel.Reject(ssh.ConnectionFailed, "dial failed: "+err.Error())
				continue
			}
			ch, chReqs, err := newChannel.Accept()
			if err != nil {
				_ = upstream.Close()
				continue
			}
			go func() {
				defer upstream.Close()
				defer ch.Close()
				go func() { _, _ = io.Copy(upstream, ch) }()
				_, _ = io.Copy(ch, upstream)
			}()
			go ssh.DiscardRequests(chReqs)
			continue
		}

		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		ch, channelReqs, err := newChannel.Accept()
		if err != nil {
			return
		}
		go s.handleSession(ch, channelReqs)
	}
}

func (s *SshServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()

	shellStarted := false
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			_ = req.Reply(true, nil)

		case "shell":
			_ = req.Reply(true, nil)
			shellStarted = true
			session := &SshSession{Stdin: ch, Stdout: ch}
			handler := s.Session
			if handler == nil {
				handler = defaultSshSession(s.Banner, s.Prompt)
			}
			go handler(session)
			// shell 会话由 handler 自行结束(连接关闭时 Read 返回错误)

		case "subsystem": // sftp
			var payload struct{ Name string }
			_ = ssh.Unmarshal(req.Payload, &payload)
			ok := payload.Name == "sftp" && s.RootDir != ""
			_ = req.Reply(ok, nil)
			if !ok {
				continue
			}
			server, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(s.RootDir))
			if err != nil {
				return
			}
			_ = server.Serve() // 客户端断开时返回错误，属正常结束
			return

		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
	_ = shellStarted
}

// defaultSshSession 默认会话行为：输出横幅与提示符，回显命令并输出 "out:命令"，多行命令逐行处理
func defaultSshSession(banner, prompt string) func(*SshSession) {
	return func(s *SshSession) {
		write := func(str string) {
			_, _ = s.Stdout.Write([]byte(str))
		}
		write(banner + "\n" + prompt)

		buf := make([]byte, 4096)
		var pending []byte
		for {
			n, err := s.Stdin.Read(buf)
			if err != nil {
				return
			}
			pending = append(pending, buf[:n]...)
			for {
				i := bytes.IndexByte(pending, '\n')
				if i < 0 {
					break
				}
				cmd := strings.TrimSpace(string(pending[:i]))
				pending = pending[i+1:]
				if cmd != "" {
					write("out:" + cmd + "\n")
				}
				write(prompt)
			}
		}
	}
}
