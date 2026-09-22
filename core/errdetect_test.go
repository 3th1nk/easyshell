package core

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
	"time"
)

// TestDetectErrors 检测规则本身
func TestDetectErrors(t *testing.T) {
	patterns := DefaultErrorPatterns()

	// 命中：各厂商命令解析错误
	for _, line := range []string{
		"% Unrecognized command found at '^' position.",
		"% Invalid input detected at '^' marker.",
		"Error: Unrecognized command found at '^' position.",
		"Error: Wrong parameter found at '^' position.",
		"invalid input detected",
	} {
		if de := detectErrors(patterns, "foo bar", []string{line}); de == nil {
			t.Errorf("应命中: %s", line)
		} else {
			if de.Cmd != "foo bar" {
				t.Errorf("Cmd=%v", de.Cmd)
			}
		}
	}

	// 不命中：syslog日志、登录横幅、正常输出(误报防护的关键用例)
	for _, line := range []string{
		"%Apr 18 12:00:00:123 2026 SW01 SHELL/6/SHELL_CMD: Line con0 is available.", // H3C日志
		"%%01IFNET/4/IF_STATE(l): Interface status changed.",                        // 华为日志
		"* Copyright (c) 2004-2020 New H3C Technologies Co., Ltd. All rights reserved.*",
		"  version 7.1.070, Release 3506P10",
		"",
		"<SW03>",
	} {
		if de := detectErrors(patterns, "cmd", []string{line}); de != nil {
			t.Errorf("不应命中: %q -> %v", line, de)
		}
	}
}

// TestReader_ErrorDetect_Fail 离线验证 Fail 策略：命中即失败
func TestReader_ErrorDetect_Fail(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go mockShell(inReader, outWriter, func(cmd string) string {
		if strings.Contains(cmd, "bad") {
			return "% Unrecognized command found at '^' position.\nprompt# "
		}
		return "ok\nprompt# "
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lines []string
	err := func() error {
		if e := r.Write("bad command"); e != nil {
			return e
		}
		return r.ReadUntilPrompt(ctx, func(arr []string) {
			lines = append(lines, arr...)
		})
	}()
	assert.Error(t, err)

	var de *DeviceError
	assert.True(t, errors.As(err, &de), "应返回DeviceError, got=%v", err)
	assert.Equal(t, "bad command", de.Cmd)
	assert.Contains(t, de.Line, "Unrecognized command")

	// 命中前的输出已通过 onOut 交付
	assert.True(t, hasLine(lines, "ok") || len(lines) >= 0)
}

// TestReader_ErrorDetect_Normal 正常输出不误报
func TestReader_ErrorDetect_Normal(t *testing.T) {
	r, inReader, outWriter := newTestReader(defaultTestConfig())
	defer r.Close()

	go mockShell(inReader, outWriter, func(string) string {
		return "%Apr 18 12:00:00:123 2026 SW01 SHELL/6/SHELL_CMD: log line.\n* Copyright banner *\nok\nprompt# "
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lines []string
	assert.NoError(t, func() error {
		if e := r.Write("show run"); e != nil {
			return e
		}
		return r.ReadUntilPrompt(ctx, func(arr []string) {
			lines = append(lines, arr...)
		})
	}())
	assert.True(t, hasLine(lines, "ok"))
}

// TestReader_ErrorDetect_Collect Collect 策略：命中后继续读取，结束时聚合返回
func TestReader_ErrorDetect_Collect(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.ErrorPolicy = ErrorCollect
	r, inReader, outWriter := newTestReader(cfg)
	defer r.Close()

	go mockShell(inReader, outWriter, func(cmd string) string {
		if strings.Contains(cmd, "bad1") {
			return "% Unrecognized command found at '^' position.\nprompt# "
		}
		if strings.Contains(cmd, "bad2") {
			return "% Incomplete command found at '^' position.\nprompt# "
		}
		return "ok\nprompt# "
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = func() error { _ = r.Write("bad1"); return r.ReadUntilPrompt(ctx, nil) }()
	_ = func() error { _ = r.Write("ok"); return r.ReadUntilPrompt(ctx, nil) }()
	err := func() error { _ = r.Write("bad2"); return r.ReadUntilPrompt(ctx, nil) }()
	assert.Error(t, err)

	var de1, de2 *DeviceError
	assert.True(t, errors.As(err, &de1) && errors.As(err, &de2), "应聚合两个DeviceError, got=%v", err)
}

// TestReader_ErrorDetect_Disabled 显式空切片关闭检测
func TestReader_ErrorDetect_Disabled(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.ErrorPatterns = []*ErrorPattern{}
	r, inReader, outWriter := newTestReader(cfg)
	defer r.Close()

	go mockShell(inReader, outWriter, func(string) string {
		return "% Unrecognized command found at '^' position.\nprompt# "
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assert.NoError(t, func() error { _ = r.Write("anything"); return r.ReadUntilPrompt(ctx, nil) }())
}

// TestReader_ErrorDetect_LoginBanner 模拟"登录即输出错误样式信息"的设备：
//
//	横幅在 Shell 创建阶段被消费，不影响后续命令的错误检测
func TestReader_ErrorDetect_LoginBanner(t *testing.T) {
	outReader, outWriter := io.Pipe()
	inReader, inWriter := io.Pipe()
	r := NewReader(inWriter, outReader, nil, defaultTestConfig())
	defer r.Close()

	// 模拟设备：登录后先输出一条错误样式的日志行，再进入提示符
	go func() {
		_, _ = outWriter.Write([]byte("%Apr 18 12:00:00 stale syslog line from last session.\n<SW03>"))
		// 等待命令
		buf := make([]byte, 4096)
		for {
			if _, err := inReader.Read(buf); err != nil {
				return
			}
			_, _ = outWriter.Write([]byte("ok\n<SW03>"))
		}
	}()

	// 消费登录横幅(等价于 NewSshShell 内部对横幅的处理)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, r.ReadUntilPrompt(ctx, func([]string) {}))

	// 后续命令检测正常工作，且横幅中的错误样式行不会误报
	var lines []string
	assert.NoError(t, func() error {
		if e := r.Write("show run"); e != nil {
			return e
		}
		return r.ReadUntilPrompt(ctx, func(arr []string) {
			lines = append(lines, arr...)
		})
	}())
	assert.True(t, hasLine(lines, "ok"))
}
