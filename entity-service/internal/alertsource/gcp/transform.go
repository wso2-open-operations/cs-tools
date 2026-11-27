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

// Package gcp transforms an inbound Google Cloud Monitoring alerting notification into the canonical alert, a Go port of ServiceNow's GCPAlertProcessor.
package gcp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "GCP"

// Built-in defaults for a field the payload does not carry.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

// severityMap maps GCP Monitoring's condition severity to the canonical severity labels ("info" maps to "Informational", not "OK").
var severityMap = map[string]string{
	"critical": "Critical",
	"error":    "Major",
	"warning":  "Minor",
	"info":     "Informational",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound GCP Monitoring notification and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}

	// GCP's notification channel webhook wraps the actual alert under "incident"; nested objects degrade to an empty map rather than erroring.
	incident, _ := payload["incident"].(map[string]any)
	documentation, _ := incident["documentation"].(map[string]any)
	resource, _ := incident["resource"].(map[string]any)
	labels, _ := resource["labels"].(map[string]any)
	userLabels, _ := incident["policy_user_labels"].(map[string]any)

	state := fieldvalue.FirstNonEmpty(fieldvalue.Str(incident, "state"), "open")
	rawSeverity := fieldvalue.Str(incident, "severity")

	var d model.Defaults
	service := d.Or("service", fieldvalue.FirstNonEmpty(fieldvalue.Str(labels, "service"), fieldvalue.Str(userLabels, "service")), defaultService)
	metricName := d.Or("metric_name", fieldvalue.FirstNonEmpty(fieldvalue.Str(incident, "policy_name"), fieldvalue.Str(incident, "condition_name")), defaultMetricName)

	var severity string
	if state == "closed" {
		severity = "OK"
	} else {
		severity = d.Or("severity", fieldvalue.FirstNonEmpty(severityMap[strings.ToLower(rawSeverity)], rawSeverity), defaultSeverity)
	}

	alert := Alert{
		Service:          service,
		MetricName:       metricName,
		Severity:         severity,
		Category:         d.Or("category", fieldvalue.FirstNonEmpty(fieldvalue.Str(labels, "category"), fieldvalue.Str(userLabels, "category")), defaultCategory),
		Environment:      d.Or("environment", fieldvalue.FirstNonEmpty(fieldvalue.Str(labels, "environment"), fieldvalue.Str(userLabels, "environment")), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(incident, "incident_id"),
		// The incident summary or the policy's documentation, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(incident, "summary"), fieldvalue.Str(documentation, "content")),
	}
	alert.Defaulted = d.List()
	return alert, nil
}
