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

func TestDashboardBrandingPayloadAndResolution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-api-branding-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	// 1. Test custom branding with icon slug resolution
	cfgCustom := &config.RuntimeConfig{
		Global: config.GlobalConfig{
			SelfhstIconCDN: "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/{icon}.svg",
			Branding: config.BrandingConfig{
				Title:    "Acme Status",
				Subtitle: "Core Cloud Services",
				Footer:   "© 2026 Acme Corp",
				Icon:     "grafana",
				Favicon:  "https://acme.corp/favicon.ico",
				TabTitle: "Acme Live Status",
			},
		},
		Zones:     map[string]config.Zone{},
		Endpoints: map[string]config.Endpoint{},
	}

	resp, err := api.BuildDashboardResponse(cfgCustom, db, func(z, e string, t time.Time) bool { return false })
	if err != nil {
		t.Fatalf("BuildDashboardResponse failed: %v", err)
	}

	if resp.Branding.Title != "Acme Status" {
		t.Errorf("expected Title 'Acme Status', got %q", resp.Branding.Title)
	}
	if resp.Branding.Subtitle != "Core Cloud Services" {
		t.Errorf("expected Subtitle 'Core Cloud Services', got %q", resp.Branding.Subtitle)
	}
	if resp.Branding.Footer != "© 2026 Acme Corp" {
		t.Errorf("expected Footer '© 2026 Acme Corp', got %q", resp.Branding.Footer)
	}
	if resp.Branding.Icon != "https://cdn.jsdelivr.net/gh/selfhst/icons/svg/grafana.svg" {
		t.Errorf("expected resolved Icon CDN URL, got %q", resp.Branding.Icon)
	}
	if resp.Branding.Favicon != "https://acme.corp/favicon.ico" {
		t.Errorf("expected Favicon URL, got %q", resp.Branding.Favicon)
	}
	if resp.Branding.TabTitle != "Acme Live Status" {
		t.Errorf("expected TabTitle 'Acme Live Status', got %q", resp.Branding.TabTitle)
	}

	// 2. Test empty branding fallback
	cfgEmpty := &config.RuntimeConfig{
		Global:    config.DefaultGlobalConfig(),
		Zones:     map[string]config.Zone{},
		Endpoints: map[string]config.Endpoint{},
	}
	// Clear branding fields to simulate uninitialized
	cfgEmpty.Global.Branding = config.BrandingConfig{}

	respDefault, err := api.BuildDashboardResponse(cfgEmpty, db, func(z, e string, t time.Time) bool { return false })
	if err != nil {
		t.Fatalf("BuildDashboardResponse failed: %v", err)
	}

	if respDefault.Branding.Title != "uptime-mon" {
		t.Errorf("expected default Title 'uptime-mon', got %q", respDefault.Branding.Title)
	}
	if respDefault.Branding.Subtitle != "Continuous GitOps Uptime & Protocol Monitoring" {
		t.Errorf("expected default Subtitle, got %q", respDefault.Branding.Subtitle)
	}
	if respDefault.Branding.Footer != "uptime-mon • Single-Binary Declarative Uptime Monitor" {
		t.Errorf("expected default Footer, got %q", respDefault.Branding.Footer)
	}
	if respDefault.Branding.TabTitle != "uptime-mon | Status Dashboard" {
		t.Errorf("expected default TabTitle 'uptime-mon | Status Dashboard', got %q", respDefault.Branding.TabTitle)
	}

	// 3. Assert full HTTP response endpoint JSON decoding
	srv := api.NewServer(cfgEmpty, db, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var httpResp api.PublicDashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&httpResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if httpResp.Branding.Title != "uptime-mon" {
		t.Errorf("expected HTTP response branding.title = 'uptime-mon', got %q", httpResp.Branding.Title)
	}
	if httpResp.Branding.TabTitle != "uptime-mon | Status Dashboard" {
		t.Errorf("expected HTTP response branding.tab_title = 'uptime-mon | Status Dashboard', got %q", httpResp.Branding.TabTitle)
	}
}

func TestDashboardUIConfigResponse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-api-ui-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	// 1. Test defaults
	cfgDefault := &config.RuntimeConfig{
		Global:    config.DefaultGlobalConfig(),
		Zones:     map[string]config.Zone{},
		Endpoints: map[string]config.Endpoint{},
	}
	respDefault, err := api.BuildDashboardResponse(cfgDefault, db, func(z, e string, t time.Time) bool { return false })
	if err != nil {
		t.Fatalf("BuildDashboardResponse failed: %v", err)
	}
	if respDefault.UI.DisableCollapse != false {
		t.Errorf("expected DisableCollapse false, got %v", respDefault.UI.DisableCollapse)
	}
	if respDefault.UI.InitiallyCollapsed != false {
		t.Errorf("expected InitiallyCollapsed false, got %v", respDefault.UI.InitiallyCollapsed)
	}

	// 2. Test initiallyCollapsed: true
	cfgInit := &config.RuntimeConfig{
		Global: config.GlobalConfig{
			InitiallyCollapsed: true,
		},
		Zones:     map[string]config.Zone{},
		Endpoints: map[string]config.Endpoint{},
	}
	respInit, err := api.BuildDashboardResponse(cfgInit, db, func(z, e string, t time.Time) bool { return false })
	if err != nil {
		t.Fatalf("BuildDashboardResponse failed: %v", err)
	}
	if respInit.UI.InitiallyCollapsed != true {
		t.Errorf("expected InitiallyCollapsed true, got %v", respInit.UI.InitiallyCollapsed)
	}
	if respInit.UI.DisableCollapse != false {
		t.Errorf("expected DisableCollapse false, got %v", respInit.UI.DisableCollapse)
	}

	// 3. Test precedence: disableCollapse overrides initiallyCollapsed
	cfgOverride := &config.RuntimeConfig{
		Global: config.GlobalConfig{
			DisableCollapse:    true,
			InitiallyCollapsed: true,
		},
		Zones:     map[string]config.Zone{},
		Endpoints: map[string]config.Endpoint{},
	}
	respOverride, err := api.BuildDashboardResponse(cfgOverride, db, func(z, e string, t time.Time) bool { return false })
	if err != nil {
		t.Fatalf("BuildDashboardResponse failed: %v", err)
	}
	if respOverride.UI.DisableCollapse != true {
		t.Errorf("expected DisableCollapse true, got %v", respOverride.UI.DisableCollapse)
	}
	if respOverride.UI.InitiallyCollapsed != false {
		t.Errorf("expected InitiallyCollapsed false when DisableCollapse is true, got %v", respOverride.UI.InitiallyCollapsed)
	}

	// 4. Test HTTP endpoint JSON response
	srv := api.NewServer(cfgOverride, db, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var httpResp api.PublicDashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&httpResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if httpResp.UI.DisableCollapse != true {
		t.Errorf("expected HTTP response ui.disable_collapse = true, got %v", httpResp.UI.DisableCollapse)
	}
	if httpResp.UI.InitiallyCollapsed != false {
		t.Errorf("expected HTTP response ui.initially_collapsed = false, got %v", httpResp.UI.InitiallyCollapsed)
	}
}

func TestDashboardDynamicVariables(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-api-vars-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := storage.Open(filepath.Join(tempDir, "uptime.db"))
	if err != nil {
		t.Fatalf("storage.Open failed: %v", err)
	}
	defer db.Close()

	cfg := &config.RuntimeConfig{
		Global: config.GlobalConfig{
			Timezone: "Europe/Berlin",
			Branding: config.BrandingConfig{
				Title:    "Dashboard ({total_endpoints} Hosts)",
				Subtitle: "Active in {total_zones} Zones on {hostname}",
				Footer:   "© {current_year} Ops ({timezone}) • v{version} • {unknown_token}",
				TabTitle: "Status {current_year}",
			},
		},
		Zones: map[string]config.Zone{
			"z1": {ID: "z1", DisplayName: "Zone 1"},
			"z2": {ID: "z2", DisplayName: "Zone 2"},
		},
		Endpoints: map[string]config.Endpoint{
			"e1": {ID: "e1", DisplayName: "Host 1", Type: "ping", Target: "1.1.1.1"},
			"e2": {ID: "e2", DisplayName: "Host 2", Type: "ping", Target: "1.0.0.1"},
			"e3": {ID: "e3", DisplayName: "Host 3", Type: "ping", Target: "8.8.8.8"},
		},
	}

	srv := api.NewServer(cfg, db, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var httpResp api.PublicDashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&httpResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	currentYear := time.Now().Format("2006")
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	// 1. Verify title
	expectedTitle := "Dashboard (3 Hosts)"
	if httpResp.Branding.Title != expectedTitle {
		t.Errorf("expected Title %q, got %q", expectedTitle, httpResp.Branding.Title)
	}

	// 2. Verify subtitle
	expectedSubtitle := "Active in 2 Zones on " + hostname
	if httpResp.Branding.Subtitle != expectedSubtitle {
		t.Errorf("expected Subtitle %q, got %q", expectedSubtitle, httpResp.Branding.Subtitle)
	}

	// 3. Verify footer with year, timezone, version, and preserved unknown token
	expectedFooter := "© " + currentYear + " Ops (Europe/Berlin) • v1.0.0 • {unknown_token}"
	if httpResp.Branding.Footer != expectedFooter {
		t.Errorf("expected Footer %q, got %q", expectedFooter, httpResp.Branding.Footer)
	}

	// 4. Verify tab title
	expectedTabTitle := "Status " + currentYear
	if httpResp.Branding.TabTitle != expectedTabTitle {
		t.Errorf("expected TabTitle %q, got %q", expectedTabTitle, httpResp.Branding.TabTitle)
	}
}
