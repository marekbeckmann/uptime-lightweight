package alerting

import (
	"time"
)

// GenericWebhookPayload represents a standard JSON payload sent to generic webhooks
type GenericWebhookPayload struct {
	Event       string                 `json:"event"` // host_down, host_up, tls_expiry, maintenance_created, maintenance_reminder
	EndpointID  string                 `json:"endpoint_id,omitempty"`
	DisplayName string                 `json:"display_name,omitempty"`
	Zone        string                 `json:"zone,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Timestamp   string                 `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// BuildGenericPayload formats a generic JSON webhook payload
func BuildGenericPayload(event, endpointID, displayName, zone, status string, details map[string]interface{}, now time.Time) GenericWebhookPayload {
	return GenericWebhookPayload{
		Event:       event,
		EndpointID:  endpointID,
		DisplayName: displayName,
		Zone:        zone,
		Status:      status,
		Timestamp:   now.Format(time.RFC3339),
		Details:     details,
	}
}
