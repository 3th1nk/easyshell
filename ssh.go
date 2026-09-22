package easyshell

import (
	"context"
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easygo/util/arrUtil"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	insecureSshCiphers      = []string{"arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc", "aes192-cbc", "aes256-cbc"}
	insecureSshKeyExchanges = []string{"diffie-hellman-group1-sha1", "diffie-hellman-group-exchange-sha1", "diffie-hellman-group-exchange-sha256"}
	insecureSshMACs         = []string{"hmac-md5", "hmac-md5-96"}
	// 按照RFC8332规范，rsa-sha2-256和rsa-sha2-512的公钥格式仍然复用ssh-rsa，但存在一些不规范的设备，其公钥格式为rsa-sha2-512，
	//而go官方库中严格按照规范解析，会提示错误(unknown key algorithm: rsa-sha2-512)；
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

// ProxyConfig 跳板机(堡垒机)配置。
//
//	支持多级链：目标主机经由 Proxy 指定的跳板机连接，跳板机自身也可再指定 Proxy。
type ProxyConfig struct {
	// Credential 跳板机的登录凭证
	Credential SshCredential
	// Proxy 跳板机的上级跳板机(多级链)
	Proxy *ProxyConfig
}

// SshConfig SSH Shell 配置(零值合法)
type SshConfig struct {
	Config
	// Credential SSH 登录凭证
	Credential SshCredential
	// Proxy 跳板机(堡垒机)配置，nil 时直连
	Proxy *ProxyConfig
	// Echo 模拟终端回显，默认 false；部分网络设备上无效(总是回显)
	Echo bool
	// Term 模拟终端类型，默认 VT100
	Term string
	// TermHeight 模拟终端高度，默认 200
	TermHeight int
	// TermWidth 模拟终端宽度，默认 256。宽度太小可能会出现乱码(多字节编码被回车换行截断)
	TermWidth int
}

func (cfg SshConfig) normalize() SshConfig {
	if cfg.Term == "" {
		cfg.Term = "VT100"
	}
	if cfg.TermHeight <= 0 {
		cfg.TermHeight = 200
	}
	if cfg.TermWidth <= 0 {
		cfg.TermWidth = 256
	}
	return cfg
}

// NewSshClient 创建 SSH 连接(直连)。
//	连接失败返回 core.Error{Op: OpDial}，认证失败返回 core.Error{Op: OpAuth}。
func NewSshClient(cred SshCredential) (*ssh.Client, error) {
	return dialSsh(cred, nil)
}

// dialSsh 建立SSH连接；proxyDialer 非 nil 时经由跳板机建立底层连接。
func dialSsh(cred SshCredential, proxyDialer func(network, addr string) (net.Conn, error)) (*ssh.Client, error) {
	addr := sshAddr(cred)
	timeout := cred.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	// 底层连接：直连 或 经由跳板机
	var conn net.Conn
	var e error
	if proxyDialer != nil {
		if conn, e = proxyDialer("tcp", addr); e != nil {
			return nil, &core.Error{Op: core.OpDial, Addr: addr, Err: e}
		}
	} else {
		if conn, e = net.DialTimeout("tcp", addr, timeout); e != nil {
			return nil, &core.Error{Op: core.OpDial, Addr: addr, Err: e}
		}
	}
	if dc, ok := conn.(interface{ SetDeadline(t time.Time) error }); ok {
		// 拨号与握手的总时限；握手完成后解除
		_ = dc.SetDeadline(time.Now().Add(timeout))
		defer func() { _ = dc.SetDeadline(time.Time{}) }()
	}

	clientConfig := &ssh.ClientConfig{
		Config:            cfgOf(cred),
		User:              cred.User,
		Auth:              sshAuthMethods(cred),
		HostKeyCallback:   hostKeyCallbackOf(cred),
		HostKeyAlgorithms: openSshHostKeyAlgorithms,
		Timeout:           timeout,
	}
	c, chans, reqs, e := ssh.NewClientConn(conn, addr, clientConfig)
	if e != nil {
		// 认证失败/主机密钥不匹配 → auth；其余(I/O、版本协商) → dial
		if strings.Contains(e.Error(), "unable to authenticate") || strings.Contains(e.Error(), "host key") || strings.Contains(e.Error(), "fingerprint") {
			return nil, &core.Error{Op: core.OpAuth, Addr: addr, Err: e}
		}
		return nil, &core.Error{Op: core.OpDial, Addr: addr, Err: e}
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// dialSshChain 建立经由跳板机链的SSH连接。
//
//	返回目标客户端与级联关闭函数 closeAll(按 目标→中间跳板→最外层跳板 的顺序关闭，
//	经跳板机的连接是上级连接上的隧道，必须先关目标再关跳板)。
func dialSshChain(cred SshCredential, proxy *ProxyConfig) (client *ssh.Client, closeAll func(), err error) {
	if proxy == nil {
		client, err = dialSsh(cred, nil)
		if err != nil {
			return nil, nil, err
		}
		return client, func() { _ = client.Close() }, nil
	}

	// 递归建立跳板机的连接(跳板机自身可能还有上级跳板)
	proxyClient, closeProxy, err := dialSshChain(proxy.Credential, proxy.Proxy)
	if err != nil {
		return nil, nil, err
	}

	targetAddr := sshAddr(cred)
	timeout := cred.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	conn, e := proxyClient.Dial("tcp", targetAddr)
	if e != nil {
		closeProxy()
		return nil, nil, &core.Error{Op: core.OpDial, Addr: targetAddr, Err: e}
	}
	if dc, ok := conn.(interface{ SetDeadline(t time.Time) error }); ok {
		_ = dc.SetDeadline(time.Now().Add(timeout))
		defer func() { _ = dc.SetDeadline(time.Time{}) }()
	}

	clientConfig := &ssh.ClientConfig{
		Config:            cfgOf(cred),
		User:              cred.User,
		Auth:              sshAuthMethods(cred),
		HostKeyCallback:   hostKeyCallbackOf(cred),
		HostKeyAlgorithms: openSshHostKeyAlgorithms,
		Timeout:           timeout,
	}
	c, chans, reqs, e := ssh.NewClientConn(conn, targetAddr, clientConfig)
	if e != nil {
		closeProxy()
		if strings.Contains(e.Error(), "unable to authenticate") || strings.Contains(e.Error(), "host key") {
			return nil, nil, &core.Error{Op: core.OpAuth, Addr: targetAddr, Err: e}
		}
		return nil, nil, &core.Error{Op: core.OpDial, Addr: targetAddr, Err: e}
	}
	client = ssh.NewClient(c, chans, reqs)
	return client, func() {
		_ = client.Close()
		closeProxy()
	}, nil
}

func sshAddr(cred SshCredential) string {
	return fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 22))
}

// sshAuthMethods 构建认证方式，优先级：密钥 → SSH Agent → 密码
func sshAuthMethods(cred SshCredential) []ssh.AuthMethod {
	var auths []ssh.AuthMethod
	if cred.PrivateKey != "" {
		if signer, err := ssh.ParsePrivateKey([]byte(cred.PrivateKey)); err == nil {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	}
	if cred.UseAgent {
		if a := sshAgentClient(cred.AgentSocket); a != nil {
			// 回调式加载：认证时才从Agent取签名
			auths = append(auths, ssh.PublicKeysCallback(a.Signers))
		}
	}
	if len(auths) == 0 && cred.Password != "" {
		auths = append(auths,
			ssh.Password(cred.Password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				return arrUtil.RepeatString(cred.Password, len(questions)), nil
			}),
		)
	}
	return auths
}

// sshAgentClient 连接SSH Agent(优先使用指定socket，否则环境变量 SSH_AUTH_SOCK)
func sshAgentClient(socket string) agent.Agent {
	if socket == "" {
		socket = os.Getenv("SSH_AUTH_SOCK")
	}
	if socket == "" {
		return nil
	}
	if conn, err := net.Dial("unix", socket); err == nil {
		return agent.NewClient(conn)
	}
	return nil
}

func hostKeyCallbackOf(cred SshCredential) ssh.HostKeyCallback {
	if cred.Fingerprint == "" {
		return ssh.InsecureIgnoreHostKey()
	}
	return func(hostname string, remote net.Addr, publicKey ssh.PublicKey) error {
		if ssh.FingerprintSHA256(publicKey) != cred.Fingerprint {
			return fmt.Errorf("ssh: host key fingerprint mismatch")
		}
		return nil
	}
}

func cfgOf(cred SshCredential) ssh.Config {
	cfg := ssh.Config{}
	cfg.SetDefaults()
	if cred.InsecureAlgorithms {
		cfg.Ciphers = append(cfg.Ciphers, insecureSshCiphers...)
		cfg.KeyExchanges = append(cfg.KeyExchanges, insecureSshKeyExchanges...)
		cfg.MACs = append(cfg.MACs, insecureSshMACs...)
	}
	return cfg
}

// NewSshShell 创建 SSH Shell 并完成登录(经由跳板机时，跳板机连接随 Shell.Close 一起关闭)。
//	登录横幅(欢迎信息、密码过期提示等)会被自动消费，通过 HeadLine() 获取。
func NewSshShell(cfg SshConfig) (*SshShell, error) {
	client, closeAll, err := dialSshChain(cfg.Credential, cfg.Proxy)
	if err != nil {
		return nil, err
	}

	shell, err := NewSshShellFromClient(client, cfg)
	if err != nil {
		closeAll()
		return nil, err
	}
	shell.ownClient = true
	shell.closeAll = closeAll
	return shell, nil
}

// NewSshShellFromClient 基于已有的 SSH 连接创建 Shell(调用方自行管理连接的关闭)。
func NewSshShellFromClient(client *ssh.Client, cfg SshConfig) (*SshShell, error) {
	cfg = cfg.normalize()

	addr := client.RemoteAddr().String()
	session, err := client.NewSession()
	if err != nil {
		return nil, &core.Error{Op: core.OpSession, Addr: addr, Err: err}
	}

	echo := util.IfInt(cfg.Echo, 1, 0)
	if err = session.RequestPty(cfg.Term, cfg.TermHeight, cfg.TermWidth, ssh.TerminalModes{
		ssh.ECHO:          uint32(echo),
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = session.Close()
		return nil, &core.Error{Op: core.OpTerm, Addr: addr, Err: err}
	}

	pIn, _ := session.StdinPipe()
	pOut, _ := session.StdoutPipe()
	pErr, _ := session.StderrPipe()

	if err = session.Shell(); err != nil {
		_ = session.Close()
		return nil, &core.Error{Op: core.OpShell, Addr: addr, Err: err}
	}
	r := core.NewReader(pIn, pOut, pErr, cfg.Config)

	// 此时可能会有一些输出(欢迎信息、日志打印、密码修改提示等)，需要读取并处理，防止影响后续操作。
	//	对于密码修改提示：总是答复否，不自动修改密码
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var headLine []string
	_ = r.ReadUntilPrompt(ctx, func(lines []string) {
		headLine = append(headLine, lines...)
	}, interceptor.AlwaysNo())
	headLine = trimEmptyLines(headLine)

	return &SshShell{shellBase: shellBase{rw: r}, client: client, session: session, headLine: headLine}, nil
}

// SshShell SSH 交互式 Shell
type SshShell struct {
	shellBase
	client    *ssh.Client
	session   *ssh.Session
	sftpCli   *sftp.Client
	sftpMu    sync.Mutex // 保护 sftpCli(SFTP取消时会在其他 goroutine 中重建)
	closeAll  func()     // 级联关闭(目标+跳板机链)
	ownClient bool
	headLine  []string
}

func (s *SshShell) Client() *ssh.Client {
	return s.client
}

func (s *SshShell) Session() *ssh.Session {
	return s.session
}

// HeadLine 返回登录后的横幅信息(欢迎信息等)
func (s *SshShell) HeadLine() []string {
	return s.headLine
}

// Close 关闭(幂等)：先关闭 SFTP/会话，再按所有权关闭目标与跳板机连接。
func (s *SshShell) Close() (err error) {
	s.sftpMu.Lock()
	if s.sftpCli != nil {
		if e := s.sftpCli.Close(); e != nil && err == nil {
			err = e
		}
		s.sftpCli = nil
	}
	s.sftpMu.Unlock()

	if s.session != nil {
		if e := s.session.Close(); e != nil && err == nil {
			err = e
		}
		s.session = nil
	}
	if s.client != nil {
		if s.ownClient {
			if e := s.client.Close(); e != nil && err == nil {
				err = e
			}
		}
		s.client = nil
	}
	if s.closeAll != nil {
		if s.ownClient {
			s.closeAll()
		}
		s.closeAll = nil
	}
	return s.shellBase.Close()
}
