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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestVerifyCallRequestBelongsToCase_ShortPages proves the ownership scan
// advances by the rows the upstream actually returned: with 60 records served
// 30 at a time (a short page for a limit of 50), the record at index 45 is
// found rather than skipped into a false "not found".
func TestVerifyCallRequestBelongsToCase_ShortPages(t *testing.T) {
	const total = 60
	ids := make([]string, total)
	for i := range ids {
		ids[i] = fmt.Sprintf("%032x", i+1)
	}
	var offsets []int
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req snCallRequestSearchPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		offsets = append(offsets, req.Pagination.Offset)
		start := req.Pagination.Offset
		end := start + 30
		if end > total {
			end = total
		}
		rows := make([]snCallRequest, 0, 30)
		for _, id := range ids[start:end] {
			rows = append(rows, snCallRequest{ID: id})
		}
		_ = json.NewEncoder(w).Encode(snCallRequestsResponse{CallRequests: rows, TotalRecords: total})
	}))
	svc := &snCallRequestService{client: client}

	if err := svc.verifyCallRequestBelongsToCase(context.Background(), "", "case", ids[45]); err != nil {
		t.Fatalf("record 45 not found (offsets requested %v): %v", offsets, err)
	}
	if len(offsets) != 2 || offsets[1] != 30 {
		t.Fatalf("offsets requested = %v, want [0 30]", offsets)
	}
}
