package api_test

import (
	"encoding/json"
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

func TestDashboardTargetConcealmentSecurityContract(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	// Seed state
	now := time.Now()
	db.SaveState(&storage.EndpointState{
		EndpointID:    "secret-internal-db",
		Status:        "UP",
		LastCheckAt:   &now,
		LastLatencyMS: 5,
	})
	db.SaveState(&storage.EndpointState{
		EndpointID:    "public-api",
		Status:        "UP",
		LastCheckAt:   &now,
		LastLatencyMS: 12,
	})

	cfg := &config.RuntimeConfig{
		Zones: map[string]config.Zone{
			"backend": {
				ID:          "backend",
				DisplayName: "Backend Services",
				Order:       1,
			},
		},
		Endpoints: map[string]config.Endpoint{
			"secret-internal-db": {
				ID:          "secret-internal-db",
				DisplayName: "Internal Database",
				Zone:        "backend",
				Type:        "tcp",
				Target:      "10.240.0.15",
				Port:        5432,
				ShowTarget:  false, // MUST BE CONCEALED
			},
			"public-api": {
				ID:          "public-api",
				DisplayName: "Public API Gateway",
				Zone:        "backend",
				Type:        "https",
				Target:      "https://api.example.com",
				ShowTarget:  true, // ALLOWED TO BE EXPOSED
			},
		},
	}

	broker := api.NewSSEBroker()
	srv := api.NewServer(cfg, db, broker, os.DirFS("."))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp api.PublicDashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode dashboard json: %v", err)
	}

	if len(resp.Zones) == 0 {
		t.Fatalf("expected at least 1 zone, got 0")
	}

	var secretEp, publicEp *api.DashboardEndpointView
	for _, z := range resp.Zones {
		for i := range z.Endpoints {
			ep := &z.Endpoints[i]
			if ep.ID == "secret-internal-db" {
				secretEp = ep
			} else if ep.ID == "public-api" {
				publicEp = ep
			}
		}
	}

	if secretEp == nil {
		t.Fatalf("secret-internal-db not found in dashboard response")
	}
	if publicEp == nil {
		t.Fatalf("public-api not found in dashboard response")
	}

	// SECURITY ASSERTION: Secret target must be nil
	if secretEp.Target != nil {
		t.Errorf("SECURITY VIOLATION: expected secret-internal-db Target to be nil, but got %q", *secretEp.Target)
	}

	// Public endpoint with ShowTarget=true must have target populated
	if publicEp.Target == nil || *publicEp.Target != "https://api.example.com" {
		t.Errorf("expected public-api Target to be 'https://api.example.com', got %v", publicEp.Target)
	}
}

func TestDashboardMaintenanceVisibilityContracts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-maint-api-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	now := time.Now()
	futureStart := now.Add(1 * time.Hour).Format("2006-01-02 15:04")
	futureEnd := now.Add(3 * time.Hour).Format("2006-01-02 15:04")
	pastStart := now.Add(-5 * time.Hour).Format("2006-01-02 15:04")
	pastEnd := now.Add(-2 * time.Hour).Format("2006-01-02 15:04")

	cfg := &config.RuntimeConfig{
		Zones: map[string]config.Zone{
			"net": {ID: "net", DisplayName: "Networking"},
		},
		Endpoints: map[string]config.Endpoint{
			"gw": {ID: "gw", DisplayName: "Core Gateway", Zone: "net", Type: "ping", Target: "10.0.0.1"},
		},
		OneOffMaintenance: map[string]config.OneOffMaintenance{
			"upcoming-upgrade": {
				ID:            "upcoming-upgrade",
				Title:         "Switch OS Upgrade",
				Message:       "Brief reload expected",
				Start:         now.Add(1 * time.Hour).Format(time.RFC3339),
				End:           now.Add(3 * time.Hour).Format(time.RFC3339),
				AffectedZones: []string{"net"},
				AffectedHosts: []string{"gw"},
			},
			"expired-maintenance": {
				ID:            "expired-maintenance",
				Title:         "Old Maintenance",
				Start:         now.Add(-5 * time.Hour).Format(time.RFC3339),
				End:           now.Add(-2 * time.Hour).Format(time.RFC3339),
				AffectedZones: []string{"net"},
				AffectedHosts: []string{"all"},
			},
		},
		ScheduledMaintenance: map[string]config.ScheduledMaintenance{
			"weekly-backup": {
				ID:            "weekly-backup",
				Title:         "Weekly Backup",
				Schedule:      "0 2 * * 0",
				Duration:      config.Duration(2 * time.Hour),
				AffectedZones: []string{"net"},
				AffectedHosts: []string{"all"},
			},
		},
	}
	_ = futureStart
	_ = futureEnd
	_ = pastStart
	_ = pastEnd

	broker := api.NewSSEBroker()
	srv := api.NewServer(cfg, db, broker, os.DirFS("."))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp api.PublicDashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	// 1. Assert expired maintenance is excluded and upcoming is included
	if len(resp.OneOffMaintenances) != 1 {
		t.Fatalf("expected 1 one_off_maintenances (expired excluded), got %d", len(resp.OneOffMaintenances))
	}
	om := resp.OneOffMaintenances[0]
	if om.Title != "Switch OS Upgrade" {
		t.Errorf("expected Title 'Switch OS Upgrade', got %q", om.Title)
	}

	// 2. Assert resolved display names for affected zones and hosts
	if len(om.AffectedZones) != 1 || om.AffectedZones[0].DisplayName != "Networking" {
		t.Errorf("expected zone display_name 'Networking', got %+v", om.AffectedZones)
	}
	if len(om.AffectedHosts) != 1 || om.AffectedHosts[0].DisplayName != "Core Gateway" {
		t.Errorf("expected host display_name 'Core Gateway', got %+v", om.AffectedHosts)
	}

	// 3. Assert scheduled maintenances count and payload
	if resp.Counts.ScheduledMaintenanceCount != 1 {
		t.Errorf("expected scheduled_maintenance_count = 1, got %d", resp.Counts.ScheduledMaintenanceCount)
	}
	if len(resp.ScheduledMaintenances) != 1 {
		t.Fatalf("expected 1 scheduled_maintenances, got %d", len(resp.ScheduledMaintenances))
	}
	sm := resp.ScheduledMaintenances[0]
	if sm.Title != "Weekly Backup" {
		t.Errorf("expected scheduled Title 'Weekly Backup', got %q", sm.Title)
	}
	if !sm.AllHosts {
		t.Errorf("expected sm.AllHosts to be true for 'all' hosts")
	}
}
