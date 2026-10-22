package config_test

import (
	"testing"
	"time"
	"uptime-mon/internal/config"
)

func TestInterpolateVariables_BasicsAndEmpty(t *testing.T) {
	ctx := config.VariableContext{
		Now:            time.Date(2026, 10, 22, 17, 30, 0, 0, time.UTC),
		Timezone:       "UTC",
		Version:        "1.0.0",
		Hostname:       "srv-mon",
		TotalEndpoints: 8,
		TotalZones:     2,
	}

	if got := config.InterpolateVariables("", ctx); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}

	plain := "Just plain text without placeholders"
	if got := config.InterpolateVariables(plain, ctx); got != plain {
		t.Errorf("expected %q, got %q", plain, got)
	}
}

func TestInterpolateVariables_DateTimeAndTimezone(t *testing.T) {
	// 2026-12-31 23:30 UTC -> 2027-01-01 00:30 in Europe/Berlin (UTC+1)
	utcTime := time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC)

	berlinCtx := config.BuildVariableContext(utcTime, "Europe/Berlin", "1.0.0", 5, 2)
	template := "Year: {current_year}, Month: {current_month}, Date: {current_date}, Time: {current_time}, TZ: {timezone}"
	resultBerlin := config.InterpolateVariables(template, berlinCtx)

	expectedBerlin := "Year: 2027, Month: 01, Date: 2027-01-01, Time: 00:30, TZ: Europe/Berlin"
	if resultBerlin != expectedBerlin {
		t.Errorf("expected %q, got %q", expectedBerlin, resultBerlin)
	}

	utcCtx := config.BuildVariableContext(utcTime, "UTC", "1.0.0", 5, 2)
	resultUTC := config.InterpolateVariables(template, utcCtx)
	expectedUTC := "Year: 2026, Month: 12, Date: 2026-12-31, Time: 23:30, TZ: UTC"
	if resultUTC != expectedUTC {
		t.Errorf("expected %q, got %q", expectedUTC, resultUTC)
	}
}

func TestInterpolateVariables_MetadataAndCounts(t *testing.T) {
	ctx := config.VariableContext{
		Now:            time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		Timezone:       "UTC",
		Version:        "2.1.0",
		Hostname:       "core-node-01",
		TotalEndpoints: 42,
		TotalZones:     7,
	}

	tmpl := "v{version} on {hostname} tracking {total_endpoints} hosts ({total_hosts}) in {total_zones} zones"
	got := config.InterpolateVariables(tmpl, ctx)
	expected := "v2.1.0 on core-node-01 tracking 42 hosts (42) in 7 zones"

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestInterpolateVariables_UnknownAndMalformedTokens(t *testing.T) {
	ctx := config.VariableContext{
		Now:            time.Date(2026, 10, 22, 17, 30, 0, 0, time.UTC),
		Timezone:       "UTC",
		Version:        "1.0.0",
		Hostname:       "srv",
		TotalEndpoints: 1,
		TotalZones:     1,
	}

	tmpl := "Copyright © {current_year} {unknown_variable} {another_key} {} {123} {{nested}}"
	got := config.InterpolateVariables(tmpl, ctx)
	expected := "Copyright © 2026 {unknown_variable} {another_key} {} {123} {{nested}}"

	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
