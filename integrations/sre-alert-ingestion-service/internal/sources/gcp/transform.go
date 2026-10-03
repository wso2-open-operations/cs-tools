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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// source identifies alerts produced by this adapter.
const source = "GCP"

// defaults are used when a field is absent from both the payload and the operator config, keyed by canonical field name.
var defaults = map[string]string{
	"METRIC_NAME": "GCPCloudAlert",
	"SERVICE":     "Managed Services",
	"CATEGORY":    "Service Interruption",
	"ENVIRONMENT": "Unknown",
	"SEVERITY":    "critical",
}

// severityMap maps GCP Monitoring's condition severity to the canonical severity labels ("info" maps to "Informational", not "OK").
var severityMap = map[string]string{
	"critical": "Critical",
	"error":    "Major",
	"warning":  "Minor",
	"info":     "Informational",
}

// ErrMissingBody is returned when the webhook is called with no body at all.
var ErrMissingBody = errors.New("MISSING REQUEST BODY DATA")

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

// Config holds operator overrides mirroring "edge.api.gcp.alert.config"; unlike other fields, "severity" is looked up lowercase.
type Config map[string]string

// LoadConfig reads operator overrides from the GCP_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := utils.AlertConfigRaw("GCP_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid GCP_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound GCP Monitoring notification and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}

	// GCP's notification channel webhook wraps the actual alert under "incident"; nested objects degrade to an empty map rather than erroring.
	incident, _ := payload["incident"].(map[string]any)
	resource, _ := incident["resource"].(map[string]any)
	labels, _ := resource["labels"].(map[string]any)
	userLabels, _ := incident["policy_user_labels"].(map[string]any)

	state := utils.FirstNonEmpty(utils.Str(incident, "state"), "open")
	rawSeverity := utils.FirstNonEmpty(utils.Str(incident, "severity"), cfg["severity"], defaults["SEVERITY"])

	var severity string
	if state == "closed" {
		severity = "OK"
	} else {
		severity = utils.FirstNonEmpty(severityMap[strings.ToLower(rawSeverity)], rawSeverity)
	}

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", utils.FirstNonEmpty(utils.Str(labels, "service"), utils.Str(userLabels, "service"))),
		MetricName:       utils.FirstNonEmpty(utils.Str(incident, "policy_name"), utils.Str(incident, "condition_name"), defaults["METRIC_NAME"]),
		Severity:         severity,
		Category:         configValue(cfg, "CATEGORY", utils.FirstNonEmpty(utils.Str(labels, "category"), utils.Str(userLabels, "category"))),
		Environment:      configValue(cfg, "ENVIRONMENT", utils.FirstNonEmpty(utils.Str(labels, "environment"), utils.Str(userLabels, "environment"))),
		Source:           source,
		UniqueIdentifier: utils.Str(incident, "incident_id"),
		Description:      utils.CompactJSON(raw),
	}
	return alert, nil
}

// configValue applies the 3-tier resolution: payload value, then operator config, then the default.
func configValue(cfg Config, field, payloadValue string) string {
	return utils.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
