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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

const testCaseID = "22222222-2222-2222-2222-222222222222"

func (f *fakeEntityCaseClient) SearchAttachments(ctx context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error) {
	f.gotSearchAttachmentsReq = req
	return entity.SearchAttachmentsResponse{Total: 3}, nil
}

func (f *fakeEntityCaseClient) GetCase(_ context.Context, id string) (entity.CaseView, error) {
	return entity.CaseView{ID: id, ProjectDetails: entity.EntityRef{ID: "proj-1"}}, nil
}

// TestSearchCaseAttachments_RouteAndQueryParams is an end-to-end check,
// through a real http.ServeMux registered with the exact pattern main.go
// uses ("GET /cases/{id}/attachments"), that the {id} path value and the
// limit/offset query params reach entity-service correctly, and that the
// response uses totalRecords — the frontend's
// apps/customer-portal/webapp/src/types/common.ts PaginationResponse field
// name, not entity-service's own "total".
func TestSearchCaseAttachments_RouteAndQueryParams(t *testing.T) {
	fake := &fakeEntityCaseClient{}
	h := NewCaseHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cases/{id}/attachments", h.SearchCaseAttachments)

	req := authedRequest(http.MethodGet, "/cases/"+testCaseID+"/attachments?limit=10&offset=5", "")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	want := entity.SearchAttachmentsRequest{
		ReferenceID:   testCaseID,
		ReferenceType: entity.ReferenceTypeCase,
		Pagination:    entity.Pagination{Limit: 10, Offset: 5},
	}
	if fake.gotSearchAttachmentsReq != want {
		t.Fatalf("entity-service SearchAttachments request = %+v, want %+v", fake.gotSearchAttachmentsReq, want)
	}

	var body struct {
		TotalRecords int `json:"totalRecords"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TotalRecords != 3 {
		t.Fatalf("totalRecords = %d, want 3 (response must use totalRecords, not total)", body.TotalRecords)
	}
}

// fakeEntityCallRequestClient records the requests it was called with.
// entityCallRequestClient is embedded (nil) so only the methods under test
// need implementing.
type fakeEntityCallRequestClient struct {
	entityCallRequestClient
	gotCreateReq entity.CreateCallRequestRequest
	gotSearchReq entity.SearchCallRequestsRequest
}

func (f *fakeEntityCallRequestClient) CreateCallRequest(ctx context.Context, req entity.CreateCallRequestRequest) (entity.CreateCallRequestResponse, error) {
	f.gotCreateReq = req
	return entity.CreateCallRequestResponse{}, nil
}

func (f *fakeEntityCallRequestClient) SearchCallRequests(ctx context.Context, req entity.SearchCallRequestsRequest) (entity.SearchCallRequestsResponse, error) {
	f.gotSearchReq = req
	return entity.SearchCallRequestsResponse{Total: 7}, nil
}

func (f *fakeEntityCallRequestClient) UpdateCallRequest(ctx context.Context, id string, req entity.UpdateCallRequestRequest) (entity.UpdateCallRequestResponse, error) {
	return entity.UpdateCallRequestResponse{CallRequest: entity.CallRequestUpdated{ID: id}}, nil
}

// GetCase is a stub for SearchCallRequests' caller-scope check (see
// caller_scope.go) — these tests never call SetCallerScope, so
// requireProjectMember treats the nil resolver as unscoped and never
// inspects this response.
func (f *fakeEntityCallRequestClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	return entity.CaseView{}, nil
}

// TestCreateCallRequest_CaseIDFromPath verifies POST /cases/{caseId}/call-requests
// forces CaseID from the path — the frontend's request body carries only
// reason/utcTimes/durationInMinutes, never a caseId field.
func TestCreateCallRequest_CaseIDFromPath(t *testing.T) {
	fake := &fakeEntityCallRequestClient{}
	h := NewCallRequestHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{caseId}/call-requests", h.CreateCallRequest)

	req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/call-requests",
		`{"reason":"discuss migration","utcTimes":["2026-08-10T10:00:00Z"],"durationInMinutes":30}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.gotCreateReq.CaseID != testCaseID {
		t.Fatalf("CaseID = %q, want %q", fake.gotCreateReq.CaseID, testCaseID)
	}
}

// TestSearchCallRequests_CaseIDFromPath verifies
// POST /cases/{caseId}/call-requests/search forces CaseID from the path
// rather than requiring (or trusting) it in the body, and that the response
// uses totalRecords for pagination.
func TestSearchCallRequests_CaseIDFromPath(t *testing.T) {
	fake := &fakeEntityCallRequestClient{}
	h := NewCallRequestHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cases/{caseId}/call-requests/search", h.SearchCallRequests)

	req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/call-requests/search",
		`{"pagination":{"limit":10,"offset":0}}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if fake.gotSearchReq.CaseID != testCaseID {
		t.Fatalf("CaseID = %q, want %q", fake.gotSearchReq.CaseID, testCaseID)
	}

	var body struct {
		TotalRecords int `json:"totalRecords"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TotalRecords != 7 {
		t.Fatalf("totalRecords = %d, want 7 (response must use totalRecords, not total)", body.TotalRecords)
	}
}

// TestPatchCallRequest_ExtraCaseIDPathSegmentDoesNotBreakRouting verifies
// PATCH /cases/{caseId}/call-requests/{id} still resolves {id} correctly now
// that the pattern has an extra leading {caseId} segment for RESTful
// nesting — caseId itself is never read by the handler, since
// entity-service's UpdateCallRequest is keyed on the call request's own id.
func TestPatchCallRequest_ExtraCaseIDPathSegmentDoesNotBreakRouting(t *testing.T) {
	fake := &fakeEntityCallRequestClient{}
	h := NewCallRequestHandler(fake)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /cases/{caseId}/call-requests/{id}", h.PatchCallRequest)

	callRequestID := "33333333-3333-3333-3333-333333333333"
	req := authedRequest(http.MethodPatch, "/cases/"+testCaseID+"/call-requests/"+callRequestID,
		`{"stateKey":6}`)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != callRequestID {
		t.Fatalf("id = %q, want %q (the {id} segment, not {caseId})", body.ID, callRequestID)
	}
}
