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

// Package icinga transforms an inbound Icinga host/service notification into the canonical alert, a Go port of ServiceNow's IcingaAlertProcessor.
package icinga

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
const source = "Icinga"

// defaults are used when a field is absent from both the payload and the operator config; SEVERITY is absent since it comes from the state maps below.
var defaults = map[string]string{
	"METRIC_NAME": "IcingaAlert",
	"SERVICE":     "managed_services",
	"CATEGORY":    "service_interruption",
	"ENVIRONMENT": "development",
}

// serviceStateMap maps an Icinga service check's state to the canonical severity labels.
var serviceStateMap = map[string]string{
	"critical": "Critical",
	"warning":  "Warning",
	"unknown":  "Major",
	"ok":       "OK",
}

// hostStateMap maps an Icinga host check's state to the canonical severity labels.
var hostStateMap = map[string]string{
	"down":        "Critical",
	"unreachable": "Major",
	"up":          "OK",
}

// ErrInvalidStructure is returned when the payload is missing either notification_type or host_name.
var ErrInvalidStructure = errors.New("invalid icinga alert payload structure")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Config holds operator overrides mirroring "edge.api.icinga.alert.config"; any field left empty falls back to the default.
type Config map[string]string

// LoadConfig reads operator overrides from the ICINGA_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("ICINGA_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid ICINGA_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound Icinga notification and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	var payload map[string]any
	if err := jsonnum.Unmarshal(raw, &payload); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrInvalidStructure, err)
	}

	// A genuine Icinga notification carries its type and the host it's about.
	notificationType := fieldvalue.Str(payload, "notification_type")
	hostName := fieldvalue.Str(payload, "host_name")
	if notificationType == "" || hostName == "" {
		return Alert{}, ErrInvalidStructure
	}
	notificationType = strings.ToUpper(notificationType)

	// Icinga sends either a host check (no service_name) or a service check (service_name present).
	hostDisplayName := fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "host_display_name"), hostName)
	hostState := fieldvalue.Str(payload, "host_state")
	serviceName := fieldvalue.Str(payload, "service_name")
	serviceState := fieldvalue.Str(payload, "service_state")

	// Custom vars, passed from Icinga host/service vars.
	vars, _ := payload["vars"].(map[string]any)

	var severity string
	if notificationType == "RECOVERY" {
		severity = "OK"
	} else {
		severity = mapSeverity(serviceState, hostState, serviceName)
	}

	// Icinga uses the host!service pattern as the natural unique key.
	uniqueIdentifier := hostName
	if serviceName != "" {
		uniqueIdentifier = hostName + "!" + serviceName
	}

	alert := Alert{
		Service:          configValue(cfg, "SERVICE", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "service"), fieldvalue.Str(payload, "service"))),
		MetricName:       configValue(cfg, "METRIC_NAME", buildMetricName(hostDisplayName, serviceName)),
		Severity:         severity,
		Category:         configValue(cfg, "CATEGORY", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "category"), fieldvalue.Str(payload, "category"))),
		Environment:      configValue(cfg, "ENVIRONMENT", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "environment"), fieldvalue.Str(payload, "environment"))),
		Source:           source,
		UniqueIdentifier: uniqueIdentifier,
		// The check's plugin output, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "service_output"), fieldvalue.Str(payload, "host_output")),
	}
	return alert, nil
}

// mapSeverity routes to serviceStateMap or hostStateMap, falling back to the raw state value when it isn't recognized.
func mapSeverity(serviceState, hostState, serviceName string) string {
	if serviceName != "" {
		return fieldvalue.FirstNonEmpty(serviceStateMap[strings.ToLower(serviceState)], serviceState)
	}
	return fieldvalue.FirstNonEmpty(hostStateMap[strings.ToLower(hostState)], hostState)
}

// buildMetricName builds "Icinga Alert: <host> / <service>", or "" when both are absent.
func buildMetricName(hostDisplayName, serviceName string) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{hostDisplayName, serviceName} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Icinga Alert: " + strings.Join(parts, " / ")
}

// configValue applies the 3-tier resolution: payload value, then operator config, then the default.
func configValue(cfg Config, field, payloadValue string) string {
	return fieldvalue.FirstNonEmpty(payloadValue, cfg[field], defaults[field])
}
