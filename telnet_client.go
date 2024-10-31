package easyshell

import (
	"fmt"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/pkg/telnet"
	"time"
)

type TelnetCredential struct {
	Host     string `json:"host"`               // IP地址
	Port     int    `json:"port,omitempty"`     // 端口，默认23
	User     string `json:"user,omitempty"`     // 用户名，可选
	Password string `json:"password,omitempty"` // 密码，可选
	Timeout  int    `json:"timeout,omitempty"`  // 连接超时时间（秒），默认15秒
}

func NewTelnetClient(cred *TelnetCredential) (*telnet.Client, error) {
	timeout := util.IfEmptyInt(cred.Timeout, 15)
	return telnet.NewClient(&telnet.ClientConfig{
		Addr:     fmt.Sprintf("%s:%d", cred.Host, util.IfEmptyInt(cred.Port, 23)),
		User:     cred.User,
		Password: cred.Password,
		Timeout:  time.Duration(timeout) * time.Second,
	})
}
