// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package datadog transforms an inbound Datadog monitor webhook into the canonical alert, a Go port of ServiceNow's DatadogIncidentUtils.
package datadog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// source identifies alerts produced by this adapter.
const source = "Datadog"

// Alert-state values, derived from $ALERT_TRANSITION or an explicit "state" payload field.
const (
	stateActive    = "ACTIVE"
	stateCompleted = "COMPLETED"
	stateError     = "ERROR"
)

// defaults are used when a field is absent from the payload, every tag alias, and the operator config, keyed by canonical field name.
var defaults = map[string]string{
	"METRIC_NAME": "DatadogAlert",
	"SERVICE":     "managed_services",
	"CATEGORY":    "service_interruption",
	"ENVIRONMENT": "production",
	"SEVERITY":    "1",
}

// numericSeverityMap maps Datadog's numeric monitor tag severity to the canonical severity labels.
var numericSeverityMap = map[string]string{
	"1": "Critical",
	"2": "Major",
	"3": "Minor",
	"4": "Warning",
	"5": "OK",
}

// transitionStateMap maps Datadog's $ALERT_TRANSITION webhook variable to an internal lifecycle state.
var transitionStateMap = map[string]string{
	"Triggered":    stateActive,
	"Re-Triggered": stateActive,
	"Renotify":     stateActive,
	"Escalated":    stateActive,
	"Warn":         stateActive,
	"Recovered":    stateCompleted,
	"No Data":      stateError,
}

// tagFieldMap maps a key:value tag key from Datadog's $TAGS to the canonical field it can supply, matched case-insensitively.
var tagFieldMap = map[string]string{
	"service":     "SERVICE",
	"svc":         "SERVICE",
	"env":         "ENVIRONMENT",
	"environment": "ENVIRONMENT",
	"category":    "CATEGORY",
	"severity":    "SEVERITY",
	"sev":         "SEVERITY",
	"metric":      "METRIC_NAME",
	"metric_name": "METRIC_NAME",
}

// ErrInvalidStructure is returned when the payload is missing both event/monitor id and event/monitor name.
var ErrInvalidStructure = errors.New("invalid datadog alert payload structure")

// Alert is the canonical alert model handed to the core component.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	Description      string `json:"description"`
}

// Config holds operator overrides mirroring "edge.api.datadog.alert.config"; any field left empty falls back to the default.
type Config map[string]string

// LoadConfig reads operator overrides from the DATADOG_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := utils.AlertConfigRaw("DATADOG_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid DATADOG_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Datadog webhook and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// monitor_id/monitor_name are the legacy webhook fields; event_id/event_name are the current ones.
	eventID := utils.FirstNonEmpty(utils.Str(payload, "monitor_id"), utils.Str(payload, "event_id"))
	eventName := utils.FirstNonEmpty(utils.Str(payload, "monitor_name"), utils.Str(payload, "event_name"))
	if eventID == "" && eventName == "" {
		return Alert{}, ErrInvalidStructure
	}

	triggerName := utils.Str(payload, "trigger_name")
	alertID := utils.Str(payload, "alert_id")
	tags := parseTags(utils.Str(payload, "tags"))
	state := resolveState(payload)

	metricNameValue := utils.FirstNonEmpty(utils.Str(payload, "metric_name"), buildMetricName(eventName, triggerName))
	uniqueIdentifier := utils.FirstNonEmpty(utils.Str(payload, "unique_identifier"), alertID)

	alert := Alert{
		Service:          configValue(cfg, tags, "SERVICE", utils.Str(payload, "service")),
		MetricName:       configValue(cfg, tags, "METRIC_NAME", metricNameValue),
		Severity:         resolveSeverity(state, configValue(cfg, tags, "SEVERITY", utils.Str(payload, "severity"))),
		Category:         configValue(cfg, tags, "CATEGORY", utils.Str(payload, "category")),
		Environment:      configValue(cfg, tags, "ENVIRONMENT", utils.Str(payload, "environment")),
		Source:           source,
		UniqueIdentifier: uniqueIdentifier,
		Description:      utils.CompactJSON(raw),
	}
	return alert, nil
}

// resolveState derives the alert's internal lifecycle state: an explicit "state" field wins, else $ALERT_TRANSITION, else ACTIVE.
func resolveState(payload map[string]any) string {
	if s := utils.Str(payload, "state"); s != "" {
		return s
	}
	transition := utils.FirstNonEmpty(utils.Str(payload, "transition"), utils.Str(payload, "alert_transition"))
	if state, ok := transitionStateMap[transition]; ok {
		return state
	}
	return stateActive
}

// resolveSeverity maps the lifecycle state and resolved severity to a canonical label: COMPLETED is always OK, ERROR is always Critical.
func resolveSeverity(state, rawSeverity string) string {
	switch state {
	case stateCompleted:
		return "OK"
	case stateError:
		return "Critical"
	}
	if rawSeverity == "" {
		rawSeverity = defaults["SEVERITY"]
	}
	if mapped, ok := numericSeverityMap[rawSeverity]; ok {
		return mapped
	}
	return rawSeverity
}

// parseTags splits Datadog's comma-separated "key:value" $TAGS string into a map keyed by lowercased tag key.
func parseTags(tags string) map[string]string {
	result := make(map[string]string)
	for _, tag := range strings.Split(tags, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(tag), ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

// tagValue returns the tag value for a canonical field, scanning every tag-key alias that maps to it.
func tagValue(tags map[string]string, field string) string {
	for tagKey, mappedField := range tagFieldMap {
		if mappedField == field {
			if v, ok := tags[tagKey]; ok && v != "" {
				return v
			}
		}
	}
	return ""
}

// buildMetricName joins the event and trigger names into a human-readable metric name, or "" when neither is present.
func buildMetricName(eventName, triggerName string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{eventName, triggerName} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Datadog Alert: " + strings.Join(parts, " / ")
}

// configValue applies the 4-tier resolution: payload value, then tag alias, then operator config, then the default.
func configValue(cfg Config, tags map[string]string, field, payloadValue string) string {
	return utils.FirstNonEmpty(payloadValue, tagValue(tags, field), cfg[field], defaults[field])
}
