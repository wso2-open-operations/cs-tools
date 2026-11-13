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

// Package openobserve transforms an inbound OpenObserve alert webhook into the canonical alert, a Go port of ServiceNow's OpenObserveIncidentUtils.
package openobserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
)

// source identifies alerts produced by this adapter.
const source = "OpenObserve"

// defaults are used when a field is absent from both the payload and the operator config; SHORT_DESCRIPTION and DESCRIPTION have no fallback.
var defaults = map[string]string{
	"URGENCY":     "3",
	"IMPACT":      "3",
	"CALLER_ID":   "openobserve",
	"CATEGORY":    "service_interruption",
	"SERVICE":     "managed_services",
	"ENVIRONMENT": "production",
}

// prioritySeverityMap maps ServiceNow's native urgency/impact scale (1=High...3=Low) to the canonical severity labels; unmapped resolves to "".
var prioritySeverityMap = map[string]string{
	"1": "Critical",
	"2": "Major",
	"3": "Minor",
}

// ErrInvalidStructure is returned when the payload is missing both short_description and correlation_id.
var ErrInvalidStructure = errors.New("invalid openobserve alert payload structure")

// Alert is the canonical alert model, extended with the ServiceNow Incident-native fields this source's reference script also populates.
type Alert struct {
	Service          string `json:"service"`
	MetricName       string `json:"metric_name"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Environment      string `json:"environment"`
	Source           string `json:"source"`
	UniqueIdentifier string `json:"unique_identifier"`
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	Urgency          string `json:"urgency"`
	Impact           string `json:"impact"`
	CorrelationID    string `json:"correlation_id"`
	CallerID         string `json:"caller_id"`
}

// Config holds operator overrides mirroring "edge.api.openobserve.alert.config"; any field left empty falls back to the default.
type Config map[string]string

// LoadConfig reads operator overrides from the OPENOBSERVE_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("OPENOBSERVE_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid OPENOBSERVE_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound OpenObserve webhook and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// A genuine OpenObserve alert carries either its resolved short_description or a correlation_id.
	if fieldvalue.Str(payload, "short_description") == "" && fieldvalue.Str(payload, "correlation_id") == "" {
		return Alert{}, ErrInvalidStructure
	}

	shortDescription := fieldvalue.Str(payload, "short_description")
	description := fieldvalue.Str(payload, "description")
	correlationID := fieldvalue.Str(payload, "correlation_id")

	rawUrgency := configValue(cfg, "URGENCY", fieldvalue.Str(payload, "urgency"))
	rawImpact := configValue(cfg, "IMPACT", fieldvalue.Str(payload, "impact"))
	callerID := configValue(cfg, "CALLER_ID", fieldvalue.Str(payload, "caller_id"))

	// metric_name has no config/default fallback; it's built directly from the payload, or left empty.
	metricName := buildMetricName(shortDescription, correlationID)

	// Severity maps off urgency first, falling back to impact; an unmapped pair resolves to an empty severity.
	severity := mapSeverity(rawUrgency)
	if severity == "" {
		severity = mapSeverity(rawImpact)
	}

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", fieldvalue.Str(payload, "service")),
		MetricName:       metricName,
		Severity:         severity,
		Category:         configValue(cfg, "CATEGORY", fieldvalue.Str(payload, "category")),
		Environment:      configValue(cfg, "ENVIRONMENT", fieldvalue.Str(payload, "environment")),
		Source:           source,
		UniqueIdentifier: correlationID,
		ShortDescription: fieldvalue.FirstNonEmpty(shortDescription, metricName),
		// The alert's own text, never the payload; the raw body is kept in raw_alerts.
		Description:   description,
		Urgency:       rawUrgency,
		Impact:        rawImpact,
		CorrelationID: correlationID,
		CallerID:      callerID,
	}
	return alert, nil
}

// buildMetricName mirrors the reference script: short_description wins outright, otherwise correlation_id, otherwise empty.
func buildMetricName(shortDescription, correlationID string) string {
	if shortDescription != "" {
		return shortDescription
	}
	if correlationID != "" {
		return "OpenObserve Alert: " + correlationID
	}
	return ""
}

// mapSeverity maps a ServiceNow urgency/impact value (1-3) to a canonical severity label; an unmapped value resolves to "".
func mapSeverity(rawValue string) string {
	if rawValue == "" {
		return ""
	}
	return prioritySeverityMap[rawValue]
}

// configValue applies the 3-tier resolution: payload value, then operator config, then the default.
func configValue(cfg Config, field, payloadValue string) string {
	return fieldvalue.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
