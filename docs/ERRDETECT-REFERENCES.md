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
- 锐捷 Ruijie：官方 RG-WLAN/RGOS 配置指南的 CLI 错误提示表确认与 Cisco IOS
  三种样式完全一致，由 `cisco-*` 规则覆盖，无单独规则

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

## 待真机沉淀(骨架厂商)

- 山石 StoneOS / Array APV：无公开错误样式资料，待真机输出后在
  `vendors/hillstone.yaml`、`vendors/array.yaml` 的 `ErrorPatterns` 字段补充
