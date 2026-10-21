package prober_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/prober"
)

func TestTCPProber(t *testing.T) {
	// Start a local TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start tcp listener: %v", err)
	}
	defer ln.Close()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	p := prober.NewTCPProber()
	ep := &config.Endpoint{
		ID:     "test-tcp",
		Target: host,
		Port:   port,
	}

	res := p.Probe(context.Background(), ep, 2*time.Second)
	if res.Status != "UP" {
		t.Fatalf("expected TCP probe UP, got DOWN with error: %v", res.Error)
	}

	// Test non-listening port
	epDown := &config.Endpoint{
		ID:     "test-tcp-down",
		Target: "127.0.0.1",
		Port:   65432,
	}
	resDown := p.Probe(context.Background(), epDown, 500*time.Millisecond)
	if resDown.Status != "DOWN" {
		t.Fatalf("expected non-listening port to report DOWN, got: %s", resDown.Status)
	}
}

func TestHTTPSProberStatusCodesAndTLS(t *testing.T) {
	// 1. Setup TLS test server with self-signed certificate
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		case "/redirect":
			w.WriteHeader(http.StatusMovedPermanently)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	p := prober.NewHTTPSProber()

	// 2. Test self-signed cert rejected when ignore_tls_errors: false
	epStrict := &config.Endpoint{
		ID:                  "test-strict-tls",
		Target:              ts.URL + "/ok",
		IgnoreTLSErrors:     false,
		AcceptedStatusCodes: []string{"20X"},
	}
	resStrict := p.Probe(context.Background(), epStrict, 2*time.Second)
	if resStrict.Status != "DOWN" {
		t.Errorf("expected self-signed cert to fail when ignore_tls_errors is false, got UP")
	}

	// 3. Test self-signed cert accepted when ignore_tls_errors: true
	epInsecure := &config.Endpoint{
		ID:                  "test-insecure-tls",
		Target:              ts.URL + "/ok",
		IgnoreTLSErrors:     true,
		AcceptedStatusCodes: []string{"20X"},
	}
	resInsecure := p.Probe(context.Background(), epInsecure, 2*time.Second)
	if resInsecure.Status != "UP" {
		t.Fatalf("expected ignore_tls_errors: true to pass, got DOWN: %v", resInsecure.Error)
	}
	if resInsecure.TLSExpiryAt == nil {
		t.Errorf("expected TLS certificate expiry to be captured")
	}

	// 4. Test status code matching wildcards and lists
	epRedirect := &config.Endpoint{
		ID:                  "test-redirect",
		Target:              ts.URL + "/redirect",
		IgnoreTLSErrors:     true,
		AcceptedStatusCodes: []string{"30X"},
	}
	resRedirect := p.Probe(context.Background(), epRedirect, 2*time.Second)
	if resRedirect.Status != "UP" {
		t.Errorf("expected 30X to match status 301, got DOWN: %v", resRedirect.Error)
	}

	// 5. Test 500 error when expecting 20X
	epError := &config.Endpoint{
		ID:                  "test-error",
		Target:              ts.URL + "/error",
		IgnoreTLSErrors:     true,
		AcceptedStatusCodes: []string{"20X"},
	}
	resError := p.Probe(context.Background(), epError, 2*time.Second)
	if resError.Status != "DOWN" {
		t.Errorf("expected 500 to fail 20X expectation, got UP")
	}
}

func TestDNSProberDoH(t *testing.T) {
	// Query Cloudflare DoH endpoint
	p := prober.NewDNSProber()
	ep := &config.Endpoint{
		ID:        "doh-test",
		Hostname:  "cloudflare.com",
		Server:    "https://cloudflare-dns.com/dns-query",
		Protocol:  "doh",
		QueryType: "A",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := p.Probe(ctx, ep, 5*time.Second)
	if res.Status != "UP" {
		t.Logf("Note: DoH test skipped or failed due to network environment: %v", res.Error)
	}
}

func TestPingProberFallback(t *testing.T) {
	p := prober.NewPingProber()
	ep := &config.Endpoint{
		ID:     "ping-local",
		Target: "127.0.0.1",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res := p.Probe(ctx, ep, 2*time.Second)
	// Localhost ping might fail if container/OS has no ping privilege; verify graceful error handling
	if res.Status == "DOWN" && res.Error == nil {
		t.Errorf("expected error message when ping fails")
	}
}
