package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// VariableContext holds runtime values used to resolve dynamic template placeholders
type VariableContext struct {
	Now            time.Time
	Timezone       string
	Version        string
	Hostname       string
	TotalEndpoints int
	TotalZones     int
}

// BuildVariableContext constructs a context populated with the current time in the given timezone,
// operating system hostname, default version if empty, and endpoint/zone counts.
func BuildVariableContext(now time.Time, tz string, version string, totalEndpoints int, totalZones int) VariableContext {
	if version == "" {
		version = "1.0.0"
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "localhost"
	}

	// Resolve timezone location
	loc := time.UTC
	if tz != "" {
		if loadedLoc, err := time.LoadLocation(tz); err == nil {
			loc = loadedLoc
		}
	} else {
		tz = "UTC"
	}

	if now.IsZero() {
		now = time.Now()
	}
	nowInLoc := now.In(loc)

	return VariableContext{
		Now:            nowInLoc,
		Timezone:       tz,
		Version:        version,
		Hostname:       hostname,
		TotalEndpoints: totalEndpoints,
		TotalZones:     totalZones,
	}
}

// InterpolateVariables replaces pre-defined template placeholders in a string.
// Supported tokens:
// - {current_year}   (e.g., "2026")
// - {current_month}  (e.g., "10")
// - {current_date}   (e.g., "2026-10-22")
// - {current_time}   (e.g., "17:30")
// - {timezone}       (e.g., "Europe/Berlin")
// - {version}        (e.g., "1.0.0")
// - {hostname}       (e.g., "srv-mon-01")
// - {total_endpoints} / {total_hosts} (e.g., "4")
// - {total_zones}    (e.g., "2")
//
// Unrecognized tokens are left intact verbatim.
func InterpolateVariables(text string, ctx VariableContext) string {
	if text == "" || !strings.Contains(text, "{") {
		return text
	}

	replacements := []string{
		"{current_year}", ctx.Now.Format("2006"),
		"{current_month}", ctx.Now.Format("01"),
		"{current_date}", ctx.Now.Format("2006-01-02"),
		"{current_time}", ctx.Now.Format("15:04"),
		"{timezone}", ctx.Timezone,
		"{version}", ctx.Version,
		"{hostname}", ctx.Hostname,
		"{total_endpoints}", strconv.Itoa(ctx.TotalEndpoints),
		"{total_hosts}", strconv.Itoa(ctx.TotalEndpoints),
		"{total_zones}", strconv.Itoa(ctx.TotalZones),
	}

	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(text)
}
