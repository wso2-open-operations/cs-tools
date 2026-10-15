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
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// announcementAttachmentsUnavailableMsg is what a data source that cannot copy
// a request's files onto cases says. It is shared by the service (which refuses
// to take a file nobody could send) and the plain Postgres case service.
const announcementAttachmentsUnavailableMsg = "announcement attachments are not available on this data source: a case's files are held in ServiceNow, and this deployment does not use it"

// caseAttachmentListPageSize is the page size when the files already on a case
// are listed to find which of a request's files it has. A case holds a handful
// of files, so this is one page in practice; the loop pages anyway.
const caseAttachmentListPageSize = 50

// CaseAttachmentFanOutClient is the narrow subset of an attachment-capable
// CaseService that copying a request's files onto a case needs. It is its own
// interface, rather than more methods on caseFanOutClient or CaseService,
// because only the data sources that keep a case's files in ServiceNow
// implement it for real (the plain Postgres case service answers
// SupportsAnnouncementAttachments false), and because a caller that has no
// token on its context -- the scheduled publish -- must be able to name who
// acts, which CaseService.CreateCaseAttachment cannot take (it resolves the
// actor from x-user-id-token).
type CaseAttachmentFanOutClient interface {
	// SearchCaseAttachments lists a case's files; used to find which of a
	// request's files a case already has, so a retry never attaches one twice.
	SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error)
	// CreateCaseAttachmentAs attaches one file (req.File, a base64 data URI) to
	// the case req.ReferenceID as actorEmail.
	CreateCaseAttachmentAs(ctx context.Context, req domain.CreateAttachmentRequest, actorEmail string) (domain.CreateAttachmentResponse, error)
	// SupportsAnnouncementAttachments reports whether this data source can
	// hold a copy of a request's files on a case at all.
	SupportsAnnouncementAttachments() bool
}

// WithAttachmentFanOut gives an AnnouncementRequestService the client it copies
// a request's files onto cases with. It is a post-construction step, not a
// constructor parameter, so every NewAnnouncementRequestService call site that
// has nothing to do with attachments keeps compiling and behaving as before --
// the same shape as WithProductCategoryEnforcement. With no client set, the
// service refuses to take a file (a file nobody could send is not stored) and
// a request that somehow has files is not published past them (AutoPublish).
func WithAttachmentFanOut(svc AnnouncementRequestService, client CaseAttachmentFanOutClient) AnnouncementRequestService {
	if s, ok := svc.(*announcementRequestService); ok {
		s.attach = client
	}
	return svc
}

// requireAttachmentSupport refuses an operation that only makes sense where a
// request's files can be sent.
func (s *announcementRequestService) requireAttachmentSupport() error {
	if s.attach == nil || !s.attach.SupportsAnnouncementAttachments() {
		return &apierror.ServiceUnavailableError{Msg: announcementAttachmentsUnavailableMsg}
	}
	return nil
}

// attachmentChangeMode decides how an attachment change interacts with the
// request's state, exactly as an edit of its content does (see Update):
//   - draft: a plain change.
//   - pending_approval: the request is out for review, so changing its files
//     sends it back to draft, clearing the audience snapshot and the dry run
//     (revertToDraft). A silent change under the reviewer is not safe, and the
//     dry-run case no longer shows what is about to be sent.
//   - approved: allowed in place, the same accepted trade-off a post-approval
//     edit of the text is. The dry-run case does not carry the change.
//   - published: refused; the cases it created are the record from then on.
func attachmentChangeMode(current domain.AnnouncementRequest) (expected domain.AnnouncementRequestState, revertToDraft bool, err error) {
	switch current.State {
	case domain.AnnouncementRequestStateDraft, domain.AnnouncementRequestStateApproved:
		return current.State, false, nil
	case domain.AnnouncementRequestStatePendingApproval:
		return current.State, true, nil
	default:
		return "", false, &apierror.ConflictError{Msg: "the files of a published announcement request cannot be changed"}
	}
}

// AddAttachment implements AnnouncementRequestService.
//
// The file's type comes from its name, not from the type the caller declares:
// the extension must be one this feature accepts (a PDF above all) and the
// bytes must be that kind of file, so a renamed executable cannot ride along.
func (s *announcementRequestService) AddAttachment(ctx context.Context, id string, req domain.CreateAnnouncementRequestAttachmentRequest) (domain.AnnouncementRequestAttachment, error) {
	if err := s.requireAttachmentSupport(); err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AnnouncementRequestAttachment{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	name, fileType, err := validateAnnouncementAttachmentName(req.Name)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	content, err := decodeAnnouncementAttachmentFile(req.File)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	if err := checkAnnouncementAttachmentContent(name, fileType.kind, content); err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	expected, revert, err := attachmentChangeMode(current)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}

	var email *string
	if e := strings.TrimSpace(req.ActorEmail); e != "" {
		email = &e
	}
	return s.repo.AddAttachment(ctx, id, expected, revert, domain.AnnouncementRequestAttachmentContent{
		AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{
			Name:           name,
			Type:           fileType.mime,
			SizeBytes:      len(content),
			CreatedBy:      req.ActorID,
			CreatedByEmail: email,
		},
		Content: content,
	})
}

// DeleteAttachment implements AnnouncementRequestService.
func (s *announcementRequestService) DeleteAttachment(ctx context.Context, id, attachmentID, actorID string) error {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return err
	}
	if err := validateUUIDs("attachmentId", []string{attachmentID}); err != nil {
		return err
	}
	if strings.TrimSpace(actorID) == "" {
		return &apierror.ValidationError{Msg: "actorId is required"}
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	expected, revert, err := attachmentChangeMode(current)
	if err != nil {
		return err
	}
	return s.repo.DeleteAttachment(ctx, id, attachmentID, expected, revert)
}

// ListAttachments implements AnnouncementRequestService.
func (s *announcementRequestService) ListAttachments(ctx context.Context, id string) (domain.SearchAnnouncementRequestAttachmentsResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SearchAnnouncementRequestAttachmentsResponse{}, err
	}
	// Confirms the request exists (a NotFoundError, not an empty list, for a
	// bad id) before listing what may legitimately be zero files.
	if _, err := s.repo.Get(ctx, id); err != nil {
		return domain.SearchAnnouncementRequestAttachmentsResponse{}, err
	}
	attachments, err := s.repo.ListAttachments(ctx, id)
	if err != nil {
		return domain.SearchAnnouncementRequestAttachmentsResponse{}, err
	}
	return domain.SearchAnnouncementRequestAttachmentsResponse{Attachments: attachments}, nil
}

// GetAttachmentContent implements AnnouncementRequestService.
func (s *announcementRequestService) GetAttachmentContent(ctx context.Context, id, attachmentID string) (domain.AnnouncementRequestAttachmentContent, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AnnouncementRequestAttachmentContent{}, err
	}
	if err := validateUUIDs("attachmentId", []string{attachmentID}); err != nil {
		return domain.AnnouncementRequestAttachmentContent{}, err
	}
	return s.repo.GetAttachmentContent(ctx, id, attachmentID)
}

// AttachToCase implements AnnouncementRequestService.
//
// This is the browser-driven publish's way of giving each case it just created
// the request's files: the bytes never leave the server, so a large audience
// does not mean the same PDF uploaded from the browser once per project. It
// accepts only a case the request legitimately owns: an announcement case that
// is the request's recorded dry run (while it is a draft) or that belongs to one
// of its resolved projects (while it is approved), so a caller holding a request
// id cannot use it to put the request's files on any other case.
//
// Idempotent: a file the case already has (same name and size) is left alone,
// so a retry after a partial failure only attaches what is missing.
func (s *announcementRequestService) AttachToCase(ctx context.Context, id, caseID string, req domain.AnnouncementRequestActorRequest) (domain.AttachAnnouncementRequestAttachmentsToCaseResponse, error) {
	if err := s.requireAttachmentSupport(); err != nil {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, err
	}
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, err
	}
	if err := validateUUIDs("caseId", []string{caseID}); err != nil {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, err
	}
	if strings.TrimSpace(req.ActorID) == "" {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, &apierror.ValidationError{Msg: "actorId is required"}
	}
	if strings.TrimSpace(req.ActorEmail) == "" {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, &apierror.ValidationError{Msg: "actorEmail is required to attach files"}
	}

	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, err
	}
	if err := s.requireOwnedCase(ctx, current, caseID); err != nil {
		return domain.AttachAnnouncementRequestAttachmentsToCaseResponse{}, err
	}
	return s.attachFilesToCase(ctx, id, caseID, req.ActorEmail)
}

// requireOwnedCase refuses a case the request has no business putting its files
// on (see AttachToCase).
func (s *announcementRequestService) requireOwnedCase(ctx context.Context, current domain.AnnouncementRequest, caseID string) error {
	switch current.State {
	case domain.AnnouncementRequestStateDraft:
		if current.DryRunCaseID == nil || *current.DryRunCaseID != caseID {
			return &apierror.ConflictError{Msg: "while a request is a draft its files can only be attached to its recorded dry-run case"}
		}
	case domain.AnnouncementRequestStateApproved:
		// Checked below against the case's own project.
	default:
		return &apierror.ConflictError{Msg: "files can only be attached to a case while the request is a draft or approved, not " + string(current.State)}
	}

	view, err := s.cases.GetCaseByID(ctx, caseID)
	if err != nil {
		return err
	}
	if view.Type == nil || *view.Type != "announcement" {
		return &apierror.ValidationError{Msg: "caseId must be an announcement case"}
	}
	if current.State == domain.AnnouncementRequestStateApproved {
		if view.ProjectDetails == nil {
			return &apierror.ValidationError{Msg: "caseId must belong to one of the request's resolved projects"}
		}
		for _, projectID := range current.ResolvedProjectIDs {
			if projectID == view.ProjectDetails.ID {
				return nil
			}
		}
		return &apierror.ValidationError{Msg: "caseId must belong to one of the request's resolved projects"}
	}
	return nil
}

// attachFilesToCase copies every file of the request onto the case that does
// not have it yet, one at a time and in the order they were attached. It stops
// at the first file that fails: what was attached stays, and a retry skips it.
// The files are read one at a time too, so a request is never held in memory
// whole.
func (s *announcementRequestService) attachFilesToCase(ctx context.Context, requestID, caseID, actorEmail string) (domain.AttachAnnouncementRequestAttachmentsToCaseResponse, error) {
	var result domain.AttachAnnouncementRequestAttachmentsToCaseResponse
	files, err := s.repo.ListAttachments(ctx, requestID)
	if err != nil {
		return result, err
	}
	if len(files) == 0 {
		return result, nil
	}
	if err := s.requireAttachmentSupport(); err != nil {
		return result, err
	}

	onCase, err := s.listCaseAttachments(ctx, caseID)
	if err != nil {
		return result, err
	}
	for _, f := range files {
		if onCase[attachmentKey(f.Name, f.SizeBytes)] {
			result.AlreadyAttached++
			continue
		}
		withBytes, err := s.repo.GetAttachmentContent(ctx, requestID, f.ID)
		if err != nil {
			return result, err
		}
		_, err = s.attach.CreateCaseAttachmentAs(ctx, domain.CreateAttachmentRequest{
			ReferenceID:   caseID,
			ReferenceType: domain.ReferenceTypeCase,
			Name:          f.Name,
			Type:          f.Type,
			File:          buildAnnouncementAttachmentDataURI(f.Type, withBytes.Content),
		}, actorEmail)
		if err != nil {
			slog.ErrorContext(ctx, "announcement request: copying a file onto a case failed",
				"announcementRequestID", requestID, "caseId", caseID, "file", f.Name, "error", err)
			return result, fmt.Errorf("attach %q to case %s: %w", f.Name, caseID, err)
		}
		result.Attached++
	}
	return result, nil
}

// attachmentKey identifies a file on a case. Name and size are what the case
// keeps of it and what the request's own copy shares, which is enough here: a
// request refuses two files of one name, so within one announcement a name and
// size is one file.
func attachmentKey(name string, size int) string {
	return fmt.Sprintf("%d:%s", size, strings.ToLower(name))
}

// listCaseAttachments returns the key of every file the case already has.
func (s *announcementRequestService) listCaseAttachments(ctx context.Context, caseID string) (map[string]bool, error) {
	have := map[string]bool{}
	for offset := 0; ; {
		page, err := s.attach.SearchCaseAttachments(ctx, domain.SearchAttachmentsRequest{
			ReferenceID:   caseID,
			ReferenceType: domain.ReferenceTypeCase,
			Pagination:    domain.Pagination{Limit: caseAttachmentListPageSize, Offset: offset},
		})
		if err != nil {
			return nil, err
		}
		for _, a := range page.Attachments {
			have[attachmentKey(a.Name, a.SizeBytes)] = true
		}
		if !page.HasMore || len(page.Attachments) == 0 {
			return have, nil
		}
		offset += len(page.Attachments)
	}
}
