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

// Package azure transforms an inbound Azure Monitor Common Alert Schema webhook into the canonical alert, a Go port of ServiceNow's "Azure Alert API".
package azure

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
const source = "Azure"

// defaults mirror "edge.api.azure.alert.config" so its JSON can be pasted into AZURE_ALERT_CONFIG unchanged.
var defaults = map[string]string{
	"metric_name": "AzureCloudAlert",
	"service":     "Managed Services",
	"category":    "Service Interruption",
	"environment": "Unknown",
	"severity":    "Critical",
}

// severityMap maps Azure Monitor's Sev0-Sev4 severity to the canonical severity labels.
var severityMap = map[string]string{
	"Sev0": "Critical",
	"Sev1": "Major",
	"Sev2": "Minor",
	"Sev3": "Warning",
	"Sev4": "OK",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Config holds operator overrides, keyed the same way as defaults; any field left empty falls back to the default.
type Config map[string]string

// LoadConfig reads operator overrides from the AZURE_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("AZURE_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid AZURE_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Azure Monitor Common Alert Schema webhook and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var envelope map[string]any
	if err := jsonnum.Unmarshal(raw, &envelope); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}

	data, _ := envelope["data"].(map[string]any)
	essentials, _ := data["essentials"].(map[string]any)
	alertContext, _ := data["alertContext"].(map[string]any)
	// Cost alerts don't include a customProperties field, so default to an empty object.
	props, _ := data["customProperties"].(map[string]any)

	rawSeverity := fieldvalue.FirstNonEmpty(fieldvalue.Str(essentials, "severity"), cfg["severity"], defaults["severity"])
	monitorCondition := fieldvalue.Str(essentials, "monitorCondition")

	var mappedSeverity string
	if monitorCondition == "Resolved" {
		mappedSeverity = severityMap["Sev4"] // "OK"
	} else {
		// A value outside Sev0-Sev4, such as the "Critical" default, passes through unchanged.
		mappedSeverity = fieldvalue.FirstNonEmpty(severityMap[rawSeverity], rawSeverity)
	}

	alert := Alert{
		Service:          configValue(cfg, props, "service", ""),
		MetricName:       configValue(cfg, props, "metric_name", resolveMetricName(essentials, alertContext)),
		Severity:         mappedSeverity,
		Category:         configValue(cfg, props, "category", ""),
		Environment:      configValue(cfg, props, "environment", ""),
		Source:           fieldvalue.FirstNonEmpty(cfg["source"], source),
		UniqueIdentifier: fieldvalue.Str(essentials, "alertId"),
		// The rule's own text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(essentials, "description"),
	}
	return alert, nil
}

// resolveMetricName mirrors the reference script's cost-alert special case: a Cost Management alert uses its budget name instead of alertRule.
func resolveMetricName(essentials, alertContext map[string]any) string {
	if fieldvalue.Str(essentials, "monitoringService") == "CostAlerts" {
		alertData, _ := alertContext["AlertData"].(map[string]any)
		if budgetName := fieldvalue.Str(alertData, "BudgetName"); budgetName != "" {
			return "Cost Alert: " + budgetName
		}
	}
	return fieldvalue.Str(essentials, "alertRule")
}

// configValue applies the 3-tier resolution: a direct value, else customProperties, then operator config, then the default.
func configValue(cfg Config, props map[string]any, field, directValue string) string {
	payloadValue := directValue
	if payloadValue == "" {
		payloadValue = fieldvalue.Str(props, field)
	}
	return fieldvalue.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
