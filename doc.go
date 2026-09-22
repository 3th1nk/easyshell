// Package easyshell 支持本地执行命令、通过 SSH/TELNET 协议在主机与网络设备上远程执行交互式命令。
//
// # 特性
//
//   - 统一的 Shell 接口抽象(CmdShell/SshShell/TelnetShell 可互换使用)
//   - 自定义提示符匹配规则，默认规则支持自动纠正(AutoPrompt)
//   - 默认自动识别 GB18030 编码并转换为 UTF8，支持自定义解码器
//   - 内置字符过滤器：退格、CRLF 归一化、ANSI/ECMA-48 转义序列剔除(有状态，跨网络分包安全)
//   - 内置拦截器：密码交互(Password)、选项交互(Yes/No)、网络设备自动翻页(More)、继续执行(Continue)
//   - 延迟返回输出内容(按时间间隔或累计大小)
//   - 录制原始输入输出并回放(record 包)
//   - SFTP 文件上传(原子写入)/下载/递归删除
//
// # 基本用法
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
// # 约束
//
//   - 同一 Shell 同一时刻只允许一个读操作(并发 Read/Run 返回 core.ErrConcurrentRead)；
//   - 所有阻塞方法接受 context 控制取消/超时；
//   - 参数风格：构造用 Config 结构体按值传入(零值合法)，调用级可选项用 Options 结构体可变参数。
package easyshell
