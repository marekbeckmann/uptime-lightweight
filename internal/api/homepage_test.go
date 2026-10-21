package api_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uptime-mon/internal/api"
	"uptime-mon/internal/config"
	"uptime-mon/internal/storage"
)

func TestHomepageSummaryAuthAndMaintenanceExclusion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-homepage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	// 1. Generate token
	tokenID, tokenSecret, err := db.GenerateToken("Test Homepage")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// 2. Set up endpoints
	now := time.Now()
	db.SaveState(&storage.EndpointState{
		EndpointID:  "ep-up",
		Status:      "UP",
		LastCheckAt: &now,
	})
	db.SaveState(&storage.EndpointState{
		EndpointID:  "ep-down",
		Status:      "DOWN",
		LastCheckAt: &now,
	})
	db.SaveState(&storage.EndpointState{
		EndpointID:  "ep-maint",
		Status:      "DOWN", // Prober sees down, but covered by maintenance!
		LastCheckAt: &now,
	})

	cfg := &config.RuntimeConfig{
		Endpoints: map[string]config.Endpoint{
			"ep-up":    {ID: "ep-up", DisplayName: "Up Service", Zone: "default"},
			"ep-down":  {ID: "ep-down", DisplayName: "Down Service", Zone: "default"},
			"ep-maint": {ID: "ep-maint", DisplayName: "Maint Service", Zone: "default"},
		},
	}

	broker := api.NewSSEBroker()
	srv := api.NewServer(cfg, db, broker, os.DirFS("."))
	// Maintenance function returns true for ep-maint
	srv.IsMaintenanceActive = func(epID, zone string, t time.Time) bool {
		return epID == "ep-maint"
	}

	// Test 1: Unauthenticated request should return 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/status/summary", nil)
	recUnauth := httptest.NewRecorder()
	srv.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for missing token, got %d", recUnauth.Code)
	}

	// Test 2: Authenticated with Headers
	reqHeaders := httptest.NewRequest(http.MethodGet, "/api/v1/status/summary", nil)
	reqHeaders.Header.Set("X-API-Token-Id", tokenID)
	reqHeaders.Header.Set("X-API-Token-Secret", tokenSecret)
	recHeaders := httptest.NewRecorder()
	srv.ServeHTTP(recHeaders, reqHeaders)

	if recHeaders.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with header auth, got %d", recHeaders.Code)
	}

	var resp api.HomepageSummaryResponse
	if err := json.NewDecoder(recHeaders.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	// 1 Up, 1 Down, 1 Maintenance (excluded from Down!)
	if resp.HostsUp != 1 {
		t.Errorf("expected Hosts Up = 1, got %d", resp.HostsUp)
	}
	if resp.HostsDown != 1 {
		t.Errorf("expected Hosts Down = 1 (maint host must be excluded), got %d", resp.HostsDown)
	}

	// Test 3: Authenticated with Bearer token
	reqBearer := httptest.NewRequest(http.MethodGet, "/api/v1/status/summary", nil)
	reqBearer.Header.Set("Authorization", fmt.Sprintf("Bearer %s:%s", tokenID, tokenSecret))
	recBearer := httptest.NewRecorder()
	srv.ServeHTTP(recBearer, reqBearer)
	if recBearer.Code != http.StatusOK {
		t.Errorf("expected 200 OK with Bearer auth, got %d", recBearer.Code)
	}

	// Test 4: Authenticated with Basic Auth
	reqBasic := httptest.NewRequest(http.MethodGet, "/api/v1/status/summary", nil)
	b64Auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", tokenID, tokenSecret)))
	reqBasic.Header.Set("Authorization", "Basic "+b64Auth)
	recBasic := httptest.NewRecorder()
	srv.ServeHTTP(recBasic, reqBasic)
	if recBasic.Code != http.StatusOK {
		t.Errorf("expected 200 OK with Basic auth, got %d", recBasic.Code)
	}

	// Test 5: Revoked token should return 401 immediately
	if err := db.RevokeToken(tokenID); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	reqRevoked := httptest.NewRequest(http.MethodGet, "/api/v1/status/summary", nil)
	reqRevoked.Header.Set("X-API-Token-Id", tokenID)
	reqRevoked.Header.Set("X-API-Token-Secret", tokenSecret)
	recRevoked := httptest.NewRecorder()
	srv.ServeHTTP(recRevoked, reqRevoked)
	if recRevoked.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for revoked token, got %d", recRevoked.Code)
	}
}
