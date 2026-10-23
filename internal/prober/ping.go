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

	// Try privileged first; if that succeeds and receives packets, return UP
	pinger.SetPrivileged(true)
	err = pinger.Run()
	if err == nil && pinger.Statistics().PacketsRecv > 0 {
		stats := pinger.Statistics()
		return ProbeResult{
			Status:  "UP",
			Latency: stats.AvgRtt,
		}
	}

	// Retry with unprivileged socket fallback
	pingerFallback, errFallback := probing.NewPinger(target)
	if errFallback == nil {
		pingerFallback.Count = 1
		pingerFallback.Timeout = timeout
		pingerFallback.SetPrivileged(false)
		errRun := pingerFallback.Run()
		if errRun == nil && pingerFallback.Statistics().PacketsRecv > 0 {
			stats := pingerFallback.Statistics()
			return ProbeResult{
				Status:  "UP",
				Latency: stats.AvgRtt,
			}
		}

		// Fallback completed with 0 packets received (timeout / packet loss)
		if errRun == nil && pingerFallback.Statistics().PacketsRecv == 0 {
			return ProbeResult{
				Status:  "DOWN",
				Latency: time.Since(start),
				Error:   fmt.Errorf("0 packets received from %s (packet loss 100%%)", target),
			}
		}

		// Fallback failed with network/run error
		if errRun != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: time.Since(start),
				Error:   fmt.Errorf("ping probe failed for %s: %w", target, errRun),
			}
		}
	}

	// If unprivileged fallback could not be initialized, report error
	finalErr := err
	if finalErr == nil {
		finalErr = fmt.Errorf("0 packets received from %s (packet loss 100%%)", target)
	}
	return ProbeResult{
		Status:  "DOWN",
		Latency: time.Since(start),
		Error:   finalErr,
	}
}
