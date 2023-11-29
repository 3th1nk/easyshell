package easyshell

import (
	"github.com/3th1nk/easyshell/core"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/telnet"
	"strings"
	"time"
)

type TelnetShellConfig struct {
	core.Config
	Credential *TelnetCredential
}

func (c *TelnetShellConfig) EnsureInit() {
}

func NewTelnetShell(config *TelnetShellConfig) (*TelnetShell, error) {
	if config == nil {
		config = &TelnetShellConfig{}
	}
	config.EnsureInit()

	client, e := NewTelnetClient(config.Credential)
	if e != nil {
		return nil, e
	}

	shell, err := NewTelnetShellFromClient(client, config)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	shell.ownClient = true
	return shell, nil
}

func NewTelnetShellFromClient(client *telnet.Client, config *TelnetShellConfig) (*TelnetShell, error) {
	if config == nil {
		config = &TelnetShellConfig{}
	}
	config.EnsureInit()

	r := core.New(client, client, nil, config.Config)
	// 读取提示符
	_ = r.Write("")
	_ = r.ReadToEndLine(3*time.Second, func(lines []string) {})

	headLine := misc.TrimEmptyLine(strings.Split(client.Welcome(), "\n"))
	return &TelnetShell{
		ReadWriter: r,
		client:     client,
		headLine:   headLine,
	}, nil
}

type TelnetShell struct {
	*core.ReadWriter
	client    *telnet.Client
	ownClient bool
	headLine  []string
}

func (this *TelnetShell) Client() *telnet.Client {
	return this.client
}

func (this *TelnetShell) HeadLine() []string {
	return this.headLine
}

func (this *TelnetShell) Close() (err error) {
	if this.client != nil {
		if this.ownClient {
			err = this.client.Close()
		}
		this.client = nil
	}
	this.ReadWriter.Stop()
	return
}
