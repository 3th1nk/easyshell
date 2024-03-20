package easyshell

import (
	"context"
	"errors"
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easygo/util/arrUtil"
	"github.com/3th1nk/easyshell/core"
	"github.com/3th1nk/easyshell/internal/misc"
	"github.com/3th1nk/easyshell/pkg/interceptor"
	"github.com/3th1nk/easyshell/pkg/replay"
	"github.com/stretchr/testify/assert"
	"io"
	"regexp"
	"testing"
	"time"
)

var (
	hostCred = &SshCredential{
		Host:     "172.16.66.42",
		Port:     22,
		User:     "root",
		Password: "geesunn@123",
	}
	netCredCisco = &SshCredential{
		Host:               "192.168.2.14",
		Port:               22,
		User:               "admin",
		Password:           "geesunn123",
		InsecureAlgorithms: true,
	}
	netCredArray = &SshCredential{
		Host:               "192.168.1.16",
		Port:               22,
		User:               "array",
		Password:           "admin",
		InsecureAlgorithms: true,
	}
	netCredH3C = &SshCredential{
		Host:               "192.168.2.3",
		Port:               22,
		User:               "admin",
		Password:           "geesunn123",
		InsecureAlgorithms: true,
	}
)

func TestSshShell_Term(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	s.Write(`export | grep TERM && locale`)

	var out []string
	s.ReadToEndLine(time.Minute, func(lines []string) {
		out = append(out, lines...)
		for _, line := range lines {
			util.PrintTimeLn(line)
		}
	})

	assert.True(t, misc.HasLine(out, "declare -x"))
	assert.True(t, misc.HasLine(out, "LANG="))
}

func TestSshShell_Ping(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -c 4 -i 0.5",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	start := time.Now()
	var out []string
	for _, cmd := range cmdList {
		util.Println("======================================================= %v", cmd)
		s.Write(cmd)
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.Equal(t, 4, misc.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, misc.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, misc.HasLine(out, "4 packets transmitted"))
}

func TestSshShell_PingLazyInterval(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
		Config:     core.Config{LazyOutInterval: 2 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 5",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string
	start := time.Now()
	for _, cmd := range cmdList {
		util.Println("======================================================= %v", cmd)
		s.Write(cmd)
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.LessOrEqual(t, 20, misc.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, misc.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, misc.HasLine(out, "rtt min/avg/max/mdev ="))
}

func TestSshShell_PingLazySize(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
		Config:     core.Config{LazyOutSize: 200},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 4",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string
	start := time.Now()
	for _, cmd := range cmdList {
		util.Println("======================================================= %v", cmd)
		s.Write(cmd)
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.LessOrEqual(t, 12, misc.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, misc.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, misc.HasLine(out, "packets transmitted"))
}

func TestSshShell_PingLazy(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
		Config:     core.Config{LazyOutInterval: time.Second, LazyOutSize: 200},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 4",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string
	start := time.Now()
	for _, cmd := range cmdList {
		util.Println("======================================================= %v", cmd)
		s.Write(cmd)
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	util.PrintTimeLn("End: took=%v", time.Since(start))

	assert.LessOrEqual(t, 16, misc.LineCount(out, "bytes from", "icmp_seq="))
	assert.True(t, misc.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, misc.HasLine(out, "rtt min/avg/max/mdev ="))
}

func TestSshShell_Cancel(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
		Config:     core.Config{LazyOutInterval: time.Second, LazyOutSize: 200},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()

	var out []string
	start := time.Now()
	_ = s.Write("top -b")
	err = s.Read(ctx, true, func(lines []string) {
		out = append(out, lines...)
		for _, line := range lines {
			util.PrintTimeLn(line)
		}
	})
	if err == io.EOF {
		util.PrintTimeLn("-> EOF")
	} else if err != nil {
		util.PrintTimeLn("-> error: %v", err)
		assert.True(t, errors.Is(err, context.Canceled))
	}
	util.PrintTimeLn("End: took=%v", time.Since(start))
}

func TestSshShell_ReadInput(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: hostCred,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	s.Write(`echo -e "请输入一个数字:\n" && read num && echo "你输入的数字是: $num"`)

	var out []string
	pwdInterceptor := interceptor.Password("请输入一个数字", "123", true)
	s.ReadToEndLine(time.Minute, func(lines []string) {
		out = append(out, lines...)
		for _, line := range lines {
			util.PrintTimeLn(line)
		}
	}, pwdInterceptor)

	assert.True(t, arrUtil.ContainsString(out, "你输入的数字是: 123"))
}

func TestSshShell_Sudo(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Config: core.Config{
			ShowPrompt: true,
		},
		Credential: hostCred,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string
	for _, cmd := range []string{
		"whoami",
		"su root",
		"whoami",
	} {
		var arr []interceptor.Interceptor
		if cmd == "su root" {
			pwdInterceptor := interceptor.Password("password:", "123456", true)
			arr = append(arr, pwdInterceptor)
		}

		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		}, arr...)
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	assert.True(t, misc.Contains(out, "password:"))
	assert.True(t, arrUtil.ContainsString(out, "root"))
}

func TestSshShell_HostScript_Err(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Config: core.Config{
			ShowPrompt: true,
		},
		Credential: hostCred,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string
	for _, cmd := range []string{
		"bash /root/linux_add.sh",
	} {
		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}
}

func TestSshShell_NetDevice_Cisco(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: netCredCisco,
		Config: core.Config{
			ShowPrompt: true,
		},
		TermHeight: 10,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

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

func TestSshShell_NetDevice_Array(t *testing.T) {
	s, err := NewSshShell(&SshShellConfig{
		Credential: netCredArray,
		Config: core.Config{
			ShowPrompt: true,
		},
		Echo:       true,
		TermHeight: 10,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.HeadLine() {
		util.PrintTimeLn(line)
	}

	for _, cmd := range []string{
		"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", // 超长输入触发缩进
	} {
		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		})
		if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}
}

func TestSshShell_NetDevice_H3C(t *testing.T) {
	ro := replay.NewWriter("./pkg/replay/testdata/WorkSW03_ssh.data")
	defer ro.Close()

	s, err := NewSshShell(&SshShellConfig{
		Credential: netCredH3C,
		Config: core.Config{
			PromptRegex: []*regexp.Regexp{
				regexp.MustCompile(`WorkSW03[\s\S]*[$#%>\]:]+\s*$`),
			},
			AutoPrompt:      true,
			ShowPrompt:      false,
			LazyOutInterval: 500 * time.Millisecond,
			LazyOutSize:     8192,
			RawOut:          ro,
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

	for _, cmd := range []string{
		"display saved-configuration",
		"display current-configuration",
	} {
		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
}
