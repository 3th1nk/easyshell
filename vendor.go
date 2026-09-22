package easyshell

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/3th1nk/easyshell/v2/core"
	"github.com/3th1nk/easyshell/v2/interceptor"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Vendor 设备厂商/系统类型
type Vendor string

const (
	VendorGeneric   Vendor = ""           // 未知厂商(无禁用分页命令，依赖 More 拦截器兜底)
	VendorH3C       Vendor = "h3c"        // H3C/Comware
	VendorCiscoIOS  Vendor = "cisco-ios"  // Cisco IOS/IOS-XE
	VendorCiscoNXOS Vendor = "cisco-nxos" // Cisco NX-OS
	VendorHuawei    Vendor = "huawei"     // 华为 VRP
	VendorJuniper   Vendor = "juniper"    // Juniper Junos
	VendorRuijie    Vendor = "ruijie"     // 锐捷(类Cisco语法)
	VendorHPComware Vendor = "comware"    // HP Comware(同H3C语法)
)

// VendorProfile 厂商驱动：把该厂商设备的"方言"整合为一组可复用的配置。
//
//	内置了 H3C/Cisco/华为/Juniper/锐捷 等常见厂商的 profile(见 VendorProfileOf)；
//	自定义厂商可通过 RegisterVendorProfile 注册，或从 JSON/YAML 配置文件批量加载
//	(见 LoadVendorProfilesFile)。
type VendorProfile struct {
	// Vendor 厂商/系统类型
	Vendor Vendor
	// PagingDisable 禁用分页命令(可能为空，未知厂商依赖 More 拦截器兜底)
	PagingDisable string
	// SaveConfigCmd 保存配置命令
	SaveConfigCmd string
	// SaveConfigConfirm 保存配置过程中的确认提示(如华为 save 的 Y/N 确认)，nil 无确认
	SaveConfigConfirm *regexp.Regexp
	// SaveConfigAnswer 确认提示的应答内容
	SaveConfigAnswer string
	// ErrorPatterns 厂商特有的错误模式(叠加在内置默认规则之上)
	ErrorPatterns []*core.ErrorPattern
}

var (
	// vendorMu 保护 vendorProfiles(支持运行时注册/加载配置文件)
	vendorMu sync.RWMutex
	// vendorProfiles 内置厂商驱动
	vendorProfiles = map[Vendor]*VendorProfile{
		VendorH3C: {
			Vendor:        VendorH3C,
			PagingDisable: "screen-length disable",
			SaveConfigCmd: "save force", // force 跳过 Y/N 确认
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
)

// VendorProfileOf 返回厂商驱动；未知厂商返回 nil(仅 More 拦截器兜底，保存配置等需自行处理)。
func VendorProfileOf(v Vendor) *VendorProfile {
	vendorMu.RLock()
	defer vendorMu.RUnlock()
	return vendorProfiles[v]
}

// RegisterVendorProfile 注册/覆盖厂商驱动(自定义厂商接入用)。
//
//	注意：注册与加载应在程序启动阶段完成，避免与运行中的会话产生并发可见性问题。
func RegisterVendorProfile(p *VendorProfile) {
	if p == nil || p.Vendor == VendorGeneric {
		return
	}
	vendorMu.Lock()
	defer vendorMu.Unlock()
	vendorProfiles[p.Vendor] = p
}

// PagingDisableCommand 返回厂商对应的禁用分页命令；未知厂商返回空串
//
//	(应继续依赖 More 拦截器兜底)。等价于 VendorProfileOf(v).PagingDisable。
func PagingDisableCommand(v Vendor) string {
	if p := VendorProfileOf(v); p != nil {
		return p.PagingDisable
	}
	return ""
}

// LoadVendorProfilesFile 从 JSON/YAML 配置文件加载并注册厂商驱动(按扩展名识别格式)。
//
//	文件内容支持单个对象或对象数组，字段说明见 vendorProfileDef。
//	示例(yaml)：
//	  - vendor: my-firewall
//	    paging_disable: "set cli page 0"
//	    save_config_cmd: "save config"
//	    error_patterns:
//	      - name: myfw-bad-cmd
//	        pattern: '^ERROR: unknown keyword'
func LoadVendorProfilesFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return LoadVendorProfiles(data, filepath.Ext(path))
}

// LoadVendorProfiles 从 JSON/YAML 内容加载并注册厂商驱动。
//
//	format 为 ".json" 或 ".yaml"/".yml"；解析或规则编译失败时返回错误(不产生部分注册)。
func LoadVendorProfiles(data []byte, format string) error {
	var defs []vendorProfileDef
	switch strings.ToLower(filepath.Ext(format)) {
	case ".json":
		if err := json.Unmarshal(data, &defs); err != nil {
			// 兼容单个对象
			var one vendorProfileDef
			if err2 := json.Unmarshal(data, &one); err2 != nil {
				return err
			}
			defs = []vendorProfileDef{one}
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &defs); err != nil {
			var one vendorProfileDef
			if err2 := yaml.Unmarshal(data, &one); err2 != nil {
				return err
			}
			defs = []vendorProfileDef{one}
		}
	default:
		return fmt.Errorf("vendor: unsupported config format %q (.json/.yaml/.yml)", format)
	}
	if len(defs) == 0 {
		return nil
	}

	// 先全部编译成功后再注册(避免部分注册)
	profiles := make([]*VendorProfile, 0, len(defs))
	for i := range defs {
		p, err := defs[i].compile()
		if err != nil {
			return fmt.Errorf("vendor profile #%d: %w", i, err)
		}
		profiles = append(profiles, p)
	}
	vendorMu.Lock()
	defer vendorMu.Unlock()
	for _, p := range profiles {
		vendorProfiles[p.Vendor] = p
	}
	return nil
}

// vendorProfileDef 配置文件中的厂商驱动定义(JSON/YAML 字段均为 snake_case)
type vendorProfileDef struct {
	Vendor            string `json:"vendor" yaml:"vendor"`
	PagingDisable     string `json:"paging_disable" yaml:"paging_disable"`
	SaveConfigCmd     string `json:"save_config_cmd" yaml:"save_config_cmd"`
	SaveConfigConfirm string `json:"save_config_confirm" yaml:"save_config_confirm"`
	SaveConfigAnswer  string `json:"save_config_answer" yaml:"save_config_answer"`
	ErrorPatterns     []struct {
		Name    string `json:"name" yaml:"name"`
		Pattern string `json:"pattern" yaml:"pattern"`
	} `json:"error_patterns" yaml:"error_patterns"`
}

func (d *vendorProfileDef) compile() (*VendorProfile, error) {
	if strings.TrimSpace(d.Vendor) == "" {
		return nil, fmt.Errorf("vendor is empty")
	}
	p := &VendorProfile{
		Vendor:           Vendor(d.Vendor),
		PagingDisable:    d.PagingDisable,
		SaveConfigCmd:    d.SaveConfigCmd,
		SaveConfigAnswer: d.SaveConfigAnswer,
	}
	if d.SaveConfigConfirm != "" {
		re, err := regexp.Compile(d.SaveConfigConfirm)
		if err != nil {
			return nil, fmt.Errorf("vendor %s: bad save_config_confirm: %w", d.Vendor, err)
		}
		p.SaveConfigConfirm = re
	}
	for _, ep := range d.ErrorPatterns {
		re, err := regexp.Compile(ep.Pattern)
		if err != nil {
			return nil, fmt.Errorf("vendor %s: bad error pattern %q: %w", d.Vendor, ep.Name, err)
		}
		p.ErrorPatterns = append(p.ErrorPatterns, &core.ErrorPattern{Name: ep.Name, Pattern: re})
	}
	return p, nil
}

// SaveConfig 保存设备配置(使用厂商 profile 的保存命令，自动处理确认提示)。
//
//	未知厂商返回错误；H3C/Comware 使用 save force 跳过确认。
func SaveConfig(ctx context.Context, s Shell, v Vendor, onOut func(lines []string)) error {
	p := VendorProfileOf(v)
	if p == nil || p.SaveConfigCmd == "" {
		return &core.Error{Op: core.OpShell, Err: vendorError(v)}
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
