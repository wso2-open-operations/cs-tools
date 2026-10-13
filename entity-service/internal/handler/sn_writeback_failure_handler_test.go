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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubSNWritebackFailureService struct {
	list   func(ctx context.Context, entityType, entityID string, limit int) (domain.SNWritebackFailureListResponse, error)
	replay func(ctx context.Context, id string) (domain.SNWritebackReplayResponse, error)
}

func (s stubSNWritebackFailureService) ListSNWritebackFailures(ctx context.Context, entityType, entityID string, limit int) (domain.SNWritebackFailureListResponse, error) {
	return s.list(ctx, entityType, entityID, limit)
}

func (s stubSNWritebackFailureService) ReplaySNWritebackFailure(ctx context.Context, id string) (domain.SNWritebackReplayResponse, error) {
	return s.replay(ctx, id)
}

func TestSNWritebackFailureHandler_List(t *testing.T) {
	var gotType, gotID string
	var gotLimit int
	h := NewSNWritebackFailureHandler(stubSNWritebackFailureService{
		list: func(_ context.Context, entityType, entityID string, limit int) (domain.SNWritebackFailureListResponse, error) {
			gotType, gotID, gotLimit = entityType, entityID, limit
			return domain.SNWritebackFailureListResponse{Failures: []domain.SNWritebackFailure{{ID: "f", EntityType: entityType, EntityID: entityID, Operation: "state", Payload: json.RawMessage(`{"state":"scheduled"}`), Error: "refused"}}}, nil
		},
	})
	r := httptest.NewRequest(http.MethodGet, "/sn-writeback-failures?entityType=change_request&entityId=abc&limit=5", nil)
	w := httptest.NewRecorder()
	h.ListSNWritebackFailures(w, r)
	if w.Code != http.StatusOK || gotType != "change_request" || gotID != "abc" || gotLimit != 5 {
		t.Fatalf("status %d, filters %q/%q/%d", w.Code, gotType, gotID, gotLimit)
	}
	var resp domain.SNWritebackFailureListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Failures) != 1 || resp.Failures[0].Operation != "state" {
		t.Fatalf("body %s (%v)", w.Body.String(), err)
	}

	// A limit that is not a number is a 400 before the service is asked.
	w = httptest.NewRecorder()
	h.ListSNWritebackFailures(w, httptest.NewRequest(http.MethodGet, "/sn-writeback-failures?limit=lots", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("limit=lots: status %d", w.Code)
	}
}

func TestSNWritebackFailureHandler_Replay(t *testing.T) {
	h := NewSNWritebackFailureHandler(stubSNWritebackFailureService{
		replay: func(_ context.Context, id string) (domain.SNWritebackReplayResponse, error) {
			switch id {
			case "ok":
				return domain.SNWritebackReplayResponse{Message: "replayed", Failure: domain.SNWritebackFailure{ID: id}}, nil
			case "refused":
				return domain.SNWritebackReplayResponse{}, &apierror.ValidationError{Msg: "the previous system refused it again"}
			default:
				return domain.SNWritebackReplayResponse{}, &apierror.NotFoundError{Msg: "mirror write failure not found"}
			}
		},
	})
	for id, want := range map[string]int{"ok": http.StatusOK, "refused": http.StatusBadRequest, "gone": http.StatusNotFound} {
		r := httptest.NewRequest(http.MethodPost, "/sn-writeback-failures/"+id+"/replay", nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		h.ReplaySNWritebackFailure(w, r)
		if w.Code != want {
			t.Fatalf("replay %s: status %d, want %d: %s", id, w.Code, want, w.Body.String())
		}
		if id == "ok" {
			var resp domain.SNWritebackReplayResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Failure.ID != "ok" || resp.Message != "replayed" {
				t.Fatalf("body %s (%v)", w.Body.String(), err)
			}
		}
	}
}
