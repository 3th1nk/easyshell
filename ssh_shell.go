package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/core"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/interceptor"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"time"
)

type SshShellConfig struct {
	core.Config
	Credential *SshCredential // 凭证
	Echo       bool           // 模拟终端回显，默认值 false，部分网络设备上无效（总是回显）
	Term       string         // 模拟终端类型，默认值 VT100
	TermHeight int            // 模拟终端高度，默认值 200
	TermWidth  int            // 模拟终端宽度，默认值 80
}

func (c *SshShellConfig) EnsureInit() {
	if c.Term == "" {
		c.Term = "VT100"
	}
	if c.TermHeight <= 0 {
		c.TermHeight = 200
	}
	if c.TermWidth <= 0 {
		c.TermWidth = 80
	}
}

func NewSshShell(config ...*SshShellConfig) (*SshShell, error) {
	var cfg *SshShellConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = &SshShellConfig{}
	}
	cfg.EnsureInit()

	client, e := NewSshClient(cfg.Credential)
	if e != nil {
		return nil, e
	}

	shell, err := NewSshShellFromClient(client, cfg)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	shell.ownClient = true

	return shell, nil
}

func NewSshShellFromClient(client *ssh.Client, config ...*SshShellConfig) (*SshShell, error) {
	var cfg *SshShellConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = &SshShellConfig{}
	}
	cfg.EnsureInit()

	addr := client.RemoteAddr().String()
	session, err := client.NewSession()
	if err != nil {
		return nil, &core.Error{Op: "session", Addr: addr, Err: err}
	}

	echo := util.IfInt(cfg.Echo, 1, 0)
	if err = session.RequestPty(cfg.Term, cfg.TermHeight, cfg.TermWidth, ssh.TerminalModes{
		ssh.ECHO:          uint32(echo),
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = session.Close()
		return nil, &core.Error{Op: "term", Addr: addr, Err: err}
	}

	pIn, _ := session.StdinPipe()
	pOut, _ := session.StdoutPipe()
	pErr, _ := session.StderrPipe()

	if err = session.Shell(); err != nil {
		_ = session.Close()
		return nil, &core.Error{Op: "shell", Addr: addr, Err: err}
	}
	r := core.New(pIn, pOut, pErr, cfg.Config)

	// 此时可能会有一些输出，可能是欢迎信息、日志打印、密码修改提示等，需要读取并处理，防止影响后续操作
	//	对于密码修改提示，部分设备是会提示密码过期，是否修改密码，也有设备是直接提示输入密码，这里只能处理前者，总是答复否
	var headLine []string
	_ = r.ReadToEndLine(5*time.Second, func(lines []string) {
		headLine = append(headLine, lines...)
	}, interceptor.AlwaysNo())
	headLine = misc.TrimEmptyLine(headLine)

	return &SshShell{ReadWriter: r, client: client, session: session, headLine: headLine}, nil
}

type SshShell struct {
	*core.ReadWriter
	client    *ssh.Client
	session   *ssh.Session
	sftp      *sftp.Client
	ownClient bool
	headLine  []string
}

func (this *SshShell) Client() *ssh.Client {
	return this.client
}

func (this *SshShell) Session() *ssh.Session {
	return this.session
}

func (this *SshShell) HeadLine() []string {
	return this.headLine
}

func (this *SshShell) Close() (err error) {
	if this.sftp != nil {
		if e := this.sftp.Close(); e != nil {
			err = e
		}
		this.sftp = nil
	}

	if this.session != nil {
		if e := this.session.Close(); e != nil && err == nil {
			err = e
		}
		this.session = nil
	}

	if this.client != nil {
		if this.ownClient {
			if e := this.client.Close(); e != nil && err == nil {
				err = e
			}
		}
		this.client = nil
	}
	this.ReadWriter.Stop()
	return
}
