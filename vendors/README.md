# vendors — 内置厂商驱动

本目录的 YAML 文件是 easyshell 的内置厂商驱动定义，**编译期通过 `go:embed` 嵌入二进制**，程序启动时自动加载。新增/修正一个厂商 = 修改一个 YAML 文件，无需改动 Go 代码。

## 字段说明

| 字段 | 类型 | 说明 |
|---|---|---|
| `vendor` | string | 厂商/系统类型唯一标识(必填) |
| `paging_disable` | string | 禁用分页命令(可选；留空则依赖 More 拦截器兜底) |
| `save_config_cmd` | string | 保存配置命令(可选) |
| `save_config_confirm` | string | 保存过程中的确认提示正则(可选，如华为 save 的 Y/N 确认) |
| `save_config_answer` | string | 确认提示的应答内容(需与上一项配对使用) |
| `error_patterns` | list | 厂商特有错误检测模式(叠加在库内置默认规则之上)，每项含 `name` 与 `pattern` |

## 新增/修正厂商

1. 新增一个 YAML 文件(如 `myvendor.yaml`)，文件名建议与 vendor 标识一致
2. 填写字段；**未经验证的命令请留空**——留空字段依赖 More 拦截器兜底，错误的命令会在设备上产生解析错误
3. 运行 `go test ./...` 验证(内嵌文件解析正确性由单元测试保证)
4. 提交 PR

## 运行时加载(使用方)

使用方可不修改本目录，在程序启动阶段加载自己的驱动定义覆盖/扩展内置项：

```go
if err := easyshell.LoadVendorProfilesPath("/path/to/my-vendors.yaml"); err != nil {
    // ...
}
```

同名 `vendor` 会覆盖内置定义；解析或正则编译失败时不产生部分注册。
