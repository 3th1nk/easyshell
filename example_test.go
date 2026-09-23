package easyshell_test

import (
	"context"
	"fmt"
	"github.com/3th1nk/easyshell/v2"
	"github.com/3th1nk/easyshell/v2/filter"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"log"
	"regexp"
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
		interceptor.Password(easyshell.PasswordRegex.String(), "rootPassword"),
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

// 尽力禁用输出分页(厂商驱动)，失败不中断流程
func ExampleDisablePaging() {
	s, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 获取长配置前先禁用分页：设备不识别该命令时自动由 More 拦截器兜底
	if err := easyshell.DisablePaging(ctx, s, easyshell.VendorH3C); err != nil {
		log.Fatal(err)
	}
	_ = s.Run(ctx, "display saved-configuration", func(lines []string) {
		for _, line := range lines {
			fmt.Println(line)
		}
	})
}

// 命令级超时(独立于 ctx，慢命令单独放宽)
func ExampleRunOptions() {
	s, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()

	// Timeout 只约束本次调用，超时错误可经 easyshell.IsTimeout 判断；零值不限、跟随 ctx
	err = s.Run(ctx, "ping -c 100 192.0.2.254", nil, easyshell.RunOptions{Timeout: 2 * time.Minute})
	if err != nil && !easyshell.IsTimeout(err) {
		log.Fatal(err)
	}
}

// 自定义设备错误检测规则(厂商级规则合并接入)
func ExampleNewErrorPattern() {
	// 内置默认规则 + 厂商特有规则(如山石/Array 的 "% " 前缀错误样式)合并接入
	cfg := easyshell.Config{
		ErrorPatterns: easyshell.VendorErrorPatterns(easyshell.VendorHillstone),
	}
	// 再追加自定义规则(pattern 非法时 NewErrorPattern 返回 nil)
	cfg.ErrorPatterns = append(cfg.ErrorPatterns,
		easyshell.NewErrorPattern("my-error", `(?i)^my custom error`))

	_, err := easyshell.NewSshShell(easyshell.SshConfig{
		Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
		Config:     cfg,
	})
	if err != nil {
		log.Fatal(err)
	}
}

// Telnet 自定义登录正则(非标登录提示符设备)
func ExampleNewTelnetShell() {
	// 内置登录规则覆盖 login:/username:/password: 等标准提示符；
	// 非标设备(如 "Account:")可通过 LoginUserRegex/LoginPassRegex 自定义
	s, err := easyshell.NewTelnetShell(easyshell.TelnetConfig{
		Credential:     easyshell.TelnetCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
		LoginUserRegex: regexp.MustCompile(`(?i)account:\s*$`),
		LoginPassRegex: regexp.MustCompile(`(?i)secret:\s*$`),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	_ = s.Run(context.Background(), "show version", nil)
}
