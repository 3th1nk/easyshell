package easyshell

import (
	"context"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easyshell/core"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
	"time"
)

// 设备凭据从环境变量注入(见 test_cred_test.go 顶部的环境变量说明)，未设置时相关测试跳过
var (
	netCredCiscoTelnet = telnetCredFromEnv("CISCO")
	netCredH3CTelnet   = telnetCredFromEnv("H3C")
)

func TestTelnetShell_NetDevice_Cisco(t *testing.T) {
	if netCredCiscoTelnet == nil {
		t.Skip("set EASYSHELL_TEST_CISCO to run this test")
	}
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

func TestTelnetShell_NetDevice_H3C(t *testing.T) {
	if netCredH3CTelnet == nil {
		t.Skip("set EASYSHELL_TEST_H3C to run this test")
	}
	s, err := NewTelnetShell(&TelnetShellConfig{
		Credential: netCredH3CTelnet,
		Config: core.Config{
			PromptRegex: []*regexp.Regexp{
				regexp.MustCompile(`WorkSW03[\s\S]*[$#%>\]:]+\s*$`),
			},
			ReadConfirmWait: 500 * time.Millisecond,
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
		"display saved-configuration",
	} {
		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = s.Read(ctx, true, func(lines []string) {
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
		cancel()
	}

	if out = misc.TrimEmptyLine(out); len(out) != 0 {
		assert.Equal(t, "end", out[len(out)-1])
	}
}
