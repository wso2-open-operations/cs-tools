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
package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// excludeFields is additive: a request without it decodes and re-encodes with
// no trace of it, and one with it round-trips. Bodies are decoded with
// DisallowUnknownFields, so the field must be declared to be accepted at all.
func TestSearchCasesRequestExcludeFields(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"excludeFields":["description"],"pagination":{"limit":1}}`))
	dec.DisallowUnknownFields()
	req := &SearchCasesRequest{}
	if err := dec.Decode(req); err != nil {
		t.Fatalf("excludeFields is not a known field: %v", err)
	}
	if !req.ExcludesField(SearchExcludeFieldDescription) {
		t.Errorf("description should be excluded, got %v", req.ExcludeFields)
	}
	if req.ExcludesField("subject") {
		t.Errorf("subject was not excluded")
	}
	out, _ := json.Marshal(req)
	if !strings.Contains(string(out), `"excludeFields":["description"]`) {
		t.Errorf("excludeFields did not round-trip: %s", out)
	}

	plain := &SearchCasesRequest{}
	if err := json.Unmarshal([]byte(`{"pagination":{"limit":1}}`), plain); err != nil {
		t.Fatal(err)
	}
	if plain.ExcludesField(SearchExcludeFieldDescription) {
		t.Errorf("a request that does not ask for it must not exclude description")
	}
	out, _ = json.Marshal(plain)
	if strings.Contains(string(out), "excludeFields") {
		t.Errorf("a request that does not ask for it must not carry excludeFields: %s", out)
	}
}
