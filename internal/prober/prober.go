package prober

import (
	"context"
	"time"
	"uptime-mon/internal/config"
)

// ProbeResult captures the outcome of a single probe check
type ProbeResult struct {
	Status      string        // UP or DOWN
	Latency     time.Duration
	Error       error
	TLSExpiryAt *time.Time
}

// Prober is the interface implemented by each protocol probe runner
type Prober interface {
	Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult
}
