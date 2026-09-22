package easyshell_test

import (
	"context"
	"fmt"
	"github.com/3th1nk/easyshell/v2"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/filter"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"log"
	"time"
)

// 通过 SSH 连接设备并执行命令
func ExampleNewSshShell() {
	s, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{
			Host: "192.0.2.1", Port: 22, User: "admin", Password: "***",
			Timeout: 5 * time.Second,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = s.Run(ctx, "display version", func(lines []string) {
		for _, line := range lines {
			fmt.Println(line)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}

// 执行命令并获取退出码
func ExampleExitCode() {
	s, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	_ = s.Run(ctx, "ls /not-exist", nil)
	code, err := easyshell.ExitCode(ctx, s)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("exit code:", code)
}

// 拦截器自动应答(su 密码交互)
func ExampleShell_Run() {
	s, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = s.Run(ctx, "su root", func(lines []string) {
		for _, line := range lines {
			fmt.Println(line)
		}
	}, easyshell.RunOptions{Interceptors: []interceptor.Interceptor{
		interceptor.Password(core.PasswordRegex.String(), "rootPassword"),
	}})
	if err != nil {
		log.Fatal(err)
	}
}

// 本地执行命令
func ExampleNewCmdShell() {
	s, err := easyshell.NewCmdShell(context.Background(), easyshell.CmdConfig{Command: "go version"})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = s.ReadAll(ctx, func(lines []string) {
		for _, line := range lines {
			fmt.Println(line)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}

// 字符过滤器：剔除回车与退格
func ExampleNewFilter() {
	f := filter.NewFilter()
	out := f.Push([]byte("a\rb\nc\b\n"))
	fmt.Print(string(out))
	// Output:
	// b
}
