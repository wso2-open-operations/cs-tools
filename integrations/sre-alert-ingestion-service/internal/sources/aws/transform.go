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

// Package aws transforms an inbound AWS SNS notification (wrapping a CloudWatch alarm) into the canonical alert, a Go port of ServiceNow's AWSAlertUtilsV2.
package aws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sre-alert-ingestion-service/internal/sources/jsonnum"
	"sre-alert-ingestion-service/utils"
)

// defaults mirror "edge.api.aws.alert.config" so its JSON can be pasted into AWS_ALERT_CONFIG unchanged.
var defaults = map[string]string{
	"source":      "AWS",
	"environment": "Unknown",
	"service":     "",
	"category":    "service_interruption",
	"severity":    "critical",
	"metric_name": "Unknown Metric",
}

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
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

// Config holds operator overrides, keyed lowercase like the ServiceNow "edge.api.aws.alert.config" property; empty fields fall back to defaults.
type Config map[string]string

// LoadConfig reads Config from the AWS_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := utils.AlertConfigRaw("AWS_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid AWS_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound SNS notification and produces the canonical Alert, cfg supplying operator overrides.
func Transform(raw []byte, cfg Config) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var envelope map[string]any
	if err := jsonnum.Unmarshal(raw, &envelope); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}

	base := Alert{
		Service:     configValue(cfg, "service"),
		MetricName:  configValue(cfg, "metric_name"),
		Severity:    configValue(cfg, "severity"),
		Category:    configValue(cfg, "category"),
		Environment: configValue(cfg, "environment"),
		Source:      configValue(cfg, "source"),
	}

	messageRaw := utils.Str(envelope, "Message")
	var messageObj map[string]any
	if err := jsonnum.Unmarshal([]byte(messageRaw), &messageObj); err != nil {
		// SNS Message isn't valid JSON; still produces a real alert, matching the reference script's own fallback.
		base.MetricName = "SNS Message Parse Error"
		base.Description = "Raw Payload Message: " + prettyJSON(raw)
		return base, nil
	}

	// AlarmDescription optionally carries a JSON object of field overrides; invalid JSON there degrades to no overrides, not an error.
	alarmDesc := map[string]any{}
	if adRaw := strings.TrimSpace(utils.Str(messageObj, "AlarmDescription")); adRaw != "" {
		var parsed map[string]any
		if err := jsonnum.Unmarshal([]byte(adRaw), &parsed); err == nil {
			alarmDesc = parsed
		}
	}

	// A recovered alarm (NewStateValue "OK") always forces severity "ok", overriding AlarmDescription's own override.
	if utils.Str(messageObj, "NewStateValue") == "OK" {
		alarmDesc["severity"] = "ok"
	}

	alert := Alert{
		Service:          utils.FirstNonEmpty(utils.Str(alarmDesc, "service"), base.Service),
		MetricName:       utils.FirstNonEmpty(utils.Str(messageObj, "AlarmName"), base.MetricName),
		Severity:         utils.FirstNonEmpty(utils.Str(alarmDesc, "severity"), base.Severity),
		Category:         utils.FirstNonEmpty(utils.Str(alarmDesc, "category"), base.Category),
		Environment:      utils.FirstNonEmpty(utils.Str(alarmDesc, "environment"), base.Environment),
		Source:           base.Source,
		UniqueIdentifier: utils.Str(messageObj, "AlarmArn"),
		Description:      prettyJSON([]byte(messageRaw)),
	}
	return alert, nil
}

// configValue applies the 2-tier resolution: operator config, then the hardcoded default.
func configValue(cfg Config, field string) string {
	return utils.FirstNonEmpty(cfg[field], defaults[field])
}

// prettyJSON re-indents raw JSON bytes with a 2-space indent, operating on the raw bytes to preserve field order.
func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
