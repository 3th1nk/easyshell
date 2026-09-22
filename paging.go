package easyshell

// Vendor 设备厂商/系统类型
type Vendor string

const (
	VendorGeneric    Vendor = ""            // 未知厂商(无禁用分页命令，依赖 More 拦截器兜底)
	VendorH3C        Vendor = "h3c"         // H3C/Comware
	VendorCiscoIOS   Vendor = "cisco-ios"   // Cisco IOS/IOS-XE
	VendorCiscoNXOS  Vendor = "cisco-nxos"  // Cisco NX-OS
	VendorHuawei     Vendor = "huawei"      // 华为 VRP
	VendorJuniper    Vendor = "juniper"     // Juniper Junos
	VendorRuijie     Vendor = "ruijie"      // 锐捷(类Cisco语法)
	VendorHPComware  Vendor = "comware"     // HP Comware(同H3C语法)
)

// pagingDisableCommands 各厂商的"临时禁用分页"命令(仅当前会话生效，不保存配置)。
//
//	获取长输出(如完整配置)前先执行禁用分页命令，比 More 逐页应答更快、更可靠；
//	More 拦截器保留作为未知厂商/命令的兜底。
var pagingDisableCommands = map[Vendor]string{
	VendorH3C:       "screen-length disable",
	VendorHPComware: "screen-length disable",
	VendorCiscoIOS:  "terminal length 0",
	VendorCiscoNXOS: "terminal length 0",
	VendorRuijie:    "terminal length 0",
	VendorHuawei:    "screen-length 0 temporary",
	VendorJuniper:   "set cli screen-length 0",
}

// PagingDisableCommand 返回厂商对应的禁用分页命令；未知厂商返回空串(应继续依赖 More 拦截器)。
//
//	注意：个别设备形态可能禁用了该命令或语法有差异，执行失败的输出会被错误检测规则忽略
//	(禁用分页命令本身不是合法解析目标)，此时回退 More 逐页应答即可。
func PagingDisableCommand(v Vendor) string {
	return pagingDisableCommands[v]
}
