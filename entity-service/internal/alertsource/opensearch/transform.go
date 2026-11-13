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
	"encoding/json"
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

// defaults are used when a field is absent from both the payload and the operator config, keyed by canonical field name.
var defaults = map[string]string{
	"METRIC_NAME": "OpenSearchAlert",
	"SERVICE":     "managed_services",
	"CATEGORY":    "service_interruption",
	"ENVIRONMENT": "production",
	"SEVERITY":    "1",
}

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

// Config holds operator overrides mirroring "edge.api.opensearch.alert.config"; any field left empty falls back to the default.
type Config map[string]string

// LoadConfig reads operator overrides from the OPENSEARCH_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("OPENSEARCH_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid OPENSEARCH_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound OpenSearch alert and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
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

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", fieldvalue.Str(payload, "service")),
		MetricName:       configValue(cfg, "METRIC_NAME", buildMetricName(monitorName, triggerName)),
		Severity:         resolveSeverity(state, fieldvalue.Str(payload, "severity"), cfg),
		Category:         configValue(cfg, "CATEGORY", fieldvalue.Str(payload, "category")),
		Environment:      configValue(cfg, "ENVIRONMENT", ""), // never taken from payload
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(payload, "alert_id"),
		// The trigger's message text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "message"), fieldvalue.Str(payload, "description")),
	}
	return alert, nil
}

// resolveSeverity maps the alert state and raw severity to a canonical label: COMPLETED is always OK, ERROR is always Critical.
func resolveSeverity(state, rawSeverity string, cfg Config) string {
	switch state {
	case stateCompleted:
		return "OK"
	case stateError:
		return "Critical"
	}
	if rawSeverity == "" {
		rawSeverity = fieldvalue.FirstNonEmpty(cfg["SEVERITY"], defaults["SEVERITY"])
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

// configValue applies the 3-tier resolution: payload value, then operator config, then the default.
func configValue(cfg Config, field, payloadValue string) string {
	return fieldvalue.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
