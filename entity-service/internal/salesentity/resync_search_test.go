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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package salesentity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestSearchOpportunities_SendsOnlyTheFiltersSet checks the body of a list
// search: set filters are sent, empty ones are left out (an empty customerId
// or states would change what Sales Entity returns), and the response's
// lastModifiedDate decodes for keyset paging.
func TestSearchOpportunities_SendsOnlyTheFiltersSet(t *testing.T) {
	var body map[string]any
	c := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/opportunities/search", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "006000000000001AAA", "customerId": "001000000000001AAA", "lastModifiedDate": "2026-10-01T10:00:00.000+0000"}})
		})
	})
	rows, err := c.SearchOpportunities(context.Background(), SearchFilter{CustomerID: "001000000000001AAA", Limit: 50, MinLastModifiedDate: "1970-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || deref(rows[0].LastModifiedDate) != "2026-10-01T10:00:00.000+0000" {
		t.Fatalf("rows = %+v", rows)
	}
	want := map[string]any{"customerId": "001000000000001AAA", "limit": float64(50), "minLastModifiedDate": "1970-01-01T00:00:00Z"}
	if len(body) != len(want) {
		t.Fatalf("body = %v, want exactly %v", body, want)
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%s] = %v, want %v", k, body[k], v)
		}
	}
	for _, k := range []string{"states", "ids", "minId", "accountId", "projectId"} {
		if _, ok := body[k]; ok {
			t.Errorf("body carries %q although it was not set", k)
		}
	}
}
