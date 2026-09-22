package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestVendorProfileOf(t *testing.T) {
	for _, v := range []Vendor{VendorH3C, VendorCiscoIOS, VendorCiscoNXOS, VendorHuawei, VendorJuniper, VendorRuijie, VendorHPComware} {
		p := VendorProfileOf(v)
		if !assert.NotNil(t, p, string(v)) {
			continue
		}
		assert.NotEmpty(t, p.PagingDisable, string(v))
		assert.NotEmpty(t, p.SaveConfigCmd, string(v))
		assert.Equal(t, v, p.Vendor)
	}
	assert.Nil(t, VendorProfileOf("unknown-vendor"))
}

// TestSaveConfig_UnknownVendor 未知厂商报错
func TestSaveConfig_UnknownVendor(t *testing.T) {
	srv := testsrv.NewSshServer(t)
	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.Error(t, SaveConfig(ctx, s, "unknown-vendor", nil))
}

// TestSaveConfig_Mock 离线验证保存配置(华为式确认交互)
func TestSaveConfig_Mock(t *testing.T) {
	srv := testsrv.NewSshServer(t)

	// 自定义会话：save 命令触发确认提示，回答 y 后完成
	srv.Session = func(ss *testsrv.SshSession) {
		write := func(str string) { _, _ = ss.Stdout.Write([]byte(str)) }
		write(srv.Prompt)
		buf := make([]byte, 4096)
		for {
			n, err := ss.Stdin.Read(buf)
			if err != nil {
				return
			}
			cmd := string(buf[:n])
			if n > 0 && (cmd[0] == 'y' || cmd[0] == 'Y') {
				write("Configuration saved.\n" + srv.Prompt)
				continue
			}
			write("This will overwrite the startup configuration. Are you sure? [Y/N]:")
		}
	}

	s, err := NewSshShell(SshConfig{
		Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	// 华为 profile 带 Y/N 确认交互
	assert.NoError(t, SaveConfig(context.Background(), s, VendorHuawei, nil))
}
