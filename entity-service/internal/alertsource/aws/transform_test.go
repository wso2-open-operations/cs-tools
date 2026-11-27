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

package aws

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

func snsEnvelope(message any) []byte {
	b, _ := json.Marshal(message)
	raw, _ := json.Marshal(map[string]any{
		"Type":      "Notification",
		"MessageId": "abc-123",
		"TopicArn":  "arn:aws:sns:us-east-1:123456789012:alerts",
		"Message":   string(b),
	})
	return raw
}

func sampleAlarm(overrides map[string]any) map[string]any {
	alarm := map[string]any{
		"AlarmName":        "HighCPUAlarm",
		"AlarmArn":         "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPUAlarm",
		"NewStateValue":    "ALARM",
		"AlarmDescription": `{"service":"client-example-alert-integration","severity":"critical"}`,
	}
	for k, v := range overrides {
		alarm[k] = v
	}
	return alarm
}

// Every routing signal except the service's support group must come out of the AWS payload: the named group, the SNS topic and the account.
func TestTransform_CarriesRoutingSignals(t *testing.T) {
	alarm := sampleAlarm(map[string]any{
		"AlarmDescription": `{"service":"choreo","assignment_group":" SRE - Apollo "}`,
		"AWSAccountId":     "111122223333",
	})
	a, err := Transform(snsEnvelope(alarm))
	if err != nil {
		t.Fatal(err)
	}
	if a.AssignmentGroup != "SRE - Apollo" {
		t.Errorf("AssignmentGroup = %q, want the trimmed AlarmDescription value", a.AssignmentGroup)
	}
	if a.SourceTopic != "arn:aws:sns:us-east-1:123456789012:alerts" {
		t.Errorf("SourceTopic = %q, want the envelope's TopicArn", a.SourceTopic)
	}
	if a.SourceAccount != "111122223333" {
		t.Errorf("SourceAccount = %q, want the alarm's AWSAccountId", a.SourceAccount)
	}
}

// Without AWSAccountId the account comes from the AlarmArn; without a usable Message, from the TopicArn.
func TestTransform_SourceAccountFallsBack(t *testing.T) {
	a, err := Transform(snsEnvelope(sampleAlarm(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceAccount != "123456789012" || a.AssignmentGroup != "" {
		t.Errorf("SourceAccount = %q, AssignmentGroup = %q; want the AlarmArn's account and no group", a.SourceAccount, a.AssignmentGroup)
	}

	raw := []byte(`{"Type":"Notification","TopicArn":"arn:aws:sns:eu-west-1:444455556666:sre-artemis","Message":"not json"}`)
	a, err = Transform(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceTopic != "arn:aws:sns:eu-west-1:444455556666:sre-artemis" || a.SourceAccount != "444455556666" {
		t.Errorf("parse-error alert: topic %q account %q; want both from the TopicArn", a.SourceTopic, a.SourceAccount)
	}
}

func TestArnAccount(t *testing.T) {
	for arn, want := range map[string]string{
		"arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPU": "123456789012",
		"arn:aws:sns:us-east-1:123456789012:alerts":               "123456789012",
		"not-an-arn": "",
		"":           "",
	} {
		if got := arnAccount(arn); got != want {
			t.Errorf("arnAccount(%q) = %q, want %q", arn, got, want)
		}
	}
}

func TestTransform_RejectsNonNotificationTypes(t *testing.T) {
	for _, typ := range []string{"UnsubscribeConfirmation", "SomethingNew"} {
		raw := []byte(`{"Type":"` + typ + `","TopicArn":"arn:aws:sns:us-east-1:123456789012:alerts","Message":"You have chosen to deactivate subscription."}`)
		if _, err := Transform(raw); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("%s: err = %v, want ErrUnsupportedType", typ, err)
		}
	}
}

func TestTransform_NoTypeStillTransforms(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"Message": `{"AlarmName":"HighCPUAlarm","NewStateValue":"ALARM"}`})
	if a, err := Transform(raw); err != nil || a.MetricName != "HighCPUAlarm" {
		t.Errorf("alert = %+v, err = %v", a, err)
	}
}

func TestTransform_NullAlarmDescriptionOnRecovery(t *testing.T) {
	a, err := Transform(snsEnvelope(sampleAlarm(map[string]any{"AlarmDescription": "null", "NewStateValue": "OK"})))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if a.Severity != "OK" {
		t.Errorf("severity = %q, want OK for a recovered alarm", a.Severity)
	}
}

// A field the payload leaves out takes the built-in default and is listed in Defaulted; a payload carrying every field lists nothing.
func TestTransform_Defaults(t *testing.T) {
	cases := []struct {
		name                                                 string
		raw                                                  []byte
		service, metricName, severity, category, environment string
		defaulted                                            []model.Defaulted
	}{
		{
			name: "missing fields", raw: snsEnvelope(map[string]any{"AlarmArn": "arn:a", "NewStateValue": "ALARM"}),
			metricName: "Metric Name Not Available", severity: "Critical", category: "Service Interruption", environment: "Unknown",
			defaulted: []model.Defaulted{
				{Field: "metric_name", Value: "Metric Name Not Available"},
				{Field: "severity", Value: "Critical"},
				{Field: "category", Value: "Service Interruption"},
				{Field: "environment", Value: "Unknown"},
			},
		},
		{
			name: "every field", raw: snsEnvelope(sampleAlarm(map[string]any{"AlarmDescription": `{"service":"choreo","severity":"Major","category":"Performance","environment":"Staging"}`})),
			service: "choreo", metricName: "HighCPUAlarm", severity: "Major", category: "Performance", environment: "Staging",
		},
		{
			name: "recovered alarm is not a default", raw: snsEnvelope(map[string]any{"AlarmName": "HighCPUAlarm", "NewStateValue": "OK", "AlarmDescription": `{"service":"choreo","category":"Performance","environment":"Staging"}`}),
			service: "choreo", metricName: "HighCPUAlarm", severity: "OK", category: "Performance", environment: "Staging",
		},
		{
			name: "parse error metric name is not a default", raw: []byte(`{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"not json"}`),
			metricName: "SNS Message Parse Error", severity: "Critical", category: "Service Interruption", environment: "Unknown",
			defaulted: []model.Defaulted{
				{Field: "severity", Value: "Critical"},
				{Field: "category", Value: "Service Interruption"},
				{Field: "environment", Value: "Unknown"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := Transform(c.raw)
			if err != nil {
				t.Fatal(err)
			}
			if a.Source != "AWS" || a.Service != c.service || a.MetricName != c.metricName || a.Severity != c.severity || a.Category != c.category || a.Environment != c.environment {
				t.Errorf("alert = %+v, want service %q, metric name %q, severity %q, category %q, environment %q", a, c.service, c.metricName, c.severity, c.category, c.environment)
			}
			if !reflect.DeepEqual(a.Defaulted, c.defaulted) {
				t.Errorf("Defaulted = %+v, want %+v", a.Defaulted, c.defaulted)
			}
		})
	}
}
