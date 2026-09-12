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

package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// An explicit 0 upper bound must survive marshaling. With plain int fields the
// omitempty tag stripped it, ServiceNow saw no upper bound at all, and the
// result set silently widened -- the exact opposite of what "lte 0" asks for.
func TestBuildSNTaskSLAFilter_ExplicitZeroBoundIsTransmitted(t *testing.T) {
	zero := 0
	ninety := 90

	t.Run("explicit zero max is present in the payload", func(t *testing.T) {
		sn := buildSNTaskSLAFilter(&domain.TaskSLAFilter{MaxBusinessElapsedPercent: &zero})
		raw, err := json.Marshal(sn)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := string(raw)
		want := `{"maxBusinessElapsedPercent":0}`
		if got != want {
			t.Fatalf("payload = %s, want %s", got, want)
		}
	})

	t.Run("explicit zero min is present in the payload", func(t *testing.T) {
		sn := buildSNTaskSLAFilter(&domain.TaskSLAFilter{MinBusinessElapsedPercent: &zero})
		raw, err := json.Marshal(sn)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if got, want := string(raw), `{"minBusinessElapsedPercent":0}`; got != want {
			t.Fatalf("payload = %s, want %s", got, want)
		}
	})

	t.Run("absent bounds are omitted", func(t *testing.T) {
		sn := buildSNTaskSLAFilter(&domain.TaskSLAFilter{MinBusinessElapsedPercent: &ninety})
		raw, err := json.Marshal(sn)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if got, want := string(raw), `{"minBusinessElapsedPercent":90}`; got != want {
			t.Fatalf("payload = %s, want %s", got, want)
		}
	})

	t.Run("nil filter stays nil", func(t *testing.T) {
		if sn := buildSNTaskSLAFilter(nil); sn != nil {
			t.Fatalf("buildSNTaskSLAFilter(nil) = %v, want nil", sn)
		}
	})

	t.Run("bounds are copied, not aliased", func(t *testing.T) {
		v := 42
		sn := buildSNTaskSLAFilter(&domain.TaskSLAFilter{MinBusinessElapsedPercent: &v})
		v = 7
		if *sn.MinBusinessElapsedPercent != 42 {
			t.Fatalf("MinBusinessElapsedPercent = %d, want 42 (payload must not alias the caller's value)", *sn.MinBusinessElapsedPercent)
		}
	})
}

// groupBy fans out one limit=1 case search per enumerated bucket. The buckets
// must run concurrently, and the returned groups must stay in the fixed
// caseGroupByFieldValues display order no matter which bucket answers first.
func TestSNCaseService_SearchCases_GroupBy(t *testing.T) {
	t.Run("returns buckets in enum order and totals them", func(t *testing.T) {
		// Reply with a distinct total per state so an out-of-order write shows up.
		countByState := map[string]int{
			"open":              5,
			"work_in_progress":  4,
			"waiting_on_wso2":   3,
			"awaiting_info":     2,
			"reopened":          1,
			"solution_proposed": 6,
			"closed":            7,
		}
		var inFlight, maxInFlight int32

		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				prev := atomic.LoadInt32(&maxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
					break
				}
			}
			// Hold each bucket briefly so serial execution cannot overlap and the
			// concurrency assertion below is meaningful.
			time.Sleep(20 * time.Millisecond)
			defer atomic.AddInt32(&inFlight, -1)

			var payload struct {
				Filters struct {
					StateKeys []int `json:"stateKeys"`
				} `json:"filters"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(payload.Filters.StateKeys) != 1 {
				http.Error(w, "expected exactly one stateKey per bucket call", http.StatusBadRequest)
				return
			}
			// Reverse-map the SN state id back to the domain state.
			// snStateIDMap is read-only package state; no synchronisation needed.
			total := 0
			for state, id := range snStateIDMap {
				if id == payload.Filters.StateKeys[0] {
					total = countByState[string(state)]
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cases":        []map[string]any{},
				"totalRecords": total,
				"limit":        1,
				"offset":       0,
			})
		})

		svc := NewServiceNowCaseService(newTestSNClient(t, h), nil, nil, noopSLAClockService{}, nil, "", nil)
		resp, err := svc.SearchCases(contextWithUserIDToken("token"), domain.SearchCasesRequest{
			GroupBy:    "state",
			Pagination: domain.Pagination{Limit: 20, Offset: 0},
		})
		if err != nil {
			t.Fatalf("SearchCases: %v", err)
		}

		want := caseGroupByFieldValues["state"]
		if len(resp.Groups) != len(want) {
			t.Fatalf("len(Groups) = %d, want %d", len(resp.Groups), len(want))
		}
		wantTotal := 0
		for i, key := range want {
			if resp.Groups[i].Key != key {
				t.Errorf("Groups[%d].Key = %q, want %q (enum display order must be preserved)", i, resp.Groups[i].Key, key)
			}
			if resp.Groups[i].Count != countByState[key] {
				t.Errorf("Groups[%d] (%s).Count = %d, want %d", i, key, resp.Groups[i].Count, countByState[key])
			}
			wantTotal += countByState[key]
		}
		if resp.Total != wantTotal {
			t.Errorf("Total = %d, want %d (sum of the enumerated buckets)", resp.Total, wantTotal)
		}
		if resp.Cases != nil {
			t.Errorf("Cases = %v, want nil for a grouped response", resp.Cases)
		}
		if got := atomic.LoadInt32(&maxInFlight); got < 2 {
			t.Errorf("max concurrent bucket calls = %d, want >= 2 (buckets must not run serially)", got)
		}
	})

	t.Run("one bucket failure fails the whole search", func(t *testing.T) {
		var calls int32
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"cases": []map[string]any{}, "totalRecords": 1})
		})

		svc := NewServiceNowCaseService(newTestSNClient(t, h), nil, nil, noopSLAClockService{}, nil, "", nil)
		_, err := svc.SearchCases(contextWithUserIDToken("token"), domain.SearchCasesRequest{
			GroupBy:    "state",
			Pagination: domain.Pagination{Limit: 20, Offset: 0},
		})
		if err == nil {
			t.Fatal("expected an error when a bucket call fails, got nil")
		}
	})

	t.Run("rejects an ungroupable field", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			t.Error("no upstream call expected for an invalid groupBy")
			w.WriteHeader(http.StatusInternalServerError)
		})
		svc := NewServiceNowCaseService(newTestSNClient(t, h), nil, nil, noopSLAClockService{}, nil, "", nil)
		_, err := svc.SearchCases(contextWithUserIDToken("token"), domain.SearchCasesRequest{
			GroupBy:    "tag",
			Pagination: domain.Pagination{Limit: 20, Offset: 0},
		})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
		}
	})
}
