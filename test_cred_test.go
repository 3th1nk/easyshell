package easyshell

import (
	"os"
	"strconv"
	"strings"
)

// 真实设备的连接凭据通过环境变量注入，避免泄露到代码仓库；
// 未设置对应环境变量时，相关测试自动跳过(skip)。可用环境变量如下：
//
//	EASYSHELL_TEST_SSH_LINUX_HOST/PORT/USER/PASSWORD       本地Linux主机(SSH)
//	EASYSHELL_TEST_SSH_LINUX_ROOT_PASSWORD                 本地Linux主机root密码，su root测试用
//	EASYSHELL_TEST_SSH_CISCO_HOST/PORT/USER/PASSWORD       Cisco网络设备(SSH)
//	EASYSHELL_TEST_SSH_ARRAY_HOST/PORT/USER/PASSWORD       Array APV负载均衡设备(SSH)
//	EASYSHELL_TEST_SSH_H3C_HOST/PORT/USER/PASSWORD         H3C网络设备(SSH)
//	EASYSHELL_TEST_SSH_HW_HOST/PORT/USER/PASSWORD          华为网络设备(SSH)
//	EASYSHELL_TEST_TELNET_CISCO_HOST/PORT/USER/PASSWORD    Cisco网络设备(Telnet)
//	EASYSHELL_TEST_TELNET_H3C_HOST/PORT/USER/PASSWORD      H3C网络设备(Telnet，无需USER)
const (
	envPrefixSsh    = "EASYSHELL_TEST_SSH_"
	envPrefixTelnet = "EASYSHELL_TEST_TELNET_"
)

func envValue(prefix, suffix string) string {
	return strings.TrimSpace(os.Getenv(prefix + suffix))
}

// envInt 读取整型环境变量，未设置或非法时返回默认值
func envInt(prefix, suffix string, def int) int {
	if v := envValue(prefix, suffix); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// sshCredFromEnv 从 EASYSHELL_TEST_SSH_{NAME}_HOST/PORT/USER/PASSWORD 构建 SshCredential
//
//	insecure 网络设备需要设置为 true
//	未设置 HOST 或 PASSWORD 时返回 nil，调用方据此跳过测试
func sshCredFromEnv(name string, insecure bool) *SshCredential {
	prefix := envPrefixSsh + name + "_"
	host, password := envValue(prefix, "HOST"), envValue(prefix, "PASSWORD")
	if host == "" || password == "" {
		return nil
	}
	return &SshCredential{
		Host:               host,
		Port:               envInt(prefix, "PORT", 22),
		User:               envValue(prefix, "USER"),
		Password:           password,
		InsecureAlgorithms: insecure,
	}
}

// telnetCredFromEnv 从 EASYSHELL_TEST_TELNET_{NAME}_HOST/PORT/USER/PASSWORD 构建 TelnetCredential
//
//	未设置 HOST 或 PASSWORD 时返回 nil，调用方据此跳过测试
func telnetCredFromEnv(name string) *TelnetCredential {
	prefix := envPrefixTelnet + name + "_"
	host, password := envValue(prefix, "HOST"), envValue(prefix, "PASSWORD")
	if host == "" || password == "" {
		return nil
	}
	return &TelnetCredential{
		Host:     host,
		Port:     envInt(prefix, "PORT", 23),
		User:     envValue(prefix, "USER"),
		Password: password,
	}
}
