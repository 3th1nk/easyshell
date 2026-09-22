package core

import (
	"log/slog"
	"time"
)

// KeepAliveConfig 长连接保活配置(Interval>0 时启用)。
//
//	长驻会话穿过防火墙/NAT 时，空闲连接会被静默掐断(表现为下次读取一直超时)；
//	保活通过周期性探测维持连接活性，连续失败达到阈值时判定连接死亡并回调通知。
type KeepAliveConfig struct {
	// Interval 探测间隔，默认 30 秒
	Interval time.Duration
	// MaxFailures 连续失败多少次判定连接死亡，默认 3
	MaxFailures int
	// Send 保活动作(由具体 Shell 注入传输相关的实现，如 SSH 的 keepalive 请求、
	//	Telnet 的 NOP 字节)；nil 则不启用
	Send func() error
	// OnDead 连接死亡回调(在后台 goroutine 中调用，不要在其中做耗时操作)。
	//	调用后保活 goroutine 退出；上层通常在此处触发重连/告警。
	//	注意：连接死亡不会自动中止进行中的 Read，读取会以超时/流错误收尾
	OnDead func(err error)
}

func (ka *KeepAliveConfig) normalize() {
	if ka.Interval <= 0 {
		ka.Interval = 30 * time.Second
	}
	if ka.MaxFailures <= 0 {
		ka.MaxFailures = 3
	}
}

// startKeepAlive 启动保活 goroutine(Interval>0 且 Send 非 nil 时)
func (r *Reader) startKeepAlive(ka *KeepAliveConfig) {
	if ka == nil || ka.Send == nil {
		return
	}
	ka.normalize()

	go func() {
		ticker := time.NewTicker(ka.Interval)
		defer ticker.Stop()
		failures := 0
		for {
			select {
			case <-ticker.C:
			}
			if r.closed.Load() {
				return
			}
			if err := ka.Send(); err != nil {
				failures++
				r.logf(slog.LevelWarn, "keepalive failed", "failures", failures, "err", err)
				if failures >= ka.MaxFailures {
					r.logf(slog.LevelWarn, "keepalive dead", "err", err)
					if ka.OnDead != nil {
						ka.OnDead(err)
					}
					return
				}
				continue
			}
			failures = 0
		}
	}()
}
