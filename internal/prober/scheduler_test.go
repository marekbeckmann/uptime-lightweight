package prober

import (
	"context"
	"path/filepath"
	"testing"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/storage"
)

type mockUpProber struct{}

func (m *mockUpProber) Probe(ctx context.Context, endpoint *config.Endpoint, timeout time.Duration) ProbeResult {
	return ProbeResult{
		Status:  "UP",
		Latency: 5 * time.Millisecond,
	}
}

func TestSchedulerDebounceAndHistoryRecording(t *testing.T) {
	tempDir := t.TempDir()
	db, err := storage.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	s := NewScheduler(db, nil)
	defer s.Stop()

	// Capture emitted events
	var events []TransitionEvent
	s.OnEvent(func(evt TransitionEvent) {
		events = append(events, evt)
	})

	// Pre-seed endpoint with retries: 3
	retries := 3
	ep := config.Endpoint{
		ID:          "test-ep",
		DisplayName: "Test Endpoint",
		Zone:        "core",
		Type:        "tcp",
		Target:      "127.0.0.1",
		Port:        59999, // Unopened port causes failure
		Retries:     &retries,
	}

	// 1. First probe failure: history should be recorded as DOWN, but state debounced
	s.executeProbe(ep)

	st, err := db.GetState(ep.ID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if st.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", st.ConsecutiveFailures)
	}

	hist, err := db.GetRecentHistory(ep.ID, 10)
	if err != nil {
		t.Fatalf("GetRecentHistory failed: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(hist))
	}
	if hist[0].Status != "DOWN" {
		t.Errorf("expected history record to be DOWN immediately on failure, got %s", hist[0].Status)
	}

	// Verify emitted HEARTBEAT event carries ProbeStatus DOWN
	if len(events) == 0 {
		t.Fatalf("expected emitted events")
	}
	lastEvent := events[len(events)-1]
	if lastEvent.Type != "HEARTBEAT" {
		t.Errorf("expected last event HEARTBEAT, got %s", lastEvent.Type)
	}
	if lastEvent.ProbeStatus != "DOWN" {
		t.Errorf("expected ProbeStatus DOWN, got %s", lastEvent.ProbeStatus)
	}

	// 2. Second probe failure
	s.executeProbe(ep)
	st, _ = db.GetState(ep.ID)
	if st.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", st.ConsecutiveFailures)
	}
	hist, _ = db.GetRecentHistory(ep.ID, 10)
	if len(hist) != 2 || hist[1].Status != "DOWN" {
		t.Errorf("expected second history record to be DOWN, got %v", hist)
	}

	// 3. Third probe failure -> reaches retries threshold -> state transitions to DOWN
	s.executeProbe(ep)
	st, _ = db.GetState(ep.ID)
	if st.ConsecutiveFailures != 3 {
		t.Errorf("expected 3 consecutive failures, got %d", st.ConsecutiveFailures)
	}
	if st.Status != "DOWN" {
		t.Errorf("expected state status to be DOWN after 3 failures, got %s", st.Status)
	}

	// Verify DOWN transition event was emitted
	foundDownTransition := false
	for _, e := range events {
		if e.Type == "DOWN" && e.EndpointID == ep.ID {
			foundDownTransition = true
			break
		}
	}
	if !foundDownTransition {
		t.Errorf("expected DOWN transition event to be emitted")
	}

	// 4. Test recovery: succeeding probe must immediately restore status to UP
	s.probers["tcp"] = &mockUpProber{}
	s.executeProbe(ep)

	st, _ = db.GetState(ep.ID)
	if st.Status != "UP" {
		t.Errorf("expected state status to immediately recover to UP, got %s", st.Status)
	}
	if st.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures to reset to 0, got %d", st.ConsecutiveFailures)
	}

	hist, _ = db.GetRecentHistory(ep.ID, 10)
	if len(hist) != 4 || hist[3].Status != "UP" {
		t.Errorf("expected fourth history record to be UP, got %v", hist)
	}

	// Verify UP transition event was emitted
	foundUpTransition := false
	for _, e := range events {
		if e.Type == "UP" && e.EndpointID == ep.ID {
			foundUpTransition = true
			break
		}
	}
	if !foundUpTransition {
		t.Errorf("expected UP transition event to be emitted")
	}
}

func TestSchedulerTargetHotReloadRecycling(t *testing.T) {
	tempDir := t.TempDir()
	db, err := storage.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	s := NewScheduler(db, nil)
	defer s.Stop()

	ep1 := config.Endpoint{
		ID:     "ep-1",
		Type:   "ping",
		Target: "1.1.1.1",
	}
	ep2 := config.Endpoint{
		ID:     "ep-1",
		Type:   "ping",
		Target: "10.10.10.10",
	}

	if !s.endpointNeedsRestart(&ep1, &ep2) {
		t.Errorf("expected endpointNeedsRestart=true when Target changes")
	}

	epSame := config.Endpoint{
		ID:     "ep-1",
		Type:   "ping",
		Target: "1.1.1.1",
	}
	if s.endpointNeedsRestart(&ep1, &epSame) {
		t.Errorf("expected endpointNeedsRestart=false when nothing changes")
	}

	// Test UpdateConfig restarts worker and replaces stop channel
	cfg1 := &config.RuntimeConfig{
		Global: config.DefaultGlobalConfig(),
		Endpoints: map[string]config.Endpoint{
			"ep-1": ep1,
		},
	}
	s.UpdateConfig(cfg1)

	oldChan := s.stopChannels["ep-1"]
	if oldChan == nil {
		t.Fatalf("expected stopChannel for ep-1")
	}

	cfg2 := &config.RuntimeConfig{
		Global: config.DefaultGlobalConfig(),
		Endpoints: map[string]config.Endpoint{
			"ep-1": ep2,
		},
	}
	s.UpdateConfig(cfg2)

	newChan := s.stopChannels["ep-1"]
	if newChan == oldChan {
		t.Errorf("expected new stopChannel after target update, got same channel")
	}
}
