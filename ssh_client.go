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
	insecureSshMACs         = []string{"hmac-md5", "hmac-md5-96"}
	// 按照RFC8332规范，rsa-sha2-256和rsa-sha2-512的公钥格式仍然复用ssh-rsa，但存在一些不规范的设备，其公钥格式为rsa-sha2-512，
	//而go官方库中严格按照规范解析，会提示错误(unknown key algorithm: rsa-sha2-512);
	//	使用ssh命令测试能成功连接，对比发现其使用的公钥算法是ssh-ed25519，造成该差异的原因是go官方库默认的公钥算法列表中ssh-ed25519排在最后，
	//	当ssh服务端仅允许ssh-ed25519和rsa-sha2-512时，会优先匹配rsa-sha2-512算法，为了规避该问题，参考OpenSSH公钥算法顺序调整算法列表。
	openSshHostKeyAlgorithms = []string{
		ssh.CertAlgoED25519v01,
		ssh.CertAlgoECDSA256v01, ssh.CertAlgoECDSA384v01, ssh.CertAlgoECDSA521v01,
		ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSASHA512v01,
		ssh.CertAlgoRSAv01, ssh.CertAlgoDSAv01,
		ssh.KeyAlgoED25519,
		ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
		ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSA, ssh.KeyAlgoDSA,
	}
)

type SshCredential struct {
	Host               string        `json:"host"`                          // IP地址
	Port               int           `json:"port,omitempty"`                // 端口，默认22
	User               string        `json:"user,omitempty"`                // 用户名
	Password           string        `json:"password,omitempty"`            // 密码。当密钥与密码同时存在时，优先使用密钥。
	PrivateKey         string        `json:"private_key,omitempty"`         // 密钥。当密钥与密码同时存在时，优先使用密钥。
	Timeout            time.Duration `json:"timeout,omitempty"`             // 连接超时时间，默认15秒
	InsecureAlgorithms bool          `json:"insecure_algorithms,omitempty"` // 是否允许不安全的算法
	Fingerprint        string        `json:"fingerprint,omitempty"`         // 公钥指纹，用于验证服务器身份
}

// NewSshClient 创建一个新的 SshClient
func NewSshClient(cred *SshCredential) (*ssh.Client, error) {
	addr := fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 22))
	timeout := cred.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	var auths []ssh.AuthMethod
	if cred.PrivateKey != "" {
		if signer, err := ssh.ParsePrivateKey([]byte(cred.PrivateKey)); err != nil {
			return nil, &core.Error{Op: "auth", Addr: addr, Err: fmt.Errorf("privateKey error: %v", err)}
		} else {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	} else if cred.Password != "" {
		auths = append(auths,
			ssh.Password(cred.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				return arrUtil.RepeatString(cred.Password, len(questions)), nil
			}),
		)
	}
	if len(auths) == 0 {
		return nil, &core.Error{Op: "auth", Addr: addr, Err: fmt.Errorf("no auth method")}
	}

	cfg := ssh.Config{}
	cfg.SetDefaults()
	if cred.InsecureAlgorithms {
		cfg.Ciphers = append(cfg.Ciphers, insecureSshCiphers...)
		cfg.KeyExchanges = append(cfg.KeyExchanges, insecureSshKeyExchanges...)
		cfg.MACs = append(cfg.MACs, insecureSshMACs...)
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

	c, e := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		Config:            cfg,
		User:              cred.User,
		Auth:              auths,
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: openSshHostKeyAlgorithms,
		Timeout:           timeout,
	})
	if e != nil {
		if v, _ := e.(*net.OpError); v != nil {
			return nil, &core.Error{Op: "dial", Addr: addr, Err: e}
		} else {
			return nil, &core.Error{Op: "auth", Addr: addr, Err: e}
		}
	}

	return c, nil
}
