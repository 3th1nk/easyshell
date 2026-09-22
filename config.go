package easyshell

import "github.com/3th1nk/easyshell/v2/core"

// Config 读取过程配置，字段说明见 core.Config
type Config = core.Config

// StderrPolicy stderr 处理策略，字段说明见 core.StderrPolicy
type StderrPolicy = core.StderrPolicy

// RunOptions 命令执行/读取的可选参数(提示符覆盖、拦截器)，字段说明见 core.RunOptions。
//
//	用法: s.Run(ctx, cmd, onOut, easyshell.RunOptions{Interceptors: its})
type RunOptions = core.RunOptions

const (
	StderrError  = core.StderrError
	StderrOutput = core.StderrOutput
	StderrIgnore = core.StderrIgnore
)
