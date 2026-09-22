# EasyShell
* 支持本地执行命令(windows/linux)
* 支持通过SSH/TELNET协议在主机、网络设备上远程执行交互式命令
* 支持自定义提示符匹配规则，大多数情况下使用默认提示符规则即可，使用默认提示符规则时可开启自动纠正(基于默认规则首次匹配结果，默认关闭)
* 支持自定义解码器，默认自动识别GB18030编码并转换成UTF8
* 支持自定义字符过滤器，默认自动处理退格、CRLF自动转换为LF，并剔除CSI控制字符(部分情况未处理，如：ISO 8613-3和ISO 8613-6中24位前景色和背景色设置)
* 支持自定义内容拦截器，内置拦截器包括密码交互(Password)、选项交互(Yes/No)、网络设备自动翻页(More)、网络设备继续执行(Continue)
* 支持延迟返回输出内容，可指定超过一定时间 或 内容大小 后返回
* 支持记录原始输出内容和回放，用于调试
* 并发安全：读取过程中可并发查询提示符；并发Read会被拒绝并返回错误

## 代码片段
- 本地执行命令
```
    s := NewCmdShell("ping www.baidu.com")
    if err := s.ReadAll(time.Minute, func(lines []string) {
        // handle lines
    }); err != nil {
        return err
    }
    
    ...
    
    s2 := NewCmdShell("cmd /K")
    for _, cmd := range []string{"c:", "dir"} {
        s2.Write(cmd)
        if err := s2.ReadToEndLine(time.Minute, func(lines []string) {
            // handle lines
        }); err != nil {
            return err
        }
    }
```

- 远程(SSH)执行命令
```
    cred := SshCredential{
        Host:       "192.0.2.2",
        Port:       22,
        User:       "zhangsan",
        Password:   "<password>",
        PrivateKey: "",
        InsecureAlgorithms: true,
        Timeout:    5 * time.Second,
    }
    s, err := NewSshShell(&SshShellConfig{
        Credential: &cred,
    })
    if err != nil {
        return
    }
    defer s.Close()
    
    for _, line := range s.HeadLine() {
        fmt.Println(line)
    }
	
    // Run = Write + ReadToEndLine，执行命令并读取输出直到提示符
    if err := s.Run("display version", time.Minute, func(lines []string) {
        // handle lines
    }); err != nil {
        return
    }

    // match 'password' prompt and enter the password automatically
    s.Write("su root")
    if err := s.ReadToEndLine(time.Minute, func(lines []string) {
        // handle lines
    }, interceptor.Password("password:", "<password>", true)); err != nil {
        return err
    }
```

- 设备测试凭据配置

  真实设备的测试凭据通过环境变量或仓库根目录的 `.env` 文件注入(该文件已被.gitignore忽略)，
  未配置时相关测试自动跳过。变量格式与清单见 `test_cred_test.go` 顶部注释：
```
    EASYSHELL_TEST_CISCO=admin:password@192.0.2.90
    EASYSHELL_TEST_H3C=admin:password@192.0.2.4
```
