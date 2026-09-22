package easyshell

import (
	"github.com/3th1nk/easyshell/v2/record"
)

// RecordConfig 会话录制配置。
//
//	在 SshConfig/TelnetConfig/CmdConfig 中设置后自动完成：录制器创建(含元数据填充)、
//	原始输入输出捕获、随 Shell.Close 自动关闭收尾。
type RecordConfig struct {
	// Path 录制文件路径(自动创建目录)。格式为 easyshell 二进制帧格式，
	//	可通过 record.Dump/DumpFile 转可读文本、record.DumpAsciinema 导出asciinema、
	//	eshdump 命令行工具查看
	Path string
	// CaptureInput 是否录制输入方向(命令、密码、拦截器应答；默认不录制)。
	//	注意录制文件为明文，包含密码的场景请妥善保管文件
	CaptureInput bool
	// Comment 备注(写入录制文件元数据)
	Comment string
}

// newRecorder 创建录制器(meta 由各 Shell 按凭证自动填充)
func newRecorder(cfg *RecordConfig, meta record.Meta) (*record.Writer, error) {
	if cfg == nil {
		return nil, nil
	}
	meta.Comment = cfg.Comment
	return record.NewFileWriter(cfg.Path, meta, record.Options{CaptureInput: cfg.CaptureInput})
}

// wireRecord 把录制器接入读取配置，返回需要挂到 Shell 上的录制器(可能为nil)
func wireRecord(cfg *RecordConfig, base *Config, meta record.Meta) (*record.Writer, error) {
	if cfg == nil {
		return nil, nil
	}
	rec, err := newRecorder(cfg, meta)
	if err != nil {
		return nil, err
	}
	base.RawOut = rec
	if cfg.CaptureInput {
		base.RawIn = rec.Input()
	}
	return rec, nil
}
