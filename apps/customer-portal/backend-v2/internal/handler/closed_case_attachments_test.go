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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// A closed case's attachments are read-only. entity-service still accepts the
// write and the webapp only disables the controls, so these tests cover the
// direct-API path this backend is responsible for closing — including that a
// rejected request never reaches entity-service at all.
//
// CreateCaseAttachment/PatchCaseAttachment are nested under an
// already-authorized /cases/{caseId}/... path, so their own closed-case
// guard (caseIsClosed) deliberately fails OPEN on a lookup error — a failed
// business-rule check must not block an otherwise-legitimate write, matching
// the Ballerina backend's own guard.
//
// DeleteAttachment is different: it is not nested under any case/project
// path at all, so the same GetCase lookup that recovers the case also IS the
// authorization check (see authorizeAttachmentAccess in attachments.go) and
// must fail CLOSED — any way that lookup comes up short (the attachment
// lookup failing, no referenceId, or a referenceId entity-service can't
// resolve to a case the caller may see) denies the delete rather than
// letting it through.

const testAttachmentID = "33333333-3333-3333-3333-333333333333"

// fakeClosedCaseClient serves GetCase from a canned state and records whether
// the attachment write it guards was reached. entityCaseClient is embedded
// (nil) so only the methods under test need implementing.
type fakeClosedCaseClient struct {
	entityCaseClient
	caseState        string
	getCaseErr       error
	createAttachment bool
	updateAttachment bool
}

func (f *fakeClosedCaseClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: f.caseState}, nil
}

func (f *fakeClosedCaseClient) CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error) {
	f.createAttachment = true
	return entity.CreateAttachmentResponse{}, nil
}

func (f *fakeClosedCaseClient) UpdateAttachment(ctx context.Context, id string, req entity.UpdateAttachmentRequest) (entity.UpdateAttachmentResponse, error) {
	f.updateAttachment = true
	return entity.UpdateAttachmentResponse{}, nil
}

func decodeMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Message
}

// TestCreateCaseAttachment_ClosedCase covers POST /cases/{id}/attachments for
// each representation entity-service's CaseView.State arrives as, plus the
// states that must still be allowed through.
func TestCreateCaseAttachment_ClosedCase(t *testing.T) {
	tests := map[string]struct {
		caseState  string
		getCaseErr error
		wantStatus int
	}{
		"closed as domain enum":       {caseState: "closed", wantStatus: http.StatusBadRequest},
		"closed as ServiceNow label":  {caseState: "Closed", wantStatus: http.StatusBadRequest},
		"open case":                   {caseState: "open", wantStatus: http.StatusCreated},
		"work in progress":            {caseState: "work_in_progress", wantStatus: http.StatusCreated},
		"empty state":                 {caseState: "", wantStatus: http.StatusCreated},
		"guard lookup fails, allowed": {getCaseErr: errors.New("upstream down"), wantStatus: http.StatusCreated},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeClosedCaseClient{caseState: tc.caseState, getCaseErr: tc.getCaseErr}
			h := NewCaseHandler(fake)

			mux := http.NewServeMux()
			mux.HandleFunc("POST /cases/{id}/attachments", h.CreateCaseAttachment)

			req := authedRequest(http.MethodPost, "/cases/"+testCaseID+"/attachments",
				`{"name":"logs.txt","type":"text/plain","content":"aGVsbG8="}`)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest {
				if msg := decodeMessage(t, rec); msg != ErrMsgCaseClosedForAttachmentCreate {
					t.Errorf("message = %q, want %q", msg, ErrMsgCaseClosedForAttachmentCreate)
				}
				if fake.createAttachment {
					t.Error("entity-service CreateAttachment was called for a closed case")
				}
				return
			}
			if !fake.createAttachment {
				t.Error("entity-service CreateAttachment was not called")
			}
		})
	}
}

// TestPatchCaseAttachment_ClosedCase covers
// PATCH /cases/{caseId}/attachments/{attachmentId}.
func TestPatchCaseAttachment_ClosedCase(t *testing.T) {
	tests := map[string]struct {
		caseState  string
		getCaseErr error
		wantStatus int
	}{
		"closed as domain enum":       {caseState: "closed", wantStatus: http.StatusBadRequest},
		"closed as ServiceNow label":  {caseState: "Closed", wantStatus: http.StatusBadRequest},
		"open case":                   {caseState: "open", wantStatus: http.StatusOK},
		"guard lookup fails, allowed": {getCaseErr: errors.New("upstream down"), wantStatus: http.StatusOK},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeClosedCaseClient{caseState: tc.caseState, getCaseErr: tc.getCaseErr}
			h := NewCaseHandler(fake)

			mux := http.NewServeMux()
			mux.HandleFunc("PATCH /cases/{caseId}/attachments/{attachmentId}", h.PatchCaseAttachment)

			req := authedRequest(http.MethodPatch,
				"/cases/"+testCaseID+"/attachments/"+testAttachmentID, `{"name":"renamed.txt"}`)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest {
				if msg := decodeMessage(t, rec); msg != ErrMsgCaseClosedForAttachmentUpdate {
					t.Errorf("message = %q, want %q", msg, ErrMsgCaseClosedForAttachmentUpdate)
				}
				if fake.updateAttachment {
					t.Error("entity-service UpdateAttachment was called for a closed case")
				}
				return
			}
			if !fake.updateAttachment {
				t.Error("entity-service UpdateAttachment was not called")
			}
		})
	}
}

// fakeClosedCaseAttachmentClient serves the two lookups DELETE /attachments/{id}'s
// guard needs — the attachment (for its referenceId) and the case that
// referenceId names — and records whether the delete was reached.
type fakeClosedCaseAttachmentClient struct {
	entityAttachmentClient
	referenceID       string
	getAttachmentErr  error
	caseState         string
	getCaseErr        error
	deploymentVisible bool
	deleted           bool
}

func (f *fakeClosedCaseAttachmentClient) GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error) {
	if f.getAttachmentErr != nil {
		return entity.AttachmentDetails{}, f.getAttachmentErr
	}
	return entity.AttachmentDetails{ID: id, ReferenceID: f.referenceID}, nil
}

func (f *fakeClosedCaseAttachmentClient) GetCase(ctx context.Context, id string) (entity.CaseView, error) {
	if f.getCaseErr != nil {
		return entity.CaseView{}, f.getCaseErr
	}
	return entity.CaseView{ID: id, State: f.caseState}, nil
}

func (f *fakeClosedCaseAttachmentClient) SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error) {
	if !f.deploymentVisible {
		return entity.SearchDeploymentsResponse{}, nil
	}
	return entity.SearchDeploymentsResponse{Deployments: []entity.DeploymentView{{ID: req.IDs[0]}}, Total: 1}, nil
}

func (f *fakeClosedCaseAttachmentClient) DeleteAttachment(ctx context.Context, id string) (entity.DeleteAttachmentResponse, error) {
	f.deleted = true
	return entity.DeleteAttachmentResponse{Message: "deleted"}, nil
}

// TestDeleteAttachment_ClosedCase covers DELETE /attachments/{id}, which is not
// nested under a case: the case has to be recovered from the attachment's own
// referenceId first, and that same recovery doubles as the authorization
// check (authorizeAttachmentAccess) — so every way it can come up short now
// denies the delete outright (404-shaped) rather than letting it through.
func TestDeleteAttachment_ClosedCase(t *testing.T) {
	tests := map[string]struct {
		client     fakeClosedCaseAttachmentClient
		wantStatus int
		wantDenied bool // true: access denied outright, no closed-case message expected
	}{
		"closed case": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testCaseID, caseState: "closed"},
			wantStatus: http.StatusBadRequest,
		},
		"closed case, ServiceNow label": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testCaseID, caseState: "Closed"},
			wantStatus: http.StatusBadRequest,
		},
		"open case": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testCaseID, caseState: "open"},
			wantStatus: http.StatusOK,
		},
		"reference is not a case the caller can see, denied": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testCaseID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
			wantDenied: true,
		},
		"attachment lookup fails, denied": {
			client:     fakeClosedCaseAttachmentClient{getAttachmentErr: &apierror.Error{StatusCode: http.StatusNotFound}},
			wantStatus: http.StatusNotFound,
			wantDenied: true,
		},
		"attachment has no reference, denied": {
			client:     fakeClosedCaseAttachmentClient{referenceID: "", caseState: "closed"},
			wantStatus: http.StatusNotFound,
			wantDenied: true,
		},
		// The reported bug: a deployment-tab ("Deployed" tab) attachment's own
		// uploader could never delete it, because authorizeAttachmentAccess
		// always ran GetCase against the attachment's referenceId — which is a
		// deployment id here, so GetCase could never resolve it and the delete
		// 404'd unconditionally regardless of who uploaded it. getCaseErr is
		// a 404 here because a deployment id is never a real case -- that's
		// the actual, live response GetCase returns for one.
		"deployment-referenced attachment visible to the caller: delete succeeds": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, deploymentVisible: true},
			wantStatus: http.StatusOK,
		},
		"deployment-referenced attachment outside the caller's scope: denied": {
			client:     fakeClosedCaseAttachmentClient{referenceID: testDeploymentID, getCaseErr: &apierror.Error{StatusCode: http.StatusNotFound}, deploymentVisible: false},
			wantStatus: http.StatusNotFound,
			wantDenied: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := tc.client
			h := NewAttachmentHandler(&fake)

			mux := http.NewServeMux()
			mux.HandleFunc("DELETE /attachments/{id}", h.DeleteAttachment)

			req := authedRequest(http.MethodDelete, "/attachments/"+testAttachmentID, "")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			switch {
			case tc.wantDenied:
				if fake.deleted {
					t.Error("entity-service DeleteAttachment was called for a denied request")
				}
			case tc.wantStatus == http.StatusBadRequest:
				if msg := decodeMessage(t, rec); msg != ErrMsgCaseClosedForAttachmentDelete {
					t.Errorf("message = %q, want %q", msg, ErrMsgCaseClosedForAttachmentDelete)
				}
				if fake.deleted {
					t.Error("entity-service DeleteAttachment was called for a closed case's attachment")
				}
			default:
				if !fake.deleted {
					t.Error("entity-service DeleteAttachment was not called")
				}
			}
		})
	}
}
