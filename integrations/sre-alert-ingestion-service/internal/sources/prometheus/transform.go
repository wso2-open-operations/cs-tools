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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// source identifies alerts produced by this adapter when the sender isn't detected as Grafana.
const source = "Prometheus"

// grafanaSource is used instead of source when the alert is detected as coming through Grafana.
const grafanaSource = "Grafana"

// grafanaReceiver is the Alertmanager receiver name Grafana's own default contact point uses.
const grafanaReceiver = "grafana-default-email"

// grafanaAlertPrefix prefixes a Grafana alert's metric name.
const grafanaAlertPrefix = "Grafana Alert: "

// defaults are used when a field is absent from both the payload and the operator config, keyed by canonical field name.
var defaults = map[string]string{
	"METRIC_NAME": "PrometheusAlert",
	"SERVICE":     "Managed Services",
	"CATEGORY":    "Software",
	"ENVIRONMENT": "Production",
	"SEVERITY":    "Critical",
}

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

// Config holds operator overrides mirroring "edge.api.prometheus.alert.config"; unlike other fields, "severity" and "source" are looked up lowercase.
type Config map[string]string

// LoadConfig reads operator overrides from the PROMETHEUS_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := utils.AlertConfigRaw("PROMETHEUS_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid PROMETHEUS_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Alertmanager-shaped webhook and produces one canonical Alert per entry in its "alerts" array.
func Transform(raw []byte, cfg Config) ([]Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	rawAlerts, ok := payload["alerts"].([]any)
	if !ok || len(rawAlerts) == 0 {
		return nil, ErrInvalidStructure
	}

	description := utils.CompactJSON(raw)
	receiver := utils.Str(payload, "receiver")
	batchStatus := utils.Str(payload, "status") // fallback for alerts without their own status

	alerts := make([]Alert, 0, len(rawAlerts))
	for _, rawAlert := range rawAlerts {
		alertMap, ok := rawAlert.(map[string]any)
		if !ok {
			continue
		}
		alerts = append(alerts, transformOne(alertMap, cfg, receiver, batchStatus, description))
	}
	return alerts, nil
}

func transformOne(alert map[string]any, cfg Config, receiver, batchStatus, description string) Alert {
	labels, _ := alert["labels"].(map[string]any)
	annotations, _ := alert["annotations"].(map[string]any)

	isGrafana := utils.Str(labels, "grafana_folder") != "" || receiver == grafanaReceiver
	alertSource := utils.FirstNonEmpty(cfg["source"], source)
	if isGrafana {
		alertSource = grafanaSource
	}

	// Alertmanager sends each alert's status as a string; the object form ({"state": ...}) is the legacy shape.
	alertStatus := utils.Str(alert, "status")
	if statusObj, ok := alert["status"].(map[string]any); ok {
		alertStatus = utils.Str(statusObj, "state")
	}
	alertStatus = utils.FirstNonEmpty(alertStatus, batchStatus)

	rawSeverity := utils.FirstNonEmpty(utils.Str(labels, "severity"), cfg["severity"], defaults["SEVERITY"])

	var severity string
	if alertStatus == "resolved" {
		severity = "OK"
	} else {
		severity = severityMap[strings.ToLower(rawSeverity)]
		if severity == "" {
			severity = rawSeverity
		}
	}

	return Alert{
		Service:          configValue(cfg, "SERVICE", utils.FirstNonEmpty(utils.Str(labels, "service"), utils.Str(labels, "component"), utils.Str(labels, "job"))),
		MetricName:       configValue(cfg, "METRIC_NAME", buildMetricName(isGrafana, labels, annotations)),
		Severity:         severity,
		Category:         configValue(cfg, "CATEGORY", utils.Str(labels, "category")),
		Environment:      configValue(cfg, "ENVIRONMENT", utils.FirstNonEmpty(utils.Str(labels, "environment"), utils.Str(labels, "cluster"))),
		Source:           alertSource,
		UniqueIdentifier: utils.Str(alert, "fingerprint"),
		Description:      description,
	}
}

// buildMetricName mirrors the reference script: a Grafana alert with an annotations.title uses that, prefixed; otherwise labels.alertname.
func buildMetricName(isGrafana bool, labels, annotations map[string]any) string {
	if utils.Str(labels, "grafana_folder") != "" {
		if title := utils.Str(annotations, "title"); title != "" {
			return grafanaAlertPrefix + title
		}
	}
	return utils.Str(labels, "alertname")
}

// configValue applies the 3-tier resolution: payload value, then operator config, then the default.
func configValue(cfg Config, field, payloadValue string) string {
	return utils.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
