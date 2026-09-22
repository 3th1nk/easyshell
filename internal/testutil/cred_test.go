package testutil

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFromEnv(t *testing.T) {
	for _, obj := range []struct {
		name   string
		value  string
		expect *Credential
	}{
		{"empty", "", nil},
		{"host_only", "192.0.2.3", &Credential{Host: "192.0.2.3"}},
		{"user_pass_host", "admin:passw0rd@192.0.2.3", &Credential{Host: "192.0.2.3", User: "admin", Password: "passw0rd"}},
		{"no_user", ":passw0rd@192.0.2.3", &Credential{Host: "192.0.2.3", Password: "passw0rd"}},
		{"no_pass", "admin@192.0.2.3", &Credential{Host: "192.0.2.3", User: "admin"}},
		{"with_port", "admin:passw0rd@192.0.2.3:2222", &Credential{Host: "192.0.2.3", Port: 2222, User: "admin", Password: "passw0rd"}},
		// password 中包含 '@'、':'，按最后一个 '@' 分隔、第一个 ':' 分隔
		{"at_in_password", "admin:pa@ss:w0rd@192.0.2.3", &Credential{Host: "192.0.2.3", User: "admin", Password: "pa@ss:w0rd"}},
		{"trim_space", "  admin:passw0rd@192.0.2.3  ", &Credential{Host: "192.0.2.3", User: "admin", Password: "passw0rd"}},
		// 非法端口按无端口处理
		{"invalid_port", "admin:passw0rd@192.0.2.3:abc", &Credential{Host: "192.0.2.3:abc", User: "admin", Password: "passw0rd"}},
	} {
		t.Run(obj.name, func(t *testing.T) {
			t.Setenv("EASYSHELL_TEST_UNITTEST", obj.value)
			assert.Equal(t, obj.expect, FromEnv("UNITTEST"))
		})
	}
}
