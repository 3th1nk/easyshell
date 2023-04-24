package easyshell

import (
	"github.com/3th1nk/easygo/util"
	"github.com/3th1nk/easygo/util/arrUtil"
	"github.com/3th1nk/easyshell/internal/_test"
	"github.com/3th1nk/easyshell/pkg/injector"
	"github.com/3th1nk/easyshell/pkg/reader"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

var (
	hostCred = &SshCred{
		Host:     "192.168.1.2",
		Port:     22,
		User:     "admin",
		Password: "123456",
	}
	netCred = &SshCred{
		Host:     "192.168.2.14",
		Port:     22,
		User:     "admin",
		Password: "123456",
	}
)

func TestSshShell_Term(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{
		Term: "VT100",
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

	assert.True(t, _test.HasLine(out, "declare -x"))
	assert.True(t, _test.HasLine(out, "LANG="))
}

func TestSshShell_Ping(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -c 4 -i 0.5",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
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

	assert.Equal(t, 4, _test.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, _test.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, _test.HasLine(out, "4 packets transmitted"))
}

func TestSshShell_PingLazyInterval(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{
		Config: reader.Config{LazyOutInterval: 2 * time.Second},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 5",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
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

	assert.LessOrEqual(t, 20, _test.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, _test.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, _test.HasLine(out, "rtt min/avg/max/mdev ="))
}

func TestSshShell_PingLazySize(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{
		Config: reader.Config{LazyOutSize: 200},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 4",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
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

	assert.LessOrEqual(t, 12, _test.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, _test.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, _test.HasLine(out, "packets transmitted"))
}

func TestSshShell_PingLazy(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{
		Config: reader.Config{LazyOutInterval: time.Second, LazyOutSize: 200},
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	cmdList := []string{
		"ping baidu.com -i 0.25 -w 4",
	}

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
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

	assert.LessOrEqual(t, 16, _test.LineCount(out, "bytes from", "): icmp_seq="))
	assert.True(t, _test.HasLine(out, "--- baidu.com ping statistics ---"))
	assert.True(t, _test.HasLine(out, "rtt min/avg/max/mdev ="))
}

func TestSshShell_ReadInput(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	s.Write(`echo -e "请输入一个数字:\n" && read num && echo "你输入的数字是: $num"`)

	var out []string
	pwdInjector, _ := injector.Password("请输入一个数字", "123", true)
	s.ReadToEndLine(time.Minute, func(lines []string) {
		out = append(out, lines...)
		for _, line := range lines {
			util.PrintTimeLn(line)
		}
	}, pwdInjector)

	assert.True(t, arrUtil.ContainsString(out, "你输入的数字是: 123"))
}

func TestSshShell_Sudo(t *testing.T) {
	s, err := NewSshShell(hostCred, &SshShellConfig{})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
		util.PrintTimeLn(line)
	}

	var out []string

	for _, cmd := range []string{
		"whoami",
		"su root",
		"whoami",
	} {
		var pwdInjector injector.InputInjector
		if cmd == "su root" {
			pwdInjector, _ = injector.Password("password:", "123456", true)
		}

		util.Println("======================================================= %v", cmd)
		assert.NoError(t, s.Write(cmd))
		err = s.ReadToEndLine(time.Minute, func(lines []string) {
			out = append(out, lines...)
			for _, line := range lines {
				util.PrintTimeLn(line)
			}
		}, pwdInjector)
		if err == io.EOF {
			util.PrintTimeLn("-> EOF")
		} else if err != nil {
			util.PrintTimeLn("-> error: %v", err)
		}
	}

	assert.True(t, _test.Contains(out, "password:"))
	assert.True(t, arrUtil.ContainsString(out, "root"))
}

func TestSshShell_NetDevice_Cisco(t *testing.T) {
	s, err := NewSshShell(netCred, &SshShellConfig{
		TermHeight: 10,
	})
	if !assert.NoError(t, err) {
		return
	}
	defer s.Close()

	util.Println("======================================================= first line:")
	for _, line := range s.PopHeadLine() {
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

	if out = _test.TrimEmptyLine(out); len(out) != 0 {
		assert.Equal(t, "end", out[len(out)-1])
	}
}
