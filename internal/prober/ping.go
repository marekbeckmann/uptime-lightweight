package prober

import (
	"context"
	"fmt"
	"time"
	"uptime-mon/internal/config"

	probing "github.com/prometheus-community/pro-bing"
)

// PingProber executes ICMP ping checks with unprivileged UDP fallback
type PingProber struct{}

func NewPingProber() *PingProber {
	return &PingProber{}
}

func (p *PingProber) Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult {
	start := time.Now()
	target := endpoint.Target

	pinger, err := probing.NewPinger(target)
	if err != nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: time.Since(start),
			Error:   fmt.Errorf("failed to initialize pinger for %s: %w", target, err),
		}
	}

	pinger.Count = 1
	pinger.Timeout = timeout

	// Try privileged first; if that fails to send, retry with unprivileged UDP ping
	pinger.SetPrivileged(true)
	err = pinger.Run()
	if err != nil || pinger.Statistics().PacketsRecv == 0 {
		// Retry with unprivileged socket fallback
		pingerFallback, errFallback := probing.NewPinger(target)
		if errFallback == nil {
			pingerFallback.Count = 1
			pingerFallback.Timeout = timeout
			pingerFallback.SetPrivileged(false)
			if errRun := pingerFallback.Run(); errRun == nil && pingerFallback.Statistics().PacketsRecv > 0 {
				stats := pingerFallback.Statistics()
				return ProbeResult{
					Status:  "UP",
					Latency: stats.AvgRtt,
				}
			}
		}

		if err == nil {
			err = fmt.Errorf("0 packets received from %s (packet loss 100%%)", target)
		}
		return ProbeResult{
			Status:  "DOWN",
			Latency: time.Since(start),
			Error:   err,
		}
	}

	stats := pinger.Statistics()
	return ProbeResult{
		Status:  "UP",
		Latency: stats.AvgRtt,
	}
}
