// Package testutil 提供单元测试所需的工具函数
package testutil

import (
	"os"
	"strconv"
	"strings"
)

// Credential 设备连接凭据
type Credential struct {
	Host     string
	Port     int // 0 表示未指定，由调用方按协议取默认值
	User     string
	Password string
}

// FromEnv 从环境变量 EASYSHELL_TEST_{NAME} 中解析设备连接凭据，未设置时返回 nil
//
//	格式: [user:password@]host[:port]
//	password 中可以包含 '@'，按最后一个 '@' 分隔用户信息和主机
//	示例:
//	  192.0.2.3
//	  admin:passw0rd@192.0.2.3
//	  :passw0rd@192.0.2.3:23
func FromEnv(name string) *Credential {
	s := strings.TrimSpace(os.Getenv("EASYSHELL_TEST_" + name))
	if s == "" {
		return nil
	}

	c := &Credential{}
	// [user:password@]
	if i := strings.LastIndexByte(s, '@'); i != -1 {
		c.User, c.Password, _ = strings.Cut(s[:i], ":")
		s = s[i+1:]
	}
	// host[:port]
	if i := strings.LastIndexByte(s, ':'); i != -1 {
		if port, err := strconv.Atoi(s[i+1:]); err == nil && port > 0 {
			c.Host, c.Port = s[:i], port
			return c
		}
	}
	c.Host = s
	return c
}
