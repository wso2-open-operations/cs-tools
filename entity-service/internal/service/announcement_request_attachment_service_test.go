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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	annAttReqID     = "11111111-1111-4111-8111-111111111111"
	annAttCaseID    = "22222222-2222-4222-8222-222222222222"
	annAttProjectID = "33333333-3333-4333-8333-333333333333"
)

var pdfBytes = []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")

func dataURI(mime string, b []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// fakeAttachFanOut stands in for the case service that copies files onto cases.
type fakeAttachFanOut struct {
	unsupported bool
	// onCase is what SearchCaseAttachments returns, by case id.
	onCase map[string][]domain.Attachment
	// failOn makes CreateCaseAttachmentAs fail for the file with this name.
	failOn    string
	searchErr error

	mu      sync.Mutex
	created []domain.CreateAttachmentRequest
	actors  []string
}

func (f *fakeAttachFanOut) SearchCaseAttachments(_ context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.searchErr != nil {
		return domain.SearchAttachmentsResponse{}, f.searchErr
	}
	list := f.onCase[req.ReferenceID]
	return domain.SearchAttachmentsResponse{Attachments: list, Total: len(list), Limit: req.Pagination.Limit}, nil
}

func (f *fakeAttachFanOut) CreateCaseAttachmentAs(_ context.Context, req domain.CreateAttachmentRequest, actorEmail string) (domain.CreateAttachmentResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != "" && req.Name == f.failOn {
		return domain.CreateAttachmentResponse{}, errors.New("servicenow unavailable")
	}
	f.created = append(f.created, req)
	f.actors = append(f.actors, actorEmail)
	return domain.CreateAttachmentResponse{}, nil
}

func (f *fakeAttachFanOut) SupportsAnnouncementAttachments() bool { return !f.unsupported }

func (f *fakeAttachFanOut) createdNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	names := make([]string, len(f.created))
	for i, c := range f.created {
		names[i] = c.Name
	}
	return names
}

func newAttachmentService(repo *fakeAnnouncementRequestRepo, cases *fakeCaseFanOutClient, attach CaseAttachmentFanOutClient) AnnouncementRequestService {
	if cases == nil {
		cases = &fakeCaseFanOutClient{}
	}
	return WithAttachmentFanOut(NewAnnouncementRequestService(repo, cases, stubAccess{scope: AccessScope{Unrestricted: true}}), attach)
}

func TestAnnouncementRequestService_AddAttachment(t *testing.T) {
	good := func() domain.CreateAnnouncementRequestAttachmentRequest {
		return domain.CreateAnnouncementRequestAttachmentRequest{
			ActorID:    "user-1",
			ActorEmail: "user-1@example.com",
			Name:       "Security advisory.pdf",
			Type:       "application/pdf",
			File:       dataURI("application/pdf", pdfBytes),
		}
	}
	stateRepo := func(state domain.AnnouncementRequestState) *fakeAnnouncementRequestRepo {
		return &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{ID: annAttReqID, State: state}}
	}

	t.Run("stores a PDF on a draft as a plain change", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStateDraft)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		got, err := svc.AddAttachment(context.Background(), annAttReqID, good())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotAttachmentExpected != domain.AnnouncementRequestStateDraft || repo.gotAttachmentRevert {
			t.Fatalf("expected a plain change conditioned on draft, got expected=%q revert=%v", repo.gotAttachmentExpected, repo.gotAttachmentRevert)
		}
		in := repo.gotAddAttachment
		if in.Name != "Security advisory.pdf" || in.Type != "application/pdf" || in.SizeBytes != len(pdfBytes) || string(in.Content) != string(pdfBytes) {
			t.Fatalf("expected the file stored with its name, canonical type, size and bytes, got %+v", in.AnnouncementRequestAttachment)
		}
		if in.CreatedBy != "user-1" || in.CreatedByEmail == nil || *in.CreatedByEmail != "user-1@example.com" {
			t.Fatalf("expected the actor recorded, got by=%q email=%v", in.CreatedBy, in.CreatedByEmail)
		}
		if got.ID == "" || got.Name != "Security advisory.pdf" {
			t.Fatalf("expected the saved metadata back, got %+v", got)
		}
	})

	t.Run("a request in review goes back to draft, like an edit of its content", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStatePendingApproval)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		if _, err := svc.AddAttachment(context.Background(), annAttReqID, good()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.gotAttachmentRevert || repo.gotAttachmentExpected != domain.AnnouncementRequestStatePendingApproval {
			t.Fatalf("expected revert-to-draft from pending_approval, got expected=%q revert=%v", repo.gotAttachmentExpected, repo.gotAttachmentRevert)
		}
	})

	t.Run("an approved request is changed in place", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStateApproved)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		if _, err := svc.AddAttachment(context.Background(), annAttReqID, good()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotAttachmentRevert || repo.gotAttachmentExpected != domain.AnnouncementRequestStateApproved {
			t.Fatalf("expected an in-place change conditioned on approved, got expected=%q revert=%v", repo.gotAttachmentExpected, repo.gotAttachmentRevert)
		}
	})

	t.Run("a published request cannot be changed", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStatePublished)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		_, err := svc.AddAttachment(context.Background(), annAttReqID, good())
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("expected *apierror.ConflictError, got %T: %v", err, err)
		}
		if repo.addAttachmentCalls != 0 {
			t.Fatal("a published request's files must not reach the repository")
		}
	})

	t.Run("the declared type is advisory: the name decides it", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStateDraft)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		req := good()
		req.Type = "text/html"
		if _, err := svc.AddAttachment(context.Background(), annAttReqID, req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotAddAttachment.Type != "application/pdf" {
			t.Fatalf("expected the stored type derived from the .pdf name, got %q", repo.gotAddAttachment.Type)
		}
	})

	t.Run("accepts every supported type with matching content", func(t *testing.T) {
		files := map[string][]byte{
			"a.pdf":  pdfBytes,
			"b.png":  []byte("\x89PNG\r\n\x1a\nrest"),
			"c.jpg":  {0xFF, 0xD8, 0xFF, 0xE0, 0x00},
			"d.JPEG": {0xFF, 0xD8, 0xFF, 0xE0, 0x00},
			"e.gif":  []byte("GIF89a....."),
			"f.docx": []byte("PK\x03\x04docx"),
			"g.xlsx": []byte("PK\x03\x04xlsx"),
			"h.pptx": []byte("PK\x03\x04pptx"),
			"i.doc":  {0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00},
			"j.xls":  {0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00},
			"k.ppt":  {0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00},
			"l.txt":  []byte("plain text, naïve"),
			"m.csv":  []byte("a,b\n1,2\n"),
		}
		for name, content := range files {
			t.Run(name, func(t *testing.T) {
				svc := newAttachmentService(stateRepo(domain.AnnouncementRequestStateDraft), nil, &fakeAttachFanOut{})
				req := good()
				req.Name = name
				req.File = dataURI("application/octet-stream", content)
				if _, err := svc.AddAttachment(context.Background(), annAttReqID, req); err != nil {
					t.Fatalf("expected %s accepted, got %v", name, err)
				}
			})
		}
	})

	t.Run("rejects what it must", func(t *testing.T) {
		big := make([]byte, domain.MaxAnnouncementRequestAttachmentBytes+1)
		copy(big, pdfBytes)
		cases := map[string]func(r *domain.CreateAnnouncementRequestAttachmentRequest){
			"an executable": func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "setup.exe" },
			"a script":      func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "run.sh" },
			"html":          func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "page.html" },
			"an archive":    func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "bundle.zip" },
			"no extension":  func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "advisory" },
			"a double extension that ends in a script": func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "advisory.pdf.exe" },
			"a name with a path":                       func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "../advisory.pdf" },
			"a name with a backslash path":             func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = `C:\advisory.pdf` },
			"a name with a control character":          func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "advis\x00ory.pdf" },
			"a blank name":                             func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "   " },
			"a too long name":                          func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = strings.Repeat("a", 256) + ".pdf" },
			"a .pdf that is really a PNG": func(r *domain.CreateAnnouncementRequestAttachmentRequest) {
				r.File = dataURI("application/pdf", []byte("\x89PNG\r\n\x1a\n"))
			},
			"a .pdf that is an executable": func(r *domain.CreateAnnouncementRequestAttachmentRequest) {
				r.File = dataURI("application/pdf", []byte("MZ\x90\x00 not a pdf"))
			},
			"a .docx that is not a zip": func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.Name = "x.docx" },
			"a .txt with a NUL byte": func(r *domain.CreateAnnouncementRequestAttachmentRequest) {
				r.Name = "x.txt"
				r.File = dataURI("text/plain", []byte("a\x00b"))
			},
			"a .txt that is not UTF-8": func(r *domain.CreateAnnouncementRequestAttachmentRequest) {
				r.Name = "x.txt"
				r.File = dataURI("text/plain", []byte{0xff, 0xfe, 0xfd})
			},
			"an empty file":                 func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.File = dataURI("application/pdf", nil) },
			"no file":                       func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.File = "" },
			"a file that is not a data URI": func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.File = "JVBERi0=" },
			"invalid base64":                func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.File = "data:application/pdf;base64,@@@" },
			"a file over 10 MB":             func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.File = dataURI("application/pdf", big) },
			"a missing actor":               func(r *domain.CreateAnnouncementRequestAttachmentRequest) { r.ActorID = " " },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				repo := stateRepo(domain.AnnouncementRequestStateDraft)
				svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
				req := good()
				mutate(&req)
				_, err := svc.AddAttachment(context.Background(), annAttReqID, req)
				var validation *apierror.ValidationError
				if !errors.As(err, &validation) {
					t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
				}
				if repo.addAttachmentCalls != 0 {
					t.Fatal("a refused file must not reach the repository")
				}
			})
		}
	})

	t.Run("rejects an id that is not a UUID", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStateDraft)
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		if _, err := svc.AddAttachment(context.Background(), "not-a-uuid", good()); err == nil {
			t.Fatal("expected a validation error")
		}
	})

	t.Run("a data source that cannot send the file refuses to store it", func(t *testing.T) {
		for name, attach := range map[string]CaseAttachmentFanOutClient{
			"unsupported": &fakeAttachFanOut{unsupported: true},
			"none":        nil,
		} {
			t.Run(name, func(t *testing.T) {
				repo := stateRepo(domain.AnnouncementRequestStateDraft)
				svc := newAttachmentService(repo, nil, attach)
				_, err := svc.AddAttachment(context.Background(), annAttReqID, good())
				var unavailable *apierror.ServiceUnavailableError
				if !errors.As(err, &unavailable) {
					t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
				}
				if repo.addAttachmentCalls != 0 {
					t.Fatal("nothing may be stored when the file could never be sent")
				}
			})
		}
	})

	t.Run("propagates a repository refusal (limit, duplicate name, state race)", func(t *testing.T) {
		repo := stateRepo(domain.AnnouncementRequestStateDraft)
		repo.addAttachmentErr = &apierror.ValidationError{Msg: "a request can carry at most 5 files"}
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		if _, err := svc.AddAttachment(context.Background(), annAttReqID, good()); err == nil {
			t.Fatal("expected the repository's refusal")
		}
	})
}

func TestAnnouncementRequestService_DeleteAttachment(t *testing.T) {
	attID := "44444444-4444-4444-8444-444444444444"
	for state, want := range map[domain.AnnouncementRequestState]struct{ revert bool }{
		domain.AnnouncementRequestStateDraft:           {false},
		domain.AnnouncementRequestStatePendingApproval: {true},
		domain.AnnouncementRequestStateApproved:        {false},
	} {
		t.Run(string(state), func(t *testing.T) {
			repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{ID: annAttReqID, State: state}}
			svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
			if err := svc.DeleteAttachment(context.Background(), annAttReqID, attID, "user-1"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.gotAttachmentRevert != want.revert || repo.gotAttachmentExpected != state || repo.gotDeleteAttachmentID != attID {
				t.Fatalf("unexpected repo call: expected=%q revert=%v id=%q", repo.gotAttachmentExpected, repo.gotAttachmentRevert, repo.gotDeleteAttachmentID)
			}
		})
	}

	t.Run("a published request cannot be changed", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{ID: annAttReqID, State: domain.AnnouncementRequestStatePublished}}
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		err := svc.DeleteAttachment(context.Background(), annAttReqID, attID, "user-1")
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) || repo.deleteAttachmentCalls != 0 {
			t.Fatalf("expected a conflict without touching the repository, got %v (calls=%d)", err, repo.deleteAttachmentCalls)
		}
	})

	t.Run("rejects a bad id and a missing actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{ID: annAttReqID, State: domain.AnnouncementRequestStateDraft}}
		svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})
		if err := svc.DeleteAttachment(context.Background(), annAttReqID, "nope", "user-1"); err == nil {
			t.Fatal("expected a validation error for a non-UUID attachment id")
		}
		if err := svc.DeleteAttachment(context.Background(), annAttReqID, attID, ""); err == nil {
			t.Fatal("expected a validation error for a missing actor")
		}
	})
}

func TestAnnouncementRequestService_ListAndContent(t *testing.T) {
	repo := &fakeAnnouncementRequestRepo{
		getResult: domain.AnnouncementRequest{ID: annAttReqID, State: domain.AnnouncementRequestStateDraft},
		attachments: []domain.AnnouncementRequestAttachmentContent{
			{AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{ID: "a1", Name: "one.pdf"}, Content: pdfBytes},
		},
	}
	svc := newAttachmentService(repo, nil, &fakeAttachFanOut{})

	list, err := svc.ListAttachments(context.Background(), annAttReqID)
	if err != nil || len(list.Attachments) != 1 || list.Attachments[0].Name != "one.pdf" {
		t.Fatalf("expected the one file, got %+v err=%v", list, err)
	}

	repo.getErr = &apierror.NotFoundError{Msg: "no such request"}
	if _, err := svc.ListAttachments(context.Background(), annAttReqID); err == nil {
		t.Fatal("expected a NotFoundError for a missing request, not an empty list")
	}
	repo.getErr = nil

	if _, err := svc.GetAttachmentContent(context.Background(), annAttReqID, "not-a-uuid"); err == nil {
		t.Fatal("expected a validation error for a non-UUID attachment id")
	}
}

func TestAnnouncementRequestService_AttachToCase(t *testing.T) {
	actor := domain.AnnouncementRequestActorRequest{ActorID: "user-1", ActorEmail: "user-1@example.com"}
	announcementView := func(projectID string) domain.CaseView {
		kind := "announcement"
		return domain.CaseView{ID: annAttCaseID, Type: &kind, ProjectDetails: &domain.EntityRef{ID: projectID}}
	}
	files := func() []domain.AnnouncementRequestAttachmentContent {
		return []domain.AnnouncementRequestAttachmentContent{
			{AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{ID: "a1", Name: "one.pdf", Type: "application/pdf", SizeBytes: len(pdfBytes)}, Content: pdfBytes},
			{AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{ID: "a2", Name: "two.pdf", Type: "application/pdf", SizeBytes: len(pdfBytes)}, Content: pdfBytes},
		}
	}
	approved := func() domain.AnnouncementRequest {
		return domain.AnnouncementRequest{ID: annAttReqID, State: domain.AnnouncementRequestStateApproved, ResolvedProjectIDs: []string{annAttProjectID}}
	}

	t.Run("copies every file onto a case of a resolved project, as the actor", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)

		got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Attached != 2 || got.AlreadyAttached != 0 {
			t.Fatalf("expected 2 attached, got %+v", got)
		}
		if names := attach.createdNames(); strings.Join(names, ",") != "one.pdf,two.pdf" {
			t.Fatalf("expected both files in order, got %v", names)
		}
		first := attach.created[0]
		if first.ReferenceID != annAttCaseID || first.ReferenceType != domain.ReferenceTypeCase || first.Type != "application/pdf" {
			t.Fatalf("unexpected create request: %+v", first)
		}
		if first.File != dataURI("application/pdf", pdfBytes) {
			t.Fatal("expected the file's bytes sent as a base64 data URI")
		}
		if attach.actors[0] != "user-1@example.com" {
			t.Fatalf("expected the file attached as the actor, got %q", attach.actors[0])
		}
	})

	t.Run("is idempotent: files the case already has are left alone", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}
		attach := &fakeAttachFanOut{onCase: map[string][]domain.Attachment{
			annAttCaseID: {{Name: "ONE.pdf", SizeBytes: len(pdfBytes)}},
		}}
		svc := newAttachmentService(repo, cases, attach)

		got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Attached != 1 || got.AlreadyAttached != 1 {
			t.Fatalf("expected one attached and one already there, got %+v", got)
		}
		if names := attach.createdNames(); strings.Join(names, ",") != "two.pdf" {
			t.Fatalf("expected only the missing file attached, got %v", names)
		}
	})

	t.Run("a same-named file of another size is not the same file", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()[:1]}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}
		attach := &fakeAttachFanOut{onCase: map[string][]domain.Attachment{
			annAttCaseID: {{Name: "one.pdf", SizeBytes: 5}},
		}}
		svc := newAttachmentService(repo, cases, attach)
		got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err != nil || got.Attached != 1 {
			t.Fatalf("expected the file attached, got %+v err=%v", got, err)
		}
	})

	t.Run("a failure stops at the file that failed, keeps what was attached and says so", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}
		attach := &fakeAttachFanOut{failOn: "two.pdf"}
		svc := newAttachmentService(repo, cases, attach)

		got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err == nil || !strings.Contains(err.Error(), "two.pdf") {
			t.Fatalf("expected an error naming the failed file, got %v", err)
		}
		if got.Attached != 1 {
			t.Fatalf("expected the first file reported attached, got %+v", got)
		}
		// The retry then attaches only what is missing.
		attach.failOn = ""
		attach.onCase = map[string][]domain.Attachment{annAttCaseID: {{Name: "one.pdf", SizeBytes: len(pdfBytes)}}}
		got, err = svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err != nil || got.Attached != 1 || got.AlreadyAttached != 1 {
			t.Fatalf("expected the retry to finish the job, got %+v err=%v", got, err)
		}
	})

	t.Run("nothing to copy is not an error and touches no case", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved()}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)
		got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		if err != nil || got.Attached != 0 || len(attach.created) != 0 {
			t.Fatalf("expected a no-op, got %+v err=%v created=%v", got, err, attach.created)
		}
	})

	t.Run("refuses a case outside the request's resolved projects", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		other := "55555555-5555-4555-8555-555555555555"
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(other)}}
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)
		_, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		var validation *apierror.ValidationError
		if !errors.As(err, &validation) || len(attach.created) != 0 {
			t.Fatalf("expected a validation error and nothing attached, got %v created=%v", err, attach.created)
		}
	})

	t.Run("refuses a case that is not an announcement", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		kind := "case"
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: {ID: annAttCaseID, Type: &kind, ProjectDetails: &domain.EntityRef{ID: annAttProjectID}}}}
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)
		if _, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor); err == nil || len(attach.created) != 0 {
			t.Fatalf("expected a refusal and nothing attached, got %v created=%v", err, attach.created)
		}
	})

	t.Run("refuses a case the caller cannot read", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		cases := &fakeCaseFanOutClient{} // no views: GetCaseByID answers not found
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)
		_, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		var notFound *apierror.NotFoundError
		if !errors.As(err, &notFound) || len(attach.created) != 0 {
			t.Fatalf("expected a NotFoundError and nothing attached, got %v created=%v", err, attach.created)
		}
	})

	t.Run("a draft's files go only to its recorded dry-run case", func(t *testing.T) {
		dry := annAttCaseID
		draft := domain.AnnouncementRequest{ID: annAttReqID, State: domain.AnnouncementRequestStateDraft, DryRunCaseID: &dry}
		cases := &fakeCaseFanOutClient{caseViews: map[string]domain.CaseView{annAttCaseID: announcementView(annAttProjectID)}}

		repo := &fakeAnnouncementRequestRepo{getResult: draft, attachments: files()}
		attach := &fakeAttachFanOut{}
		svc := newAttachmentService(repo, cases, attach)
		if got, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor); err != nil || got.Attached != 2 {
			t.Fatalf("expected the dry-run case to take the files, got %+v err=%v", got, err)
		}

		// Any other case, or no recorded dry run, is refused.
		otherDry := "66666666-6666-4666-8666-666666666666"
		draft.DryRunCaseID = &otherDry
		repo = &fakeAnnouncementRequestRepo{getResult: draft, attachments: files()}
		attach = &fakeAttachFanOut{}
		svc = newAttachmentService(repo, cases, attach)
		_, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) || len(attach.created) != 0 {
			t.Fatalf("expected a conflict for a case that is not the dry run, got %v created=%v", err, attach.created)
		}
		draft.DryRunCaseID = nil
		repo = &fakeAnnouncementRequestRepo{getResult: draft, attachments: files()}
		svc = newAttachmentService(repo, cases, attach)
		if _, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor); !errors.As(err, &conflict) {
			t.Fatalf("expected a conflict with no recorded dry run, got %v", err)
		}
	})

	t.Run("only a draft or an approved request attaches to cases", func(t *testing.T) {
		for _, state := range []domain.AnnouncementRequestState{domain.AnnouncementRequestStatePendingApproval, domain.AnnouncementRequestStatePublished} {
			t.Run(string(state), func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: domain.AnnouncementRequest{ID: annAttReqID, State: state}, attachments: files()}
				attach := &fakeAttachFanOut{}
				svc := newAttachmentService(repo, &fakeCaseFanOutClient{}, attach)
				_, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
				var conflict *apierror.ConflictError
				if !errors.As(err, &conflict) || len(attach.created) != 0 {
					t.Fatalf("expected a conflict, got %v created=%v", err, attach.created)
				}
			})
		}
	})

	t.Run("validates its input and its data source", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: approved(), attachments: files()}
		svc := newAttachmentService(repo, &fakeCaseFanOutClient{}, &fakeAttachFanOut{})
		if _, err := svc.AttachToCase(context.Background(), "x", annAttCaseID, actor); err == nil {
			t.Fatal("expected a validation error for a bad request id")
		}
		if _, err := svc.AttachToCase(context.Background(), annAttReqID, "x", actor); err == nil {
			t.Fatal("expected a validation error for a bad case id")
		}
		if _, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, domain.AnnouncementRequestActorRequest{ActorEmail: "a@b.c"}); err == nil {
			t.Fatal("expected a validation error for a missing actor id")
		}
		if _, err := svc.AttachToCase(context.Background(), annAttReqID, annAttCaseID, domain.AnnouncementRequestActorRequest{ActorID: "u"}); err == nil {
			t.Fatal("expected a validation error for a missing actor email")
		}
		unsupported := newAttachmentService(repo, &fakeCaseFanOutClient{}, &fakeAttachFanOut{unsupported: true})
		_, err := unsupported.AttachToCase(context.Background(), annAttReqID, annAttCaseID, actor)
		var unavailable *apierror.ServiceUnavailableError
		if !errors.As(err, &unavailable) {
			t.Fatalf("expected ServiceUnavailableError on an unsupported data source, got %v", err)
		}
	})
}

func TestAnnouncementRequestService_AutoPublishWithAttachments(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	internal := stubAccess{scope: AccessScope{Unrestricted: true}}
	files := func() []domain.AnnouncementRequestAttachmentContent {
		return []domain.AnnouncementRequestAttachmentContent{
			{AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{ID: "a1", Name: "one.pdf", Type: "application/pdf", SizeBytes: len(pdfBytes)}, Content: pdfBytes},
		}
	}
	due := func() domain.AnnouncementRequest {
		return domain.AnnouncementRequest{
			ID:                 annAttReqID,
			State:              domain.AnnouncementRequestStateApproved,
			CreatedBy:          "user-3",
			CreatedByEmail:     strPtr("user-3@example.com"),
			Subject:            "Maintenance",
			Description:        "Details",
			ResolvedProjectIDs: []string{"proj-1", "proj-2"},
			ScheduledFor:       &past,
		}
	}
	with := func(repo *fakeAnnouncementRequestRepo, cases *fakeCaseFanOutClient, attach CaseAttachmentFanOutClient) AnnouncementRequestService {
		return WithAttachmentFanOut(NewAnnouncementRequestService(repo, cases, internal), attach)
	}

	t.Run("copies the files onto every case it creates, as the request's creator", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: due(), attachments: files()}
		cases := &fakeCaseFanOutClient{}
		attach := &fakeAttachFanOut{}
		got, err := with(repo, cases, attach).AutoPublish(context.Background(), annAttReqID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != domain.AnnouncementRequestStatePublished {
			t.Fatalf("expected published, got %+v", got)
		}
		if len(attach.created) != 2 {
			t.Fatalf("expected one file per created case (2), got %d", len(attach.created))
		}
		seen := map[string]bool{}
		for i, c := range attach.created {
			seen[c.ReferenceID] = true
			if attach.actors[i] != "user-3@example.com" {
				t.Fatalf("expected the request's creator as the actor, got %q", attach.actors[i])
			}
		}
		if len(seen) != 2 {
			t.Fatalf("expected the two different cases, got %v", seen)
		}
		if len(cases.taggedCases) != 0 {
			t.Fatalf("a non-security announcement is not tagged, got %v", cases.taggedCases)
		}
	})

	t.Run("a security announcement gets its tag and its files", func(t *testing.T) {
		req := due()
		req.IsSecurityAnnouncement = true
		repo := &fakeAnnouncementRequestRepo{getResult: req, attachments: files()}
		cases := &fakeCaseFanOutClient{}
		attach := &fakeAttachFanOut{}
		if _, err := with(repo, cases, attach).AutoPublish(context.Background(), annAttReqID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.taggedCases) != 2 || len(attach.created) != 2 {
			t.Fatalf("expected a tag and a file on both cases, got tags=%v files=%d", cases.taggedCases, len(attach.created))
		}
	})

	t.Run("a file that fails to attach blocks publish and is recorded for retry, the case is not recreated", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: due(), attachments: files()}
		cases := &fakeCaseFanOutClient{}
		attach := &fakeAttachFanOut{failOn: "one.pdf"}
		_, err := with(repo, cases, attach).AutoPublish(context.Background(), annAttReqID)
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("expected a conflict (retry next tick), got %T: %v", err, err)
		}
		if repo.gotPublishID != "" {
			t.Fatal("a request with a case missing its files must not be marked published")
		}
		failed := 0
		for _, d := range repo.gotUpsertDeliveriesReq {
			if d.Status == domain.AnnouncementRequestDeliveryStatusTagFailed && d.CaseID != nil {
				failed++
			}
		}
		if failed != 2 {
			t.Fatalf("expected both projects recorded as tag_failed with their real case, got %+v", repo.gotUpsertDeliveriesReq)
		}
	})

	t.Run("retries a tag_failed delivery by attaching the files to the existing case", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{
			getResult:   due(),
			attachments: files(),
			listDeliveriesResult: []domain.AnnouncementRequestDelivery{
				{ProjectID: "proj-1", CaseID: strPtr("case-a"), Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
				{ProjectID: "proj-2", CaseID: strPtr("case-b"), Status: domain.AnnouncementRequestDeliveryStatusTagFailed},
			},
		}
		cases := &fakeCaseFanOutClient{}
		attach := &fakeAttachFanOut{}
		if _, err := with(repo, cases, attach).AutoPublish(context.Background(), annAttReqID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.createdCases) != 0 {
			t.Fatalf("no case may be recreated, got %v", cases.createdCases)
		}
		if len(attach.created) != 1 || attach.created[0].ReferenceID != "case-b" {
			t.Fatalf("expected the files attached to the tag_failed case only, got %+v", attach.created)
		}
	})

	t.Run("refuses a request with files when this data source cannot send them, without fanning out", func(t *testing.T) {
		for name, attach := range map[string]CaseAttachmentFanOutClient{"unsupported": &fakeAttachFanOut{unsupported: true}, "none": nil} {
			t.Run(name, func(t *testing.T) {
				repo := &fakeAnnouncementRequestRepo{getResult: due(), attachments: files()}
				cases := &fakeCaseFanOutClient{}
				_, err := with(repo, cases, attach).AutoPublish(context.Background(), annAttReqID)
				var unavailable *apierror.ServiceUnavailableError
				if !errors.As(err, &unavailable) || len(cases.createdCases) != 0 || repo.claimCalled {
					t.Fatalf("expected a refusal before claiming or creating anything, got %v created=%v claimed=%v", err, cases.createdCases, repo.claimCalled)
				}
			})
		}
	})

	t.Run("refuses files with no recorded creator email to attach them as", func(t *testing.T) {
		req := due()
		req.CreatedByEmail = nil
		repo := &fakeAnnouncementRequestRepo{getResult: req, attachments: files()}
		cases := &fakeCaseFanOutClient{}
		_, err := with(repo, cases, &fakeAttachFanOut{}).AutoPublish(context.Background(), annAttReqID)
		var conflict *apierror.ConflictError
		if !errors.As(err, &conflict) || len(cases.createdCases) != 0 {
			t.Fatalf("expected a conflict before any case is created, got %v", err)
		}
	})

	t.Run("a request with no files behaves exactly as before, whatever the data source", func(t *testing.T) {
		repo := &fakeAnnouncementRequestRepo{getResult: due()}
		cases := &fakeCaseFanOutClient{}
		if _, err := with(repo, cases, nil).AutoPublish(context.Background(), annAttReqID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cases.createdCases) != 2 {
			t.Fatalf("expected both cases created, got %d", len(cases.createdCases))
		}
	})
}

func TestAnnouncementAttachmentFileNaming(t *testing.T) {
	// Pins the extension table against drift from the error message's list.
	for ext := range announcementAttachmentTypes {
		if !strings.Contains(announcementAttachmentAllowedExtensions, strings.TrimPrefix(ext, ".")) {
			t.Errorf("extension %s is accepted but missing from the allowed-extensions message", ext)
		}
	}
	listed := strings.Split(announcementAttachmentAllowedExtensions, ", ")
	if len(listed) != len(announcementAttachmentTypes) {
		t.Errorf("the message lists %d extensions, the table has %d", len(listed), len(announcementAttachmentTypes))
	}
	_ = fmt.Sprintf
}
