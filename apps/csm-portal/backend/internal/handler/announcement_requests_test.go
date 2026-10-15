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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// mockEntityAnnouncementRequestClient captures the last body it was called
// with per method, and returns whatever the corresponding xxxFn is set to
// (or {} on success) — same shape as every other mock in this package.
type mockEntityAnnouncementRequestClient struct {
	searchProjectsFn          func(ctx context.Context, body []byte) ([]byte, error)
	searchProjectsByVersionFn func(ctx context.Context, body []byte) ([]byte, error)
	createFn                  func(ctx context.Context, body []byte) ([]byte, error)
	getFn                     func(ctx context.Context, id string) ([]byte, error)
	searchFn                  func(ctx context.Context, body []byte) ([]byte, error)
	updateFn                  func(ctx context.Context, id string, body []byte) ([]byte, error)
	recordDryRunFn            func(ctx context.Context, id string, body []byte) ([]byte, error)
	submitFn                  func(ctx context.Context, id string, body []byte) ([]byte, error)
	approveFn                 func(ctx context.Context, id string, body []byte) ([]byte, error)
	scheduleFn                func(ctx context.Context, id string, body []byte) ([]byte, error)
	publishFn                 func(ctx context.Context, id string, body []byte) ([]byte, error)
	createUpdateFn            func(ctx context.Context, id string, body []byte) ([]byte, error)
	listUpdatesFn             func(ctx context.Context, id string) ([]byte, error)
	recordDeliveriesFn        func(ctx context.Context, id string, body []byte) ([]byte, error)
	listDeliveriesFn          func(ctx context.Context, id string) ([]byte, error)
	createAttachmentFn        func(ctx context.Context, id string, body []byte) ([]byte, error)
	listAttachmentsFn         func(ctx context.Context, id string) ([]byte, error)
	attachmentContentFn       func(ctx context.Context, id, attachmentID string) ([]byte, string, error)
	deleteAttachmentFn        func(ctx context.Context, id, attachmentID, actorID string) ([]byte, error)
	attachToCaseFn            func(ctx context.Context, id, caseID string, body []byte) ([]byte, error)

	gotCreateAttachmentBody      []byte
	gotAttachToCaseBody          []byte
	gotAttachToCaseCaseID        string
	gotDeleteAttachmentActorID   string
	gotApproveBody               []byte
	gotScheduleBody              []byte
	gotPublishBody               []byte
	gotSubmitBody                []byte
	gotCreateUpdateBody          []byte
	gotRecordDeliveriesBody      []byte
	searchProjectsCalls          int
	searchProjectsByVersionCalls int
}

func (m *mockEntityAnnouncementRequestClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	m.searchProjectsCalls++
	if m.searchProjectsFn != nil {
		return m.searchProjectsFn(ctx, body)
	}
	return []byte(`{"projects":[],"total":0,"hasMore":false}`), nil
}

func (m *mockEntityAnnouncementRequestClient) SearchProjectsByProductVersion(ctx context.Context, body []byte) ([]byte, error) {
	m.searchProjectsByVersionCalls++
	if m.searchProjectsByVersionFn != nil {
		return m.searchProjectsByVersionFn(ctx, body)
	}
	return []byte(`{"projects":[],"total":0,"hasMore":false}`), nil
}

func (m *mockEntityAnnouncementRequestClient) CreateAnnouncementRequest(ctx context.Context, body []byte) ([]byte, error) {
	if m.createFn != nil {
		return m.createFn(ctx, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) GetAnnouncementRequest(ctx context.Context, id string) ([]byte, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return []byte(`{"id":"` + id + `"}`), nil
}

func (m *mockEntityAnnouncementRequestClient) SearchAnnouncementRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, body)
	}
	return []byte(`{"requests":[],"total":0}`), nil
}

func (m *mockEntityAnnouncementRequestClient) UpdateAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) RecordAnnouncementRequestDryRun(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.recordDryRunFn != nil {
		return m.recordDryRunFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) SubmitAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotSubmitBody = body
	if m.submitFn != nil {
		return m.submitFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) ApproveAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotApproveBody = body
	if m.approveFn != nil {
		return m.approveFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) ScheduleAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotScheduleBody = body
	if m.scheduleFn != nil {
		return m.scheduleFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) PublishAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotPublishBody = body
	if m.publishFn != nil {
		return m.publishFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) CreateAnnouncementRequestUpdate(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotCreateUpdateBody = body
	if m.createUpdateFn != nil {
		return m.createUpdateFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) ListAnnouncementRequestUpdates(ctx context.Context, id string) ([]byte, error) {
	if m.listUpdatesFn != nil {
		return m.listUpdatesFn(ctx, id)
	}
	return []byte(`{"updates":[]}`), nil
}

func (m *mockEntityAnnouncementRequestClient) RecordAnnouncementRequestDeliveries(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotRecordDeliveriesBody = body
	if m.recordDeliveriesFn != nil {
		return m.recordDeliveriesFn(ctx, id, body)
	}
	return body, nil
}

func (m *mockEntityAnnouncementRequestClient) ListAnnouncementRequestDeliveries(ctx context.Context, id string) ([]byte, error) {
	if m.listDeliveriesFn != nil {
		return m.listDeliveriesFn(ctx, id)
	}
	return []byte(`{"deliveries":[]}`), nil
}

func (m *mockEntityAnnouncementRequestClient) CreateAnnouncementRequestAttachment(ctx context.Context, id string, body []byte) ([]byte, error) {
	m.gotCreateAttachmentBody = body
	if m.createAttachmentFn != nil {
		return m.createAttachmentFn(ctx, id, body)
	}
	return []byte(`{"id":"att-1"}`), nil
}

func (m *mockEntityAnnouncementRequestClient) ListAnnouncementRequestAttachments(ctx context.Context, id string) ([]byte, error) {
	if m.listAttachmentsFn != nil {
		return m.listAttachmentsFn(ctx, id)
	}
	return []byte(`{"attachments":[]}`), nil
}

func (m *mockEntityAnnouncementRequestClient) GetAnnouncementRequestAttachmentContent(ctx context.Context, id, attachmentID string) ([]byte, string, error) {
	if m.attachmentContentFn != nil {
		return m.attachmentContentFn(ctx, id, attachmentID)
	}
	return []byte("%PDF-"), "application/pdf", nil
}

func (m *mockEntityAnnouncementRequestClient) DeleteAnnouncementRequestAttachment(ctx context.Context, id, attachmentID, actorID string) ([]byte, error) {
	m.gotDeleteAttachmentActorID = actorID
	if m.deleteAttachmentFn != nil {
		return m.deleteAttachmentFn(ctx, id, attachmentID, actorID)
	}
	return nil, nil
}

func (m *mockEntityAnnouncementRequestClient) AttachAnnouncementRequestAttachmentsToCase(ctx context.Context, id, caseID string, body []byte) ([]byte, error) {
	m.gotAttachToCaseBody = body
	m.gotAttachToCaseCaseID = caseID
	if m.attachToCaseFn != nil {
		return m.attachToCaseFn(ctx, id, caseID, body)
	}
	return []byte(`{"attached":1,"alreadyAttached":0}`), nil
}

const testAnnouncementRequestID = "11111111-1111-1111-1111-111111111111"
const testAnnouncementAttachmentID = "22222222-2222-2222-2222-222222222222"
const testAnnouncementCaseID = "33333333-3333-3333-3333-333333333333"

// ----- auth required -----

func TestAnnouncementRequestHandler_RequiresAuth(t *testing.T) {
	h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)

	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		handler http.HandlerFunc
	}{
		{"create", http.MethodPost, "/announcement-requests", `{"kind":"customer"}`, h.CreateAnnouncementRequest},
		{"get", http.MethodGet, "/announcement-requests/" + testAnnouncementRequestID, "", h.GetAnnouncementRequest},
		{"search", http.MethodPost, "/announcement-requests/search", `{"pagination":{"limit":10,"offset":0}}`, h.SearchAnnouncementRequests},
		{"update", http.MethodPatch, "/announcement-requests/" + testAnnouncementRequestID, `{}`, h.UpdateAnnouncementRequest},
		{"dry-run", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/dry-run", `{"caseId":"case-1"}`, h.RecordAnnouncementRequestDryRun},
		{"submit", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/submit", "", h.SubmitAnnouncementRequest},
		{"approve", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/approve", "", h.ApproveAnnouncementRequest},
		{"publish", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/publish", "", h.PublishAnnouncementRequest},
		{"create-update", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/updates", `{"content":"x"}`, h.CreateAnnouncementRequestUpdate},
		{"list-updates", http.MethodGet, "/announcement-requests/" + testAnnouncementRequestID + "/updates", "", h.ListAnnouncementRequestUpdates},
		{"record-deliveries", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/deliveries", `{"deliveries":[{"projectId":"p-1","status":"succeeded","caseId":"case-1"}]}`, h.RecordAnnouncementRequestDeliveries},
		{"list-deliveries", http.MethodGet, "/announcement-requests/" + testAnnouncementRequestID + "/deliveries", "", h.ListAnnouncementRequestDeliveries},
		{"create-attachment", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/attachments", `{"name":"a.pdf","file":"data:application/pdf;base64,JVBERg=="}`, h.CreateAnnouncementRequestAttachment},
		{"list-attachments", http.MethodGet, "/announcement-requests/" + testAnnouncementRequestID + "/attachments", "", h.ListAnnouncementRequestAttachments},
		{"attachment-content", http.MethodGet, "/announcement-requests/" + testAnnouncementRequestID + "/attachments/" + testAnnouncementAttachmentID + "/content", "", h.GetAnnouncementRequestAttachmentContent},
		{"delete-attachment", http.MethodDelete, "/announcement-requests/" + testAnnouncementRequestID + "/attachments/" + testAnnouncementAttachmentID, "", h.DeleteAnnouncementRequestAttachment},
		{"attach-to-case", http.MethodPost, "/announcement-requests/" + testAnnouncementRequestID + "/cases/" + testAnnouncementCaseID + "/attachments", "", h.AttachAnnouncementRequestAttachmentsToCase},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, c.target, strings.NewReader(c.body))
			r.SetPathValue("id", testAnnouncementRequestID)
			w := httptest.NewRecorder()
			c.handler(w, r)
			assertStatus(t, w, http.StatusUnauthorized)
		})
	}
}

// ----- CreateAnnouncementRequest -----

func TestCreateAnnouncementRequest(t *testing.T) {
	t.Run("forces createdBy to the authenticated caller, ignoring any client-supplied value", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		h := NewAnnouncementRequestHandler(client, nil)

		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests",
			strings.NewReader(`{"kind":"customer","subject":"Hi","createdBy":"someone-else"}`)))
		w := httptest.NewRecorder()
		h.CreateAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusCreated)
		var got struct {
			CreatedBy      string `json:"createdBy"`
			CreatedByEmail string `json:"createdByEmail"`
			Subject        string `json:"subject"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v; raw: %s", err, w.Body.String())
		}
		if got.CreatedBy != testUser.UserID {
			t.Fatalf("createdBy = %q, want the authenticated caller %q — a client-supplied value must never be trusted", got.CreatedBy, testUser.UserID)
		}
		if got.CreatedByEmail != testUser.Email {
			t.Fatalf("createdByEmail = %q, want the authenticated caller's email %q", got.CreatedByEmail, testUser.Email)
		}
		if got.Subject != "Hi" {
			t.Fatalf("expected subject forwarded unchanged, got %q", got.Subject)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests", strings.NewReader(`not json`)))
		w := httptest.NewRecorder()
		h.CreateAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("maps upstream errors generically", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create the announcement request.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityAnnouncementRequestClient{
					createFn: func(context.Context, []byte) ([]byte, error) { return nil, tc.err },
				}
				h := NewAnnouncementRequestHandler(client, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests", strings.NewReader(`{"kind":"customer"}`)))
				w := httptest.NewRecorder()
				h.CreateAnnouncementRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

// ----- GetAnnouncementRequest -----

func TestGetAnnouncementRequest(t *testing.T) {
	t.Run("rejects a non-UUID id", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/announcement-requests/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards to the entity service and returns its result", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(_ context.Context, id string) ([]byte, error) {
				return []byte(`{"id":"` + id + `","state":"draft"}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/announcement-requests/"+testAnnouncementRequestID, nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.GetAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusOK)
		got := decodeJSON[struct {
			State string `json:"state"`
		}](t, w)
		if got.State != "draft" {
			t.Fatalf("expected state forwarded unchanged, got %q", got.State)
		}
	})
}

// ----- UpdateAnnouncementRequest -----

func TestUpdateAnnouncementRequest(t *testing.T) {
	t.Run("forces actorId to the authenticated caller", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		h := NewAnnouncementRequestHandler(client, nil)

		r := withUser(httptest.NewRequest(http.MethodPatch, "/announcement-requests/"+testAnnouncementRequestID,
			strings.NewReader(`{"subject":"New subject","actorId":"someone-else"}`)))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.UpdateAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		var got struct {
			Subject string `json:"subject"`
			ActorID string `json:"actorId"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ActorID != testUser.UserID {
			t.Fatalf("actorId = %q, want the authenticated caller %q", got.ActorID, testUser.UserID)
		}
		if got.Subject != "New subject" {
			t.Fatalf("expected subject forwarded unchanged, got %q", got.Subject)
		}
	})

	t.Run("rejects a non-UUID id", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/announcement-requests/bad", strings.NewReader(`{}`)))
		r.SetPathValue("id", "bad")
		w := httptest.NewRecorder()
		h.UpdateAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("uses mapUpstreamError (not the generic variant) since this is a PATCH endpoint", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			updateFn: func(context.Context, string, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusConflict, Body: `{"message":"a published announcement request cannot be edited"}`}
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/announcement-requests/"+testAnnouncementRequestID, strings.NewReader(`{"subject":"x"}`)))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.UpdateAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, "a published announcement request cannot be edited")
	})
}

// ----- RecordAnnouncementRequestDryRun -----

func TestRecordAnnouncementRequestDryRun(t *testing.T) {
	t.Run("rejects a missing caseId", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/dry-run", strings.NewReader(`{}`)))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.RecordAnnouncementRequestDryRun(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("forwards caseId and forces actorId to the authenticated caller", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/dry-run",
			strings.NewReader(`{"caseId":"case-1","actorId":"someone-else"}`)))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.RecordAnnouncementRequestDryRun(w, r)

		assertStatus(t, w, http.StatusOK)
		var got struct {
			CaseID  string `json:"caseId"`
			ActorID string `json:"actorId"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.CaseID != "case-1" {
			t.Fatalf("expected caseId forwarded, got %q", got.CaseID)
		}
		if got.ActorID != testUser.UserID {
			t.Fatalf("actorId = %q, want the authenticated caller %q", got.ActorID, testUser.UserID)
		}
	})
}

// ----- ApproveAnnouncementRequest -----

func TestApproveAnnouncementRequest_IgnoresRequestBodyEntirely(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	// A body is deliberately never sent nor read — this transition needs
	// nothing but the authenticated caller.
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/approve", nil))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.ApproveAnnouncementRequest(w, r)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ActorID    string `json:"actorId"`
		ActorEmail string `json:"actorEmail"`
	}
	if err := json.Unmarshal(client.gotApproveBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.ActorID != testUser.UserID {
		t.Fatalf("actorId = %q, want %q", got.ActorID, testUser.UserID)
	}
	if got.ActorEmail != testUser.Email {
		t.Fatalf("actorEmail = %q, want %q", got.ActorEmail, testUser.Email)
	}
}

// ----- PublishAnnouncementRequest -----

// TestPublishAnnouncementRequest_ForwardsCaseIDsAndForcesActorID locks in
// that, unlike Approve, Publish does read its request body — caseIds, the
// webapp's own fan-out result — while actorId is still always the
// authenticated caller, never client-supplied.
func TestPublishAnnouncementRequest_ForwardsCaseIDsAndForcesActorID(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/publish",
		strings.NewReader(`{"caseIds":["case-1","case-2"],"actorId":"someone-else"}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.PublishAnnouncementRequest(w, r)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ActorID    string   `json:"actorId"`
		ActorEmail string   `json:"actorEmail"`
		CaseIDs    []string `json:"caseIds"`
	}
	if err := json.Unmarshal(client.gotPublishBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.ActorID != testUser.UserID {
		t.Fatalf("actorId = %q, want the authenticated caller %q, not the client-supplied value", got.ActorID, testUser.UserID)
	}
	if got.ActorEmail != testUser.Email {
		t.Fatalf("actorEmail = %q, want the authenticated caller's email %q", got.ActorEmail, testUser.Email)
	}
	if len(got.CaseIDs) != 2 || got.CaseIDs[0] != "case-1" || got.CaseIDs[1] != "case-2" {
		t.Fatalf("expected caseIds forwarded, got %v", got.CaseIDs)
	}
}

func TestPublishAnnouncementRequest_RejectsEmptyCaseIDs(t *testing.T) {
	for name, body := range map[string]string{
		"missing caseIds": `{}`,
		"empty caseIds":   `{"caseIds":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			client := &mockEntityAnnouncementRequestClient{}
			h := NewAnnouncementRequestHandler(client, nil)
			r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/publish", strings.NewReader(body)))
			r.SetPathValue("id", testAnnouncementRequestID)
			w := httptest.NewRecorder()
			h.PublishAnnouncementRequest(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			if client.gotPublishBody != nil {
				t.Fatal("expected the entity client never to be called for an empty caseIds")
			}
		})
	}
}

// ----- ScheduleAnnouncementRequest -----

// TestScheduleAnnouncementRequest_ForwardsScheduledForAndForcesActorID locks
// in that scheduledFor is forwarded verbatim while actorId is always the
// authenticated caller, never client-supplied — same restriction as Publish.
func TestScheduleAnnouncementRequest_ForwardsScheduledForAndForcesActorID(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/schedule",
		strings.NewReader(`{"scheduledFor":"2026-08-01T00:00:00Z","actorId":"someone-else"}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.ScheduleAnnouncementRequest(w, r)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ActorID      string  `json:"actorId"`
		ActorEmail   string  `json:"actorEmail"`
		ScheduledFor *string `json:"scheduledFor"`
	}
	if err := json.Unmarshal(client.gotScheduleBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.ActorID != testUser.UserID {
		t.Fatalf("actorId = %q, want the authenticated caller %q, not the client-supplied value", got.ActorID, testUser.UserID)
	}
	if got.ActorEmail != testUser.Email {
		t.Fatalf("actorEmail = %q, want the authenticated caller's email %q", got.ActorEmail, testUser.Email)
	}
	if got.ScheduledFor == nil || *got.ScheduledFor != "2026-08-01T00:00:00Z" {
		t.Fatalf("expected scheduledFor forwarded, got %v", got.ScheduledFor)
	}
}

// TestScheduleAnnouncementRequest_ForwardsNilScheduledForToClear locks in
// that an omitted/null scheduledFor forwards as nil (clearing the schedule),
// not an empty string or a dropped field.
func TestScheduleAnnouncementRequest_ForwardsNilScheduledForToClear(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/schedule",
		strings.NewReader(`{"scheduledFor":null}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.ScheduleAnnouncementRequest(w, r)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ScheduledFor *string `json:"scheduledFor"`
	}
	if err := json.Unmarshal(client.gotScheduleBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.ScheduledFor != nil {
		t.Fatalf("expected scheduledFor forwarded as nil, got %v", *got.ScheduledFor)
	}
}

// ----- CreateAnnouncementRequestUpdate / ListAnnouncementRequestUpdates -----

func TestCreateAnnouncementRequestUpdate_ForwardsContentAndForcesActorID(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/updates",
		strings.NewReader(`{"content":"A correction.","actorId":"someone-else"}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.CreateAnnouncementRequestUpdate(w, r)
	assertStatus(t, w, http.StatusCreated)

	var got struct {
		Content    string `json:"content"`
		ActorID    string `json:"actorId"`
		ActorEmail string `json:"actorEmail"`
	}
	if err := json.Unmarshal(client.gotCreateUpdateBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.Content != "A correction." {
		t.Fatalf("content = %q, want forwarded", got.Content)
	}
	if got.ActorEmail != testUser.Email {
		t.Fatalf("actorEmail = %q, want the authenticated caller's email %q", got.ActorEmail, testUser.Email)
	}
	if got.ActorID != testUser.UserID {
		t.Fatalf("actorId = %q, want the authenticated caller %q, not the client-supplied value", got.ActorID, testUser.UserID)
	}
}

func TestCreateAnnouncementRequestUpdate_RejectsEmptyContent(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/updates", strings.NewReader(`{}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.CreateAnnouncementRequestUpdate(w, r)
	assertStatus(t, w, http.StatusBadRequest)
	if client.gotCreateUpdateBody != nil {
		t.Fatal("expected the entity client never to be called for empty content")
	}
}

func TestListAnnouncementRequestUpdates_Passthrough(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{
		listUpdatesFn: func(ctx context.Context, id string) ([]byte, error) {
			if id != testAnnouncementRequestID {
				t.Fatalf("id = %q, want %q", id, testAnnouncementRequestID)
			}
			return []byte(`{"updates":[{"id":"u-1","content":"first"}]}`), nil
		},
	}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodGet, "/announcement-requests/"+testAnnouncementRequestID+"/updates", nil))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.ListAnnouncementRequestUpdates(w, r)
	assertStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"first"`) {
		t.Fatalf("expected the upstream response forwarded, got %s", w.Body.String())
	}
}

// ----- RecordAnnouncementRequestDeliveries / ListAnnouncementRequestDeliveries -----

func TestRecordAnnouncementRequestDeliveries_ForwardsDeliveriesAndForcesActorID(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/deliveries",
		strings.NewReader(`{"deliveries":[{"projectId":"p-1","status":"succeeded","caseId":"case-1"}],"actorId":"someone-else"}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.RecordAnnouncementRequestDeliveries(w, r)
	assertStatus(t, w, http.StatusOK)

	var got struct {
		ActorID    string `json:"actorId"`
		Deliveries []struct {
			ProjectID string `json:"projectId"`
			Status    string `json:"status"`
			CaseID    string `json:"caseId"`
		} `json:"deliveries"`
	}
	if err := json.Unmarshal(client.gotRecordDeliveriesBody, &got); err != nil {
		t.Fatalf("decode forwarded body: %v", err)
	}
	if got.ActorID != testUser.UserID {
		t.Fatalf("actorId = %q, want the authenticated caller %q, not the client-supplied value", got.ActorID, testUser.UserID)
	}
	if len(got.Deliveries) != 1 || got.Deliveries[0].ProjectID != "p-1" || got.Deliveries[0].Status != "succeeded" || got.Deliveries[0].CaseID != "case-1" {
		t.Fatalf("expected deliveries forwarded unchanged, got %+v", got.Deliveries)
	}
}

func TestRecordAnnouncementRequestDeliveries_RejectsEmptyDeliveries(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/deliveries", strings.NewReader(`{"deliveries":[]}`)))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.RecordAnnouncementRequestDeliveries(w, r)
	assertStatus(t, w, http.StatusBadRequest)
	if client.gotRecordDeliveriesBody != nil {
		t.Fatal("expected the entity client never to be called for empty deliveries")
	}
}

func TestRecordAnnouncementRequestDeliveries_RejectsMissingProjectIdOrStatus(t *testing.T) {
	for name, body := range map[string]string{
		"missing projectId": `{"deliveries":[{"status":"failed"}]}`,
		"missing status":    `{"deliveries":[{"projectId":"p-1"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			client := &mockEntityAnnouncementRequestClient{}
			h := NewAnnouncementRequestHandler(client, nil)
			r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/deliveries", strings.NewReader(body)))
			r.SetPathValue("id", testAnnouncementRequestID)
			w := httptest.NewRecorder()
			h.RecordAnnouncementRequestDeliveries(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			if client.gotRecordDeliveriesBody != nil {
				t.Fatal("expected the entity client never to be called for an incomplete delivery")
			}
		})
	}
}

func TestListAnnouncementRequestDeliveries_Passthrough(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{
		listDeliveriesFn: func(ctx context.Context, id string) ([]byte, error) {
			if id != testAnnouncementRequestID {
				t.Fatalf("id = %q, want %q", id, testAnnouncementRequestID)
			}
			return []byte(`{"deliveries":[{"id":"d-1","projectId":"p-1","status":"succeeded"}]}`), nil
		},
	}
	h := NewAnnouncementRequestHandler(client, nil)
	r := withUser(httptest.NewRequest(http.MethodGet, "/announcement-requests/"+testAnnouncementRequestID+"/deliveries", nil))
	r.SetPathValue("id", testAnnouncementRequestID)
	w := httptest.NewRecorder()
	h.ListAnnouncementRequestDeliveries(w, r)
	assertStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"d-1"`) {
		t.Fatalf("expected the upstream response forwarded, got %s", w.Body.String())
	}
}

// ----- SubmitAnnouncementRequest -----

func TestSubmitAnnouncementRequest(t *testing.T) {
	t.Run("customer specific scope: no project search, forwards the picked ids directly", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"specific","projectIds":["proj-1","proj-2"]}}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		if client.searchProjectsCalls != 0 {
			t.Fatalf("expected no project search for a specific-scope audience, got %d calls", client.searchProjectsCalls)
		}
		var got struct {
			ResolvedProjectIDs []string `json:"resolvedProjectIds"`
			ActorID            string   `json:"actorId"`
			ActorEmail         string   `json:"actorEmail"`
		}
		if err := json.Unmarshal(client.gotSubmitBody, &got); err != nil {
			t.Fatalf("decode forwarded submit body: %v", err)
		}
		if len(got.ResolvedProjectIDs) != 2 || got.ResolvedProjectIDs[0] != "proj-1" || got.ResolvedProjectIDs[1] != "proj-2" {
			t.Fatalf("expected the picked project ids forwarded unchanged, got %+v", got.ResolvedProjectIDs)
		}
		if got.ActorID != testUser.UserID {
			t.Fatalf("actorId = %q, want %q", got.ActorID, testUser.UserID)
		}
		if got.ActorEmail != testUser.Email {
			t.Fatalf("actorEmail = %q, want %q", got.ActorEmail, testUser.Email)
		}
	})

	t.Run("customer all scope: pages through project search with the mandatory exclusion injected", func(t *testing.T) {
		var gotBodies [][]byte
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"all","excludeClosureStates":["Restricted","Suspended"]}}`), nil
			},
			searchProjectsFn: func(_ context.Context, body []byte) ([]byte, error) {
				gotBodies = append(gotBodies, body)
				if len(gotBodies) == 1 {
					return []byte(`{"projects":[{"id":"proj-1"},{"id":"proj-2"}],"total":3,"hasMore":true}`), nil
				}
				return []byte(`{"projects":[{"id":"proj-3"}],"total":3,"hasMore":false}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, []string{"Excluded1"})
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		if client.searchProjectsCalls != 2 {
			t.Fatalf("expected 2 pages fetched, got %d", client.searchProjectsCalls)
		}
		var got struct {
			ResolvedProjectIDs []string `json:"resolvedProjectIds"`
		}
		if err := json.Unmarshal(client.gotSubmitBody, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got.ResolvedProjectIDs) != 3 {
			t.Fatalf("expected all 3 project ids accumulated across pages, got %+v", got.ResolvedProjectIDs)
		}
		// The mandatory excluded-project-key denylist must be injected on
		// every page, the same as a live audience resolution.
		for i, body := range gotBodies {
			var page struct {
				ExcludeProjectKeys []string `json:"excludeProjectKeys"`
			}
			if err := json.Unmarshal(body, &page); err != nil {
				t.Fatalf("decode page %d body: %v", i, err)
			}
			if len(page.ExcludeProjectKeys) != 1 || page.ExcludeProjectKeys[0] != "Excluded1" {
				t.Fatalf("page %d: expected the mandatory exclusion injected, got %+v", i, page.ExcludeProjectKeys)
			}
		}
	})

	t.Run("eol kind: pages through the product-version project search", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"eol","audienceDefinition":{"productId":"prod-1","productVersionId":"ver-1"}}`), nil
			},
			searchProjectsByVersionFn: func(_ context.Context, body []byte) ([]byte, error) {
				var req struct {
					ProductID        string `json:"productId"`
					ProductVersionID string `json:"productVersionId"`
				}
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if req.ProductID != "prod-1" || req.ProductVersionID != "ver-1" {
					t.Fatalf("expected product/version forwarded, got %+v", req)
				}
				return []byte(`{"projects":[{"id":"proj-a"}],"total":1,"hasMore":false}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusOK)
		if client.searchProjectsByVersionCalls != 1 {
			t.Fatalf("expected 1 page fetched, got %d", client.searchProjectsByVersionCalls)
		}
	})

	t.Run("rejects an empty resolved audience without calling Submit", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"specific","projectIds":[]}}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		if client.gotSubmitBody != nil {
			t.Fatal("expected Submit never called for an empty resolved audience")
		}
	})

	t.Run("rejects an unknown kind", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"bogus"}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects an unknown customer scope instead of silently resolving to 'all'", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"bogus"}}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		// An unrecognized scope must never fall through to resolving every
		// customer project — that would turn a data bug into an
		// announcement sent to the wrong audience entirely.
		if client.searchProjectsCalls != 0 {
			t.Fatalf("expected no project search for an unrecognized scope, got %d calls", client.searchProjectsCalls)
		}
	})

	t.Run("fails loudly instead of looping forever when hasMore never turns false", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"all"}}`), nil
			},
			searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
				// Always one project, always hasMore=true — a wrong upstream
				// signal that must not spin forever.
				return []byte(`{"projects":[{"id":"proj-x"}],"total":999999,"hasMore":true}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		if client.searchProjectsCalls != maxAnnouncementAudiencePages {
			t.Fatalf("expected exactly %d pages fetched before bailing out, got %d", maxAnnouncementAudiencePages, client.searchProjectsCalls)
		}
	})

	t.Run("maps upstream errors from the entity service generically", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to submit the announcement request for approval.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityAnnouncementRequestClient{
					getFn: func(context.Context, string) ([]byte, error) {
						return []byte(`{"kind":"customer","audienceDefinition":{"scope":"specific","projectIds":["proj-1"]}}`), nil
					},
					submitFn: func(context.Context, string, []byte) ([]byte, error) { return nil, tc.err },
				}
				h := NewAnnouncementRequestHandler(client, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
				r.SetPathValue("id", testAnnouncementRequestID)
				w := httptest.NewRecorder()
				h.SubmitAnnouncementRequest(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})

	t.Run("an upstream failure resolving the audience maps through the generic mapper, not a flat 400", func(t *testing.T) {
		// A genuine SearchProjects failure (here, a transient 503) must not
		// be reported as a flat 400 "bad request" — that would tell the
		// caller to fix their input and retry, when the real problem is
		// upstream and retrying as-is might well succeed.
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"all"}}`), nil
			},
			searchProjectsFn: func(context.Context, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusServiceUnavailable}
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusServiceUnavailable)
	})

	t.Run("a locally-detected audience problem (not an upstream failure) still returns 400", func(t *testing.T) {
		// An unrecognized scope is this handler's own validation, not
		// something SearchProjects ever saw — must stay a real 400, since
		// retrying without first fixing the stored data would fail the
		// same way every time.
		client := &mockEntityAnnouncementRequestClient{
			getFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"kind":"customer","audienceDefinition":{"scope":"bogus"}}`), nil
			},
		}
		h := NewAnnouncementRequestHandler(client, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/announcement-requests/"+testAnnouncementRequestID+"/submit", nil))
		r.SetPathValue("id", testAnnouncementRequestID)
		w := httptest.NewRecorder()
		h.SubmitAnnouncementRequest(w, r)

		assertStatus(t, w, http.StatusBadRequest)
		if client.searchProjectsCalls != 0 {
			t.Fatalf("expected no project search for an unrecognized scope, got %d calls", client.searchProjectsCalls)
		}
	})
}

// ----- announcement request attachments -----

func attachmentRequest(method, target, body string) *http.Request {
	r := withUser(httptest.NewRequest(method, target, strings.NewReader(body)))
	r.SetPathValue("id", testAnnouncementRequestID)
	r.SetPathValue("attachmentId", testAnnouncementAttachmentID)
	r.SetPathValue("caseId", testAnnouncementCaseID)
	return r
}

func TestCreateAnnouncementRequestAttachment(t *testing.T) {
	const target = "/announcement-requests/" + testAnnouncementRequestID + "/attachments"

	t.Run("forwards the file and forces the actor to the authenticated caller", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		h := NewAnnouncementRequestHandler(client, nil)
		w := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(w, attachmentRequest(http.MethodPost, target,
			`{"name":"advisory.pdf","type":"application/pdf","file":"data:application/pdf;base64,JVBERg==","actorId":"someone-else","actorEmail":"x@y.z"}`))
		assertStatus(t, w, http.StatusCreated)

		var got struct {
			ActorID, ActorEmail, Name, Type, File string
		}
		var raw map[string]string
		if err := json.Unmarshal(client.gotCreateAttachmentBody, &raw); err != nil {
			t.Fatalf("decode forwarded body: %v", err)
		}
		got.ActorID, got.ActorEmail, got.Name, got.Type, got.File = raw["actorId"], raw["actorEmail"], raw["name"], raw["type"], raw["file"]
		if got.ActorID != testUser.UserID || got.ActorEmail != testUser.Email {
			t.Fatalf("actor = %q/%q, want the authenticated caller %q/%q — a client-supplied value must never be trusted", got.ActorID, got.ActorEmail, testUser.UserID, testUser.Email)
		}
		if got.Name != "advisory.pdf" || got.Type != "application/pdf" || got.File != "data:application/pdf;base64,JVBERg==" {
			t.Fatalf("expected the file forwarded unchanged, got %+v", got)
		}
		if !strings.Contains(w.Body.String(), `"att-1"`) {
			t.Fatalf("expected the upstream response returned, got %s", w.Body.String())
		}
	})

	t.Run("accepts a body past the 1 MiB generic cap and refuses one past 15 MiB", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil)
		body := func(n int) string {
			return `{"name":"a.pdf","file":"data:application/pdf;base64,` + strings.Repeat("A", n) + `"}`
		}
		w := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(w, attachmentRequest(http.MethodPost, target, body(3<<20)))
		assertStatus(t, w, http.StatusCreated)

		w = httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(w, attachmentRequest(http.MethodPost, target, body(16<<20)))
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("rejects a bad id, invalid JSON and a missing name or file without calling upstream", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		h := NewAnnouncementRequestHandler(client, nil)

		r := attachmentRequest(http.MethodPost, target, `{"name":"a.pdf","file":"x"}`)
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)

		for _, body := range []string{`not json`, `{"file":"x"}`, `{"name":"a.pdf"}`, `{}`} {
			w := httptest.NewRecorder()
			h.CreateAnnouncementRequestAttachment(w, attachmentRequest(http.MethodPost, target, body))
			assertStatus(t, w, http.StatusBadRequest)
		}
		if client.gotCreateAttachmentBody != nil {
			t.Fatal("expected the entity client never to be called for a malformed request")
		}
	})

	t.Run("shows the entity service's reason for a file it refuses", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to attach the file.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityAnnouncementRequestClient{
					createAttachmentFn: func(context.Context, string, []byte) ([]byte, error) { return nil, tc.err },
				}
				h := NewAnnouncementRequestHandler(client, nil)
				w := httptest.NewRecorder()
				h.CreateAnnouncementRequestAttachment(w, attachmentRequest(http.MethodPost, target, `{"name":"a.pdf","file":"x"}`))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestListAnnouncementRequestAttachments_Passthrough(t *testing.T) {
	client := &mockEntityAnnouncementRequestClient{
		listAttachmentsFn: func(_ context.Context, id string) ([]byte, error) {
			if id != testAnnouncementRequestID {
				t.Fatalf("id = %q", id)
			}
			return []byte(`{"attachments":[{"id":"a1","name":"one.pdf"}]}`), nil
		},
	}
	h := NewAnnouncementRequestHandler(client, nil)
	w := httptest.NewRecorder()
	h.ListAnnouncementRequestAttachments(w, attachmentRequest(http.MethodGet, "/x", ""))
	assertStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"one.pdf"`) {
		t.Fatalf("expected the upstream response forwarded, got %s", w.Body.String())
	}

	for _, tc := range upstreamErrorsGeneric("Failed to list the files attached to the announcement request.") {
		t.Run(tc.name, func(t *testing.T) {
			c := &mockEntityAnnouncementRequestClient{
				listAttachmentsFn: func(context.Context, string) ([]byte, error) { return nil, tc.err },
			}
			w := httptest.NewRecorder()
			NewAnnouncementRequestHandler(c, nil).ListAnnouncementRequestAttachments(w, attachmentRequest(http.MethodGet, "/x", ""))
			assertStatus(t, w, tc.wantCode)
			assertErrorMessage(t, w, tc.wantMsg)
		})
	}
}

func TestGetAnnouncementRequestAttachmentContent(t *testing.T) {
	for _, tc := range []struct{ upstream, want string }{
		{"application/pdf", "application/pdf"},
		{"application/pdf; charset=binary", "application/pdf"},
		{"IMAGE/PNG", "image/png"},
		{"text/html", "application/octet-stream"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/octet-stream"},
	} {
		t.Run(tc.upstream, func(t *testing.T) {
			client := &mockEntityAnnouncementRequestClient{
				attachmentContentFn: func(_ context.Context, id, attachmentID string) ([]byte, string, error) {
					if id != testAnnouncementRequestID || attachmentID != testAnnouncementAttachmentID {
						t.Fatalf("ids = %q %q", id, attachmentID)
					}
					return []byte("bytes"), tc.upstream, nil
				},
			}
			w := httptest.NewRecorder()
			NewAnnouncementRequestHandler(client, nil).GetAnnouncementRequestAttachmentContent(w, attachmentRequest(http.MethodGet, "/x", ""))
			assertStatus(t, w, http.StatusOK)
			assertContentType(t, w, tc.want)
			if cd := w.Header().Get("Content-Disposition"); cd != "attachment" {
				t.Errorf("Content-Disposition = %q, want attachment", cd)
			}
			if w.Body.String() != "bytes" {
				t.Errorf("body = %q", w.Body.String())
			}
		})
	}

	t.Run("rejects a bad id", func(t *testing.T) {
		r := attachmentRequest(http.MethodGet, "/x", "")
		r.SetPathValue("attachmentId", "nope")
		w := httptest.NewRecorder()
		NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil).GetAnnouncementRequestAttachmentContent(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("maps upstream errors generically", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve the attachment content.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityAnnouncementRequestClient{
					attachmentContentFn: func(context.Context, string, string) ([]byte, string, error) { return nil, "", tc.err },
				}
				w := httptest.NewRecorder()
				NewAnnouncementRequestHandler(client, nil).GetAnnouncementRequestAttachmentContent(w, attachmentRequest(http.MethodGet, "/x", ""))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestDeleteAnnouncementRequestAttachment(t *testing.T) {
	t.Run("sends the authenticated caller as the actor and answers 204", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		w := httptest.NewRecorder()
		NewAnnouncementRequestHandler(client, nil).DeleteAnnouncementRequestAttachment(w, attachmentRequest(http.MethodDelete, "/x?actorId=someone-else", ""))
		assertStatus(t, w, http.StatusNoContent)
		if client.gotDeleteAttachmentActorID != testUser.UserID {
			t.Fatalf("actor = %q, want the authenticated caller %q, not the query value", client.gotDeleteAttachmentActorID, testUser.UserID)
		}
		if w.Body.Len() != 0 {
			t.Fatalf("a 204 must have no body, got %q", w.Body.String())
		}
	})

	t.Run("rejects a bad id", func(t *testing.T) {
		r := attachmentRequest(http.MethodDelete, "/x", "")
		r.SetPathValue("id", "nope")
		w := httptest.NewRecorder()
		NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil).DeleteAnnouncementRequestAttachment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("shows the entity service's reason (for example a published request)", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{
			deleteAttachmentFn: func(context.Context, string, string, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusConflict, Body: `{"code":409,"message":"the files of a published announcement request cannot be changed"}`}
			},
		}
		w := httptest.NewRecorder()
		NewAnnouncementRequestHandler(client, nil).DeleteAnnouncementRequestAttachment(w, attachmentRequest(http.MethodDelete, "/x", ""))
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, "the files of a published announcement request cannot be changed")
	})
}

func TestAttachAnnouncementRequestAttachmentsToCase(t *testing.T) {
	t.Run("asks upstream to copy the files, as the authenticated caller, taking nothing from the client body", func(t *testing.T) {
		client := &mockEntityAnnouncementRequestClient{}
		w := httptest.NewRecorder()
		NewAnnouncementRequestHandler(client, nil).AttachAnnouncementRequestAttachmentsToCase(w,
			attachmentRequest(http.MethodPost, "/x", `{"actorId":"someone-else","actorEmail":"evil@example.com"}`))
		assertStatus(t, w, http.StatusOK)
		if client.gotAttachToCaseCaseID != testAnnouncementCaseID {
			t.Fatalf("caseId = %q", client.gotAttachToCaseCaseID)
		}
		var raw map[string]string
		if err := json.Unmarshal(client.gotAttachToCaseBody, &raw); err != nil {
			t.Fatalf("decode forwarded body: %v", err)
		}
		if raw["actorId"] != testUser.UserID || raw["actorEmail"] != testUser.Email || len(raw) != 2 {
			t.Fatalf("forwarded body = %v, want only the authenticated caller as the actor", raw)
		}
		if !strings.Contains(w.Body.String(), `"attached":1`) {
			t.Fatalf("expected the upstream result returned, got %s", w.Body.String())
		}
	})

	t.Run("rejects a bad request or case id", func(t *testing.T) {
		for _, field := range []string{"id", "caseId"} {
			r := attachmentRequest(http.MethodPost, "/x", "")
			r.SetPathValue(field, "nope")
			w := httptest.NewRecorder()
			NewAnnouncementRequestHandler(&mockEntityAnnouncementRequestClient{}, nil).AttachAnnouncementRequestAttachmentsToCase(w, r)
			assertStatus(t, w, http.StatusBadRequest)
		}
	})

	t.Run("shows the entity service's reason", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to attach the files to the case.") {
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityAnnouncementRequestClient{
					attachToCaseFn: func(context.Context, string, string, []byte) ([]byte, error) { return nil, tc.err },
				}
				w := httptest.NewRecorder()
				NewAnnouncementRequestHandler(client, nil).AttachAnnouncementRequestAttachmentsToCase(w, attachmentRequest(http.MethodPost, "/x", ""))
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
