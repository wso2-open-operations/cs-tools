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

import "testing"

func TestTransform_SeverityOutsideSevNPassesThrough(t *testing.T) {
	noSeverity := []byte(`{"data":{"essentials":{"alertId":"a1","monitorCondition":"Fired"}}}`)
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"built-in default", Config{}, "Critical"},
		{"operator override", Config{"severity": "Major"}, "Major"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := Transform(noSeverity, c.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if a.Severity != c.want {
				t.Errorf("severity = %q, want %q", a.Severity, c.want)
			}
		})
	}
}

func TestTransform_SevNStillMapped(t *testing.T) {
	a, err := Transform([]byte(`{"data":{"essentials":{"alertId":"a1","severity":"Sev2","monitorCondition":"Fired"}}}`), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Severity != "Minor" {
		t.Errorf("severity = %q, want Minor", a.Severity)
	}
}
