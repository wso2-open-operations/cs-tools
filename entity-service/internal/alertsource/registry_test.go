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

import "testing"

const testDefaultServiceID = "00000000-0000-4000-8000-000000000001"

func TestRegistry_FillsWhatTheSourceLeftEmpty(t *testing.T) {
	r := New(testDefaultServiceID, nil)
	cases := map[string]struct {
		route, body, metricName string
	}{
		"azure":                       {"azure", `{"data":{"essentials":{"alertId":"a1","monitorCondition":"Fired"}}}`, "Azure Alert"},
		"gcp":                         {"gcp", `{"incident":{"incident_id":"i1","state":"open"}}`, "GCP Alert"},
		"aws":                         {"aws", `{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"{\"AlarmArn\":\"arn:a\",\"NewStateValue\":\"ALARM\"}"}`, "AWS Alert"},
		"site24x7 without a status":   {"site24x7", `{"MONITOR_ID":"1"}`, "Site24x7 Alert"},
		"servicenow without a source": {"servicenow", `{"description":"disk full"}`, "ServiceNow Alert"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			transform, _ := r.Lookup(tc.route)
			alerts, err := transform([]byte(tc.body))
			if err != nil || len(alerts) != 1 {
				t.Fatalf("transform = %v, %v", alerts, err)
			}
			a := alerts[0]
			if a.Service != testDefaultServiceID || a.MetricName != tc.metricName || a.Severity != DefaultSeverity {
				t.Fatalf("service %q, metric name %q, severity %q; want %q, %q and %q", a.Service, a.MetricName, a.Severity, testDefaultServiceID, tc.metricName, DefaultSeverity)
			}
		})
	}
}

func TestRegistry_KeepsWhatTheSourceSent(t *testing.T) {
	r := New(testDefaultServiceID, nil)
	transform, _ := r.Lookup("servicenow")
	alerts, err := transform([]byte(`{"service":"checkout","metric_name":"cpu","severity":"major","source":"Test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a := alerts[0]; a.Service != "checkout" || a.MetricName != "cpu" || a.Severity != "major" {
		t.Fatalf("alert = %+v, want the source's own service, metric name and severity", a)
	}
}

func TestRegistry_ReadsSite24x7TagsWithoutConfiguration(t *testing.T) {
	r := New(testDefaultServiceID, nil)
	transform, _ := r.Lookup("site24x7")
	alerts, err := transform([]byte(`{"STATUS":"DOWN","MONITORNAME":"api","MONITOR_ID":"1","TAGS":["Service:checkout","Environment:prod"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if a := alerts[0]; a.Service != "checkout" || a.Environment != "prod" {
		t.Fatalf("alert = %+v, want the service and environment read from the tags", a)
	}
}
