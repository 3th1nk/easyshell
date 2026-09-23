# 设备错误模式出处参考

`DefaultErrorPatterns` 各规则的出处与真实输出样例。仅记录链接、标题与一行样例，
不复制厂商文档正文(版权考虑)；样例来自官方文档/社区一手信息，欢迎用真机输出修正。

设计原则：只匹配**命令解析器错误**，`^` 锚定行首。宁可漏检、避免误报——
宽松模式(`% Error`/`command not found` 等)刻意不纳入，需要时调用方经
`Config.ErrorPatterns` 追加(理由见 `internal/core/errdetect.go` 注释)。

最近核对日期：2026-09-23

## Cisco IOS/IOS-XE

- 出处：Cisco Catalyst 9300 文档 "Using the Command-Line Interface" 的
  "Understanding CLI Error Messages" 一节(cisco.com)
- 样例：
  - `% Invalid input detected at '^' marker.`(插入符指向解析失败位置)
  - `% Incomplete command.`
  - `% Ambiguous command: "show con"`

## Cisco NX-OS

- 出处：Cisco Nexus 9000 Series NX-OS Programmability Guide(cisco.com)；
  Cisco Community Nexus 迁移讨论
- 样例：
  - `% Invalid command at '^' marker.`(注意与 IOS 的 "Invalid input" 样式不同)
  - 变体 `Invalid interface format at '^' marker.`(待真机确认是否带 `%` 前缀，暂未纳入)
- 锐捷 Ruijie：官方《RG-S5700H系列 RGOS 11.4(1)B42 配置手册》V2.0 §1.3.6
  带样例确认与 Cisco IOS 三种样式完全一致，由 `cisco-*` 规则覆盖，无单独规则；
  提示符体系(Ruijie> / Ruijie# / Ruijie(config)#)与保存命令(`write`，执行模式直接
  执行)亦经该手册官方确认，见 vendors/ruijie.yaml

## H3C / Comware

- 出处：Comware 命令行手册"命令行错误信息"一节(与华为 VRP 同构的 `^` position 样式)
- 样例：
  - `% Unrecognized command found at '^' position.`
  - `% Incomplete command found at '^' position.`
  - `% Ambiguous command found at '^' position.`
  - `% Too many parameters found at '^' position.`
  - `% Wrong parameter found at '^' position.`

## 华为 VRP

- 出处：华为官方支持《解读命令行的错误信息》(support.huawei.com，2024-10 更新)
- 官方五种全覆盖：
  - `Error: Unrecognized command found at '^' position.`
  - `Error: Incomplete command found at '^' position.`
  - `Error: Ambiguous command found at '^' position.`
  - `Error: Wrong parameter found at '^' position.`
  - `Error: Too many parameters found at '^' position.`

## Juniper Junos

- 出处：Juniper Junos OS 官方文档(juniper.net)
- 样例：
  - `syntax error, expecting <statement> or <identifier>`
  - 注：`error:` 前缀样式多属 commit 阶段配置校验错误(如
    `error: configuration check-out failed`)，非命令行解析错误，刻意不纳入

## 山石 StoneOS

- 出处：《StoneOS 命令行手册》官方三版一致(R7 / V5.5R10 全系列 / 5.5R2-4，
  厂商提供 PDF)+内部生产真机印证
- 官方错误信息三种："Unrecognized command"(找不到命令/参数类型错/值越界)、
  "Incomplete command"(不完整)、"Ambiguous command"(不明确)。手册表格为简化文本，
  真机实际输出带 `% ` 前缀(生产错误正则 `^%\s`)
- 规则(厂商级，见 vendors/hillstone.yaml)：
  - `^%?\s*(Unrecognized|Incomplete|Ambiguous) command` 按官方文本精确匹配(可选 % 前缀)
  - `^%\s` 兜底其余 `% ` 前缀错误(% 后空白，排除 `%TAG-` 样式日志行)
- 其它官方确认：分页提示符 `--More--`(回车下一行/q 退出/任意键下一页)；
  `terminal length 0` 关闭分页(仅当前连接有效)；`save [string]` 任何模式可执行、
  无确认；提示符 `hostname#`/`hostname(config)#`/`hostname(config-if-eth0/0)#`，
  VRouter 变体带 `[name]` 后缀

## Array APV(生产真机验证)

- 出处：内部生产环境(证券运维自动化)真机验证，生产错误检测正则 `(^%\s)|\^`
  ——错误行以 `% ` 开头(% 后空白，可排除 `%TAG-` 样式的日志行)；
  《Array APV 用户手册》8.5.0 第 4 章确认权限三级与配置模式体系(无错误样式样例)
- 规则:`^%\s`(厂商级 ErrorPatterns，叠加在全局默认之上，见 vendors/hillstone.yaml、
  vendors/array.yaml)
- 已沉淀的其它真机经验：Array 提权 `enable`(提示 "Enable password:"，缺省密码为空)/
  禁用分页 `no pager`(配置模式内)/保存 `write memory`(Config 级别执行)；
  `config terminal force` 强制进入配置模式需应答 "YES"；Array 输入超长回缩时
  发送 `` $`+退格+`\r\n\r` `` 序列(filter 的 apv 状态已内置处理)
