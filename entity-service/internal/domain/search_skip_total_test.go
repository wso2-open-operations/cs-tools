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

// Request bodies are decoded with DisallowUnknownFields, so a search that sends
// skipTotal to a build that does not declare it is a 400. This pins that every
// search request that honours it declares it, and that it defaults to false.
func TestSearchRequestsDeclareSkipTotal(t *testing.T) {
	requests := map[string]func() any{
		"cases":           func() any { return &SearchCasesRequest{} },
		"incidents":       func() any { return &SearchIncidentsRequest{} },
		"change requests": func() any { return &SearchChangeRequestsRequest{} },
		"problems":        func() any { return &SearchProblemsRequest{} },
		"conversations":   func() any { return &SearchConversationsRequest{} },
	}
	for name, newReq := range requests {
		t.Run(name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(`{"skipTotal":true,"pagination":{"limit":5}}`))
			dec.DisallowUnknownFields()
			req := newReq()
			if err := dec.Decode(req); err != nil {
				t.Fatalf("skipTotal is not a known field: %v", err)
			}
			out, _ := json.Marshal(req)
			if !strings.Contains(string(out), `"skipTotal":true`) {
				t.Errorf("skipTotal did not round-trip: %s", out)
			}

			plain := newReq()
			if err := json.Unmarshal([]byte(`{"pagination":{"limit":5}}`), plain); err != nil {
				t.Fatal(err)
			}
			out, _ = json.Marshal(plain)
			if strings.Contains(string(out), "skipTotal") {
				t.Errorf("a request that does not ask for it must not carry skipTotal: %s", out)
			}
		})
	}
}
