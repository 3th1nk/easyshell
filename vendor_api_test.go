package easyshell

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/3th1nk/easyshell/v2/internal/testsrv"
	"github.com/stretchr/testify/assert"
)

// TestVendorErrorPatterns 厂商级错误规则接线：默认规则 + 厂商特有规则合并
func TestVendorErrorPatterns(t *testing.T) {
	// 山石：默认规则不含厂商级规则，合并后应命中 % Unrecognized(无 found at 后缀)
	def := DefaultErrorPatterns()
	for _, p := range def {
		if p.Pattern.MatchString("% Unrecognized command") {
			t.Fatalf("前置假设失败：默认规则已含山石样式")
		}
	}
	merged := VendorErrorPatterns(VendorHillstone)
	assert.Greater(t, len(merged), len(def), "应叠加厂商级规则")
	assert.NotNil(t, VendorErrorPatterns("not-exist"), "未知厂商返回默认规则")
	assert.Equal(t, len(def), len(VendorErrorPatterns("not-exist")))

	// 命中验证(真机样式，官方手册为裸文本)
	for _, line := range []string{"% Unrecognized command", "% Incomplete command", "% Ambiguous command", "% Invalid input"} {
		hit := false
		for _, p := range merged {
			if p.Pattern.MatchString(line) {
				hit = true
				break
			}
		}
		assert.True(t, hit, "山石规则应命中: %q", line)
	}
}

// TestDisablePaging 尽力禁用分页：识别失败不报错，硬错误仍然返回
func TestDisablePaging(t *testing.T) {
	// 场景1：设备正常识别
	srv := testsrv.NewSshServer(t)
	s, err := NewSshShell(SshConfig{Credential: SshCredential{Host: hostOf(srv.Addr), Port: portOf(srv.Addr), User: srv.User, Password: srv.Password, Timeout: 3 * time.Second}})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	assert.NoError(t, DisablePaging(ctx, s, VendorH3C))

	// 场景2：设备不识别禁用分页命令(错误检测命中) → 返回 nil 而非 DeviceError
	srv2 := testsrv.NewSshServer(t)
	srv2.Session = func(sess *testsrv.SshSession) {
		buf := make([]byte, 4096)
		for {
			n, err := sess.Stdin.Read(buf)
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(string(buf[:n]))
			out := "ok\n" + srv2.Prompt
			if strings.Contains(cmd, "screen-length") {
				out = "% Invalid input detected at '^' marker.\n" + srv2.Prompt
			}
			_, _ = sess.Stdout.Write([]byte(out))
		}
	}
	s2, err := NewSshShell(SshConfig{Credential: SshCredential{Host: hostOf(srv2.Addr), Port: portOf(srv2.Addr), User: srv2.User, Password: srv2.Password, Timeout: 3 * time.Second}})
	if !assert.NoError(t, err) {
		return
	}
	defer s2.Close()
	err = DisablePaging(ctx, s2, VendorH3C)
	assert.NoError(t, err, "设备不识别时应吞掉 DeviceError, got=%v", err)
	var de *DeviceError
	assert.False(t, errors.As(err, &de))

	// 场景3：未知厂商(无禁用分页命令) → 直接 nil 不发命令
	assert.NoError(t, DisablePaging(ctx, s, Vendor("not-exist")))
}
