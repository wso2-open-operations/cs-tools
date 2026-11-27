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

package datadog

import (
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

// TestTransform_ConflictingTagAliasesAreStable checks the first alias in tagAliases always wins, so repeat deliveries keep one fingerprint.
func TestTransform_ConflictingTagAliasesAreStable(t *testing.T) {
	raw := []byte(`{"event_name":"high memory","transition":"Triggered","alert_id":"148502937",
		"tags":"environment:staging,env:production,svc:b,service:a"}`)
	for i := 0; i < 50; i++ {
		a, err := Transform(raw)
		if err != nil {
			t.Fatal(err)
		}
		if a.Environment != "production" || a.Service != "a" {
			t.Fatalf("run %d: environment = %q, service = %q; want production, a", i, a.Environment, a.Service)
		}
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
			name: "missing fields", raw: `{"event_id":"1","alert_id":"9","transition":"Triggered"}`,
			metricName: "Datadog Alert", severity: "Critical", category: "Service Interruption", environment: "Production",
			defaulted: []model.Defaulted{
				{Field: "metric_name", Value: "Datadog Alert"},
				{Field: "severity", Value: "Critical"},
				{Field: "category", Value: "Service Interruption"},
				{Field: "environment", Value: "Production"},
			},
		},
		{
			name: "recovered severity is not a default", raw: `{"event_name":"mem","alert_id":"9","transition":"Recovered","tags":"service:api,category:Performance,env:staging"}`,
			service: "api", metricName: "Datadog Alert: mem", severity: "OK", category: "Performance", environment: "staging",
		},
		{
			name: "every field", raw: `{"event_name":"mem","alert_id":"9","transition":"Triggered","service":"api","severity":"2","category":"Performance","environment":"staging"}`,
			service: "api", metricName: "Datadog Alert: mem", severity: "Major", category: "Performance", environment: "staging",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := Transform([]byte(c.raw))
			if err != nil {
				t.Fatal(err)
			}
			if a.Source != "Datadog" || a.Service != c.service || a.MetricName != c.metricName || a.Severity != c.severity || a.Category != c.category || a.Environment != c.environment {
				t.Errorf("alert = %+v, want service %q, metric name %q, severity %q, category %q, environment %q", a, c.service, c.metricName, c.severity, c.category, c.environment)
			}
			if !reflect.DeepEqual(a.Defaulted, c.defaulted) {
				t.Errorf("Defaulted = %+v, want %+v", a.Defaulted, c.defaulted)
			}
		})
	}
}
