package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"uptime-mon/internal/config"
)

func TestLoadAndValidate(t *testing.T) {
	root, err := filepath.Abs("../../config")
	if err != nil {
		t.Fatalf("failed to get config path: %v", err)
	}

	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("skipping test as config directory does not exist at relative path")
	}

	rc, err := config.Load(root)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(rc.Zones) < 2 {
		t.Errorf("expected at least 2 zones, got %d", len(rc.Zones))
	}
	if len(rc.Endpoints) < 4 {
		t.Errorf("expected at least 4 endpoints, got %d", len(rc.Endpoints))
	}

	errs := config.Validate(rc)
	if len(errs) > 0 {
		t.Fatalf("Validate failed with errors: %v", errs)
	}
}

func TestMatchStatusCode(t *testing.T) {
	tests := []struct {
		code     int
		patterns []string
		expected bool
	}{
		{200, []string{"20X"}, true},
		{204, []string{"20X"}, true},
		{301, []string{"20X"}, false},
		{301, []string{"20X", "30X"}, true},
		{201, []string{"200-204"}, true},
		{205, []string{"200-204"}, false},
		{500, []string{"500"}, true},
		{502, []string{"500"}, false},
	}

	for _, tt := range tests {
		got := config.MatchStatusCode(tt.code, tt.patterns)
		if got != tt.expected {
			t.Errorf("MatchStatusCode(%d, %v) = %v; want %v", tt.code, tt.patterns, got, tt.expected)
		}
	}
}

func TestMaintenanceTitleFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-cfg-title-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	maintDir := filepath.Join(tempDir, "maintenance")
	schedDir := filepath.Join(maintDir, "scheduled")
	os.MkdirAll(schedDir, 0755)

	// One-off with title
	titledOneOff := `
id: "upgrade-db"
title: "Database Cluster Upgrade"
message: "Upgrading postgres to v16"
start_date: "2026-10-25"
start_time: "02:00"
end_date: "2026-10-25"
end_time: "04:00"
`
	os.WriteFile(filepath.Join(maintDir, "upgrade-db.yml"), []byte(titledOneOff), 0644)

	// One-off without title (must fallback to id)
	fallbackOneOff := `
id: "switch-reboot"
message: "Switch reboot"
start_date: "2026-10-25"
start_time: "02:00"
end_date: "2026-10-25"
end_time: "04:00"
`
	os.WriteFile(filepath.Join(maintDir, "switch-reboot.yml"), []byte(fallbackOneOff), 0644)

	// Scheduled with title
	titledSched := `
id: "weekly-backup"
title: "Weekly Backup & Snapshot"
message: "Snapshotting disks"
schedule: "0 2 * * 0"
`
	os.WriteFile(filepath.Join(schedDir, "weekly-backup.yml"), []byte(titledSched), 0644)

	// Scheduled without title (must fallback to id)
	fallbackSched := `
id: "nightly-vacuum"
message: "Nightly vacuum"
schedule: "0 3 * * *"
`
	os.WriteFile(filepath.Join(schedDir, "nightly-vacuum.yml"), []byte(fallbackSched), 0644)

	rc, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Assertions
	if rc.OneOffMaintenance["upgrade-db"].Title != "Database Cluster Upgrade" {
		t.Errorf("expected Title 'Database Cluster Upgrade', got %q", rc.OneOffMaintenance["upgrade-db"].Title)
	}
	if rc.OneOffMaintenance["switch-reboot"].Title != "switch-reboot" {
		t.Errorf("expected Title fallback to 'switch-reboot', got %q", rc.OneOffMaintenance["switch-reboot"].Title)
	}
	if rc.ScheduledMaintenance["weekly-backup"].Title != "Weekly Backup & Snapshot" {
		t.Errorf("expected Title 'Weekly Backup & Snapshot', got %q", rc.ScheduledMaintenance["weekly-backup"].Title)
	}
	if rc.ScheduledMaintenance["nightly-vacuum"].Title != "nightly-vacuum" {
		t.Errorf("expected Title fallback to 'nightly-vacuum', got %q", rc.ScheduledMaintenance["nightly-vacuum"].Title)
	}
}

func TestBrandingConfigParsingAndDefaults(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test completely omitted branding in config.yml
	emptyCfg := `
interval: "30s"
timeout: "3s"
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.yml"), []byte(emptyCfg), 0644); err != nil {
		t.Fatalf("failed to write config.yml: %v", err)
	}

	rc, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if rc.Global.Branding.Title != "uptime-mon" {
		t.Errorf("expected default Title 'uptime-mon', got %q", rc.Global.Branding.Title)
	}
	if rc.Global.Branding.Subtitle != "Continuous GitOps Uptime & Protocol Monitoring" {
		t.Errorf("expected default Subtitle, got %q", rc.Global.Branding.Subtitle)
	}
	if rc.Global.Branding.Footer != "uptime-mon • Single-Binary Declarative Uptime Monitor" {
		t.Errorf("expected default Footer, got %q", rc.Global.Branding.Footer)
	}
	if rc.Global.Branding.TabTitle != "uptime-mon | Status Dashboard" {
		t.Errorf("expected default TabTitle, got %q", rc.Global.Branding.TabTitle)
	}

	// 2. Test custom branding with tab_title derived from title
	customCfg := `
branding:
  title: "Homelab Ops"
  subtitle: "Internal Infrastructure & Services"
  footer: "Operated by Platform Team"
  icon: "proxmox"
  favicon: "https://example.com/favicon.png"
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.yml"), []byte(customCfg), 0644); err != nil {
		t.Fatalf("failed to write custom config.yml: %v", err)
	}

	rcCustom, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if rcCustom.Global.Branding.Title != "Homelab Ops" {
		t.Errorf("expected Title 'Homelab Ops', got %q", rcCustom.Global.Branding.Title)
	}
	if rcCustom.Global.Branding.Subtitle != "Internal Infrastructure & Services" {
		t.Errorf("expected custom Subtitle, got %q", rcCustom.Global.Branding.Subtitle)
	}
	if rcCustom.Global.Branding.Footer != "Operated by Platform Team" {
		t.Errorf("expected custom Footer, got %q", rcCustom.Global.Branding.Footer)
	}
	if rcCustom.Global.Branding.Icon != "proxmox" {
		t.Errorf("expected Icon 'proxmox', got %q", rcCustom.Global.Branding.Icon)
	}
	if rcCustom.Global.Branding.Favicon != "https://example.com/favicon.png" {
		t.Errorf("expected Favicon URL, got %q", rcCustom.Global.Branding.Favicon)
	}
	if rcCustom.Global.Branding.TabTitle != "Homelab Ops | Status Dashboard" {
		t.Errorf("expected derived TabTitle 'Homelab Ops | Status Dashboard', got %q", rcCustom.Global.Branding.TabTitle)
	}
}

func TestZoneCollapseConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-collapse-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Defaults should be false
	rcDefault, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if rcDefault.Global.DisableCollapse != false {
		t.Errorf("expected DisableCollapse default false, got %v", rcDefault.Global.DisableCollapse)
	}
	if rcDefault.Global.InitiallyCollapsed != false {
		t.Errorf("expected InitiallyCollapsed default false, got %v", rcDefault.Global.InitiallyCollapsed)
	}

	// 2. Test camelCase
	camelCfg := `
disableCollapse: true
initiallyCollapsed: true
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.yml"), []byte(camelCfg), 0644); err != nil {
		t.Fatalf("failed to write config.yml: %v", err)
	}
	rcCamel, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if rcCamel.Global.DisableCollapse != true {
		t.Errorf("expected DisableCollapse true, got %v", rcCamel.Global.DisableCollapse)
	}
	if rcCamel.Global.InitiallyCollapsed != true {
		t.Errorf("expected InitiallyCollapsed true, got %v", rcCamel.Global.InitiallyCollapsed)
	}

	// 3. Test snake_case
	snakeCfg := `
disable_collapse: true
initially_collapsed: true
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.yml"), []byte(snakeCfg), 0644); err != nil {
		t.Fatalf("failed to write config.yml: %v", err)
	}
	rcSnake, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if rcSnake.Global.DisableCollapse != true {
		t.Errorf("expected DisableCollapse true, got %v", rcSnake.Global.DisableCollapse)
	}
	if rcSnake.Global.InitiallyCollapsed != true {
		t.Errorf("expected InitiallyCollapsed true, got %v", rcSnake.Global.InitiallyCollapsed)
	}
}

func TestAlertingConfigMultiFileAndFormats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-alerting-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	alertDir := filepath.Join(tempDir, "alerting")
	if err := os.MkdirAll(alertDir, 0755); err != nil {
		t.Fatalf("failed to create alerting dir: %v", err)
	}

	// 1. File with AlertingConfig struct: { webhooks: [...] }
	f1 := `
webhooks:
  - id: "webhook-1"
    type: "discord"
    url: "https://discord.com/api/webhooks/test1"
    events: ["host_down"]
`
	if err := os.WriteFile(filepath.Join(alertDir, "file1.yml"), []byte(f1), 0644); err != nil {
		t.Fatalf("write file1: %v", err)
	}

	// 2. File with array format: [ { id: ... } ]
	f2 := `
- id: "webhook-2"
  type: "generic"
  url: "https://example.com/webhook2"
  events: ["host_up"]
- id: "webhook-3"
  type: "discord"
  url: "https://discord.com/api/webhooks/test3"
  events: ["tls_expiry"]
`
	if err := os.WriteFile(filepath.Join(alertDir, "file2.yaml"), []byte(f2), 0644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	// 3. File with single webhook format
	f3 := `
id: "webhook-4"
type: "generic"
url: "https://example.com/webhook4"
events: ["host_down", "host_up"]
`
	if err := os.WriteFile(filepath.Join(alertDir, "file3.yml"), []byte(f3), 0644); err != nil {
		t.Fatalf("write file3: %v", err)
	}

	rc, err := config.Load(tempDir)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if len(rc.Alerting.Webhooks) != 4 {
		t.Fatalf("expected 4 webhooks loaded across 3 files, got %d", len(rc.Alerting.Webhooks))
	}

	webhookIDs := make(map[string]bool)
	for _, w := range rc.Alerting.Webhooks {
		webhookIDs[w.ID] = true
	}

	for _, expectedID := range []string{"webhook-1", "webhook-2", "webhook-3", "webhook-4"} {
		if !webhookIDs[expectedID] {
			t.Errorf("expected webhook %q to be loaded", expectedID)
		}
	}
}
