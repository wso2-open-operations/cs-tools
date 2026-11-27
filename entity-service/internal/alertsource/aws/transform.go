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

// Package aws transforms an inbound AWS SNS notification of a CloudWatch alarm into the canonical alert; the mapping is a Go port of ServiceNow's AWSAlertUtilsV2.
package aws

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/fieldvalue"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/jsonnum"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// source identifies alerts produced by this adapter.
const source = "AWS"

// Built-in defaults for a field the payload does not carry; an empty default leaves the field empty.
const (
	defaultMetricName  = "Metric Name Not Available"
	defaultService     = ""
	defaultCategory    = "Service Interruption"
	defaultEnvironment = "Unknown"
	defaultSeverity    = "Critical"
)

// parseErrorMetricName is the deliberate metric name of an SNS message that is not JSON; it is not a default.
const parseErrorMetricName = "SNS Message Parse Error"

// ErrMissingBody is returned when the webhook is called with no body, or a body that isn't valid JSON at all.
var ErrMissingBody = errors.New("missing or invalid request body")

// ErrUnsupportedType is returned for an SNS message that is neither a Notification nor a subscription confirmation.
var ErrUnsupportedType = errors.New("unsupported SNS message type")

// Alert is the canonical alert model handed to the core component.
type Alert = model.Alert

// Transform parses a raw inbound SNS notification of a CloudWatch alarm and produces the canonical Alert, filling gaps with the built-in defaults.
func Transform(raw []byte) (Alert, error) {
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

	// Where the notification came from, known even when the Message itself cannot be parsed.
	topicArn := fieldvalue.Str(envelope, "TopicArn")

	var d model.Defaults
	messageRaw := fieldvalue.Str(envelope, "Message")
	var messageObj map[string]any
	if err := jsonnum.Unmarshal([]byte(messageRaw), &messageObj); err != nil {
		// SNS Message isn't valid JSON; still produces a real alert, matching the reference script's own fallback.
		alert := Alert{
			Service:     d.Or("service", "", defaultService),
			MetricName:  parseErrorMetricName,
			Severity:    d.Or("severity", "", defaultSeverity),
			Category:    d.Or("category", "", defaultCategory),
			Environment: d.Or("environment", "", defaultEnvironment),
			Source:      source,
			// A non-JSON SNS Message is the human-readable text itself; the raw body is kept in raw_alerts.
			Description:   strings.TrimSpace(messageRaw),
			SourceTopic:   topicArn,
			SourceAccount: arnAccount(topicArn),
		}
		alert.Defaulted = d.List()
		return alert, nil
	}

	// AlarmDescription optionally carries a JSON object of field overrides; invalid JSON there degrades to no overrides, not an error.
	alarmDesc := map[string]any{}
	if adRaw := strings.TrimSpace(fieldvalue.Str(messageObj, "AlarmDescription")); adRaw != "" {
		var parsed map[string]any
		if err := jsonnum.Unmarshal([]byte(adRaw), &parsed); err == nil && parsed != nil {
			alarmDesc = parsed
		}
	}

	// A recovered alarm (NewStateValue "OK") always forces severity "OK", the label every source uses, overriding AlarmDescription's own override.
	if fieldvalue.Str(messageObj, "NewStateValue") == "OK" {
		alarmDesc["severity"] = "OK"
	}

	alert := Alert{
		Service:          d.Or("service", fieldvalue.Str(alarmDesc, "service"), defaultService),
		MetricName:       d.Or("metric_name", fieldvalue.Str(messageObj, "AlarmName"), defaultMetricName),
		Severity:         d.Or("severity", fieldvalue.Str(alarmDesc, "severity"), defaultSeverity),
		Category:         d.Or("category", fieldvalue.Str(alarmDesc, "category"), defaultCategory),
		Environment:      d.Or("environment", fieldvalue.Str(alarmDesc, "environment"), defaultEnvironment),
		Source:           source,
		UniqueIdentifier: fieldvalue.Str(messageObj, "AlarmArn"),
		// The alarm's state reason, never the payload; the raw body is kept in raw_alerts.
		Description: fieldvalue.Str(messageObj, "NewStateReason"),
		// An alarm may name its own CSM assignment group in AlarmDescription, which beats every other routing signal.
		AssignmentGroup: strings.TrimSpace(fieldvalue.Str(alarmDesc, "assignment_group")),
		SourceTopic:     topicArn,
		SourceAccount: fieldvalue.FirstNonEmpty(
			fieldvalue.Str(messageObj, "AWSAccountId"),
			arnAccount(fieldvalue.Str(messageObj, "AlarmArn")),
			arnAccount(topicArn),
		),
	}
	alert.Defaulted = d.List()
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
