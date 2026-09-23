# Changelog

## Unreleased

### 新增
- 根包导出 `NewErrorPattern`(自定义错误检测规则的构建函数，与 `ErrorPattern`/`DefaultErrorPatterns` 配套)
- Telnet 自定义登录正则(`TelnetConfig.LoginUserRegex/LoginPassRegex/LoginPromptRegex`，
  nil 时使用内置规则)，覆盖非标登录提示符设备(v1 能力在 v2 包结构收敛后的恢复)
- `RunOptions.Timeout` 命令级超时：独立于 ctx、只约束本次调用，慢命令单独放宽/
  交互命令单独收紧；零值不限跟随 ctx，超时错误可经 `IsTimeout` 判断
- 错误模式库扩充：新增 Cisco NX-OS(`% Invalid command at`)与华为 VRP
  `Error: Too many parameters`；锐捷等类Cisco语法由 cisco-* 规则覆盖。
  出处与样例归档至 docs/ERRDETECT-REFERENCES.md；
  宽松模式(`% Error`/`command not found`)经评估刻意不纳入(误报代价高于漏报)

## v2.2.0

### 新增
- 文件传输协议自动选择：优先 SFTP，老设备无 SFTP 子系统时自动降级 SCP(exec 通道)，
  可 `TransferOptions.Protocol` 强制；含 mock SCP 协议端离线测试
- 厂商驱动配置批量加载：`LoadVendorProfilesPath`/`LoadVendorProfiles`(YAML/JSON)
- known_hosts 主机密钥校验(KnownHostsCallback + SshCredential.HostKeyCallback)：
  支持标准 known_hosts 格式(含 hashed hostname)
- filter 状态机 fuzz 测试(30万+随机输入零失败，含切分一致性与转义不泄漏不变量)
- example_test.go：pkg.go.dev 可运行示例
- 厂商驱动骨架：山石 StoneOS / Array APV(待验证字段留空走 More 兜底)

### 修复
- telnet 客户端 Write 经由 net.Pipe 测试暴露的转义边界问题加固
- 目录上传跳过 md5 校验(此前对目录执行 md5sum 报 is a directory)

### 变更
- 包结构收敛：core 读取循环/错误类型下沉 `internal/`，公共别名迁移至根包
  (`easyshell.Error`/`IsTimeout`/`OpOf`/`RunOptions`/`Config` 等)；`telnet` 包收敛为
  internal，外部统一经 `Shell` 接口使用
- 传输 API 重构：`SftpUpload/SftpOptions{HashVerify}` → `Upload/TransferOptions`
  (上传校验默认开启，改由 `NoVerify` 关闭；`ScpUpload/ScpDown` 收敛为 internal)；
  移除 sftp 客户端缓存，改为一次性客户端；上传临时文件改为 `.eshtemp` 隐藏文件
- `NewSshShellFromClient`/`NewTelnetShellFromClient` 降级为非导出
- interceptor 移除零使用的组合糖函数 `LastLineRegex/LastLinePattern/LastLinePassword`
  (语义仍可经 `Pattern(pattern, input, LastLine, showOut...)` 表达)

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
