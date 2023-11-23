package easyshell

import (
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/telnet"
	"github.com/3th1nk/easyshell/reader"
	"strings"
)

type TelnetShellConfig struct {
	reader.Config
	Credential *TelnetCredential
}

func ensureInitTelnetShellConfig(c *TelnetShellConfig) {
	if c == nil {
		c = &TelnetShellConfig{}
	}
}

func NewTelnetShell(config *TelnetShellConfig) (*TelnetShell, error) {
	ensureInitTelnetShellConfig(config)

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
	ensureInitTelnetShellConfig(config)

	r := reader.New(client, client, nil, config.Config)
	headLine := misc.TrimEmptyLine(strings.Split(client.Welcome(), "\n"))
	return &TelnetShell{
		Reader:   r,
		client:   client,
		headLine: headLine,
	}, nil
}

type TelnetShell struct {
	*reader.Reader
	client    *telnet.Client
	ownClient bool
	headLine  []string
}

func (this *TelnetShell) Client() *telnet.Client {
	return this.client
}

func (this *TelnetShell) PopHeadLine() []string {
	headLine := this.headLine
	this.headLine = nil
	return headLine
}

func (this *TelnetShell) Close() (err error) {
	if this.client != nil {
		if this.ownClient {
			err = this.client.Close()
		}
		this.client = nil
	}
	this.Reader.Stop()
	return
}
