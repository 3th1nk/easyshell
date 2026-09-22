# EasyShell v2

* 支持本地执行命令(windows/linux)、通过 SSH/TELNET 协议在主机与网络设备上远程执行交互式命令
* 统一的 `Shell` 接口抽象：CmdShell/SshShell/TelnetShell 可互换使用
* 支持自定义提示符匹配规则，默认规则支持自动纠正(AutoPrompt)；提示符/拦截器匹配只扫尾部窗口，超大配置行(16MB+)无线性退化
* 默认自动识别 GB18030 编码并转换为 UTF8，支持自定义解码器；只解码完整行，多字节字符跨网络分包不被截断
* 有状态字符过滤器：转义序列(含 OSC/DCS 等字符串序列)跨分包安全剔除、退格按 rune 回退、EL/ED 擦除实时作用于当前行、CRLF 归一化(含 H3C `\r\r\n`)
* 内置拦截器：密码交互(Password)、选项交互(Yes/No)、网络设备自动翻页(More)、继续执行(Continue)；拦截器命中后先丢弃未完成行再写入应答，无内容粘连
* 延迟返回输出内容(按时间间隔或累计大小)；stderr 三种处理策略(Error/Output/Ignore)
* 录制原始输入输出并回放(record 包，二进制帧格式，含时间戳与方向)
* 设备错误检测：命令解析错误(H3C/Cisco/华为/Juniper)命中即报 *core.DeviceError，Fail/Collect 两种策略；登录横幅不参与检测
* 跳板机/堡垒机：多级代理链(SshConfig.Proxy)；SSH Agent 认证(UseAgent)
* 命令级提示符覆盖(RunPrompt)：提示符动态变化场景严格匹配；配置模式状态推断(InConfigMode)
* 结构化日志钩子(Config.Logger *slog.Logger)：会话关键事件接入运维日志体系
* 多命令脚本(RunScript)：顺序执行、失败定位到命令
* SFTP 文件上传(临时文件+rename 原子写)/下载/递归删除
* 并发安全：读取过程中可并发查询 Prompt/IsPrompt；并发 Read 被拒绝并返回明确错误
* 离线可测试：内置 mock SSH/Telnet/SFTP 服务(internal/testsrv)

## 安装

```
go get github.com/3th1nk/easyshell/v2
```

要求 Go 1.21+。

## 代码片段

- SSH 执行命令
```go
    s, err := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: easyshell.SshCredential{
            Host: "192.0.2.1", Port: 22, User: "admin", Password: "***",
            Timeout: 5 * time.Second, InsecureAlgorithms: true,
        },
    })
    if err != nil {
        return err
    }
    defer s.Close()

    ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()

    // Run = 写命令 + 读输出直到提示符
    err = s.Run(ctx, "display version", func(lines []string) {
        for _, line := range lines {
            fmt.Println(line)
        }
    })
```

- 密码交互(su/sudo 场景)
```go
    err = s.Run(ctx, "su root", nil,
        interceptor.Password("Password:", rootPassword))
    _, err = easyshell.ExitCode(ctx, s) // echo $? 获取上一条命令退出码
```

- 提示符动态变化的场景(进入配置模式、su/sudo)：命令级提示符覆盖 + 会话状态推断
```go
    // RunPrompt 指定本次命令的结束提示符(严格匹配，默认宽松规则不参与)：
    //  既避免宽松规则误匹配输出，也避免变化后的新提示符匹配不到而超时
    err = s.RunPrompt(ctx, "system-view", regexp.MustCompile(`\[SW03[\]-][^>]*>\s*$`), onOut)
    if s.InConfigMode() { // 启发式：H3C/华为 [name] 视图、Cisco (config) 样式
        // 配置模式下继续执行命令...
        err = s.Run(ctx, "quit", nil)
    }

- 跳板机/堡垒机(多级链)
```go
    s, err := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: targetCred,
        Proxy: &easyshell.ProxyConfig{ // 多级链: Proxy 自身可再指定 Proxy
            Credential: bastionCred,
        },
    })
```

- 多命令脚本(失败定位到命令)
```go
    err = easyshell.RunScript(ctx, s, func(cmd string, lines []string) {
        fmt.Println("==", cmd)
    }, "screen-length disable", "display version", "display clock")
```

- 录制导出为 asciinema(可直接用播放器回放)
```go
    record.DumpAsciinema(recFile, os.Stdout) // 输入帧不导出，防敏感信息泄露
```

- 设备错误检测(输出中的命令解析错误自动失败)
```go
    // 默认开启：命中 H3C/Cisco/华为/Juniper 命令解析错误即返回 *core.DeviceError
    //  (登录横幅不参与检测，登录即输出错误样式信息的设备不会误报)
    err = s.Run(ctx, "disp lay ver", onOut)
    var de *core.DeviceError
    if errors.As(err, &de) {
        fmt.Println("命令错误:", de.Cmd, de.Line, de.PatternName)
    }

    // 策略与规则可配(Config)：
    cfg.ErrorPolicy = easyshell.ErrorCollect       // 命中不中断，结束时聚合返回
    cfg.ErrorPatterns = append(core.DefaultErrorPatterns(),
        core.NewErrorPattern("my-error", `(?i)^my custom error`))
    cfg.ErrorPatterns = []*core.ErrorPattern{}     // 显式关闭检测
```

- 禁用分页(获取长配置推荐先禁用分页，比 More 逐页应答更快)
```go
    s.Run(ctx, easyshell.PagingDisableCommand(easyshell.VendorH3C), nil) // screen-length disable
    s.Run(ctx, "display saved-configuration", onOut)                     // More 拦截器仍作为兜底
```

- Telnet / 本地命令
```go
    ts, err := easyshell.NewTelnetShell(easyshell.TelnetConfig{
        Credential: easyshell.TelnetCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
    })

    cs, err := easyshell.NewCmdShell(ctx, easyshell.CmdConfig{Command: "ping www.baidu.com"})
```

- 录制与回放
```go
    // 方式一(推荐)：Record 配置——填路径即可，元数据自动填充、随 Close 自动收尾
    s, _ := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: cred,
        Record:     &easyshell.RecordConfig{Path: "session.eshrec", CaptureInput: true},
    })
    // ... 执行命令 ...
    s.Close()

    // 方式二(高级)：record.Writer 手动接入 RawOut/RawIn，可实现自定义捕获管道
    rec, _ := record.NewFileWriter("session.eshrec",
        record.Meta{Host: host, Protocol: "ssh"}, record.Options{CaptureInput: true})
    _ = easyshell.Config{RawOut: rec, RawIn: rec.Input()}

    // 回放(按时序/倍速)，或 AsReader 接 core.Reader 做交互式重放
    player, _ := record.Open("session.eshrec")
    _ = player.Play(ctx, os.Stdout)
```

## 设备测试

真实设备的测试凭据通过环境变量或 `.env` 文件注入(已被 .gitignore 忽略)，未配置时相关测试自动跳过。变量格式与清单见 `test_cred_test.go` 顶部注释：

```
EASYSHELL_TEST_CISCO=admin:password@192.0.2.90
EASYSHELL_TEST_H3C=admin:password@192.0.2.4
```

测试分层：`TestMock*`(离线，默认执行) / `TestDevice_*`(真机，需凭据)。

## v1 → v2 迁移对照

| v1 | v2 |
|---|---|
| `NewSshShell(&SshShellConfig{Credential: &cred})` | `NewSshShell(SshConfig{Credential: cred})`(凭证为值类型) |
| `NewCmdShell("cmd")` | `NewCmdShell(ctx, CmdConfig{Command: "cmd"})`(返回 error，空命令报错) |
| `s.ReadToEndLine(30*time.Second, f, its...)` | `s.ReadUntilPrompt(ctx, f, its...)`(超时用 `context.WithTimeout`) |
| `s.ReadAll(30*time.Second, f, its...)` | `s.ReadAll(ctx, f, its...)` |
| `s.Write(cmd)` + `s.Read(ctx, true, f, its...)` | 保持不变；或直接用 `s.Run(ctx, cmd, f, its...)` |
| `s.IsEndLine(s)` | `s.IsPrompt(s)` |
| `s.Stop()` | 删除，统一为 `Close()`(幂等) |
| `core.IsTimeout(err)` / `err.(*core.Error).Timeout()` | 仅 `core.IsTimeout(err)`(方法套已删除)；新增 `core.OpOf(err)` |
| telnet 裸错误 | `core.Error{Op: dial/auth/read}` 可程序化判断 |
| `replay.NewWriter(path)`(吞错误返回 nil) | `record.NewFileWriter(path, meta, opts...)`(返回 error)，二进制帧格式 |
| `filter.NewDefaultFilter(opt)` / `filter.IFilter.Do` | `filter.NewFilter(opts ...Options)` / `filter.FilterFunc` 或实现 `filter.Filter`(有状态) |
| `filter.CrTrimModeOnlyCr` | `filter.CRTrimDropCR` |
| `sftp.Upload(l, r, true)` | `s.SftpUpload(l, r, SftpOptions{Force: true})`(上传原子化) |
| `telnet.ClientConfig.UserRegex/PassRegex/PromptRegex` | `telnet.Config.LoginUserRegex/LoginPassRegex/LoginPromptRegex` |
| `telnet.Client.ReadUtil/ReadUtil2/SkipUtil*` | `ReadUntil/SkipUntil`(其余删除或私有化) |
| `telnet.Client.FirstPrompt()` | `telnet.Client.Prompt()` |
| 内嵌 `shell.ReadWriter`(透传使用) | 不再暴露；`Shell` 接口覆盖原有用法 |

## 已知限制

- 多行拦截器命中时，匹配窗口内的前缀行不会作为输出返回(v1 即有，保持不变)
- EL/ED 擦除按"清空当前行"的保守语义处理(无列位置信息)，可用 `filter.Options.Erase=false` 关闭
- Decoder 契约：目标编码的多字节字符中不能出现 0x0A 字节(GBK/GB18030/UTF-8 满足，UTF-16 不适用)
- 录制文件为明文(含密码帧)，请妥善保管；Dump 时 secret 帧默认打码
