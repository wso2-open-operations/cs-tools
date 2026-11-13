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

package model

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestAlert_JSONKeysMatchAlertsCore guards the contract: alerts-core parses exactly these keys.
func TestAlert_JSONKeysMatchAlertsCore(t *testing.T) {
	raw, err := json.Marshal(Alert{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "category,description,environment,metric_name,service,severity,source,unique_identifier"
	if got := strings.Join(keys, ","); got != want {
		t.Errorf("keys = %s, want %s", got, want)
	}
}
