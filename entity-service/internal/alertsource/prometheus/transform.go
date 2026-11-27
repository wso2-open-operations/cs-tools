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

// Package prometheus transforms an inbound Prometheus Alertmanager (or Grafana-compatible) webhook into a slice of canonical alerts, a Go port of ServiceNow's PrometheusAlertProcessor.
package prometheus

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter when the sender isn't detected as Grafana.
const source = "Prometheus"

// grafanaSource is used instead of source when the alert is detected as coming through Grafana.
const grafanaSource = "Grafana"

// grafanaReceiver is the Alertmanager receiver name Grafana's own default contact point uses.
const grafanaReceiver = "grafana-default-email"

// grafanaAlertPrefix prefixes a Grafana alert's metric name.
const grafanaAlertPrefix = "Grafana Alert: "

// Built-in defaults for a field the payload does not carry.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

// severityMap maps Prometheus/Grafana's lowercase severity label to the canonical severity labels; an unmapped value passes through as-is.
var severityMap = map[string]string{
	"critical":      "Critical",
	"high":          "Major",
	"low":           "Minor",
	"warning":       "Warning",
	"informational": "OK",
}

// ErrInvalidStructure is returned when the payload has no non-empty "alerts" array.
var ErrInvalidStructure = errors.New("invalid prometheus alert payload structure")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound Alertmanager-shaped webhook and produces one canonical Alert per entry in its "alerts" array, each with its own defaults.
func Transform(raw []byte) ([]Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	rawAlerts, ok := payload["alerts"].([]any)
	if !ok || len(rawAlerts) == 0 {
		return nil, ErrInvalidStructure
	}

	receiver := fieldvalue.Str(payload, "receiver")
	batchStatus := fieldvalue.Str(payload, "status") // fallback for alerts without their own status

	alerts := make([]Alert, 0, len(rawAlerts))
	for _, rawAlert := range rawAlerts {
		alertMap, ok := rawAlert.(map[string]any)
		if !ok {
			continue
		}
		alerts = append(alerts, transformOne(alertMap, receiver, batchStatus))
	}
	return alerts, nil
}

func transformOne(alert map[string]any, receiver, batchStatus string) Alert {
	labels, _ := alert["labels"].(map[string]any)
	annotations, _ := alert["annotations"].(map[string]any)

	isGrafana := fieldvalue.Str(labels, "grafana_folder") != "" || receiver == grafanaReceiver
	alertSource := source
	if isGrafana {
		alertSource = grafanaSource
	}

	// Alertmanager sends each alert's status as a string; the object form ({"state": ...}) is the legacy shape.
	alertStatus := fieldvalue.Str(alert, "status")
	if statusObj, ok := alert["status"].(map[string]any); ok {
		alertStatus = fieldvalue.Str(statusObj, "state")
	}
	alertStatus = fieldvalue.FirstNonEmpty(alertStatus, batchStatus)

	rawSeverity := fieldvalue.Str(labels, "severity")

	var d model.Defaults
	service := d.Or("service", fieldvalue.FirstNonEmpty(fieldvalue.Str(labels, "service"), fieldvalue.Str(labels, "component"), fieldvalue.Str(labels, "job")), defaultService)
	metricName := d.Or("metric_name", buildMetricName(isGrafana, labels, annotations), defaultMetricName)

	var severity string
	if alertStatus == "resolved" {
		severity = "OK"
	} else {
		// An unmapped label passes through as-is; no label at all takes the default.
		severity = d.Or("severity", fieldvalue.FirstNonEmpty(severityMap[strings.ToLower(rawSeverity)], rawSeverity), defaultSeverity)
	}

	a := Alert{
		Service:          service,
		MetricName:       metricName,
		Severity:         severity,
		Category:         d.Or("category", fieldvalue.Str(labels, "category"), defaultCategory),
		Environment:      d.Or("environment", fieldvalue.FirstNonEmpty(fieldvalue.Str(labels, "environment"), fieldvalue.Str(labels, "cluster")), defaultEnvironment),
		Source:           alertSource,
		UniqueIdentifier: fieldvalue.Str(alert, "fingerprint"),
		// The alert's own annotation text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(annotations, "description"), fieldvalue.Str(annotations, "summary")),
	}
	a.Defaulted = d.List()
	return a
}

// buildMetricName mirrors the reference script: a Grafana alert with an annotations.title uses that, prefixed; otherwise labels.alertname.
// It checks grafana_folder rather than isGrafana, exactly as alert-ingestion does, because the metric name is part of the fingerprint.
func buildMetricName(_ bool, labels, annotations map[string]any) string {
	if fieldvalue.Str(labels, "grafana_folder") != "" {
		if title := fieldvalue.Str(annotations, "title"); title != "" {
			return grafanaAlertPrefix + title
		}
	}
	return fieldvalue.Str(labels, "alertname")
}
