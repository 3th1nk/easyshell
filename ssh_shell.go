package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/_test"
	"github.com/3th1nk/easyshell/pkg/errors"
	"github.com/3th1nk/easyshell/pkg/reader"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"time"
)

type SshShellConfig struct {
	reader.Config

	Echo       bool   // 模拟终端回显，默认值 false
	Term       string // 模拟终端类型，默认值 VT100
	TermHeight int    // 模拟终端高度，默认值 200
	TermWidth  int    // 模拟终端宽度，默认值 80
}

func NewSshShell(cred *SshCred, config *SshShellConfig) (*SshShell, error) {
	client, e := NewSshClient(cred)
	if e != nil {
		return nil, e
	}

	shell, err := NewSshShellFromClient(client, config)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	shell.ownClient = true

	return shell, nil
}

func NewSshShellFromClient(client *ssh.Client, config *SshShellConfig) (*SshShell, error) {
	addr := client.RemoteAddr().String()

	session, err := client.NewSession()
	if err != nil {
		return nil, &errors.Error{Op: "session", Addr: addr, Err: err}
	}

	if config.Term == "" {
		config.Term = "VT100"
	}
	if config.TermHeight <= 0 {
		config.TermHeight = 200
	}
	if config.TermWidth <= 0 {
		config.TermWidth = 80
	}
	echo := util.IfInt(config.Echo, 1, 0)
	if err = session.RequestPty(config.Term, config.TermHeight, config.TermWidth, ssh.TerminalModes{
		ssh.ECHO:          uint32(echo), // disable echoing
		ssh.TTY_OP_ISPEED: 14400,        // input speed
		ssh.TTY_OP_OSPEED: 14400,        // output speed
	}); err != nil {
		_ = session.Close()
		return nil, &errors.Error{Op: "term", Addr: addr, Err: err}
	}

	pIn, _ := session.StdinPipe()
	pOut, _ := session.StdoutPipe()
	pErr, _ := session.StderrPipe()

	if err = session.Shell(); err != nil {
		_ = session.Close()
		return nil, &errors.Error{Op: "shell", Addr: addr, Err: err}
	}
	r := reader.New(pIn, pOut, pErr, config.Config)

	var headLine []string
	_ = r.ReadToEndLine(3*time.Second, func(lines []string) {
		headLine = append(headLine, lines...)
	})
	headLine = _test.TrimEmptyLine(headLine)

	return &SshShell{Reader: r, client: client, session: session, headLine: headLine}, nil
}

type SshShell struct {
	*reader.Reader
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

func (this *SshShell) SftpClient(opt ...sftp.ClientOption) (*sftp.Client, error) {
	if this.sftp == nil {
		var err error
		if this.sftp, err = sftp.NewClient(this.client, opt...); err != nil {
			return nil, &errors.Error{Op: "sftp", Addr: this.client.RemoteAddr().String(), Err: err}
		}
	}
	return this.sftp, nil
}

func (this *SshShell) PopHeadLine() []string {
	headLine := this.headLine
	this.headLine = nil
	return headLine
}

func (this *SshShell) Close() (err error) {
	if this.sftp != nil {
		if e := this.sftp.Close(); e != nil {
			err = e
		}
	}
	if e := this.session.Close(); e != nil && err == nil {
		err = e
	}
	if this.ownClient {
		if e := this.client.Close(); e != nil && err == nil {
			err = e
		}
	}
	this.sftp, this.session, this.client = nil, nil, nil
	this.Reader.Stop()
	return
}
