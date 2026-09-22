// Package testutil 提供单元测试所需的工具函数
package testutil

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Credential 设备连接凭据
type Credential struct {
	Host     string
	Port     int // 0 表示未指定，由调用方按协议取默认值
	User     string
	Password string
}

// Getenv 返回环境变量 EASYSHELL_TEST_{NAME} 的值
//
//	优先读取系统环境变量，其次查找 .env 文件(go test 的工作目录是测试包所在目录，会向上级目录查找)
//	.env 文件路径可通过环境变量 EASYSHELL_TEST_ENV 指定，默认为当前目录及各级上级目录中的 .env
func Getenv(name string) string {
	key := "EASYSHELL_TEST_" + name
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return strings.TrimSpace(loadEnvFile()[key])
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
	s := Getenv(name)
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

var (
	envOnce sync.Once
	envVars map[string]string
)

func loadEnvFile() map[string]string {
	envOnce.Do(func() {
		envVars = map[string]string{}
		path := findEnvFile()
		if path == "" {
			return
		}
		if data, err := os.ReadFile(path); err == nil {
			envVars = parseEnvFile(data)
		}
	})
	return envVars
}

// parseEnvFile 解析 .env 文件内容：每行 KEY=VALUE，支持 # 注释、成对的单引号/双引号
func parseEnvFile(data []byte) map[string]string {
	vars := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// 去掉成对的单引号、双引号
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		vars[k] = v
	}
	return vars
}

func findEnvFile() string {
	if p := os.Getenv("EASYSHELL_TEST_ENV"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// go test 的工作目录是测试包所在目录，向上级查找以便支持仓库根目录的 .env
	dir, _ := os.Getwd()
	for {
		p := filepath.Join(dir, ".env")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
