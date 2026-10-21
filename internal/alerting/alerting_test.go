package alerting_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"uptime-mon/internal/alerting"
	"uptime-mon/internal/config"
	"uptime-mon/internal/storage"
)

func TestDiscordEmbedFormatting(t *testing.T) {
	now := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	lastUp := now.Add(-1 * time.Hour).Add(-14 * time.Minute).Add(-22 * time.Second)

	// 1. Host Down Embed
	downEmbed := alerting.BuildHostDownEmbed("Primary Gateway", "networking", "connection refused", &lastUp, now)
	if downEmbed.Color != alerting.ColorRed {
		t.Errorf("expected red color for down embed, got %x", downEmbed.Color)
	}
	if len(downEmbed.Fields) < 3 {
		t.Errorf("expected at least 3 fields, got %d", len(downEmbed.Fields))
	}

	// 2. Host Up Embed
	downtime := 1*time.Hour + 14*time.Minute + 22*time.Second
	upEmbed := alerting.BuildHostUpEmbed("Primary Gateway", "networking", downtime, now)
	if upEmbed.Color != alerting.ColorGreen {
		t.Errorf("expected green color for up embed, got %x", upEmbed.Color)
	}

	foundDuration := false
	for _, f := range upEmbed.Fields {
		if f.Name == "Time Passed Since Down" && f.Value == "1h 14m 22s" {
			foundDuration = true
			break
		}
	}
	if !foundDuration {
		t.Errorf("expected formatted duration '1h 14m 22s' in fields: %+v", upEmbed.Fields)
	}
}

func TestAlertDeduplicationAcrossRestarts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-alert-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "uptime.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	var receivedCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	webhooks := []config.WebhookTarget{
		{
			ID:     "test-discord",
			Type:   "discord",
			URL:    ts.URL,
			Events: []string{"maintenance_created"},
		},
	}

	// First dispatcher instance
	d1 := alerting.NewDispatcher(db, webhooks)
	now := time.Now()
	d1.DispatchMaintenanceCreated("maint-1", "Upgrade firmware", "Tomorrow 02:00", []string{"networking"}, []string{"all"}, now)

	// Wait for async dispatch
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if receivedCount != 1 {
		t.Errorf("expected 1 alert delivered, got %d", receivedCount)
	}
	mu.Unlock()

	// Simulate daemon restart: create a new dispatcher with same SQLite DB
	d2 := alerting.NewDispatcher(db, webhooks)
	// Try sending again for the same maintenance key
	d2.DispatchMaintenanceCreated("maint-1", "Upgrade firmware", "Tomorrow 02:00", []string{"networking"}, []string{"all"}, now)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if receivedCount != 1 {
		t.Errorf("expected alert to be deduplicated across restart, but received count is %d", receivedCount)
	}
	mu.Unlock()
}

func TestGenericWebhookDelivery(t *testing.T) {
	var capturedPayload alerting.GenericWebhookPayload
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		json.Unmarshal(body, &capturedPayload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tempDir, _ := os.MkdirTemp("", "uptimemon-test-*")
	defer os.RemoveAll(tempDir)
	db, _ := storage.Open(filepath.Join(tempDir, "uptime.db"))
	defer db.Close()

	webhooks := []config.WebhookTarget{
		{
			ID:     "generic-test",
			Type:   "generic",
			URL:    ts.URL,
			Events: []string{"host_down"},
		},
	}

	d := alerting.NewDispatcher(db, webhooks)
	now := time.Now()
	d.DispatchHostDown("core-dns", "Core DNS", "networking", "timeout after 5s", &now, now)

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if capturedPayload.Event != "host_down" || capturedPayload.EndpointID != "core-dns" {
		t.Errorf("unexpected generic payload captured: %+v", capturedPayload)
	}
}
