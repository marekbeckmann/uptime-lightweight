package prober

import (
	"context"
	"log"
	"sync"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/storage"
)

// TransitionEvent is emitted when status changes or a probe runs
type TransitionEvent struct {
	EndpointID      string
	DisplayName     string
	Zone            string
	Type            string // "DOWN", "UP", "HEARTBEAT", "TLS_EXPIRY"
	PreviousStatus  string
	CurrentStatus   string
	LatencyMS       int
	Timestamp       time.Time
	DowntimeDuration time.Duration
	LastUpAt        *time.Time
	LastError       string
	TLSExpiryAt     *time.Time
	WarnDays        int
}

// MaintenanceChecker interface to avoid circular dependency
type MaintenanceChecker interface {
	IsEndpointInMaintenance(endpointID, zoneID string, now time.Time) bool
}

// Scheduler manages active probe tickers for all configured endpoints
type Scheduler struct {
	db                 *storage.DB
	maintChecker       MaintenanceChecker
	probers            map[string]Prober
	eventCallbacks     []func(TransitionEvent)
	endpoints          map[string]config.Endpoint
	global             config.GlobalConfig
	mu                 sync.RWMutex
	stopChannels       map[string]chan struct{}
	ctx                context.Context
	cancel             context.CancelFunc
}

func NewScheduler(db *storage.DB, maintChecker MaintenanceChecker) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		db:           db,
		maintChecker: maintChecker,
		probers: map[string]Prober{
			"ping":  NewPingProber(),
			"tcp":   NewTCPProber(),
			"dns":   NewDNSProber(),
			"https": NewHTTPSProber(),
		},
		endpoints:    make(map[string]config.Endpoint),
		stopChannels: make(map[string]chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// OnEvent registers a listener for probe updates and transitions
func (s *Scheduler) OnEvent(fn func(TransitionEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventCallbacks = append(s.eventCallbacks, fn)
}

func (s *Scheduler) emitEvent(evt TransitionEvent) {
	s.mu.RLock()
	cbs := make([]func(TransitionEvent), len(s.eventCallbacks))
	copy(cbs, s.eventCallbacks)
	s.mu.RUnlock()

	for _, cb := range cbs {
		cb(evt)
	}
}

// UpdateConfig updates the running probe loops based on new RuntimeConfig
func (s *Scheduler) UpdateConfig(cfg *config.RuntimeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.global = cfg.Global
	newEndpoints := cfg.Endpoints

	// 1. Stop probes for removed endpoints
	for id, stopCh := range s.stopChannels {
		if _, exists := newEndpoints[id]; !exists {
			close(stopCh)
			delete(s.stopChannels, id)
			delete(s.endpoints, id)
			log.Printf("[scheduler] stopped probe for removed endpoint %s", id)
		}
	}

	// 2. Start or update probes
	for id, ep := range newEndpoints {
		oldEp, exists := s.endpoints[id]
		s.endpoints[id] = ep

		// If new or interval changed, recycle probe worker
		needsRestart := !exists
		if exists {
			oldInterval := s.getInterval(&oldEp)
			newInterval := s.getInterval(&ep)
			if oldInterval != newInterval {
				needsRestart = true
			}
		}

		if needsRestart {
			if stopCh, ok := s.stopChannels[id]; ok {
				close(stopCh)
			}
			stopCh := make(chan struct{})
			s.stopChannels[id] = stopCh
			go s.runEndpointLoop(ep, stopCh)
			log.Printf("[scheduler] started/updated probe loop for %s (type=%s, interval=%v)", id, ep.Type, s.getInterval(&ep))
		}
	}
}

func (s *Scheduler) getInterval(ep *config.Endpoint) time.Duration {
	if ep.Interval != nil && ep.Interval.Duration() > 0 {
		return ep.Interval.Duration()
	}
	if s.global.Interval.Duration() > 0 {
		return s.global.Interval.Duration()
	}
	return 60 * time.Second
}

func (s *Scheduler) getTimeout(ep *config.Endpoint) time.Duration {
	if ep.Timeout != nil && ep.Timeout.Duration() > 0 {
		return ep.Timeout.Duration()
	}
	if s.global.Timeout.Duration() > 0 {
		return s.global.Timeout.Duration()
	}
	return 5 * time.Second
}

func (s *Scheduler) getRetries(ep *config.Endpoint) int {
	if ep.Retries != nil && *ep.Retries > 0 {
		return *ep.Retries
	}
	if s.global.Retries > 0 {
		return s.global.Retries
	}
	return 3
}

func (s *Scheduler) runEndpointLoop(ep config.Endpoint, stopCh chan struct{}) {
	interval := s.getInterval(&ep)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial probe immediately
	s.executeProbe(ep)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-stopCh:
			return
		case <-ticker.C:
			s.executeProbe(ep)
		}
	}
}

func (s *Scheduler) executeProbe(ep config.Endpoint) {
	prober, ok := s.probers[ep.Type]
	if !ok {
		log.Printf("[scheduler] no prober for type %s on endpoint %s", ep.Type, ep.ID)
		return
	}

	timeout := s.getTimeout(&ep)
	ctx, cancel := context.WithTimeout(s.ctx, timeout+500*time.Millisecond)
	defer cancel()

	result := prober.Probe(ctx, &ep, timeout)
	now := time.Now()
	latencyMS := int(result.Latency.Milliseconds())
	errMsg := ""
	if result.Error != nil {
		errMsg = result.Error.Error()
	}

	// 1. Check if covered by maintenance
	inMaint := false
	if s.maintChecker != nil {
		inMaint = s.maintChecker.IsEndpointInMaintenance(ep.ID, ep.Zone, now)
	}

	// 2. Fetch existing state from SQLite
	state, err := s.db.GetState(ep.ID)
	if err != nil {
		log.Printf("[scheduler] failed to get state for %s: %v", ep.ID, err)
		return
	}

	previousStatus := state.Status
	retries := s.getRetries(&ep)

	var effectiveStatus string
	if inMaint {
		effectiveStatus = "MAINTENANCE"
	} else if result.Status == "UP" {
		effectiveStatus = "UP"
		state.ConsecutiveFailures = 0
		state.LastUpAt = &now
		state.LastError = ""
	} else {
		// Failed check
		state.ConsecutiveFailures++
		state.LastError = errMsg
		if state.ConsecutiveFailures >= retries {
			effectiveStatus = "DOWN"
		} else {
			// Debouncing - maintain previous status until retries exhausted
			if previousStatus == "UP" {
				effectiveStatus = "UP"
			} else {
				effectiveStatus = previousStatus
			}
		}
	}

	// 3. State transition alerts logic
	if !inMaint {
		if previousStatus != "DOWN" && effectiveStatus == "DOWN" {
			state.DownSince = &now
			s.emitEvent(TransitionEvent{
				EndpointID:     ep.ID,
				DisplayName:    ep.DisplayName,
				Zone:           ep.Zone,
				Type:           "DOWN",
				PreviousStatus: previousStatus,
				CurrentStatus:  effectiveStatus,
				LatencyMS:      latencyMS,
				Timestamp:      now,
				LastUpAt:       state.LastUpAt,
				LastError:      errMsg,
			})
		} else if previousStatus == "DOWN" && effectiveStatus == "UP" {
			var downDuration time.Duration
			if state.DownSince != nil {
				downDuration = now.Sub(*state.DownSince)
			}
			state.DownSince = nil
			s.emitEvent(TransitionEvent{
				EndpointID:       ep.ID,
				DisplayName:      ep.DisplayName,
				Zone:             ep.Zone,
				Type:             "UP",
				PreviousStatus:   previousStatus,
				CurrentStatus:    effectiveStatus,
				LatencyMS:        latencyMS,
				Timestamp:        now,
				DowntimeDuration: downDuration,
				LastUpAt:         &now,
			})
		}
	}

	// 4. Check TLS expiry warning triggers
	if result.TLSExpiryAt != nil {
		state.TLSExpiryAt = result.TLSExpiryAt
		daysLeft := int(time.Until(*result.TLSExpiryAt).Hours() / 24)
		for _, warnThreshold := range ep.TLSIgnoreWarnDays {
			if daysLeft <= warnThreshold && state.LastTLSWarnThreshold != warnThreshold {
				state.LastTLSWarnThreshold = warnThreshold
				s.emitEvent(TransitionEvent{
					EndpointID:  ep.ID,
					DisplayName: ep.DisplayName,
					Zone:        ep.Zone,
					Type:        "TLS_EXPIRY",
					Timestamp:   now,
					TLSExpiryAt: result.TLSExpiryAt,
					WarnDays:    daysLeft,
				})
				break
			}
		}
	}

	// 5. Save updated state and probe history
	state.Status = effectiveStatus
	state.LastCheckAt = &now
	state.LastLatencyMS = latencyMS

	if err := s.db.SaveState(state); err != nil {
		log.Printf("[scheduler] error saving state for %s: %v", ep.ID, err)
	}

	if err := s.db.RecordProbe(ep.ID, effectiveStatus, latencyMS, errMsg, now); err != nil {
		log.Printf("[scheduler] error recording probe history for %s: %v", ep.ID, err)
	}

	// Emit heartbeat event for SSE stream
	s.emitEvent(TransitionEvent{
		EndpointID:    ep.ID,
		DisplayName:   ep.DisplayName,
		Zone:          ep.Zone,
		Type:          "HEARTBEAT",
		CurrentStatus: effectiveStatus,
		LatencyMS:     latencyMS,
		Timestamp:     now,
		LastError:     errMsg,
	})
}

// Stop terminates all probe goroutines
func (s *Scheduler) Stop() {
	s.cancel()
}
