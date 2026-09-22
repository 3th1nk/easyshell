package easyshell

import (
	"context"
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"github.com/3th1nk/easyshell/v2/internal/telnet"
	"github.com/3th1nk/easyshell/v2/record"
	"strings"
	"time"
)

// TelnetConfig Telnet Shell 配置(零值合法)
type TelnetConfig struct {
	Config
	// Credential Telnet 登录凭证
	Credential TelnetCredential
	// Record 会话录制配置，nil 时不录制
	Record *RecordConfig
	// Echo 是否允许回显(取决于设备是否支持)，部分网络设备上无效(总是回显)
	Echo bool
	// SuppressGA 是否抑制 "go ahead" 命令
	SuppressGA bool
}

// NewTelnetShell 创建 Telnet Shell 并完成登录。
func NewTelnetShell(cfg TelnetConfig) (*TelnetShell, error) {
	rec, err := wireRecord(cfg.Record, &cfg.Config, record.Meta{
		Host: cfg.Credential.Host, Port: util.IfEmptyInt(cfg.Credential.Port, 23),
		Protocol: "telnet", User: cfg.Credential.User,
	})
	if err != nil {
		return nil, err
	}

	client, err := telnet.NewClient(telnet.Config{
		Addr:       fmt.Sprintf("%s:%d", cfg.Credential.Host, util.IfEmptyInt(cfg.Credential.Port, 23)),
		User:       cfg.Credential.User,
		Password:   cfg.Credential.Password,
		Timeout:    cfg.Credential.Timeout,
		Echo:       cfg.Echo,
		SuppressGA: cfg.SuppressGA,
	})
	if err != nil {
		if rec != nil {
			_ = rec.Close()
		}
		return nil, err
	}

	shell, err := newTelnetShellFromClient(client, cfg)
	if err != nil {
		_ = client.Close()
		if rec != nil {
			_ = rec.Close()
		}
		return nil, err
	}
	shell.ownClient = true
	shell.recorder = rec
	return shell, nil
}

// NewTelnetShellFromClient 基于已有的 telnet 连接创建 Shell(调用方自行管理连接的关闭)。
func newTelnetShellFromClient(client *telnet.Client, cfg TelnetConfig) (*TelnetShell, error) {
	// 保活动作：telnet NOP 命令
	if cfg.KeepAlive != nil && cfg.KeepAlive.Interval > 0 {
		send := cfg.KeepAlive.Send
		if send == nil {
			send = func() error {
				_, err := client.Write([]byte{241}) // NOP
				return err
			}
		}
		cfg.KeepAlive.Send = send
	}

	r := core.NewReader(client, client, nil, cfg.Config)
	_ = r.Write("")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = r.ReadUntilPrompt(ctx, func([]string) {})

	headLine := trimEmptyLines(strings.Split(client.Welcome(), "\n"))
	return &TelnetShell{
		shellBase: shellBase{rw: r},
		client:    client,
		headLine:  headLine,
	}, nil
}

// TelnetShell Telnet 交互式 Shell
type TelnetShell struct {
	shellBase
	client    *telnet.Client
	ownClient bool
	headLine  []string
}

// HeadLine 返回登录后的欢迎信息
func (s *TelnetShell) HeadLine() []string {
	return s.headLine
}

// SetEcho 设置是否允许回显
func (s *TelnetShell) SetEcho(echo bool) error {
	return s.client.SetEcho(echo)
}

// SetSuppressGA 设置是否抑制 "go ahead" 命令
func (s *TelnetShell) SetSuppressGA(suppressGA bool) error {
	return s.client.SetSuppressGA(suppressGA)
}

// Close 关闭(幂等)
func (s *TelnetShell) Close() error {
	if s.client != nil {
		if s.ownClient {
			_ = s.client.Close()
		}
		s.client = nil
	}
	return s.shellBase.Close()
}
