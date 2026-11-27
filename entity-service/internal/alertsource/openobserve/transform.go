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
	"errors"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "OpenObserve"

// Built-in defaults for a field the payload does not carry; severity takes its default only when neither urgency nor impact maps to one.
const (
	defaultMetricName  = "OpenObserve Alert"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Production"
	defaultSeverity    = "Critical"
)

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
	// Defaulted lists the fields filled with the built-in default, as on model.Alert.
	Defaulted []model.Defaulted `json:"defaulted,omitempty"`
}

// Transform parses a raw inbound OpenObserve webhook and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
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

	rawUrgency := fieldvalue.Str(payload, "urgency")
	rawImpact := fieldvalue.Str(payload, "impact")

	var d model.Defaults
	service := d.Or("service", fieldvalue.Str(payload, "service"), defaultService)
	metricName := d.Or("metric_name", buildMetricName(shortDescription, correlationID), defaultMetricName)

	// Severity maps off urgency first, falling back to impact; when neither maps it takes the default.
	severity := d.Or("severity", fieldvalue.FirstNonEmpty(mapSeverity(rawUrgency), mapSeverity(rawImpact)), defaultSeverity)

	alert := Alert{
		Service:          service,
		MetricName:       metricName,
		Severity:         severity,
		Category:         d.Or("category", fieldvalue.Str(payload, "category"), defaultCategory),
		Environment:      d.Or("environment", fieldvalue.Str(payload, "environment"), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: correlationID,
		ShortDescription: fieldvalue.FirstNonEmpty(shortDescription, metricName),
		// The alert's own text, never the payload; the raw body is kept in raw_alerts.
		Description:   description,
		Urgency:       rawUrgency,
		Impact:        rawImpact,
		CorrelationID: correlationID,
		CallerID:      fieldvalue.Str(payload, "caller_id"),
	}
	alert.Defaulted = d.List()
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
