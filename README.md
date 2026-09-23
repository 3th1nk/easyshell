# EasyShell v2

* 支持本地执行命令(windows/linux)、通过 SSH/TELNET 协议在主机与网络设备上远程执行交互式命令
* 统一的 `Shell` 接口抽象：CmdShell/SshShell/TelnetShell 可互换使用
* 支持自定义提示符匹配规则，默认规则支持自动纠正(AutoPrompt)；提示符/拦截器匹配只扫尾部窗口，超大配置行(16MB+)无线性退化
* 默认自动识别 GB18030 编码并转换为 UTF8，支持自定义解码器；只解码完整行，多字节字符跨网络分包不被截断
* 有状态字符过滤器：转义序列(含 OSC/DCS 等字符串序列)跨分包安全剔除、退格按 rune 回退、EL/ED 擦除实时作用于当前行、CRLF 归一化(含 H3C `\r\r\n`)
* 内置拦截器：密码交互(Password)、选项交互(AlwaysYes/AlwaysNo)、网络设备自动翻页(More)、继续执行(Continue)；拦截器命中后先丢弃未完成行再写入应答，无内容粘连
* 延迟返回输出内容(按时间间隔或累计大小)；stderr 三种处理策略(Error/Output/Ignore)
* 录制原始输入输出并回放(record 包，二进制帧格式，含时间戳与方向)
* 设备错误检测：命令解析错误(H3C/Cisco/华为/Juniper)命中即报 *easyshell.DeviceError，Fail/Collect 两种策略；登录横幅不参与检测
* 精简的接口参数：每次调用的可选项(提示符覆盖/拦截器)统一为 RunOptions 可变参数结构体
* 跳板机/堡垒机：多级代理链(SshConfig.Proxy)；SSH Agent 认证(UseAgent)
* 厂商驱动(VendorProfile)：禁用分页/保存配置(含确认交互)/错误模式整合，支持自定义厂商注册
* 长连接保活(Config.KeepAlive)：周期探测+失败阈值+死亡回调，防空闲断连
* 命令级提示符覆盖(RunOptions.Prompt)：提示符动态变化场景严格匹配；配置模式状态推断(InConfigMode)
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

## 包结构

调用方只需关心 4 个公共包，各司其职：

| 包 | 职责 |
|---|---|
| `easyshell`(本包) | Shell 接口与全部高层 API(唯一入口) |
| `interceptor` | 输出拦截器(密码/翻页/选项/自定义) |
| `filter` | 字符过滤器(默认内置，支持自定义实现) |
| `record` | 录制与回放 |

读取循环、错误类型等实现细节全部收敛在 `internal/`，通过本包的别名暴露
(`easyshell.Error`、`easyshell.IsTimeout`、`easyshell.RunOptions`、`easyshell.Config` 等)。

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
    // RunOptions.Prompt 指定本次命令的结束提示符(严格匹配，默认宽松规则不参与)：
    //  既避免宽松规则误匹配输出，也避免变化后的新提示符匹配不到而超时
    err = s.Run(ctx, "system-view", onOut,
        easyshell.RunOptions{Prompt: regexp.MustCompile(`\[SW03[\]-][^>]*\]\s*$`)})
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

- 厂商驱动与保存配置
```go
    s.Run(ctx, easyshell.VendorProfileOf(easyshell.VendorH3C).PagingDisable, nil)
    err = easyshell.SaveConfig(ctx, s, easyshell.VendorHuawei, nil) // 自动处理 Y/N 确认
```

- 长连接保活(防防火墙/NAT 空闲断连)
```go
    s, _ := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: cred,
        Config: easyshell.Config{KeepAlive: &easyshell.KeepAliveConfig{
            Interval: 30 * time.Second, // SSH keepalive 请求由库注入
            OnDead:   func(err error) { /* 重连/告警 */ },
        }},
    })
```

- 文件传输校验与进度
```go
    err = s.Upload(ctx, "local.bin", "/remote/path.bin", easyshell.TransferOptions{
        Force:    true, // 目标已存在时覆盖
        Progress: func(transferred, total int64) { fmt.Printf("\r%d/%d", transferred, total) },
    }) // 上传后自动 md5sum 校验(NoVerify 关闭)；协议自动选择，SFTP 不可用时降级 SCP
```

- 录制导出为 asciinema(可直接用播放器回放)
```go
    record.DumpAsciinema(recFile, os.Stdout) // 输入帧不导出，防敏感信息泄露
```

- 设备错误检测(输出中的命令解析错误自动失败)
```go
    // 默认开启：命中 H3C/Cisco/华为/Juniper 命令解析错误即返回 *easyshell.DeviceError
    //  (登录横幅不参与检测，登录即输出错误样式信息的设备不会误报)
    err = s.Run(ctx, "disp lay ver", onOut)
    var de *easyshell.DeviceError
    if errors.As(err, &de) {
        fmt.Println("命令错误:", de.Cmd, de.Line, de.PatternName)
    }

    // 策略与规则可配(Config)：
    cfg.ErrorPolicy = easyshell.ErrorCollect       // 命中不中断，结束时聚合返回
    cfg.ErrorPatterns = append(easyshell.DefaultErrorPatterns(),
        easyshell.NewErrorPattern("my-error", `(?i)^my custom error`))
    cfg.ErrorPatterns = []*easyshell.ErrorPattern{} // 显式关闭检测
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

    // 回放(按时序/倍速)，或经交互式重放接入 Shell(见 record 包文档)
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

## 文档

- [v1 → v2 迁移指南](docs/MIGRATION.md)
- [路线图](docs/ROADMAP.md)
- [更新日志](CHANGELOG.md)

## 注意事项

- `onOut` 回调在读取流程中**同步调用**：阻塞它会推迟读取与提示符判定(设备可能在等待拦截器应答)。
  需要重处理时请在回调内自行异步化(缓冲 + goroutine)，丢弃/堆积策略由业务决定
- 同一 Shell 并发 Read 会返回 `easyshell.ErrConcurrentRead`(读取过程中可并发查询 Prompt/IsPrompt/InConfigMode)

## 已知限制

- 多行拦截器命中时，匹配窗口内的前缀行不会作为输出返回(v1 即有，保持不变)
- EL/ED 擦除按"清空当前行"的保守语义处理(无列位置信息)，可用 `filter.Options.Erase=false` 关闭
- Decoder 契约：目标编码的多字节字符中不能出现 0x0A 字节(GBK/GB18030/UTF-8 满足，UTF-16 不适用)
- 录制文件为明文(含密码帧)，请妥善保管；Dump 时 secret 帧默认打码
