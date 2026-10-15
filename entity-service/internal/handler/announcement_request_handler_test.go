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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// searchCapturingService records the request Search was called with; every
// other AnnouncementRequestService method is left nil-embedded, so a test
// that accidentally reaches one fails loudly instead of silently passing.
type searchCapturingService struct {
	service.AnnouncementRequestService
	got domain.SearchAnnouncementRequestsRequest
}

func (s *searchCapturingService) Search(_ context.Context, req domain.SearchAnnouncementRequestsRequest) (domain.SearchAnnouncementRequestsResponse, error) {
	s.got = req
	return domain.SearchAnnouncementRequestsResponse{}, nil
}

// decodeRequest rejects unknown JSON fields outright, so the wire contract of
// POST /announcement-requests/search is worth pinning: the new `states` list
// must be accepted, and the older bodies (single `state`, the scheduled-publish
// cron's `readyForScheduledPublish`) must keep decoding unchanged.
func TestSearchAnnouncementRequests_RequestBodies(t *testing.T) {
	post := func(t *testing.T, body string) (*httptest.ResponseRecorder, *searchCapturingService) {
		t.Helper()
		svc := &searchCapturingService{}
		h := NewAnnouncementRequestHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/announcement-requests/search", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.SearchAnnouncementRequests(rec, req)
		return rec, svc
	}

	t.Run("accepts a states list", func(t *testing.T) {
		rec, svc := post(t, `{"states":["draft","approved"],"pagination":{"offset":0,"limit":10}}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if len(svc.got.States) != 2 || svc.got.States[0] != domain.AnnouncementRequestStateDraft || svc.got.States[1] != domain.AnnouncementRequestStateApproved {
			t.Fatalf("States = %v, want [draft approved]", svc.got.States)
		}
		if svc.got.State != nil {
			t.Fatalf("State = %v, want nil", *svc.got.State)
		}
	})

	t.Run("still accepts the single state field", func(t *testing.T) {
		rec, svc := post(t, `{"state":"pending_approval","pagination":{"offset":0,"limit":10}}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if svc.got.State == nil || *svc.got.State != domain.AnnouncementRequestStatePendingApproval {
			t.Fatalf("State = %v, want pending_approval", svc.got.State)
		}
		if len(svc.got.States) != 0 {
			t.Fatalf("States = %v, want none", svc.got.States)
		}
	})

	t.Run("still accepts the scheduled-publish cron body", func(t *testing.T) {
		rec, svc := post(t, `{"readyForScheduledPublish":true,"pagination":{"offset":0,"limit":100}}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if !svc.got.ReadyForScheduledPublish {
			t.Fatal("ReadyForScheduledPublish not decoded")
		}
	})

	// The service tells "states sent but empty" from "states omitted" by
	// nil-ness (so a contradictory `state` + `"states": []` is rejected), which
	// only works if the JSON decode keeps that distinction.
	t.Run("keeps an explicit empty states list distinct from an omitted one", func(t *testing.T) {
		rec, svc := post(t, `{"states":[],"pagination":{"offset":0,"limit":10}}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if svc.got.States == nil {
			t.Fatal("explicit empty States decoded as nil, indistinguishable from omitted")
		}
		_, omitted := post(t, `{"pagination":{"offset":0,"limit":10}}`)
		if omitted.got.States != nil {
			t.Fatalf("omitted States = %v, want nil", omitted.got.States)
		}
	})

	t.Run("accepts no state filter at all", func(t *testing.T) {
		rec, svc := post(t, `{"pagination":{"offset":0,"limit":10}}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if svc.got.State != nil || len(svc.got.States) != 0 {
			t.Fatalf("got State=%v States=%v, want neither", svc.got.State, svc.got.States)
		}
	})

	t.Run("still rejects an unknown field", func(t *testing.T) {
		rec, _ := post(t, `{"stateList":["draft"],"pagination":{"offset":0,"limit":10}}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

// attachmentCapturingService records what the attachment endpoints hand the
// service; every other method is left nil-embedded so reaching one fails loudly.
type attachmentCapturingService struct {
	service.AnnouncementRequestService

	addID, deleteID, deleteAttachmentID, deleteActor string
	addReq                                           domain.CreateAnnouncementRequestAttachmentRequest
	contentResult                                    domain.AnnouncementRequestAttachmentContent
	attachID, attachCaseID                           string
	attachReq                                        domain.AnnouncementRequestActorRequest
	err                                              error
}

func (s *attachmentCapturingService) AddAttachment(_ context.Context, id string, req domain.CreateAnnouncementRequestAttachmentRequest) (domain.AnnouncementRequestAttachment, error) {
	s.addID, s.addReq = id, req
	return domain.AnnouncementRequestAttachment{ID: "att-1", Name: req.Name}, s.err
}

func (s *attachmentCapturingService) DeleteAttachment(_ context.Context, id, attachmentID, actorID string) error {
	s.deleteID, s.deleteAttachmentID, s.deleteActor = id, attachmentID, actorID
	return s.err
}

func (s *attachmentCapturingService) GetAttachmentContent(_ context.Context, _, _ string) (domain.AnnouncementRequestAttachmentContent, error) {
	return s.contentResult, s.err
}

func (s *attachmentCapturingService) AttachToCase(_ context.Context, id, caseID string, req domain.AnnouncementRequestActorRequest) (domain.AttachAnnouncementRequestAttachmentsToCaseResponse, error) {
	s.attachID, s.attachCaseID, s.attachReq = id, caseID, req
	return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{Attached: 2}, s.err
}

func TestAnnouncementRequestAttachmentEndpoints(t *testing.T) {
	t.Run("POST attachment decodes the body, hands the service the id and answers 201", func(t *testing.T) {
		svc := &attachmentCapturingService{}
		h := NewAnnouncementRequestHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/announcement-requests/r1/attachments",
			strings.NewReader(`{"actorId":"u1","actorEmail":"u1@example.com","name":"a.pdf","type":"application/pdf","file":"data:application/pdf;base64,JVBERg=="}`))
		req.SetPathValue("id", "r1")
		rec := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if svc.addID != "r1" || svc.addReq.Name != "a.pdf" || svc.addReq.ActorID != "u1" || svc.addReq.File == "" {
			t.Fatalf("unexpected call: id=%q req=%+v", svc.addID, svc.addReq)
		}
	})

	t.Run("POST attachment accepts a body well past the generic 1 MiB cap, and refuses one past 15 MiB", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&attachmentCapturingService{})
		body := func(n int) string {
			return `{"actorId":"u1","name":"a.pdf","file":"data:application/pdf;base64,` + strings.Repeat("A", n) + `"}`
		}
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body(3<<20)))
		req.SetPathValue("id", "r1")
		rec := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("a 3 MiB body must be accepted, got %d: %s", rec.Code, rec.Body.String())
		}
		req = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body(16<<20)))
		req.SetPathValue("id", "r1")
		rec = httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "10 MB") {
			t.Fatalf("a 16 MiB body must be refused naming the file-size limit, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST attachment rejects an unknown field", func(t *testing.T) {
		h := NewAnnouncementRequestHandler(&attachmentCapturingService{})
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"actorId":"u1","name":"a.pdf","file":"x","extra":1}`))
		rec := httptest.NewRecorder()
		h.CreateAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("GET content is always a download, with the file's own name and type only when the type is on the allowlist", func(t *testing.T) {
		for _, tc := range []struct{ mime, wantType string }{
			{"application/pdf", "application/pdf"},
			{"image/png", "image/png"},
			{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "application/octet-stream"},
			{"text/html", "application/octet-stream"},
		} {
			svc := &attachmentCapturingService{contentResult: domain.AnnouncementRequestAttachmentContent{
				AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{Name: `Q3 "advisory".pdf`, Type: tc.mime},
				Content:                       []byte("bytes"),
			}}
			h := NewAnnouncementRequestHandler(svc)
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			h.GetAnnouncementRequestAttachmentContent(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != "bytes" {
				t.Fatalf("%s: status=%d body=%q", tc.mime, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != tc.wantType {
				t.Errorf("%s: Content-Type = %q, want %q", tc.mime, got, tc.wantType)
			}
			cd := rec.Header().Get("Content-Disposition")
			if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, `filename="Q3 \"advisory\".pdf"`) {
				t.Errorf("%s: Content-Disposition = %q, want an attachment with the quoted file name", tc.mime, cd)
			}
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s: missing nosniff", tc.mime)
			}
		}
	})

	t.Run("DELETE takes the actor from the query string and answers 204", func(t *testing.T) {
		svc := &attachmentCapturingService{}
		h := NewAnnouncementRequestHandler(svc)
		req := httptest.NewRequest(http.MethodDelete, "/x?actorId=u9", nil)
		req.SetPathValue("id", "r1")
		req.SetPathValue("attachmentId", "a1")
		rec := httptest.NewRecorder()
		h.DeleteAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
		}
		if svc.deleteID != "r1" || svc.deleteAttachmentID != "a1" || svc.deleteActor != "u9" {
			t.Fatalf("unexpected call: %q %q %q", svc.deleteID, svc.deleteAttachmentID, svc.deleteActor)
		}
	})

	t.Run("attach to case passes the request and case ids and the actor, and reports what was attached", func(t *testing.T) {
		svc := &attachmentCapturingService{}
		h := NewAnnouncementRequestHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"actorId":"u1","actorEmail":"u1@example.com"}`))
		req.SetPathValue("id", "r1")
		req.SetPathValue("caseId", "c1")
		rec := httptest.NewRecorder()
		h.AttachAnnouncementRequestAttachmentsToCase(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"attached":2`) {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
		if svc.attachID != "r1" || svc.attachCaseID != "c1" || svc.attachReq.ActorEmail != "u1@example.com" {
			t.Fatalf("unexpected call: %q %q %+v", svc.attachID, svc.attachCaseID, svc.attachReq)
		}
	})

	t.Run("a service error becomes its HTTP status", func(t *testing.T) {
		svc := &attachmentCapturingService{err: &apierror.ConflictError{Msg: "published"}}
		h := NewAnnouncementRequestHandler(svc)
		req := httptest.NewRequest(http.MethodDelete, "/x?actorId=u1", nil)
		rec := httptest.NewRecorder()
		h.DeleteAnnouncementRequestAttachment(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
	})
}
