package easyshell

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"os"
	"path/filepath"
	"strings"
)

// KnownHostsCallback 基于 OpenSSH known_hosts 文件构建主机密钥校验回调。
//
//	支持标准 known_hosts 格式(含 hashed hostname 与 [host]:port 形式的非标准端口)；
//	可传入多个文件路径(任一匹配即通过)；支持 ~ 前缀(自动展开为用户主目录)。
//	使用方式：将返回的回调赋给 SshCredential.HostKeyCallback。
//
//	注意：设备的主机密钥必须已存在于 known_hosts 中(可先用 ssh-keyscan 生成)，
//	否则连接将被拒绝；未知主机的自动录入存在 TOFU 安全风险，需自行权衡。
func KnownHostsCallback(paths ...string) (ssh.HostKeyCallback, error) {
	expanded := make([]string, 0, len(paths))
	for _, p := range paths {
		expanded = append(expanded, expandHome(p))
	}
	return knownhosts.New(expanded...)
}

// expandHome 展开 ~ 前缀为用户主目录
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
