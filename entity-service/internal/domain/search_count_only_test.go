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

// Request bodies are decoded with DisallowUnknownFields, so a search that
// sends countOnly to a build that does not declare it is a 400. countOnly is
// cases-only (unlike skipTotal, which five search request types declare --
// see TestSearchRequestsDeclareSkipTotal), since entity-service only
// implements it for case/pie dashboard widgets' searches.
func TestSearchCasesRequestDeclaresCountOnly(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"countOnly":true,"pagination":{"limit":1}}`))
	dec.DisallowUnknownFields()
	req := &SearchCasesRequest{}
	if err := dec.Decode(req); err != nil {
		t.Fatalf("countOnly is not a known field: %v", err)
	}
	out, _ := json.Marshal(req)
	if !strings.Contains(string(out), `"countOnly":true`) {
		t.Errorf("countOnly did not round-trip: %s", out)
	}

	plain := &SearchCasesRequest{}
	if err := json.Unmarshal([]byte(`{"pagination":{"limit":1}}`), plain); err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(plain)
	if strings.Contains(string(out), "countOnly") {
		t.Errorf("a request that does not ask for it must not carry countOnly: %s", out)
	}
}
