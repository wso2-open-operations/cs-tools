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

package alertsource

import (
	"slices"
	"testing"
)

func TestParseRIAFlag(t *testing.T) {
	tests := []struct {
		value any
		want  RIAFlag
	}{
		{"true", RIAOn},
		{" TRUE ", RIAOn},
		{"false", RIAOff},
		{"False", RIAOff},
		{true, RIAOn},
		{false, RIAOff},
		{"yes", RIAUnset},
		{"", RIAUnset},
		{nil, RIAUnset},
		{1.0, RIAUnset},
	}
	for _, tt := range tests {
		if got := ParseRIAFlag(tt.value); got != tt.want {
			t.Errorf("ParseRIAFlag(%#v) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestRIALabels_ReadsEachSourceWhereItKeepsService(t *testing.T) {
	tests := []struct {
		name   string
		source string
		body   string
		want   []RIAFlag
	}{
		{"azure", "azure", `{"data":{"customProperties":{"service":"svc","ria":"true"}}}`, []RIAFlag{RIAOn}},
		{"gcp", "gcp", `{"incident":{"policy_user_labels":{"ria":"false"}}}`, []RIAFlag{RIAOff}},
		{"icinga", "icinga", `{"vars":{"ria":true}}`, []RIAFlag{RIAOn}},
		{"aws", "aws", `{"Message":"{\"AlarmDescription\":\"{\\\"service\\\":\\\"svc\\\",\\\"ria\\\":\\\"true\\\"}\"}"}`, []RIAFlag{RIAOn}},
		{"aws without a description", "aws", `{"Message":"{\"AlarmDescription\":null}"}`, []RIAFlag{RIAUnset}},
		{"aws budget text", "aws", `{"Message":"AWS Budget Notification"}`, []RIAFlag{RIAUnset}},
		{"site24x7", "site24x7", `{"TAGS":["Service:svc","RIA:true"]}`, []RIAFlag{RIAOn}},
		{"site24x7 lower case tag", "site24x7", `{"TAGS":["ria: false"]}`, []RIAFlag{RIAOff}},
		{"prometheus per alert", "prometheus", `{"alerts":[{"labels":{"ria":"true"}},"skipped",{"labels":{}},{"labels":{"ria":"false"}}]}`, []RIAFlag{RIAOn, RIAUnset, RIAOff}},
		{"servicenow object", "servicenow", `{"service":"svc","ria":"true"}`, []RIAFlag{RIAOn}},
		{"servicenow array", "servicenow", `[{"ria":"true"},{"ria":"false"},{}]`, []RIAFlag{RIAOn, RIAOff, RIAUnset}},
		{"datadog top level", "datadog", `{"ria":"TRUE"}`, []RIAFlag{RIAOn}},
		{"no label", "elasticsearch", `{"rule_name":"x"}`, []RIAFlag{RIAUnset}},
		{"not json", "azure", `not json`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RIALabels(tt.source, []byte(tt.body)); !slices.Equal(got, tt.want) {
				t.Fatalf("RIALabels(%s) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestRIALabels_MatchesTheTransformAlertCount(t *testing.T) {
	registry := New("00000000-0000-4000-8000-000000000001", nil)
	bodies := map[string]string{
		"prometheus": `{"receiver":"r","status":"firing","alerts":[{"labels":{"alertname":"a","ria":"true"}},7,{"labels":{"alertname":"b"}}]}`,
		"servicenow": `[{"service":"s","metric_name":"m","ria":"true"},{"service":"s","metric_name":"n"}]`,
	}
	for source, body := range bodies {
		transform, _ := registry.Lookup(source)
		alerts, err := transform([]byte(body))
		if err != nil {
			t.Fatalf("%s transform: %v", source, err)
		}
		if labels := RIALabels(source, []byte(body)); len(labels) != len(alerts) {
			t.Errorf("%s: %d labels for %d alerts, want one per alert", source, len(labels), len(alerts))
		}
	}
}
