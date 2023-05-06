# EasyShell
* 支持本地执行命令(windows/linux)
* 支持通过SSH在主机、网络设备上远程执行交互式命令
* 支持自定义解码器，未指定时自动识别GB18030编码并转换成UTF8
* 支持自定义字符过滤器，未指定时自动处理退格，并剔除CSI控制字符(部分情况未处理，如：ISO 8613-3和ISO 8613-6中24位前景色和背景色设置)
* 支持自定义提示符匹配规则，未指定时使用默认提示符规则，并支持基于默认规则的匹配内容自动纠正
* 支持自定义对指定输出内容的交互行为，内置密码输入的交互方法
* 支持延迟返回输出内容，延迟策略包括 超过指定间隔 或 输出内容超过指定长度
* 自动处理网络设备翻页(More)、继续执行(Continue)的场景

## 代码片段
- 本地执行命令
```
    s := NewCmdShell("ping www.baidu.com", &CmdShellConfig{})
    if err := s.ReadAll(time.Minute, func(lines []string) {
        // handle lines
    }); err != nil {
        return err
    }
    
    ...
    
    s2 := NewCmdShell("cmd /K", &CmdShellConfig{})
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
    cred := SshCred{
        Host:       "192.168.1.2",
        Port:       22,
        User:       "zhangsan",
        Password:   "123456",
        PrivateKey: "",
        Timeout:    5,
    }
    s, err := NewSshShell(&cred, &SshShellConfig{})
    if err != nil {
        return
    }
    defer s.Close()
    
    for _, line := range s.PopHeadLine() {
        fmt.Println(line)
    }
	
    // match 'password' prompt and enter the password automatically
    s.Write("su root")
    pwdInjector, _ = injector.Password("password:", "123456", true)
    if err := s.ReadToEndLine(time.Minute, func(lines []string) {
        // handle lines
    }, pwdInjector); err != nil {
        return err
    }
```