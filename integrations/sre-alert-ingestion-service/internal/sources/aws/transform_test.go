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
	"testing"
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

// The core routes an alert-born incident to an assignment group by, in order: the group the alarm
// names, its service's support group, then the SNS topic and AWS account it came from. Everything
// but the service's group has to come out of the AWS payload here.
func TestTransform_CarriesRoutingSignals(t *testing.T) {
	alarm := sampleAlarm(map[string]any{
		"AlarmDescription": `{"service":"choreo","assignment_group":" SRE - Apollo "}`,
		"AWSAccountId":     "111122223333",
	})
	a, err := Transform(snsEnvelope(alarm), Config{}, nil)
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
	a, err := Transform(snsEnvelope(sampleAlarm(nil)), Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceAccount != "123456789012" || a.AssignmentGroup != "" {
		t.Errorf("SourceAccount = %q, AssignmentGroup = %q; want the AlarmArn's account and no group", a.SourceAccount, a.AssignmentGroup)
	}

	raw := []byte(`{"Type":"Notification","TopicArn":"arn:aws:sns:eu-west-1:444455556666:sre-artemis","Message":"not json"}`)
	a, err = Transform(raw, Config{}, nil)
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
		if _, err := Transform(raw, Config{}, nil); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("%s: err = %v, want ErrUnsupportedType", typ, err)
		}
	}
}

func TestTransform_NoTypeStillTransforms(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"Message": `{"AlarmName":"HighCPUAlarm","NewStateValue":"ALARM"}`})
	if a, err := Transform(raw, Config{}, nil); err != nil || a.MetricName != "HighCPUAlarm" {
		t.Errorf("alert = %+v, err = %v", a, err)
	}
}
