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

package azure

import (
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

func TestTransform_SeverityOutsideSevNPassesThrough(t *testing.T) {
	a, err := Transform([]byte(`{"data":{"essentials":{"alertId":"a1","severity":"Sev9","monitorCondition":"Fired"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "Sev9" {
		t.Errorf("severity = %q, want Sev9 unchanged", a.Severity)
	}
}

func TestTransform_SevNStillMapped(t *testing.T) {
	a, err := Transform([]byte(`{"data":{"essentials":{"alertId":"a1","severity":"Sev2","monitorCondition":"Fired"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "Minor" {
		t.Errorf("severity = %q, want Minor", a.Severity)
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
			name: "missing fields", raw: `{"data":{"essentials":{"alertId":"a1","monitorCondition":"Fired"}}}`,
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
			name: "resolved severity is not a default", raw: `{"data":{"essentials":{"alertId":"a1","alertRule":"rt","monitorCondition":"Resolved"},"customProperties":{"service":"api","category":"Performance","environment":"Staging"}}}`,
			service: "api", metricName: "rt", severity: "OK", category: "Performance", environment: "Staging",
		},
		{
			name: "every field", raw: `{"data":{"essentials":{"alertId":"a1","alertRule":"rt","severity":"Sev1","monitorCondition":"Fired"},"customProperties":{"service":"api","category":"Performance","environment":"Staging"}}}`,
			service: "api", metricName: "rt", severity: "Major", category: "Performance", environment: "Staging",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := Transform([]byte(c.raw))
			if err != nil {
				t.Fatal(err)
			}
			if a.Source != "Azure" || a.Service != c.service || a.MetricName != c.metricName || a.Severity != c.severity || a.Category != c.category || a.Environment != c.environment {
				t.Errorf("alert = %+v, want service %q, metric name %q, severity %q, category %q, environment %q", a, c.service, c.metricName, c.severity, c.category, c.environment)
			}
			if !reflect.DeepEqual(a.Defaulted, c.defaulted) {
				t.Errorf("Defaulted = %+v, want %+v", a.Defaulted, c.defaulted)
			}
		})
	}
}
