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

import "testing"

// TestTransform_ConflictingTagAliasesAreStable: with two aliases for one field, the first in tagAliases always wins,
// so repeat deliveries keep one fingerprint.
func TestTransform_ConflictingTagAliasesAreStable(t *testing.T) {
	raw := []byte(`{"event_name":"high memory","transition":"Triggered","alert_id":"148502937",
		"tags":"environment:staging,env:production,svc:b,service:a"}`)
	for i := 0; i < 50; i++ {
		a, err := Transform(raw, Config{})
		if err != nil {
			t.Fatal(err)
		}
		if a.Environment != "production" || a.Service != "a" {
			t.Fatalf("run %d: environment = %q, service = %q; want production, a", i, a.Environment, a.Service)
		}
	}
}
