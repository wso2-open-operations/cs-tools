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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "Azure"

// Built-in defaults for a field the payload does not carry.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

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

// Transform parses a raw inbound Azure Monitor Common Alert Schema webhook and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
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

	rawSeverity := fieldvalue.Str(essentials, "severity")
	monitorCondition := fieldvalue.Str(essentials, "monitorCondition")

	var d model.Defaults
	service := d.Or("service", payloadValue(props, "service", ""), defaultService)
	metricName := d.Or("metric_name", payloadValue(props, "metric_name", resolveMetricName(essentials, alertContext)), defaultMetricName)

	var severity string
	if monitorCondition == "Resolved" {
		severity = severityMap["Sev4"] // "OK"
	} else {
		// A value outside Sev0-Sev4 passes through unchanged; no severity at all takes the default.
		severity = d.Or("severity", fieldvalue.FirstNonEmpty(severityMap[rawSeverity], rawSeverity), defaultSeverity)
	}

	alert := Alert{
		Service:          service,
		MetricName:       metricName,
		Severity:         severity,
		Category:         d.Or("category", payloadValue(props, "category", ""), defaultCategory),
		Environment:      d.Or("environment", payloadValue(props, "environment", ""), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(essentials, "alertId"),
		// The rule's own text, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(essentials, "description"),
	}
	alert.Defaulted = d.List()
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

// payloadValue returns a direct value, else the field from customProperties, or "" when the payload carries neither.
func payloadValue(props map[string]any, field, directValue string) string {
	return fieldvalue.FirstNonEmpty(directValue, fieldvalue.Str(props, field))
}
