// Package easyshell 支持本地执行命令、通过 SSH/TELNET 协议在主机与网络设备上远程执行交互式命令。
//
// # 能力概览
//
//   - 统一的 Shell 接口抽象(CmdShell/SshShell/TelnetShell 可互换使用)
//   - 提示符匹配：默认宽松规则兜底 + 命令级严格覆盖(RunOptions.Prompt) + 自动纠正(AutoPrompt)
//   - 有状态字符过滤器：ANSI/ECMA-48 转义序列跨网络分包安全剔除、退格按 rune 回退、
//     擦除序列实时作用于当前行、CRLF 归一化(含 H3C \r\r\n)
//   - 拦截器：密码交互、选项应答、网络设备自动翻页(More)、继续执行(Continue)、自定义扩展
//   - 设备错误检测：各厂商命令解析错误命中即报，Fail/Collect 两种策略
//   - 编码：默认 GB18030 自动识别转 UTF8；只解码完整行，多字节字符跨分包不被截断
//   - 连接：SSH 直连/多级跳板链、密码/密钥/Agent 认证、主机指纹校验、长连接保活
//   - 文件：SFTP 上传(原子上传)/下载/递归删除，支持传输校验与进度
//   - 录制回放：原始输入输出二进制录制、asciinema 导出、交互式重放
//   - 可观测：结构化日志钩子(slog)、超大输出尾部窗口匹配(16MB+ 单行无线性退化)
//
// # 快速上手
//
//	s, err := easyshell.NewSshShell(easyshell.SshConfig{
//	    Credential: easyshell.SshCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
//	})
//	if err != nil {
//	    return err
//	}
//	defer s.Close()
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
//	defer cancel()
//	err = s.Run(ctx, "display version", func(lines []string) {
//	    for _, line := range lines {
//	        fmt.Println(line)
//	    }
//	})
//
// # 使用约束
//
//   - 同一 Shell 同一时刻只允许一个读操作(并发 Read/Run 返回 core.ErrConcurrentRead)；
//   - 读取过程中可并发调用 Prompt/IsPrompt/InConfigMode；
//   - onOut 回调在读取流程中同步调用：阻塞它会推迟读取与提示符判定
//     (设备可能在等待拦截器应答)，需要重处理时请在回调内自行异步化；
//   - Close 幂等，调用后所有操作返回 core.ErrClosed。
//
// 更多内容：README(特性与示例)、docs/MIGRATION.md(v1→v2 迁移)、docs/ROADMAP.md(路线图)。
package easyshell
