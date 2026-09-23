package easyshell

import "github.com/3th1nk/easyshell/v2/internal/core"

// Config 读取过程配置，字段说明见 core.Config
type Config = core.Config

// StderrPolicy stderr 处理策略，字段说明见 core.StderrPolicy
type StderrPolicy = core.StderrPolicy

// RunOptions 命令执行/读取的可选参数(提示符覆盖、拦截器)，字段说明见 core.RunOptions。
//
//	用法: s.Run(ctx, cmd, onOut, easyshell.RunOptions{Interceptors: its})
type RunOptions = core.RunOptions

// ErrorPolicy 命令输出错误检测的命中处理策略
type ErrorPolicy = core.ErrorPolicy

const (
	ErrorFail    = core.ErrorFail    // 命中即失败
	ErrorCollect = core.ErrorCollect // 命中后继续读取，结束时聚合返回
)

// ErrorPattern 命令输出错误检测规则
type ErrorPattern = core.ErrorPattern

// DefaultErrorPatterns 内置默认错误检测规则(仅命令解析器错误)
func DefaultErrorPatterns() []*core.ErrorPattern { return core.DefaultErrorPatterns() }

// NewErrorPattern 构建自定义检测规则(pattern 非法时返回 nil)
func NewErrorPattern(name, pattern string) *ErrorPattern { return core.NewErrorPattern(name, pattern) }

// KeepAliveConfig 长连接保活配置
type KeepAliveConfig = core.KeepAliveConfig

const (
	StderrError  = core.StderrError
	StderrOutput = core.StderrOutput
	StderrIgnore = core.StderrIgnore
)
