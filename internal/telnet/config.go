package telnet

import (
	"crypto/tls"
	"github.com/3th1nk/easyshell/v2/internal/core"
	"regexp"
	"time"
)

// Config telnet 客户端配置(零值合法，未设置的字段使用库默认值)
type Config struct {
	Addr     string
	User     string
	Password string
	// LoginUserRegex 匹配用户名输入提示符的正则，nil 时使用内置规则
	LoginUserRegex *regexp.Regexp
	// LoginPassRegex 匹配密码输入提示符的正则，nil 时使用内置规则
	LoginPassRegex *regexp.Regexp
	// LoginPromptRegex 匹配命令提示符的正则，nil 时使用内置规则
	LoginPromptRegex *regexp.Regexp
	// Timeout 连接与登录超时时间，默认 15 秒
	Timeout time.Duration
	// WriteTimeout 写超时时间，默认 5 秒
	WriteTimeout time.Duration
	// UnixWriteMode 开启后 Write 会把 '\n'(LF) 转换为 '\r\n'(CR LF)
	UnixWriteMode bool
	// Echo 是否允许回显(取决于服务端是否支持)，部分网络设备上无效(总是回显)
	Echo bool
	// SuppressGA 是否抑制 "go ahead" 命令
	SuppressGA bool
	// TLS 启用 TLS 连接(类 telnet over TLS，如部分思科设备的 CISCO-TTLS 场景)
	TLS *tls.Config
}

func (cfg Config) normalize() Config {
	if cfg.LoginUserRegex == nil {
		cfg.LoginUserRegex = core.UsernameRegex
	}
	if cfg.LoginPassRegex == nil {
		cfg.LoginPassRegex = core.PasswordRegex
	}
	if cfg.LoginPromptRegex == nil {
		cfg.LoginPromptRegex = core.DefaultPromptRegex
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 5 * time.Second
	}
	return cfg
}
