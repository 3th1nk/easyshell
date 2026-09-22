package core

import (
	"regexp"
	"strconv"
)

// DeviceError 设备命令解析错误(输出命中错误检测规则)
type DeviceError struct {
	// Cmd 触发错误的命令(最后一次 Write 的命令)
	Cmd string
	// PatternName 命中的规则名
	PatternName string
	// Line 命中的输出行
	Line string
}

func (e *DeviceError) Error() string {
	s := "device error"
	if e.PatternName != "" {
		s += " (" + e.PatternName + ")"
	}
	if e.Cmd != "" {
		s += " on command " + strconv.Quote(e.Cmd)
	}
	if e.Line != "" {
		s += ": " + e.Line
	}
	return s
}

// ErrorPattern 错误检测规则
type ErrorPattern struct {
	// Name 规则名(用于错误报告与日志)
	Name string
	// Pattern 匹配模式(按行匹配，建议行首锚定以避免误命中)
	Pattern *regexp.Regexp
}

// NewErrorPattern 构建检测规则(pattern 非法则忽略)
func NewErrorPattern(name, pattern string) *ErrorPattern {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return &ErrorPattern{Name: name, Pattern: re}
}

// DefaultErrorPatterns 内置默认检测规则。
//
//	只匹配"命令解析器错误"(输入的命令不合法)，与设备日志/syslog/登录横幅天然可区分，
//	因此登录时自动输出错误样式的设备不会被误报；需要检测其他错误时在调用方追加自定义规则。
func DefaultErrorPatterns() []*ErrorPattern {
	defs := []struct{ name, pattern string }{
		// Cisco IOS/NXOS 命令解析错误
		{"cisco-invalid-input", `^% Invalid input detected`},
		{"cisco-incomplete", `^% Incomplete command\.`},
		{"cisco-ambiguous", `^% Ambiguous command:`},
		// H3C/Comware 命令解析错误
		{"h3c-unrecognized", `^% Unrecognized command found at`},
		{"h3c-incomplete", `^% Incomplete command found at`},
		{"h3c-ambiguous", `^% Ambiguous command found at`},
		{"h3c-too-many-parameters", `^% Too many parameters found`},
		{"h3c-wrong-parameter", `^% Wrong parameter found at`},
		// 华为 VRP 命令解析错误
		{"huawei-unrecognized", `^Error: Unrecognized command`},
		{"huawei-incomplete", `^Error: Incomplete command`},
		{"huawei-ambiguous", `^Error: Ambiguous command`},
		{"huawei-wrong-parameter", `^Error: Wrong parameter`},
		// Juniper Junos
		{"juniper-syntax", `^syntax error, expecting`},
		// 通用
		{"generic-invalid-input", `(?i)^invalid input`},
	}
	patterns := make([]*ErrorPattern, 0, len(defs))
	for _, d := range defs {
		if p := NewErrorPattern(d.name, d.pattern); p != nil {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// match 按行匹配，返回命中的规则名
func (p *ErrorPattern) match(line string) bool {
	return p.Pattern != nil && p.Pattern.MatchString(line)
}

// detectErrors 在多行内容中检测错误，返回首个命中的 DeviceError(无命中返回 nil)
func detectErrors(patterns []*ErrorPattern, cmd string, lines []string) *DeviceError {
	for _, line := range lines {
		for _, p := range patterns {
			if p != nil && p.match(line) {
				return &DeviceError{Cmd: cmd, PatternName: p.Name, Line: line}
			}
		}
	}
	return nil
}
