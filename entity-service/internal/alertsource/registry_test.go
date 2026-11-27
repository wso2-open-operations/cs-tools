// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// A field the source left out takes the source's own built-in default.
func TestRegistry_FillsGapsWithAlertIngestionDefaults(t *testing.T) {
	r := New()
	cases := map[string]struct {
		route, body, service, metricName, severity string
	}{
		"azure":      {"azure", `{"data":{"essentials":{"alertId":"a1","monitorCondition":"Fired"}}}`, "Managed Services", "Metric Name Not Available", "Critical"},
		"aws":        {"aws", `{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"{\"AlarmArn\":\"arn:a\",\"NewStateValue\":\"ALARM\"}"}`, "", "Metric Name Not Available", "Critical"},
		"site24x7":   {"site24x7", `{"MONITOR_ID":"1"}`, "", "", "Critical"},
		"servicenow": {"servicenow", `{"description":"disk full"}`, "", "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			transform, _ := r.Lookup(tc.route)
			alerts, err := transform([]byte(tc.body))
			if err != nil || len(alerts) != 1 {
				t.Fatalf("transform = %v, %v", alerts, err)
			}
			a := alerts[0]
			if a.Service != tc.service || a.MetricName != tc.metricName || a.Severity != tc.severity {
				t.Fatalf("service %q, metric name %q, severity %q; want %q, %q and %q", a.Service, a.MetricName, a.Severity, tc.service, tc.metricName, tc.severity)
			}
		})
	}
}

func TestRegistry_KeepsWhatTheSourceSent(t *testing.T) {
	r := New()
	transform, _ := r.Lookup("servicenow")
	alerts, err := transform([]byte(`{"service":"checkout","metric_name":"cpu","severity":"major","source":"Test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a := alerts[0]; a.Service != "checkout" || a.MetricName != "cpu" || a.Severity != "major" {
		t.Fatalf("alert = %+v, want the source's own service, metric name and severity", a)
	}
}

// OpenObserve's defaulted list survives the conversion to the canonical alert.
func TestRegistry_KeepsOpenObserveDefaulted(t *testing.T) {
	transform, _ := New().Lookup("openobserve")
	alerts, err := transform([]byte(`{"short_description":"p99 high","correlation_id":"c1","service":"api","urgency":"1","category":"Performance","environment":"Staging"}`))
	if err != nil || len(alerts) != 1 {
		t.Fatalf("transform = %v, %v", alerts, err)
	}
	if alerts[0].Severity != "Critical" || alerts[0].Defaulted != nil {
		t.Fatalf("alert = %+v, want severity Critical from urgency and nothing defaulted", alerts[0])
	}
	alerts, err = transform([]byte(`{"correlation_id":"c1"}`))
	if err != nil || len(alerts) != 1 {
		t.Fatalf("transform = %v, %v", alerts, err)
	}
	want := []model.Defaulted{
		{Field: "service", Value: "Managed Services"},
		{Field: "severity", Value: "Critical"},
		{Field: "category", Value: "Service Interruption"},
		{Field: "environment", Value: "Production"},
	}
	if !reflect.DeepEqual(alerts[0].Defaulted, want) {
		t.Fatalf("Defaulted = %+v, want %+v", alerts[0].Defaulted, want)
	}
}
