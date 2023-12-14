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
	insecureSshCiphers      = []string{"arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc", "aes192-cbc", "aes256-cbc"}
	insecureSshKeyExchanges = []string{"diffie-hellman-group1-sha1", "diffie-hellman-group-exchange-sha1", "diffie-hellman-group-exchange-sha256"}
)

type SshCredential struct {
	Host               string `json:"host"`                          // IP地址
	Port               int    `json:"port,omitempty"`                // 端口，默认22
	User               string `json:"user,omitempty"`                // 用户名
	Password           string `json:"password,omitempty"`            // 密码。当密钥与密码同时存在时，优先使用密钥。
	PrivateKey         string `json:"private_key,omitempty"`         // 密钥。当密钥与密码同时存在时，优先使用密钥。
	Timeout            int    `json:"timeout,omitempty"`             // 连接超时时间（秒），默认15秒
	InsecureAlgorithms bool   `json:"insecure_algorithms,omitempty"` // 是否允许不安全的算法
	Fingerprint        string `json:"fingerprint,omitempty"`         // 公钥指纹，用于验证服务器身份
}

// NewSshClient 创建一个新的 SshClient
func NewSshClient(cred *SshCredential) (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 22))
	timeout := util.IfInt(cred.Timeout > 0, cred.Timeout, 15)

	cfg := ssh.Config{}
	cfg.SetDefaults()
	if cred.InsecureAlgorithms {
		cfg.Ciphers = append(cfg.Ciphers, insecureSshCiphers...)
		cfg.KeyExchanges = append(cfg.KeyExchanges, insecureSshKeyExchanges...)
	}

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if cred.Fingerprint != "" {
		hostKeyCallback = func(hostname string, remote net.Addr, publicKey ssh.PublicKey) error {
			if ssh.FingerprintSHA256(publicKey) != cred.Fingerprint {
				return fmt.Errorf("ssh: host key fingerprint mismatch")
			}
			return nil
		}
	}
	sshCfg := &ssh.ClientConfig{
		Config:          cfg,
		User:            cred.User,
		HostKeyCallback: hostKeyCallback,
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
