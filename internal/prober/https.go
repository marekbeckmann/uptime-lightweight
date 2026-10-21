package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"uptime-mon/internal/config"
)

// HTTPSProber executes HTTP and HTTPS status & certificate validation
type HTTPSProber struct{}

func NewHTTPSProber() *HTTPSProber {
	return &HTTPSProber{}
}

func (p *HTTPSProber) Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult {
	start := time.Now()

	method := strings.ToUpper(endpoint.Method)
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.Target, nil)
	if err != nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: time.Since(start),
			Error:   fmt.Errorf("invalid http request: %w", err),
		}
	}
	req.Header.Set("User-Agent", "uptime-mon/1.0 (+https://github.com/uptime-mon)")

	var leafCertExpiry *time.Time

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: endpoint.IgnoreTLSErrors,
		},
		ResponseHeaderTimeout: timeout,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow redirects automatically if status codes like 301/302 are explicitly checked
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   err,
		}
	}
	defer func() {
		// Drain and close body to allow connection reuse
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Extract TLS certificate expiration if HTTPS
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		expiry := resp.TLS.PeerCertificates[0].NotAfter
		leafCertExpiry = &expiry
	}

	// Verify status code
	accepted := endpoint.AcceptedStatusCodes
	if len(accepted) == 0 {
		accepted = []string{"20X"}
	}

	if !config.MatchStatusCode(resp.StatusCode, accepted) {
		return ProbeResult{
			Status:      "DOWN",
			Latency:     latency,
			TLSExpiryAt: leafCertExpiry,
			Error:       fmt.Errorf("received HTTP status %d, expected matching %v", resp.StatusCode, accepted),
		}
	}

	return ProbeResult{
		Status:      "UP",
		Latency:     latency,
		TLSExpiryAt: leafCertExpiry,
	}
}
