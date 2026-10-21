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
