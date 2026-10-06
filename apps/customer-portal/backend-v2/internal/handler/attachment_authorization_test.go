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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// GET /attachments/{id}/content, GET /attachments/{id}, and DELETE
// /attachments/{id} are not nested under any project/case path, so unlike
// every other route in this backend there is no trusted path segment to
// scope them — before this fix, any authenticated caller could read or
// delete any other customer's attachment just by knowing or guessing its
// UUID. These tests cover GetAttachment/GetAttachmentContent specifically
// (DeleteAttachment's own equivalent coverage lives in
// closed_case_attachments_test.go, TestDeleteAttachment_ClosedCase).

// fakeAttachmentAuthzClient serves GetAttachment/GetAttachmentContent/GetCase/
// SearchDeployments from canned values and records whether the guarded
// upstream call (content fetch, or the metadata response itself) was reached.
type fakeAttachmentAuthzClient struct {
	entityAttachmentClient
	referenceID       string
	referenceType     *entity.ReferenceType
	getAttachmentErr  error
	getCaseErr        error
	searchDeploysErr  error
	deploymentVisible bool
	contentReached    bool
}

func (f *fakeAttachmentAuthzClient) GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error) {
	if f.getAttachmentErr != nil {
		return entity.AttachmentDetails{}, f.getAttachmentErr
	}
	return entity.AttachmentDetails{ID: id, ReferenceID: f.referenceID, ReferenceType: f.referenceType, Name: "logs.txt"}, nil
}

func (f *fakeAttachmentAuthzClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: "open"}, nil
}

func (f *fakeAttachmentAuthzClient) SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	if f.searchDeploysErr != nil {
		return entity.SearchDeploymentsResponse{}, f.searchDeploysErr
	}
	if !f.deploymentVisible {
		return entity.SearchDeploymentsResponse{}, nil
	}
	return entity.SearchDeploymentsResponse{Deployments: []entity.DeploymentView{{ID: req.IDs[0]}}, Total: 1}, nil
}

func (f *fakeAttachmentAuthzClient) GetAttachmentContent(ctx context.Context, id string) ([]byte, string, error) {
	f.contentReached = true
	return []byte("file bytes"), "text/plain", nil
}

func attachmentAuthzTestCases() map[string]struct {
	client     fakeAttachmentAuthzClient
	wantStatus int
	wantAllow  bool
} {
	return map[string]struct {
		client     fakeAttachmentAuthzClient
		wantStatus int
		wantAllow  bool
	}{
		"caller can see the referenced case: allowed": {
			client:     fakeAttachmentAuthzClient{referenceID: testCaseID},
			wantStatus: http.StatusOK,
			wantAllow:  true,
		},
		"referenced case is outside the caller's scope: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testCaseID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
		},
		"no reference at all: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: ""},
			wantStatus: http.StatusNotFound,
		},
		"attachment lookup itself fails: denied": {
			client:     fakeAttachmentAuthzClient{getAttachmentErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
		},
		"deployment-referenced attachment visible to the caller: allowed": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, referenceType: refType(entity.ReferenceTypeDeployment), deploymentVisible: true},
			wantStatus: http.StatusOK,
			wantAllow:  true,
		},
		"deployment-referenced attachment outside the caller's scope: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, referenceType: refType(entity.ReferenceTypeDeployment), deploymentVisible: false},
			wantStatus: http.StatusNotFound,
		},
		"deployment lookup itself fails: denied": {
			client:     fakeAttachmentAuthzClient{referenceID: testDeploymentID, referenceType: refType(entity.ReferenceTypeDeployment), searchDeploysErr: &apierror.Error{StatusCode: http.StatusServiceUnavailable}},
			wantStatus: http.StatusServiceUnavailable,
		},
	}
}

func refType(t entity.ReferenceType) *entity.ReferenceType { return &t }

// TestGetAttachment_Authorization covers GET /attachments/{id}.
func TestGetAttachment_Authorization(t *testing.T) {
	for name, tc := range attachmentAuthzTestCases() {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}", h.GetAttachment)

			req := authedRequest(http.MethodGet, "/attachments/"+testAttachmentID, "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestGetAttachmentContent_Authorization covers GET /attachments/{id}/content
// — the raw-binary-download route, distinct from metadata above. The
// authorization check must run before the content fetch, not after: an
// unauthorized caller must never reach entity-service's own binary download
// at all.
func TestGetAttachmentContent_Authorization(t *testing.T) {
	for name, tc := range attachmentAuthzTestCases() {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("GET /attachments/{id}/content", h.GetAttachmentContent)

			req := authedRequest(http.MethodGet, "/attachments/"+testAttachmentID+"/content", "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if fake.contentReached != tc.wantAllow {
				t.Errorf("entity GetAttachmentContent reached = %v, want %v", fake.contentReached, tc.wantAllow)
			}
		})
	}
}
