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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/site24x7"
)

// A field the source left out takes the source's own built-in default, as in alert-ingestion, so the fingerprint matches v1.
func TestRegistry_FillsGapsWithAlertIngestionDefaults(t *testing.T) {
	r := New(nil)
	cases := map[string]struct {
		route, body, service, metricName, severity string
	}{
		"azure":      {"azure", `{"data":{"essentials":{"alertId":"a1","monitorCondition":"Fired"}}}`, "Managed Services", "AzureCloudAlert", "Critical"},
		"aws":        {"aws", `{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:1:t","Message":"{\"AlarmArn\":\"arn:a\",\"NewStateValue\":\"ALARM\"}"}`, "", "Unknown Metric", "critical"},
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
	r := New(nil)
	transform, _ := r.Lookup("servicenow")
	alerts, err := transform([]byte(`{"service":"checkout","metric_name":"cpu","severity":"major","source":"Test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a := alerts[0]; a.Service != "checkout" || a.MetricName != "cpu" || a.Severity != "major" {
		t.Fatalf("alert = %+v, want the source's own service, metric name and severity", a)
	}
}

// With no SITE24X7_ALERT_CONFIG, alert-ingestion rejects a Site24x7 alert without STATUS, and so does the consumer.
func TestRegistry_RejectsASite24x7AlertWithoutStatus(t *testing.T) {
	transform, _ := New(nil).Lookup("site24x7")
	if _, err := transform([]byte(`{"MONITOR_ID":"1"}`)); !errors.Is(err, site24x7.ErrMissingStatus) {
		t.Fatalf("err = %v, want ErrMissingStatus", err)
	}
}
