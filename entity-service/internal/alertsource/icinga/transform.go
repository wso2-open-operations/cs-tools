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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "Icinga"

// Built-in defaults for a field the payload does not carry; severity takes its default only when the state maps below yield none.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = "Managed Services"
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

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

// Transform parses a raw inbound Icinga notification and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
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

	var d model.Defaults
	service := d.Or("service", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "service"), fieldvalue.Str(payload, "service")), defaultService)
	metricName := d.Or("metric_name", buildMetricName(hostDisplayName, serviceName), defaultMetricName)

	var severity string
	if notificationType == "RECOVERY" {
		severity = "OK"
	} else {
		severity = d.Or("severity", mapSeverity(serviceState, hostState, serviceName), defaultSeverity)
	}

	// Icinga uses the host!service pattern as the natural unique key.
	uniqueIdentifier := hostName
	if serviceName != "" {
		uniqueIdentifier = hostName + "!" + serviceName
	}

	alert := Alert{
		Service:          service,
		MetricName:       metricName,
		Severity:         severity,
		Category:         d.Or("category", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "category"), fieldvalue.Str(payload, "category")), defaultCategory),
		Environment:      d.Or("environment", fieldvalue.FirstNonEmpty(fieldvalue.Str(vars, "environment"), fieldvalue.Str(payload, "environment")), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: uniqueIdentifier,
		// The check's plugin output, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.FirstNonEmpty(fieldvalue.Str(payload, "service_output"), fieldvalue.Str(payload, "host_output")),
	}
	alert.Defaulted = d.List()
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
