package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/prober"
	"uptime-mon/internal/storage"
)

// Dispatcher manages webhook delivery with deduplication and retries
type Dispatcher struct {
	db         *storage.DB
	webhooks   []config.WebhookTarget
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewDispatcher(db *storage.DB, webhooks []config.WebhookTarget) *Dispatcher {
	return &Dispatcher{
		db:       db,
		webhooks: webhooks,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (d *Dispatcher) UpdateWebhooks(webhooks []config.WebhookTarget) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.webhooks = webhooks
}

// HandleTransitionEvent processes events emitted by the probe scheduler
func (d *Dispatcher) HandleTransitionEvent(evt prober.TransitionEvent) {
	switch evt.Type {
	case "DOWN":
		d.DispatchHostDown(evt.EndpointID, evt.DisplayName, evt.Zone, evt.LastError, evt.LastUpAt, evt.Timestamp)
	case "UP":
		d.DispatchHostUp(evt.EndpointID, evt.DisplayName, evt.Zone, evt.DowntimeDuration, evt.Timestamp)
	case "TLS_EXPIRY":
		if evt.TLSExpiryAt != nil {
			d.DispatchTLSExpiry(evt.EndpointID, evt.DisplayName, evt.Zone, *evt.TLSExpiryAt, evt.WarnDays, evt.Timestamp)
		}
	}
}

// DispatchHostDown sends alerts for host down transitions
func (d *Dispatcher) DispatchHostDown(endpointID, displayName, zone, reason string, lastUp *time.Time, now time.Time) {
	discordEmbed := BuildHostDownEmbed(displayName, zone, reason, lastUp, now)
	genericPayload := BuildGenericPayload("host_down", endpointID, displayName, zone, "DOWN", map[string]interface{}{
		"failure_reason": reason,
		"last_up_at":     lastUp,
	}, now)

	d.broadcast("host_down", discordEmbed, genericPayload)
}

// DispatchHostUp sends alerts for host recovery
func (d *Dispatcher) DispatchHostUp(endpointID, displayName, zone string, downtime time.Duration, now time.Time) {
	discordEmbed := BuildHostUpEmbed(displayName, zone, downtime, now)
	genericPayload := BuildGenericPayload("host_up", endpointID, displayName, zone, "UP", map[string]interface{}{
		"downtime_seconds": int(downtime.Seconds()),
		"downtime_human":   FormatDuration(downtime),
	}, now)

	d.broadcast("host_up", discordEmbed, genericPayload)
}

// DispatchTLSExpiry sends warnings for approaching certificate expiration
func (d *Dispatcher) DispatchTLSExpiry(endpointID, displayName, zone string, expiry time.Time, daysLeft int, now time.Time) {
	discordEmbed := BuildTLSEmbed(displayName, zone, expiry, daysLeft, now)
	genericPayload := BuildGenericPayload("tls_expiry", endpointID, displayName, zone, "UP", map[string]interface{}{
		"expiry_date":    expiry.Format(time.RFC3339),
		"days_remaining": daysLeft,
	}, now)

	d.broadcast("tls_expiry", discordEmbed, genericPayload)
}

// DispatchMaintenanceCreated sends an announcement when maintenance is registered
func (d *Dispatcher) DispatchMaintenanceCreated(key, message, when string, affectedZones, affectedHosts []string, now time.Time) {
	// Deduplicate using SQLite
	if d.isMaintenanceCreatedSent(key) {
		return
	}

	discordEmbed := BuildMaintenanceEmbed(false, message, when, affectedZones, affectedHosts, now)
	genericPayload := BuildGenericPayload("maintenance_created", "", key, "", "MAINTENANCE", map[string]interface{}{
		"message":        message,
		"when":           when,
		"affected_zones": affectedZones,
		"affected_hosts": affectedHosts,
	}, now)

	d.broadcast("maintenance_created", discordEmbed, genericPayload)
	d.recordMaintenanceCreated(key, now)
}

// DispatchMaintenanceReminder sends a reminder prior to maintenance start
func (d *Dispatcher) DispatchMaintenanceReminder(key, message, when string, affectedZones, affectedHosts []string, windowStart, now time.Time) {
	// Deduplicate using SQLite
	if d.isMaintenanceReminderSent(key, windowStart) {
		return
	}

	discordEmbed := BuildMaintenanceEmbed(true, message, when, affectedZones, affectedHosts, now)
	genericPayload := BuildGenericPayload("maintenance_reminder", "", key, "", "MAINTENANCE", map[string]interface{}{
		"message":        message,
		"when":           when,
		"affected_zones": affectedZones,
		"affected_hosts": affectedHosts,
		"window_start":   windowStart.Format(time.RFC3339),
	}, now)

	d.broadcast("maintenance_reminder", discordEmbed, genericPayload)
	d.recordMaintenanceReminder(key, windowStart)
}

func (d *Dispatcher) broadcast(eventType string, discordEmbed DiscordEmbed, genericPayload GenericWebhookPayload) {
	d.mu.RLock()
	targets := make([]config.WebhookTarget, len(d.webhooks))
	copy(targets, d.webhooks)
	d.mu.RUnlock()

	for _, target := range targets {
		if !containsEvent(target.Events, eventType) {
			continue
		}

		go func(t config.WebhookTarget) {
			var body []byte
			var err error
			if strings.ToLower(t.Type) == "discord" {
				payload := DiscordWebhookPayload{
					Username: "uptime-mon",
					Embeds:   []DiscordEmbed{discordEmbed},
				}
				body, err = json.Marshal(payload)
			} else {
				body, err = json.Marshal(genericPayload)
			}

			if err != nil {
				log.Printf("[alerting] error marshaling payload for %s: %v", t.ID, err)
				return
			}

			d.sendWithRetry(t.URL, body)
		}(target)
	}
}

func (d *Dispatcher) sendWithRetry(targetURL string, body []byte) {
	maxRetries := 3
	backoff := 500 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(context.Background(), "POST", targetURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("[alerting] request create error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "uptime-mon-alerting/1.0")

		resp, err := d.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return // Success
			}
			log.Printf("[alerting] webhook returned HTTP %d on attempt %d", resp.StatusCode, attempt)
		} else {
			log.Printf("[alerting] webhook delivery error on attempt %d: %v", attempt, err)
		}

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
}

func containsEvent(events []string, target string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if strings.EqualFold(e, target) {
			return true
		}
	}
	return false
}

// Deduplication queries in SQLite
func (d *Dispatcher) isMaintenanceCreatedSent(key string) bool {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(1) FROM maintenance_notifications
		WHERE maintenance_key = ? AND created_sent_at IS NOT NULL`, key).Scan(&count)
	return err == nil && count > 0
}

func (d *Dispatcher) recordMaintenanceCreated(key string, sentAt time.Time) {
	d.db.Exec(`
		INSERT INTO maintenance_notifications (maintenance_key, config_hash, created_sent_at)
		VALUES (?, '', ?)
		ON CONFLICT(maintenance_key) DO UPDATE SET created_sent_at = excluded.created_sent_at`,
		key, sentAt)
}

func (d *Dispatcher) isMaintenanceReminderSent(key string, windowStart time.Time) bool {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(1) FROM maintenance_notifications
		WHERE maintenance_key = ? AND last_reminder_window_start = ?`, key, windowStart).Scan(&count)
	return err == nil && count > 0
}

func (d *Dispatcher) recordMaintenanceReminder(key string, windowStart time.Time) {
	d.db.Exec(`
		INSERT INTO maintenance_notifications (maintenance_key, config_hash, last_reminder_window_start)
		VALUES (?, '', ?)
		ON CONFLICT(maintenance_key) DO UPDATE SET last_reminder_window_start = excluded.last_reminder_window_start`,
		key, windowStart)
}
