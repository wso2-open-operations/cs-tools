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

package site24x7

import (
	"errors"
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
)

func TestTransform_RejectsNullBody(t *testing.T) {
	_, err := Transform([]byte("null"))
	if !errors.Is(err, ErrMissingBody) {
		t.Fatalf("Transform(null) error = %v, want ErrMissingBody", err)
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
			name: "missing fields", raw: `{"MONITOR_ID":"1"}`,
			severity: "Critical", category: "Service Interruption", environment: "Production",
			defaulted: []model.Defaulted{
				{Field: "severity", Value: "Critical"},
				{Field: "category", Value: "Service Interruption"},
				{Field: "environment", Value: "Production"},
			},
		},
		{
			name: "unrecognised status", raw: `{"STATUS":"MAINTENANCE","MONITORNAME":"api","MONITOR_ID":"1","TAGS":["Service:api","Environment:Staging","Category:Performance"]}`,
			service: "api", metricName: "api", severity: "Critical", category: "Performance", environment: "Staging",
			defaulted: []model.Defaulted{{Field: "severity", Value: "Critical"}},
		},
		{
			name: "every field", raw: `{"STATUS":"TROUBLE","MONITORNAME":"api","MONITOR_ID":"1","TAGS":["Service:api","Environment:Staging","Category:Performance"]}`,
			service: "api", metricName: "api", severity: "Minor", category: "Performance", environment: "Staging",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := Transform([]byte(c.raw))
			if err != nil {
				t.Fatal(err)
			}
			if a.Source != "Site24x7" || a.Service != c.service || a.MetricName != c.metricName || a.Severity != c.severity || a.Category != c.category || a.Environment != c.environment {
				t.Errorf("alert = %+v, want service %q, metric name %q, severity %q, category %q, environment %q", a, c.service, c.metricName, c.severity, c.category, c.environment)
			}
			if !reflect.DeepEqual(a.Defaulted, c.defaulted) {
				t.Errorf("Defaulted = %+v, want %+v", a.Defaulted, c.defaulted)
			}
		})
	}
}
