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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// caseAttachmentSNMirror is implemented by *snCaseService -- the four
// ServiceNow-facing attachment operations caseAttachmentDualWriteService
// forwards to it (CreateCaseAttachment, GetCaseAttachmentContent,
// DeleteCaseAttachment, UpdateAttachment). Narrower than the full CaseService
// interface for the same reason deploymentSNCreator is (deployment_service.go):
// named for exactly what this type needs, rather than forcing every
// CaseService implementer (including a test double) to also satisfy methods
// it never calls.
type caseAttachmentSNMirror interface {
	CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error)
	GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error)
	DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error)
	UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error)
}

// caseAttachmentDualWriteService implements CaseService for
// DATA_SOURCE=postgres-servicenow-dual-write's case-attachment routes
// (routes.go's caseAttachmentOverrideSvc). It replaces the old "attachments
// are ServiceNow-only, permanently" behavior: file bytes still live only in
// ServiceNow (SFTPGo is never used in this mode), but metadata is now also
// written into and read from Postgres, matching this codebase's
// reads-from-Postgres/writes-go-to-both policy for every other entity.
//
// Design: this type embeds *caseService (the plain, Postgres-backed
// implementation) and overrides only the four methods whose behavior must
// differ (CreateCaseAttachment, ConfirmCaseAttachment, GetCaseAttachmentContent,
// DeleteCaseAttachment, UpdateAttachment -- five, despite the doc summary
// above counting the ServiceNow-facing four; Confirm is overridden too, to a
// 503, not forwarded to ServiceNow). Every other CaseService method --
// SearchCaseAttachments, GetAttachmentByID, and every non-attachment method
// (UpdateCase, SearchCaseActivities, tags, feedback, ...) -- forwards to the
// embedded *caseService unchanged via Go's method promotion. Composition,
// not copied logic: SearchCaseAttachments/GetAttachmentByID already do
// exactly what this mode wants (read Postgres; GetAttachmentByID's Content
// stays nil, same as the plain Postgres data source, since bytes never live
// in Postgres for a dual-write row either -- see CreateCaseAttachmentFromServiceNow's
// own doc comment for why storage_key is NULL instead).
//
// routes.go never calls this type's other (non-attachment) methods --
// attachmentHandler only registers the six attachment routes against it, see
// routes.go's activeAttachmentSvc -- but they must still be correct, since
// they satisfy the CaseService interface and nothing prevents a future
// caller from reaching them; promotion from the embedded *caseService (which
// IS fully wired with the same repo/userRepo/etc. activeCaseSvc uses) makes
// that automatic rather than a maintenance trap.
type caseAttachmentDualWriteService struct {
	*caseService
	// snMirror is snCaseMirrorSvc, reused as-is from routes.go's other three
	// dual-write mirror purposes (CreateCase, patchCaseFields,
	// CreateBareCaseComment) -- see routes.go's own doc comment on
	// snCaseMirrorSvc for the full picture.
	snMirror caseAttachmentSNMirror
	// snWriteback dispatches DeleteCaseAttachment/UpdateAttachment's
	// best-effort, asynchronous ServiceNow mirror writes -- the single
	// shared SNWritebackDispatcher instance, same as every other dual-write
	// service's identically-named field.
	snWriteback *SNWritebackDispatcher
}

// NewCaseAttachmentDualWriteService constructs the CaseService routes.go's
// DataSourcePostgresServiceNowDualWrite case assigns to caseAttachmentOverrideSvc.
//
// base must be the exact *caseService instance activeCaseSvc resolves to in
// that same branch (service.NewCaseServiceWithSNWriteback's return value) --
// its Postgres-backed attachment reads (SearchCaseAttachments,
// GetAttachmentByID) and its resolveActor/repo are reused unmodified via
// embedding; see the type's own doc comment. mirror is snCaseMirrorSvc,
// already constructed there for the other three dual-write mirror purposes.
// dispatcher is the single shared SNWritebackDispatcher.
func NewCaseAttachmentDualWriteService(base CaseService, mirror caseAttachmentSNMirror, dispatcher *SNWritebackDispatcher) CaseService {
	plain, ok := base.(*caseService)
	if !ok {
		// Cannot happen with the real constructor -- routes.go always passes
		// activeCaseSvc, which in DataSourcePostgresServiceNowDualWrite mode
		// is always the *caseService NewCaseServiceWithSNWriteback returns.
		// A panic here is a startup-time wiring bug, not a runtime condition
		// a request could trigger.
		panic(fmt.Sprintf("service.NewCaseAttachmentDualWriteService: base is a %T, not *caseService", base))
	}
	return &caseAttachmentDualWriteService{caseService: plain, snMirror: mirror, snWriteback: dispatcher}
}

// CreateCaseAttachment implements CaseService for
// DATA_SOURCE=postgres-servicenow-dual-write: ServiceNow-FIRST and
// SYNCHRONOUS, exactly like caseService.createCaseSNFirst and
// deploymentService.createDeploymentSNFirst and for the same reason -- a
// Postgres-first create could leave a Postgres metadata row with no
// ServiceNow counterpart (an attachment Postgres thinks exists but whose
// bytes were never actually uploaded anywhere) if the mirror write then
// failed. Calling ServiceNow first, and only writing to Postgres once that
// succeeds, makes that impossible.
//
// The actor is resolved BEFORE calling ServiceNow, not after: this data
// source's case_attachment.uploaded_by is a real FK to "user"(id) (unlike
// deployment.created_by/deployed_product.created_by, which are free-text
// VARCHAR columns ServiceNow's own createdBy string can populate directly),
// so a Postgres user record must exist for the caller before there's any
// point attempting the ServiceNow upload at all -- failing fast here avoids
// creating a ServiceNow attachment that Postgres could never record.
//
// A deployment-referenced attachment skips both the actor resolution and the
// Postgres insert entirely -- same "no Postgres deployment attachment table
// yet" gap SearchCaseAttachments already works around (see that method's own
// doc comment). case_attachment.case_id has a hard FK into "case", so a
// deployment id can never satisfy it; attempting the insert below failed
// every deployment attachment upload with a 23503 foreign-key violation,
// reported back to the caller as an error even though ServiceNow had already
// accepted it -- the live-reported symptom was "the upload doesn't show as
// submitted, but shows up after a page refresh" (a refresh re-reads via
// SearchCaseAttachments' own, already-correct ServiceNow fallback). There is
// nothing to resolve an actor for or write to Postgres on this path, so the
// ServiceNow response is returned directly.
func (s *caseAttachmentDualWriteService) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if req.ReferenceType == domain.ReferenceTypeDeployment {
		return s.snMirror.CreateCaseAttachment(ctx, req)
	}

	user, err := s.resolveActor(ctx)
	if err != nil {
		return domain.CreateAttachmentResponse{}, err
	}

	snResp, err := s.snMirror.CreateCaseAttachment(ctx, req)
	if err != nil {
		// ServiceNow never accepted the attachment -- nothing is written to
		// Postgres at all, by construction (s.repo.CreateCaseAttachmentFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateAttachmentResponse{}, err
	}

	a, err := s.repo.CreateCaseAttachmentFromServiceNow(ctx, req, snResp.Attachment.ID, snResp.Attachment.SizeBytes, user.ID, snResp.Attachment.CreatedOn)
	if err != nil {
		// ServiceNow already has the attachment at this point -- this is now
		// real drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather than
		// only returned, same convention as createDeploymentSNFirst's and
		// createDeployedProductSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create case attachment: ServiceNow attachment created but the Postgres metadata insert failed",
			"attachmentId", snResp.Attachment.ID, "caseId", req.ReferenceID, "error", err)
		return domain.CreateAttachmentResponse{}, err
	}

	return domain.CreateAttachmentResponse{
		Message: snResp.Message,
		Attachment: domain.AttachmentDetail{
			ID:        a.ID,
			SizeBytes: a.SizeBytes,
			CreatedOn: a.CreatedOn,
			CreatedBy: user.Email,
			// DownloadURL comes from ServiceNow's own reply, not the Postgres
			// row: this service holds no bytes for this row (storage_key is
			// NULL), so Postgres has no download location of its own -- only
			// ServiceNow does.
			DownloadURL: snResp.Attachment.DownloadURL,
			Status:      a.Status,
		},
	}, nil
}

// ConfirmCaseAttachment implements CaseService. Under
// DATA_SOURCE=postgres-servicenow-dual-write, every attachment create is
// synchronous and immediately 'complete' (see CreateCaseAttachment above) --
// there is no pending/in-progress upload state to confirm, exactly as
// already true for the plain ServiceNow data source (snCaseService.ConfirmCaseAttachment).
// Overridden here rather than inherited from the embedded *caseService,
// whose own ConfirmCaseAttachment implements the real pending->complete
// transition for the CSM-native (Postgres, SFTPGo-backed) data source --
// that transition has no meaning for a row this mode ever creates.
func (s *caseAttachmentDualWriteService) ConfirmCaseAttachment(_ context.Context, _ string) (domain.ConfirmAttachmentResponse, error) {
	return domain.ConfirmAttachmentResponse{}, &apierror.ServiceUnavailableError{Msg: "confirming an attachment is not supported for this data source: ServiceNow's attachment upload is synchronous, there is no pending upload to confirm"}
}

// GetCaseAttachmentContent implements CaseService. The actual file bytes
// live only in ServiceNow in this mode (see CreateCaseAttachment's own doc
// comment), so this forwards straight to snMirror -- which already converts
// attachmentID to the real ServiceNow sys_id via uuidToSysid internally
// (sn_case_service.go), and that round-trips correctly because
// CreateCaseAttachmentFromServiceNow stores id = sysidToUUID(the real sys_id)
// for every row this mode creates, the same identity convention
// DataSource=servicenow itself relies on. No extra conversion needed here.
func (s *caseAttachmentDualWriteService) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	return s.snMirror.GetCaseAttachmentContent(ctx, attachmentID)
}

// DeleteCaseAttachment implements CaseService. Unlike CreateCaseAttachment,
// this is Postgres-first with an asynchronous ServiceNow mirror -- the same
// shape as every other dual-write UPDATE/DELETE (e.g.
// deployedProductService.UpdateDeployedProduct): a failed async mirror write
// here just means ServiceNow's copy of an attachment Postgres has already
// deleted is stale until retried, not a permanent orphan the way a failed
// async CREATE would be.
//
// The embedded *caseService.DeleteCaseAttachment is called directly (not
// s.DeleteCaseAttachment, which would recurse into this very method): it
// already does everything the Postgres half of this needs -- UUID
// validation, the authentication check, and the repository delete.
func (s *caseAttachmentDualWriteService) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	resp, err := s.caseService.DeleteCaseAttachment(ctx, req)
	if err != nil {
		return domain.DeleteAttachmentResponse{}, err
	}

	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "case_attachment", req.AttachmentID, "delete", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.DeleteCaseAttachment(writeCtx, req)
				return err
			},
		)
	}

	return resp, nil
}

// UpdateAttachment implements CaseService. Same Postgres-first/async-mirror
// shape as DeleteCaseAttachment above, for the same reason -- for a "case"
// reference, where the embedded *caseService.UpdateAttachment enforces this
// data source's existing rename rule (reference type "case" only,
// description forbidden).
//
// A deployment-referenced attachment skips the Postgres path entirely and
// goes straight to ServiceNow, synchronously -- same "no Postgres
// deployment attachment table yet" gap CreateCaseAttachment/GetAttachmentByID/
// DeleteCaseAttachment already work around (see CreateCaseAttachment's own
// doc comment). validatePGAttachmentUpdate unconditionally rejects any
// ReferenceType other than "case" ("only 'case' is supported for this data
// source"), so routing a deployment-referenced update through
// *caseService.UpdateAttachment first -- as an earlier revision of this
// method did, reasoning it was just reusing an existing, harmless
// restriction -- made editing a deployment-tab attachment's name 400
// unconditionally, regardless of whether ServiceNow itself would have
// allowed it. There is nothing to write to Postgres or mirror
// asynchronously on this path, so the ServiceNow response is returned
// directly, the same as CreateCaseAttachment's own deployment branch.
func (s *caseAttachmentDualWriteService) UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	if req.ReferenceType == domain.ReferenceTypeDeployment {
		return s.snMirror.UpdateAttachment(ctx, req)
	}

	resp, err := s.caseService.UpdateAttachment(ctx, req)
	if err != nil {
		return domain.UpdateAttachmentResponse{}, err
	}

	if s.snWriteback != nil {
		s.snWriteback.Dispatch(ctx, "case_attachment", req.AttachmentID, "update", req,
			func(writeCtx context.Context) error {
				_, err := s.snMirror.UpdateAttachment(writeCtx, req)
				return err
			},
		)
	}

	return resp, nil
}
