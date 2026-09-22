# v1 → v2 迁移指南

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


## 使用方迁移要点

1. `shell.ReadWriter` 透传模式不再可用——改用 `Shell` 接口方法(覆盖原有全部用法)
2. 超时参数改 context：`s.ReadToEndLine(30*time.Second, f)` → `s.ReadUntilPrompt(ctx, f)`
3. 命令执行推荐直接用 `s.Run(ctx, cmd, onOut)`(等价 Write+ReadUntilPrompt)
4. 退出码：`echo $?` + 逐行解析可改用 `easyshell.ExitCode(ctx, s)`
5. 长输出场景建议先执行厂商禁用分页命令：`easyshell.PagingDisableCommand(easyshell.VendorH3C)`
6. 提示符会变化的命令(su/进入配置模式)：用 `s.Run(ctx, cmd, onOut, easyshell.RunOptions{Prompt: re})`
7. `Config.Filter` 自定义过滤器签名变为 `filter.FilterFunc`/`filter.Filter`(有状态)
8. 录制改用 `Record *RecordConfig` 配置(元数据自动填充、随 Close 收尾)
