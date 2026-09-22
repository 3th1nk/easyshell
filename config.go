package easyshell

import "github.com/3th1nk/easyshell/v2/core"

// Config 读取过程配置，字段说明见 core.Config
type Config = core.Config

// StderrPolicy stderr 处理策略，字段说明见 core.StderrPolicy
type StderrPolicy = core.StderrPolicy

const (
	StderrError  = core.StderrError
	StderrOutput = core.StderrOutput
	StderrIgnore = core.StderrIgnore
)
