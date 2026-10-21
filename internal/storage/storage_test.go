package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"uptime-mon/internal/storage"
)

func TestStorageOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-test-*")
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

	// 1. Test EndpointState default when non-existent
	state, err := db.GetState("test-endpoint")
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if state.Status != "PENDING" {
		t.Errorf("expected initial status PENDING, got %s", state.Status)
	}

	// 2. Test saving and updating EndpointState
	now := time.Now().Truncate(time.Second)
	state.Status = "UP"
	state.ConsecutiveFailures = 0
	state.LastCheckAt = &now
	state.LastUpAt = &now
	state.LastLatencyMS = 42

	if err := db.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := db.GetState("test-endpoint")
	if err != nil {
		t.Fatalf("GetState after save failed: %v", err)
	}
	if loaded.Status != "UP" || loaded.LastLatencyMS != 42 {
		t.Errorf("unexpected loaded state: %+v", loaded)
	}

	// 3. Test ProbeHistory insertion and retrieval
	if err := db.RecordProbe("test-endpoint", "UP", 42, "", now.Add(-2*time.Minute)); err != nil {
		t.Fatalf("RecordProbe 1 failed: %v", err)
	}
	if err := db.RecordProbe("test-endpoint", "DOWN", 0, "connection refused", now.Add(-1*time.Minute)); err != nil {
		t.Fatalf("RecordProbe 2 failed: %v", err)
	}
	if err := db.RecordProbe("test-endpoint", "UP", 38, "", now); err != nil {
		t.Fatalf("RecordProbe 3 failed: %v", err)
	}

	history, err := db.GetRecentHistory("test-endpoint", 10)
	if err != nil {
		t.Fatalf("GetRecentHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history records, got %d", len(history))
	}
	// Verify oldest-to-newest ordering
	if history[0].LatencyMS != 42 || history[2].LatencyMS != 38 {
		t.Errorf("expected chronological order, got: %+v", history)
	}

	// 4. Test Uptime calculation
	pct, err := db.CalculateUptimePercentage("test-endpoint", 24*time.Hour)
	if err != nil {
		t.Fatalf("CalculateUptimePercentage failed: %v", err)
	}
	// 2 UP out of 3 total = ~66.67%
	if pct < 66.0 || pct > 67.0 {
		t.Errorf("expected ~66.67%% uptime, got %f", pct)
	}
}
