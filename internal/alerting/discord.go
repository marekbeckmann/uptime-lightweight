package alerting

import (
	"fmt"
	"time"
)

// DiscordWebhookPayload represents the JSON sent to Discord webhook URLs
type DiscordWebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds"`
}

type DiscordEmbed struct {
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Color       int           `json:"color"` // decimal color
	Fields      []EmbedField  `json:"fields,omitempty"`
	Timestamp   string        `json:"timestamp,omitempty"`
	Footer      *EmbedFooter  `json:"footer,omitempty"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text"`
}

// Colors
const (
	ColorRed   = 0xf43f5e // Down
	ColorGreen = 0x10b981 // Up
	ColorAmber = 0xf59e0b // Warning / TLS / Reminder
	ColorBlue  = 0x3b82f6 // Maintenance
)

// BuildHostDownEmbed formats the Discord embed for Host Down events
func BuildHostDownEmbed(displayName, zone, reason string, lastUp *time.Time, now time.Time) DiscordEmbed {
	lastUpStr := "Never (or since startup)"
	if lastUp != nil {
		diff := now.Sub(*lastUp).Round(time.Second)
		lastUpStr = fmt.Sprintf("%s (%s ago)", lastUp.Format(time.RFC3339), FormatDuration(diff))
	}

	return DiscordEmbed{
		Title:       fmt.Sprintf("🚨 Host Down: %s", displayName),
		Description: fmt.Sprintf("Service in zone **%s** has transitioned to **DOWN**.", zone),
		Color:       ColorRed,
		Timestamp:   now.Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "Affected Host", Value: fmt.Sprintf("**%s** (`%s`)", displayName, zone), Inline: true},
			{Name: "Last Successful Connection", Value: lastUpStr, Inline: true},
			{Name: "Failure Reason", Value: fmt.Sprintf("```\n%s\n```", reason), Inline: false},
		},
		Footer: &EmbedFooter{Text: "uptime-mon alerting"},
	}
}

// BuildHostUpEmbed formats the Discord embed for Host Up events
func BuildHostUpEmbed(displayName, zone string, downtime time.Duration, now time.Time) DiscordEmbed {
	downtimeStr := FormatDuration(downtime)
	if downtime <= 0 {
		downtimeStr = "< 1m"
	}

	return DiscordEmbed{
		Title:       fmt.Sprintf("✅ Host Recovered: %s", displayName),
		Description: fmt.Sprintf("Service in zone **%s** has recovered to **UP**.", zone),
		Color:       ColorGreen,
		Timestamp:   now.Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "Affected Host", Value: fmt.Sprintf("**%s** (`%s`)", displayName, zone), Inline: true},
			{Name: "Time Passed Since Down", Value: downtimeStr, Inline: true},
		},
		Footer: &EmbedFooter{Text: "uptime-mon alerting"},
	}
}

// BuildTLSEmbed formats the Discord embed for TLS certificate expiry warnings
func BuildTLSEmbed(displayName, zone string, expiry time.Time, daysLeft int, now time.Time) DiscordEmbed {
	return DiscordEmbed{
		Title:       fmt.Sprintf("⚠️ TLS Certificate Expiry Warning: %s", displayName),
		Description: fmt.Sprintf("Certificate for **%s** in zone **%s** is approaching expiration.", displayName, zone),
		Color:       ColorAmber,
		Timestamp:   now.Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "Affected Host", Value: displayName, Inline: true},
			{Name: "Days Remaining", Value: fmt.Sprintf("%d days", daysLeft), Inline: true},
			{Name: "Certificate Expiration Date", Value: expiry.Format("2006-01-02 15:04:05 UTC"), Inline: false},
		},
		Footer: &EmbedFooter{Text: "uptime-mon alerting"},
	}
}

// BuildMaintenanceEmbed formats the Discord embed for maintenance created/reminder
func BuildMaintenanceEmbed(isReminder bool, message, when string, affectedZones, affectedHosts []string, now time.Time) DiscordEmbed {
	title := "🛠️ Scheduled Maintenance Registered"
	color := ColorBlue
	if isReminder {
		title = "⏰ Upcoming Maintenance Reminder"
		color = ColorAmber
	}

	zonesStr := "All"
	if len(affectedZones) > 0 {
		zonesStr = fmt.Sprintf("`%v`", affectedZones)
	}
	hostsStr := "All"
	if len(affectedHosts) > 0 {
		hostsStr = fmt.Sprintf("`%v`", affectedHosts)
	}

	return DiscordEmbed{
		Title:       title,
		Description: message,
		Color:       color,
		Timestamp:   now.Format(time.RFC3339),
		Fields: []EmbedField{
			{Name: "When", Value: when, Inline: true},
			{Name: "Affected Zones", Value: zonesStr, Inline: true},
			{Name: "Affected Hosts", Value: hostsStr, Inline: true},
		},
		Footer: &EmbedFooter{Text: "uptime-mon alerting"},
	}
}

// FormatDuration converts time.Duration into human readable format like "1h 14m 22s"
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
