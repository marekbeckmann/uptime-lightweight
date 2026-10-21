package prober

import (
	"context"
	"fmt"
	"net"
	"time"
	"uptime-mon/internal/config"
)

// TCPProber verifies TCP 3-way handshake
type TCPProber struct{}

func NewTCPProber() *TCPProber {
	return &TCPProber{}
}

func (p *TCPProber) Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult {
	start := time.Now()
	addr := net.JoinHostPort(endpoint.Target, fmt.Sprintf("%d", endpoint.Port))

	dialer := net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	latency := time.Since(start)

	if err != nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   fmt.Errorf("tcp handshake to %s failed: %w", addr, err),
		}
	}
	_ = conn.Close()

	return ProbeResult{
		Status:  "UP",
		Latency: latency,
	}
}
