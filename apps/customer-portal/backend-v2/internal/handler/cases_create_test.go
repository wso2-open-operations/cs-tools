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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type mockCreateCaseClient struct {
	entityCaseClient
	project       entity.ProjectDetailsView
	getProjectErr error
	createdCase   entity.CreateCaseResponse
	createCaseErr error
	gotProjectID  string
}

func (m *mockCreateCaseClient) GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error) {
	m.gotProjectID = id
	if m.getProjectErr != nil {
		return entity.ProjectDetailsView{}, m.getProjectErr
	}
	return m.project, nil
}

func (m *mockCreateCaseClient) CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error) {
	if m.createCaseErr != nil {
		return entity.CreateCaseResponse{}, m.createCaseErr
	}
	return m.createdCase, nil
}

func TestCreateCase_SuspendedProject_Forbidden(t *testing.T) {
	suspended := "Suspended"
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID: "11111111-1111-1111-1111-111111111111",
			ProjectClosureFields: entity.ProjectClosureFields{
				ClosureState: &suspended,
			},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
}

// A lapsed contract used to be refused here, which stopped a customer raising a
// case the moment their contract expired. Per customer request an expired
// project creates cases exactly like an active one; only an explicit suspended
// closure state refuses (see TestCreateCase_SuspendedProject_Forbidden).
func TestCreateCase_ExpiredProject_Success(t *testing.T) {
	pastDate := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID:      "11111111-1111-1111-1111-111111111111",
			EndDate: pastDate,
		},
		createdCase: entity.CreateCaseResponse{
			Case: entity.CreateCaseDetails{
				ID:     "22222222-2222-2222-2222-222222222222",
				Number: "CS12345",
			},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCase_ActiveProject_Success(t *testing.T) {
	futureDate := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID:      "11111111-1111-1111-1111-111111111111",
			EndDate: futureDate,
		},
		createdCase: entity.CreateCaseResponse{
			Case: entity.CreateCaseDetails{
				ID:     "22222222-2222-2222-2222-222222222222",
				Number: "CS12345",
			},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCase_InvalidProjectID_BadRequest(t *testing.T) {
	mock := &mockCreateCaseClient{}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"not-a-valid-uuid","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

// A client still holding a bare sysid from before ids were dashed must not be
// rejected, and entity-service must receive the dashed form.
func TestCreateCase_BareSysIDProjectID_ForwardedDashed(t *testing.T) {
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID:      "11111111-1111-1111-1111-111111111111",
			EndDate: time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
		},
		createdCase: entity.CreateCaseResponse{
			Case: entity.CreateCaseDetails{ID: "22222222-2222-2222-2222-222222222222", Number: "CS12345"},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111111111111111111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	if want := "11111111-1111-1111-1111-111111111111"; mock.gotProjectID != want {
		t.Errorf("entity GetProject got project id %q, want %q", mock.gotProjectID, want)
	}
}

// createCaseJSONBody builds a valid CreateCaseRequest body of exactly
// totalBytes whose single inline attachment carries the padding as its file.
func createCaseJSONBody(totalBytes int) string {
	const prefix = `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details","attachments":[{"name":"logs.txt","file":"`
	const suffix = `"}]}`
	padLen := totalBytes - len(prefix) - len(suffix)
	if padLen < 0 {
		padLen = 0
	}
	return prefix + strings.Repeat("A", padLen) + suffix
}

// TestCreateCase_BodySizeLimit covers POST /cases with inline base64
// attachments: a ~6 MB body (a 5 MB file) must not be rejected as too large,
// while a body over 15 MiB must be rejected with 413.
func TestCreateCase_BodySizeLimit(t *testing.T) {
	tests := map[string]struct {
		bodySize   int
		wantStatus int
	}{
		"just over the old 1 MiB cap succeeds": {
			bodySize:   (1 << 20) + 1024,
			wantStatus: http.StatusCreated,
		},
		"6 MB body with an inline attachment succeeds": {
			bodySize:   6 * 1000 * 1000,
			wantStatus: http.StatusCreated,
		},
		"over the 15 MiB cap is rejected": {
			bodySize:   (15 << 20) + 1024,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mock := &mockCreateCaseClient{
				project: entity.ProjectDetailsView{
					ID:      "11111111-1111-1111-1111-111111111111",
					EndDate: time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
				},
				createdCase: entity.CreateCaseResponse{
					Case: entity.CreateCaseDetails{
						ID:     "22222222-2222-2222-2222-222222222222",
						Number: "CS12345",
					},
				},
			}
			h := NewCaseHandler(mock)

			req := authedRequest(http.MethodPost, "/cases", createCaseJSONBody(tc.bodySize))
			rec := httptest.NewRecorder()

			h.CreateCase(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusRequestEntityTooLarge {
				if msg := decodeMessage(t, rec); msg != ErrMsgTooLarge {
					t.Errorf("message = %q, want %q", msg, ErrMsgTooLarge)
				}
				if mock.gotProjectID != "" {
					t.Error("entity-service GetProject was called for an oversized body")
				}
			}
		})
	}
}
