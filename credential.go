package easyshell

import "time"

// SshCredential SSH 登录凭证
type SshCredential struct {
	Host               string        `json:"host"`                          // IP地址
	Port               int           `json:"port,omitempty"`                // 端口，默认22
	User               string        `json:"user,omitempty"`                // 用户名
	Password           string        `json:"password,omitempty"`            // 密码。当密钥/Agent与密码同时存在时，优先使用密钥/Agent。
	PrivateKey         string        `json:"private_key,omitempty"`         // 密钥。当密钥与密码同时存在时，优先使用密钥。
	UseAgent           bool          `json:"use_agent,omitempty"`           // 使用SSH Agent认证(认证方式优先级：密钥 → Agent → 密码)
	AgentSocket        string        `json:"agent_socket,omitempty"`        // Agent的unix socket路径，空值使用环境变量 SSH_AUTH_SOCK
	Timeout            time.Duration `json:"timeout,omitempty"`             // 连接超时时间，默认15秒
	InsecureAlgorithms bool          `json:"insecure_algorithms,omitempty"` // 是否允许不安全的算法(老旧网络设备需要)
	Fingerprint        string        `json:"fingerprint,omitempty"`         // 公钥指纹(SHA256，base64)，用于验证服务器身份
}

// TelnetCredential Telnet 登录凭证
type TelnetCredential struct {
	Host     string        `json:"host"`               // IP地址
	Port     int           `json:"port,omitempty"`     // 端口，默认23
	User     string        `json:"user,omitempty"`     // 用户名，可选
	Password string        `json:"password,omitempty"` // 密码，可选
	Timeout  time.Duration `json:"timeout,omitempty"`  // 连接超时时间，默认15秒
}
