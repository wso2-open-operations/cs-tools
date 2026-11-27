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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "Datadog"

// Alert-state values, derived from $ALERT_TRANSITION or an explicit "state" payload field.
const (
	stateActive    = "ACTIVE"
	stateCompleted = "COMPLETED"
	stateError     = "ERROR"
)

// Built-in defaults for a field absent from the payload and every tag alias; an empty default leaves the field empty.
const (
	defaultMetricName  = "Datadog Alert"
	defaultService     = ""
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Production"
	defaultSeverity    = "Critical"
)

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

// ErrInvalidStructure is returned when the payload is missing both event/monitor id and event/monitor name.
var ErrInvalidStructure = errors.New("invalid datadog alert payload structure")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound Datadog webhook and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// monitor_id/monitor_name are the legacy webhook fields; event_id/event_name are the current ones.
	eventID := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "monitor_id"), fieldvalue.Str(payload, "event_id"))
	eventName := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "monitor_name"), fieldvalue.Str(payload, "event_name"))
	if eventID == "" && eventName == "" {
		return Alert{}, ErrInvalidStructure
	}

	triggerName := fieldvalue.Str(payload, "trigger_name")
	alertID := fieldvalue.Str(payload, "alert_id")
	tags := parseTags(fieldvalue.Str(payload, "tags"))
	state := resolveState(payload)

	metricNameValue := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "metric_name"), buildMetricName(eventName, triggerName))
	uniqueIdentifier := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "unique_identifier"), alertID)

	var d model.Defaults
	alert := Alert{
		Service:          d.Or("service", payloadValue(tags, "SERVICE", fieldvalue.Str(payload, "service")), defaultService),
		MetricName:       d.Or("metric_name", payloadValue(tags, "METRIC_NAME", metricNameValue), defaultMetricName),
		Severity:         resolveSeverity(&d, state, payloadValue(tags, "SEVERITY", fieldvalue.Str(payload, "severity"))),
		Category:         d.Or("category", payloadValue(tags, "CATEGORY", fieldvalue.Str(payload, "category")), defaultCategory),
		Environment:      d.Or("environment", payloadValue(tags, "ENVIRONMENT", fieldvalue.Str(payload, "environment")), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: uniqueIdentifier,
		// Datadog's message text ($EVENT_MSG), never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "body"), fieldvalue.Str(payload, "text_only_msg"), fieldvalue.Str(payload, "event_msg")),
	}
	alert.Defaulted = d.List()
	return alert, nil
}

// resolveState derives the alert's internal lifecycle state: an explicit "state" field wins, else $ALERT_TRANSITION, else ACTIVE.
func resolveState(payload map[string]any) string {
	if s := fieldvalue.Str(payload, "state"); s != "" {
		return s
	}
	transition := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "transition"), fieldvalue.Str(payload, "alert_transition"))
	if state, ok := transitionStateMap[transition]; ok {
		return state
	}
	return stateActive
}

// resolveSeverity maps the lifecycle state and payload severity to a canonical label: COMPLETED is always OK, ERROR is always Critical, no severity takes the default.
func resolveSeverity(d *model.Defaults, state, rawSeverity string) string {
	switch state {
	case stateCompleted:
		return "OK"
	case stateError:
		return "Critical"
	}
	if rawSeverity == "" {
		return d.Or("severity", "", defaultSeverity)
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

// tagAliases lists the $TAGS keys (lowercased by parseTags) that can supply each canonical field, in priority order. A fixed order keeps the result, and so the fingerprint, stable when a payload carries two aliases with different values.
var tagAliases = map[string][]string{
	"SERVICE":     {"service", "svc"},
	"ENVIRONMENT": {"env", "environment"},
	"CATEGORY":    {"category"},
	"SEVERITY":    {"severity", "sev"},
	"METRIC_NAME": {"metric", "metric_name"},
}

// tagValue returns the first non-empty tag value among a canonical field's aliases.
func tagValue(tags map[string]string, field string) string {
	for _, k := range tagAliases[field] {
		if v := tags[k]; v != "" {
			return v
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

// payloadValue returns the payload's own value, else the first tag alias, or "" when the payload carries neither.
func payloadValue(tags map[string]string, field, value string) string {
	return fieldvalue.FirstNonEmpty(value, tagValue(tags, field))
}
