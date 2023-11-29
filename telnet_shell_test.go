package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/core"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	netCredCiscoTelnet = &TelnetCredential{
		Host:     "192.168.2.14",
		Port:     23,
		User:     "admin",
		Password: "geesunn123",
	}
)

func TestTelnetShell_NetDevice_Cisco(t *testing.T) {
	s, err := NewTelnetShell(&TelnetShellConfig{
		Credential: netCredCiscoTelnet,
		Config: core.Config{
			ShowPrompt: true,
		},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}
	util.PrintTimeLn(s.Prompt())

	var out []string
	for _, cmd := range []string{
		"show version",
		"show ip int bri",
		"show ip route",
		"show run",
	} {
		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	if out = misc.TrimEmptyLine(out); len(out) != 0 {
		assert.Equal(t, "end", out[len(out)-1])
	}
}
