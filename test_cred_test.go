package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/internal/testutil"
)

// 真实设备的连接凭据通过环境变量注入，避免泄露到代码仓库；
// 未设置对应环境变量时，相关测试自动跳过(skip)。可用环境变量如下(格式: [user:password@]host[:port])：
//
//	EASYSHELL_TEST_LINUX                    本地Linux主机，SSH测试；root密码另设 EASYSHELL_TEST_LINUX_ROOT_PASSWORD
//	EASYSHELL_TEST_CISCO                    Cisco网络设备，SSH、Telnet测试共用
//	EASYSHELL_TEST_ARRAY                    Array APV负载均衡设备，SSH测试
//	EASYSHELL_TEST_H3C                      H3C网络设备，SSH、Telnet测试共用
//	EASYSHELL_TEST_HW                       华为网络设备，SSH测试
//
//	示例: export EASYSHELL_TEST_H3C=admin:passw0rd@192.0.2.3
func sshCredFromEnv(name string, insecure bool) *SshCredential {
	c := testutil.FromEnv(name)
	if c == nil {
		return nil
	}
	return &SshCredential{
		Host:               c.Host,
		Port:               util.IfEmptyInt(c.Port, 22),
		User:               c.User,
		Password:           c.Password,
		InsecureAlgorithms: insecure,
	}
}

func telnetCredFromEnv(name string) *TelnetCredential {
	c := testutil.FromEnv(name)
	if c == nil {
		return nil
	}
	return &TelnetCredential{
		Host:     c.Host,
		Port:     util.IfEmptyInt(c.Port, 23),
		User:     c.User,
		Password: c.Password,
	}
}
