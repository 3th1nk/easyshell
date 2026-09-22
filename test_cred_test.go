package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/v2/internal/testutil"
)

// 真实设备的连接凭据通过环境变量或 .env 文件注入(已被.gitignore忽略)，避免泄露到代码仓库；
// 未设置时相关测试自动跳过(skip)。可用环境变量如下(格式: [user:password@]host[:port])：
//
//	EASYSHELL_TEST_LINUX                    本地Linux主机，SSH测试；root密码另设 EASYSHELL_TEST_LINUX_ROOT_PASSWORD
//	EASYSHELL_TEST_CISCO                    Cisco网络设备，SSH、Telnet测试共用
//	EASYSHELL_TEST_ARRAY                    Array APV负载均衡设备，SSH测试
//	EASYSHELL_TEST_H3C                      H3C网络设备，SSH、Telnet测试共用
//	EASYSHELL_TEST_HW                       华为网络设备，SSH测试
//
//	示例: EASYSHELL_TEST_H3C=admin:passw0rd@192.0.2.3
func sshCredFromEnv(name string, insecure bool) (SshCredential, bool) {
	c := testutil.FromEnv(name)
	if c == nil {
		return SshCredential{}, false
	}
	return SshCredential{
		Host:               c.Host,
		Port:               util.IfEmptyInt(c.Port, 22),
		User:               c.User,
		Password:           c.Password,
		InsecureAlgorithms: insecure,
	}, true
}

func telnetCredFromEnv(name string) (TelnetCredential, bool) {
	c := testutil.FromEnv(name)
	if c == nil {
		return TelnetCredential{}, false
	}
	return TelnetCredential{
		Host:     c.Host,
		Port:     util.IfEmptyInt(c.Port, 23),
		User:     c.User,
		Password: c.Password,
	}, true
}
