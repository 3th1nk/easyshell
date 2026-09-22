package easyshell

import (
	"context"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"regexp"
)

// VendorProfile 厂商驱动：把该厂商设备的"方言"整合为一组可复用的配置。
//
//	内置了 H3C/Cisco/华为/Juniper/锐捷 等常见厂商的 profile(见 VendorProfileOf)；
//	自定义厂商可自行构建或基于内置 profile 修改。
type VendorProfile struct {
	// Vendor 厂商/系统类型
	Vendor Vendor
	// PagingDisable 禁用分页命令(可能为空，未知厂商依赖 More 拦截器兜底)
	PagingDisable string
	// SaveConfigCmd 保存配置命令
	SaveConfigCmd string
	// SaveConfigConfirm 保存配置过程中的确认提示(如 H3C save 的 Y/N 确认)，nil 无确认
	SaveConfigConfirm *regexp.Regexp
	// SaveConfigAnswer 确认提示的应答内容
	SaveConfigAnswer string
	// ErrorPatterns 厂商特有的错误模式(叠加在内置默认规则之上)
	ErrorPatterns []*core.ErrorPattern
}

// vendorProfiles 内置厂商驱动
var vendorProfiles = map[Vendor]*VendorProfile{
	VendorH3C: {
		Vendor:        VendorH3C,
		PagingDisable: "screen-length disable",
		SaveConfigCmd: "save force", // force 跳过 Y/N 确认
		ErrorPatterns: nil,
	},
	VendorHPComware: {
		Vendor:        VendorHPComware,
		PagingDisable: "screen-length disable",
		SaveConfigCmd: "save force",
	},
	VendorCiscoIOS: {
		Vendor:        VendorCiscoIOS,
		PagingDisable: "terminal length 0",
		SaveConfigCmd: "write memory",
	},
	VendorCiscoNXOS: {
		Vendor:        VendorCiscoNXOS,
		PagingDisable: "terminal length 0",
		SaveConfigCmd: "copy running-config startup-config",
	},
	VendorRuijie: {
		Vendor:        VendorRuijie,
		PagingDisable: "terminal length 0",
		SaveConfigCmd: "write memory",
	},
	VendorHuawei: {
		Vendor:            VendorHuawei,
		PagingDisable:     "screen-length 0 temporary",
		SaveConfigCmd:     "save",
		SaveConfigConfirm: regexp.MustCompile(`(?i)are you sure|y/n`),
		SaveConfigAnswer:  "y",
	},
	VendorJuniper: {
		Vendor:        VendorJuniper,
		PagingDisable: "set cli screen-length 0",
		SaveConfigCmd: "commit",
	},
}

// VendorProfileOf 返回厂商驱动；未知厂商返回 nil(仅 More 拦截器兜底，保存配置等需自行处理)。
func VendorProfileOf(v Vendor) *VendorProfile {
	return vendorProfiles[v]
}

// RegisterVendorProfile 注册/覆盖厂商驱动(自定义厂商接入用)
func RegisterVendorProfile(p *VendorProfile) {
	if p != nil && p.Vendor != VendorGeneric {
		vendorProfiles[p.Vendor] = p
	}
}

// SaveConfig 保存设备配置(使用厂商 profile 的保存命令，自动处理确认提示)。
//
//	未知厂商返回错误；H3C/Comware 使用 save force 跳过确认。
func SaveConfig(ctx context.Context, s Shell, v Vendor, onOut func(lines []string)) error {
	p := VendorProfileOf(v)
	if p == nil || p.SaveConfigCmd == "" {
		return &core.Error{Op: core.OpShell, Err: errNoVendorProfile(v)}
	}

	opts := RunOptions{}
	if p.SaveConfigConfirm != nil {
		opts.Interceptors = append(opts.Interceptors,
			interceptor.Pattern(p.SaveConfigConfirm.String(), p.SaveConfigAnswer, interceptor.LastLine, true))
		// 保存确认后设备通常还会输出提示并再次需要回车确认的场景
		opts.Interceptors = append(opts.Interceptors,
			interceptor.Pattern(`(?i)continue\s*\?`, "\n", interceptor.LastLine, true))
	}
	return s.Run(ctx, p.SaveConfigCmd, onOut, opts)
}

type vendorError Vendor

func (e vendorError) Error() string {
	return "no vendor profile for " + string(e)
}

func errNoVendorProfile(v Vendor) error { return vendorError(v) }
