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

package prometheus

import (
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// A Grafana alert found by its receiver alone keeps alertname as its metric name, as alert-ingestion does, so the fingerprint matches; only grafana_folder switches to the title.
func TestTransform_GrafanaMetricNameMatchesAlertIngestion(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"receiver only":  {`{"receiver":"grafana-default-email","status":"firing","alerts":[{"labels":{"alertname":"HighLatency"},"annotations":{"title":"Checkout latency"}}]}`, "HighLatency"},
		"grafana_folder": {`{"receiver":"other","status":"firing","alerts":[{"labels":{"alertname":"HighLatency","grafana_folder":"apps"},"annotations":{"title":"Checkout latency"}}]}`, "Grafana Alert: Checkout latency"},
	} {
		t.Run(name, func(t *testing.T) {
			alerts, err := Transform([]byte(tc.body))
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			if len(alerts) != 1 || alerts[0].MetricName != tc.want || alerts[0].Source != "Grafana" {
				t.Fatalf("alerts = %+v, want one Grafana alert named %q", alerts, tc.want)
			}
		})
	}
}

// A field the payload leaves out takes the built-in default and is listed in Defaulted; a payload carrying every field lists nothing.
func TestTransform_Defaults(t *testing.T) {
	cases := []struct {
		name                                                 string
		raw                                                  string
		service, metricName, severity, category, environment string
		defaulted                                            []model.Defaulted
	}{
		{
			name: "missing fields", raw: `{"status":"firing","alerts":[{"labels":{},"fingerprint":"f1"}]}`,
			service: "Managed Services", metricName: "Metric Name Not Available", severity: "Critical", category: "Service Interruption", environment: "Unknown",
			defaulted: []model.Defaulted{
				{Field: "service", Value: "Managed Services"},
				{Field: "metric_name", Value: "Metric Name Not Available"},
				{Field: "severity", Value: "Critical"},
				{Field: "category", Value: "Service Interruption"},
				{Field: "environment", Value: "Unknown"},
			},
		},
		{
			name: "resolved severity is not a default", raw: `{"status":"resolved","alerts":[{"labels":{"alertname":"Crash","service":"api","category":"Performance","environment":"staging"},"fingerprint":"f1"}]}`,
			service: "api", metricName: "Crash", severity: "OK", category: "Performance", environment: "staging",
		},
		{
			name: "every field", raw: `{"status":"firing","alerts":[{"labels":{"alertname":"Crash","service":"api","severity":"high","category":"Performance","environment":"staging"},"fingerprint":"f1"}]}`,
			service: "api", metricName: "Crash", severity: "Major", category: "Performance", environment: "staging",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alerts, err := Transform([]byte(c.raw))
			if err != nil || len(alerts) != 1 {
				t.Fatalf("Transform = %+v, %v", alerts, err)
			}
			a := alerts[0]
			if a.Source != "Prometheus" || a.Service != c.service || a.MetricName != c.metricName || a.Severity != c.severity || a.Category != c.category || a.Environment != c.environment {
				t.Errorf("alert = %+v, want service %q, metric name %q, severity %q, category %q, environment %q", a, c.service, c.metricName, c.severity, c.category, c.environment)
			}
			if !reflect.DeepEqual(a.Defaulted, c.defaulted) {
				t.Errorf("Defaulted = %+v, want %+v", a.Defaulted, c.defaulted)
			}
		})
	}
}

// Each alert in a batch records only its own defaults.
func TestTransform_DefaultsPerAlert(t *testing.T) {
	raw := `{"status":"firing","alerts":[
		{"labels":{"alertname":"Crash","service":"api","severity":"critical","category":"Performance","environment":"staging"},"fingerprint":"f1"},
		{"labels":{"alertname":"Crash","service":"api","category":"Performance","environment":"staging"},"fingerprint":"f2"}]}`
	alerts, err := Transform([]byte(raw))
	if err != nil || len(alerts) != 2 {
		t.Fatalf("Transform = %+v, %v", alerts, err)
	}
	if alerts[0].Defaulted != nil {
		t.Errorf("first alert Defaulted = %+v, want nil", alerts[0].Defaulted)
	}
	if want := []model.Defaulted{{Field: "severity", Value: "Critical"}}; !reflect.DeepEqual(alerts[1].Defaulted, want) {
		t.Errorf("second alert Defaulted = %+v, want %+v", alerts[1].Defaulted, want)
	}
}
