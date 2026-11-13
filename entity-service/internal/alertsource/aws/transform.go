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

// Package aws transforms an inbound AWS SNS notification (a CloudWatch alarm or an AWS Budgets alert) into the canonical alert; the alarm mapping is a Go port of ServiceNow's AWSAlertUtilsV2.
package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
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
var ErrMissingBody = errors.New("missing or invalid request body")

// ErrUnsupportedType is returned for an SNS message that is neither a Notification nor a subscription confirmation.
var ErrUnsupportedType = errors.New("unsupported SNS message type")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Config holds operator overrides, keyed lowercase like the ServiceNow "edge.api.aws.alert.config" property; empty fields fall back to defaults.
type Config map[string]string

// LoadConfig reads Config from the AWS_ALERT_CONFIG env var.
func LoadConfig() (Config, error) {
	raw := fieldvalue.AlertConfigRaw("AWS_ALERT_CONFIG")
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid AWS_ALERT_CONFIG: %w", err)
	}
	return cfg, nil
}

// Transform parses a raw inbound SNS notification and produces the canonical Alert, cfg supplying operator overrides and tagger (optional) the tags of a budget alert's budget.
func Transform(raw []byte, cfg Config, tagger BudgetTagFetcher) (Alert, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return Alert{}, ErrMissingBody
	}

	var envelope map[string]any
	if err := jsonnum.Unmarshal(raw, &envelope); err != nil {
		return Alert{}, fmt.Errorf("%w: %v", ErrMissingBody, err)
	}
	if t := fieldvalue.Str(envelope, "Type"); t != "" && t != "Notification" {
		return Alert{}, fmt.Errorf("%w %q", ErrUnsupportedType, t)
	}

	base := Alert{
		Service:     configValue(cfg, "service"),
		MetricName:  configValue(cfg, "metric_name"),
		Severity:    configValue(cfg, "severity"),
		Category:    configValue(cfg, "category"),
		Environment: configValue(cfg, "environment"),
		Source:      configValue(cfg, "source"),
		// Where the notification came from, known even when the Message itself cannot be parsed.
		SourceTopic:   fieldvalue.Str(envelope, "TopicArn"),
		SourceAccount: arnAccount(fieldvalue.Str(envelope, "TopicArn")),
	}

	messageRaw := fieldvalue.Str(envelope, "Message")
	var messageObj map[string]any
	if err := jsonnum.Unmarshal([]byte(messageRaw), &messageObj); err != nil {
		if subject := fieldvalue.Str(envelope, "Subject"); isBudget(subject, messageRaw) {
			return fromBudget(subject, messageRaw, base, tagger), nil
		}
		// SNS Message isn't valid JSON; still produces a real alert, matching the reference script's own fallback.
		base.MetricName = "SNS Message Parse Error"
		// A non-JSON SNS Message is the human-readable text itself; the raw body is kept in raw_alerts.
		base.Description = strings.TrimSpace(messageRaw)
		return base, nil
	}

	// AlarmDescription optionally carries a JSON object of field overrides; invalid JSON there degrades to no overrides, not an error.
	alarmDesc := map[string]any{}
	if adRaw := strings.TrimSpace(fieldvalue.Str(messageObj, "AlarmDescription")); adRaw != "" {
		var parsed map[string]any
		if err := jsonnum.Unmarshal([]byte(adRaw), &parsed); err == nil && parsed != nil {
			alarmDesc = parsed
		}
	}

	// A recovered alarm (NewStateValue "OK") always forces severity "ok", overriding AlarmDescription's own override.
	if fieldvalue.Str(messageObj, "NewStateValue") == "OK" {
		alarmDesc["severity"] = "ok"
	}

	alert := Alert{
		Service:          fieldvalue.FirstNonEmpty(fieldvalue.Str(alarmDesc, "service"), base.Service),
		MetricName:       fieldvalue.FirstNonEmpty(fieldvalue.Str(messageObj, "AlarmName"), base.MetricName),
		Severity:         fieldvalue.FirstNonEmpty(fieldvalue.Str(alarmDesc, "severity"), base.Severity),
		Category:         fieldvalue.FirstNonEmpty(fieldvalue.Str(alarmDesc, "category"), base.Category),
		Environment:      fieldvalue.FirstNonEmpty(fieldvalue.Str(alarmDesc, "environment"), base.Environment),
		Source:           base.Source,
		UniqueIdentifier: fieldvalue.Str(messageObj, "AlarmArn"),
		// The alarm's state reason, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(messageObj, "NewStateReason"),
		// An alarm may name its own CSM assignment group in AlarmDescription, which beats every other routing signal.
		AssignmentGroup: strings.TrimSpace(fieldvalue.Str(alarmDesc, "assignment_group")),
		SourceTopic:     base.SourceTopic,
		SourceAccount: fieldvalue.FirstNonEmpty(
			fieldvalue.Str(messageObj, "AWSAccountId"),
			arnAccount(fieldvalue.Str(messageObj, "AlarmArn")),
			base.SourceAccount,
		),
	}
	return alert, nil
}

// arnAccount returns the account id of an ARN (arn:partition:service:region:account:resource), or "".
func arnAccount(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[4]
}

// configValue applies the 2-tier resolution: operator config, then the hardcoded default.
func configValue(cfg Config, field string) string {
	return fieldvalue.FirstNonEmpty(cfg[field], defaults[field])
}
