package easyshell

import (
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easygo/util/arrUtil"
	"github.com/3th1nk/easyshell/core"
	"golang.org/x/crypto/ssh"
	"net"
	"time"
)

var (
	supportedKeyExchanges = []string{
		// default key-exchange algorithms in golang.org/x/crypto@v0.8.0
		"curve25519-sha256",
		"curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
		"ecdh-sha2-nistp521",
		"diffie-hellman-group14-sha1",
		"diffie-hellman-group14-sha256",
		// support but not default
		"diffie-hellman-group1-sha1",
		// forbidden algorithms
		// 	TODO maybe necessary in some cases
		"diffie-hellman-group-exchange-sha1",
		"diffie-hellman-group-exchange-sha256",
	}

	supportedCiphers = []string{
		// default ciphers in golang.org/x/crypto@v0.8.0
		"aes128-ctr", "aes192-ctr", "aes256-ctr",
		"aes128-gcm@openssh.com", "aes256-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		// support but might not recommend
		"arcfour", "arcfour128", "arcfour256",
		"aes128-cbc", "3des-cbc",
		// other
		"aes192-cbc", "aes256-cbc",
	}
)

type SshCredential struct {
	Host       string `json:"host"`                  // IP地址
	Port       int    `json:"port,omitempty"`        // 端口，默认22
	User       string `json:"user,omitempty"`        // 用户名
	Password   string `json:"password,omitempty"`    // 密码。当密钥与密码同时存在时，优先使用密钥。
	PrivateKey string `json:"private_key,omitempty"` // 密钥。当密钥与密码同时存在时，优先使用密钥。
	Timeout    int    `json:"timeout,omitempty"`     // 连接超时时间（秒），默认15秒
}

// NewSshClient 创建一个新的 SshClient
func NewSshClient(cred *SshCredential) (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 22))
	timeout := util.IfInt(cred.Timeout > 0, cred.Timeout, 15)

	sshCfg := &ssh.ClientConfig{
		Config: ssh.Config{
			KeyExchanges: supportedKeyExchanges,
			Ciphers:      supportedCiphers,
		},
		User:            cred.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * time.Duration(timeout),
	}
	if cred.PrivateKey != "" {
		if signer, err := ssh.ParsePrivateKey([]byte(cred.PrivateKey)); err != nil {
			return nil, &core.Error{Op: "auth", Addr: addr, Err: fmt.Errorf("privateKey error: %v", err)}
		} else {
			sshCfg.Auth = append(sshCfg.Auth, ssh.PublicKeys(signer))
		}
	} else if cred.Password != "" {
		sshCfg.Auth = append(sshCfg.Auth,
			ssh.Password(cred.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				return arrUtil.RepeatString(cred.Password, len(questions)), nil
			}),
		)
	}
	if len(sshCfg.Auth) == 0 {
		return nil, &core.Error{Op: "auth", Addr: addr, Err: fmt.Errorf("no auth method")}
	}

	c, e := ssh.Dial("tcp", addr, sshCfg)
	if e != nil {
		if v, _ := e.(*net.OpError); v != nil {
			return nil, &core.Error{Op: "dial", Addr: addr, Err: e}
		} else {
			return nil, &core.Error{Op: "auth", Addr: addr, Err: e}
		}
	}

	return c, nil
}
