# Roadmap

## 已发布

### v2.1.0
- 厂商驱动体系(VendorProfile)：禁用分页/保存配置(含确认交互)/错误模式整合，支持自定义注册
- 长连接保活(KeepAlive)：周期探测 + 失败阈值 + 死亡回调
- 文件传输校验(HashVerify)与进度回调(Progress)
- 设备错误检测：内置各厂商命令解析错误模式，Fail/Collect 策略
- 命令级提示符覆盖(RunOptions.Prompt)与配置模式推断(InConfigMode)
- 跳板机多级链、SSH Agent 认证、slog 日志钩子、RunScript、SFTP ctx 取消
- 录制回放(record 二进制帧格式)、asciinema 导出

### v2.0.0
- 全新架构：Shell 统一接口、有状态过滤器(转义序列状态机)、core 读循环重构
- 离线 mock 测试体系(mock SSH/Telnet/SFTP)

### v1.0.0
- 初版：SSH/Telnet/Cmd 交互式命令执行

## 计划中

### v2.3.0(候选)
- [ ] 设备错误模式库扩充(基于真机 fixture 沉淀；山石/Array 待真机验证，
      骨架外厂商错误样式见 docs/ERRDETECT-REFERENCES.md)

## 远期(评估中)

- [ ] 自动重连策略(ReconnectPolicy)：语义复杂点在重连后会话状态(配置模式/su)丢失，默认关闭、显式开启
- [ ] 终端单元格仿真：集成 vt10x 作为可选 filter 实现，还原"只刷屏不吐行"的全屏交互设备；
      当前行级处理 + Erase 保守语义已覆盖设备 CLI 场景，遇到真实需求再启动
- [ ] 文本模板结构化解析(TextFSM 兼容)：倾向业务层组合第三方库而非内置

## 明确不做

- 异步 API：Go 并发模型天然覆盖
- Nornir 式批量编排/设备清单管理：超出库边界，业务层用 goroutine + errgroup 组合
- gNMI/gRPC telemetry：不同协议族，非 CLI 库边界
