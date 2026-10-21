package prober

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"uptime-mon/internal/config"

	"github.com/miekg/dns"
)

// DNSProber executes DNS resolution checks across UDP, TCP, DoT, and DoH
type DNSProber struct {
	httpClient *http.Client
}

func NewDNSProber() *DNSProber {
	return &DNSProber{
		httpClient: &http.Client{},
	}
}

func (p *DNSProber) Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult {
	start := time.Now()

	// 1. Resolve query type
	qtype := dns.TypeA
	switch strings.ToUpper(endpoint.QueryType) {
	case "AAAA":
		qtype = dns.TypeAAAA
	case "CNAME":
		qtype = dns.TypeCNAME
	case "TXT":
		qtype = dns.TypeTXT
	case "MX":
		qtype = dns.TypeMX
	}

	hostname := endpoint.Hostname
	if !strings.HasSuffix(hostname, ".") {
		hostname = hostname + "."
	}

	msg := new(dns.Msg)
	msg.SetQuestion(hostname, qtype)
	msg.RecursionDesired = true

	proto := strings.ToLower(endpoint.Protocol)
	if proto == "" {
		proto = "udp"
	}

	// 2. Handle DNS-over-HTTPS (DoH RFC 8484)
	if proto == "doh" {
		packed, err := msg.Pack()
		if err != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: time.Since(start),
				Error:   fmt.Errorf("dns pack error: %w", err),
			}
		}

		serverURL := endpoint.Server
		if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
			serverURL = "https://" + serverURL
		}

		req, err := http.NewRequestWithContext(ctx, "POST", serverURL, bytes.NewReader(packed))
		if err != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: time.Since(start),
				Error:   fmt.Errorf("doh request build error: %w", err),
			}
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		latency := time.Since(start)
		if err != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: latency,
				Error:   fmt.Errorf("doh request failed: %w", err),
			}
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ProbeResult{
				Status:  "DOWN",
				Latency: latency,
				Error:   fmt.Errorf("doh server returned HTTP %d", resp.StatusCode),
			}
		}

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: latency,
				Error:   fmt.Errorf("reading doh response body: %w", err),
			}
		}

		respMsg := new(dns.Msg)
		if err := respMsg.Unpack(respBytes); err != nil {
			return ProbeResult{
				Status:  "DOWN",
				Latency: latency,
				Error:   fmt.Errorf("unpacking doh dns response: %w", err),
			}
		}

		return evaluateDNSResponse(respMsg, latency)
	}

	// 3. Handle UDP, TCP, and DoT (tcp-tls)
	server := endpoint.Server
	port := endpoint.Port
	if port == 0 {
		if proto == "dot" {
			port = 853
		} else {
			port = 53
		}
	}
	serverAddr := net.JoinHostPort(server, fmt.Sprintf("%d", port))

	client := &dns.Client{
		Timeout: timeout,
	}

	switch proto {
	case "tcp":
		client.Net = "tcp"
	case "dot":
		client.Net = "tcp-tls"
		client.TLSConfig = &tls.Config{
			ServerName: server,
		}
	default:
		client.Net = "udp"
	}

	respMsg, latency, err := client.ExchangeContext(ctx, msg, serverAddr)
	if err != nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   fmt.Errorf("dns %s exchange to %s failed: %w", proto, serverAddr, err),
		}
	}

	return evaluateDNSResponse(respMsg, latency)
}

func evaluateDNSResponse(resp *dns.Msg, latency time.Duration) ProbeResult {
	if resp == nil {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   fmt.Errorf("empty dns response received"),
		}
	}

	if resp.Rcode != dns.RcodeSuccess {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   fmt.Errorf("dns server responded with Rcode: %s", dns.RcodeToString[resp.Rcode]),
		}
	}

	if len(resp.Answer) == 0 {
		return ProbeResult{
			Status:  "DOWN",
			Latency: latency,
			Error:   fmt.Errorf("dns query succeeded but returned 0 answers"),
		}
	}

	return ProbeResult{
		Status:  "UP",
		Latency: latency,
	}
}
