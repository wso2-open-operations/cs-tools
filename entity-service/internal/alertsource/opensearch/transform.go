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

// Package opensearch transforms an inbound OpenSearch alerting monitor payload into the canonical alert, a Go port of ServiceNow's OpenSearchIncidentUtils.
package opensearch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "OpenSearch"

// Alert-state values OpenSearch sends in the "state" field.
const (
	stateCompleted = "COMPLETED"
	stateError     = "ERROR"
)

// Built-in defaults for a field the payload does not carry.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

// numericSeverityMap maps OpenSearch's native numeric severity to the canonical severity labels.
var numericSeverityMap = map[string]string{
	"1": "Critical",
	"2": "Major",
	"3": "Minor",
	"4": "Warning",
	"5": "OK",
}

// ErrInvalidStructure is returned when the payload is missing both monitor_id and monitor_name.
var ErrInvalidStructure = errors.New("invalid opensearch alert payload structure")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound OpenSearch alert and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// A genuine OpenSearch alert carries a monitor identity.
	if fieldvalue.Str(payload, "monitor_id") == "" && fieldvalue.Str(payload, "monitor_name") == "" {
		return Alert{}, ErrInvalidStructure
	}

	monitorName := fieldvalue.Str(payload, "monitor_name")
	triggerName := fieldvalue.Str(payload, "trigger_name")
	state := fieldvalue.Str(payload, "state")

	var d model.Defaults
	alert := Alert{
		Service:          d.Or("service", fieldvalue.Str(payload, "service"), defaultService),
		MetricName:       d.Or("metric_name", buildMetricName(monitorName, triggerName), defaultMetricName),
		Severity:         resolveSeverity(&d, state, fieldvalue.Str(payload, "severity")),
		Category:         d.Or("category", fieldvalue.Str(payload, "category"), defaultCategory),
		Environment:      d.Or("environment", "", defaultEnvironment), // never taken from payload
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(payload, "alert_id"),
		// The trigger's message text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "message"), fieldvalue.Str(payload, "description")),
	}
	alert.Defaulted = d.List()
	return alert, nil
}

// resolveSeverity maps the alert state and raw severity to a canonical label: COMPLETED is always OK, ERROR is always Critical, no severity takes the default.
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

// buildMetricName joins the monitor and trigger names into a human-readable metric name, or "" when neither is present.
func buildMetricName(monitorName, triggerName string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{monitorName, triggerName} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "OpenSearch Alert: " + strings.Join(parts, " / ")
}
