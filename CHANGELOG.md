# Changelog

## v2.2.0

### 新增
- SCP 传输兜底(ScpUpload/ScpDown)：老设备无 SFTP 子系统时走 exec 通道传输，
  含 mock SCP 协议端离线测试
- known_hosts 主机密钥校验(KnownHostsCallback + SshCredential.HostKeyCallback)：
  支持标准 known_hosts 格式(含 hashed hostname)
- filter 状态机 fuzz 测试(30万+随机输入零失败，含切分一致性与转义不泄漏不变量)
- example_test.go：pkg.go.dev 可运行示例
- 厂商驱动骨架：山石 StoneOS / Array APV(待验证字段留空走 More 兜底)

### 修复
- telnet 客户端 Write 经由 net.Pipe 测试暴露的转义边界问题加固

## v2.1.0

### 新增
- 设备错误检测：内置各厂商命令解析错误模式，输出命中即返回 `*easyshell.DeviceError`，Fail/Collect 两种策略，登录横幅不参与检测
- 命令级提示符覆盖(RunOptions.Prompt)与配置模式推断(InConfigMode)
- 跳板机/堡垒机多级链(SshConfig.Proxy)，SSH Agent 认证(SshCredential.UseAgent)
- 长连接保活(Config.KeepAlive)：周期探测+失败阈值+死亡回调
- 厂商驱动体系(VendorProfile)：禁用分页/保存配置(含确认交互)/错误模式整合，支持自定义注册
- RunScript 多命令脚本(失败定位到命令)、ExitCode 退出码便捷方法
- 录制回放(record)：二进制帧格式、输入方向捕获、asciinema v2 导出、Dump 可读转储
- SFTP：HashVerify 传输校验、Progress 进度回调、ctx 取消、原子上传(temp+rename)
- 结构化日志钩子(Config.Logger *slog.Logger)
- 离线 mock 测试体系(mock SSH/Telnet/SFTP，基于 x/crypto/ssh 服务端，零外部依赖)

### 修复
- 转义序列跨网络分包截断漏处理、OSC/DCS 等字符串序列未剔除(v1 过滤器缺陷)
- 多字节字符跨分包解码乱码(只解码完整行)
- 流在行边界耗尽时 ReadAll 永不退出
- telnet IAC 转义误判无效 UTF-8 字节、错误无分类
- stderr 错误覆盖 ctx 超时语义
- 并发 Read 串流错乱、Stop 后使用 panic 等多项并发/健壮性问题

### 变更
- module 路径：`github.com/3th1nk/easyshell` → `github.com/3th1nk/easyshell/v2`
- Go 最低版本：1.18 → 1.21
- 完整破坏性变更清单见 README「v1 → v2 迁移对照」

## v1.0.0

- 初版发布：SSH/Telnet/Cmd 交互式命令执行、SFTP 上传下载删除、提示符自动纠正
