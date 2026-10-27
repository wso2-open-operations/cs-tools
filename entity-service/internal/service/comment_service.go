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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// commentTypeToEnum maps the CommentType values this Postgres data source can
// represent to comment_type_enum's labels (migration 0040: APPROVAL_HISTORY,
// COMMENT, WORK_NOTE). CommentTypeActivity maps to APPROVAL_HISTORY for
// filtering/read-back, but CreateComment refuses to write one: APPROVAL_HISTORY
// only ever arises from ServiceNow's own audit trail, never from a
// caller-authored comment.
var commentTypeToEnum = map[domain.CommentType]string{
	domain.CommentTypeComment:  "COMMENT",
	domain.CommentTypeWorkNote: "WORK_NOTE",
	domain.CommentTypeActivity: "APPROVAL_HISTORY",
}

// commentCreatedByAgent is CreateCommentRequest.CreatedBy's one legitimate
// override value -- see that field's own doc comment. CreateComment refuses
// any other non-empty value rather than trusting it at face value.
const commentCreatedByAgent = "agent"

// commentAgentDisplayName is the author name a comment written by
// commentCreatedByAgent reads back with. ServiceNow resolved the agent to its
// "Novera" user, and the customer portal relies on that: it marks a chat
// message as the assistant's only when the author's name is "Novera". There
// is no "user" row for "agent" on this data source, so without this the
// assistant's saved replies come back unattributed and render as the
// customer's own messages when a chat is resumed.
const commentAgentDisplayName = "Novera"

// commentAdminRoleName is the role.name (migration 0008's seed data, joined
// through user_role) that grants a caller admin-level visibility into a
// soft-deleted comment's real content -- see resolveCommentCallerVisibility,
// the one remaining user of this constant. Not used for the
// UpdateComment/DeleteComment/GetCommentEditHistory authorization decision --
// that path performs no authorization of its own at all (see GetComment's
// own doc comment for why) -- confirmed against this service's own existing
// role checks elsewhere (migration 0011's recompute_user_type trigger and
// user_repo_test.go both use the literal "admin"), not the CSM portal
// backend's own DefaultRoles vocabulary (a different layer,
// apps/csm-portal/backend/internal/directory/roles.go), which happens to use
// the same spelling but is not the source of truth here.
const commentAdminRoleName = "admin"

// commentDeletedPlaceholder replaces a soft-deleted comment's real content
// for a non-admin internal caller -- see SearchComments' own doc comment for
// the full visibility rule.
const commentDeletedPlaceholder = "[deleted]"

// commentEnumToType is commentTypeToEnum's read-back inverse.
var commentEnumToType = map[string]domain.CommentType{
	"COMMENT":          domain.CommentTypeComment,
	"WORK_NOTE":        domain.CommentTypeWorkNote,
	"APPROVAL_HISTORY": domain.CommentTypeActivity,
}

type commentService struct {
	repo     repository.CommentRepository
	userRepo repository.UserRepository
	// snWriteback/snMirror back CreateComment's best-effort, asynchronous
	// ServiceNow mirror write under DATA_SOURCE=postgres-servicenow-dual-write
	// -- both nil in every other mode. Set only via
	// NewCommentServiceWithSNWriteback. snMirror is the ServiceNow-backed
	// CommentService (from NewServiceNowCommentService); its CreateComment is
	// already a bare POST with no GET-before-write or notification side
	// effects (unlike case's own comment mirror, which needed a dedicated
	// CreateBareCaseComment for exactly that reason -- see that method's own
	// doc comment), so it is called directly here rather than through a
	// narrower interface.
	snWriteback *SNWritebackDispatcher
	snMirror    CommentService
	// imageLinker attaches the comment images a saved comment shows (nil: none).
	imageLinker InlineImageLinker
}

// WithCommentInlineImageLinker attaches the linker that, when a comment is
// created or edited, attaches the comment images it shows to it. A no-op if svc
// is not the Postgres-backed comment service or l is nil.
func WithCommentInlineImageLinker(svc CommentService, l InlineImageLinker) CommentService {
	if cs, ok := svc.(*commentService); ok && l != nil {
		cs.imageLinker = l
	}
	return svc
}

// NewCommentService constructs a CommentService backed by Postgres.
func NewCommentService(repo repository.CommentRepository, userRepo repository.UserRepository) CommentService {
	return &commentService{repo: repo, userRepo: userRepo}
}

// NewCommentServiceWithSNWriteback is NewCommentService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs: CreateComment dispatches
// a best-effort, asynchronous ServiceNow mirror write onto mirror after the
// Postgres write commits -- see CreateComment's own doc comment. A separate
// constructor rather than extending NewCommentService's own signature, same
// reasoning as NewCaseServiceWithSNWriteback's own doc comment: every other
// call site keeps working completely unchanged.
func NewCommentServiceWithSNWriteback(repo repository.CommentRepository, userRepo repository.UserRepository, dispatcher *SNWritebackDispatcher, mirror CommentService) CommentService {
	return &commentService{repo: repo, userRepo: userRepo, snWriteback: dispatcher, snMirror: mirror}
}

func commentRowToDomain(row repository.CommentRow) domain.Comment {
	c := domain.Comment{
		ID:           row.ID,
		ReferenceID:  row.WorkItemID,
		Content:      domain.StripCommentCodeMarkers(row.Content),
		CreatedOn:    row.CreatedOn,
		LastEditedOn: row.LastEditedAt,
		IsDeleted:    row.DeletedAt != nil,
		// comment.created_by (migration 0040) is a free-text VARCHAR, not a
		// foreign key into "user" -- it mirrors ServiceNow's sys_journal_field
		// author string, which can be an integration/automation account with
		// no local user row. This data source writes the resolved caller's
		// email into it (see CreateComment below); CommentRow.CreatedByName
		// is SearchComments' own best-effort resolution of that email against
		// "user" (empty when no row matches, e.g. an integration/automation
		// account), matching the display name case_repo.go's
		// SearchCaseActivities already resolves for the same comment on the
		// case activity timeline -- there is no id to attach either way.
		CreatedBy: domain.NewUserReference("", row.CreatedBy, commentAuthorName(row)),
	}
	if row.Type != nil {
		if t, ok := commentEnumToType[*row.Type]; ok {
			c.Type = t
		}
	}
	return c
}

// commentAuthorName is row.CreatedByName, except that the Novera agent, which
// has no "user" row to resolve against, is named commentAgentDisplayName. The
// agent is stored either as the "agent" sentinel (written by this service) or
// as its display name (carried over verbatim on migrated records).
func commentAuthorName(row repository.CommentRow) string {
	if row.CreatedByName == "" &&
		(strings.EqualFold(row.CreatedBy, commentCreatedByAgent) || strings.EqualFold(row.CreatedBy, commentAgentDisplayName)) {
		return commentAgentDisplayName
	}
	return row.CreatedByName
}

// commentCallerVisibility is the resolved caller identity SearchComments needs
// to decide, per comment row, whether a soft-deleted comment's content should
// be shown, redacted, or the row excluded entirely. See SearchComments' own
// doc comment for the rule this implements. Zero value (IsCustomer: false,
// IsAdmin: false) is the "internal, non-admin" behavior -- deliberately also
// the fallback when no identity can be resolved at all (missing/invalid
// x-user-id-token, or an email with no matching "user" row): this endpoint
// has no per-caller access scoping today (see this service's own CLAUDE.md,
// "Not yet wired"), and a machine-to-machine caller with no end-user token in
// the loop is far more likely than an actual, unauthenticated customer
// reaching this endpoint directly -- excluding rows outright for an
// unresolved identity would be a bigger behavior break for those existing
// callers than showing a redacted placeholder.
type commentCallerVisibility struct {
	IsCustomer bool
	IsAdmin    bool
}

// resolveCommentCallerVisibility never returns an error: every failure mode
// (no token, malformed token, unknown email, repository error) degrades to
// the conservative zero value rather than failing the whole search -- see
// commentCallerVisibility's own doc comment for why.
func (s *commentService) resolveCommentCallerVisibility(ctx context.Context) commentCallerVisibility {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return commentCallerVisibility{}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return commentCallerVisibility{}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return commentCallerVisibility{}
	}
	if user.UserType == domain.UserTypeCustomer || user.UserType == domain.UserTypeExternal {
		return commentCallerVisibility{IsCustomer: true}
	}
	isAdmin := false
	if roles, err := s.userRepo.GetUserRoles(ctx, user.ID); err == nil {
		for _, r := range roles {
			if r == commentAdminRoleName {
				isAdmin = true
				break
			}
		}
	}
	return commentCallerVisibility{IsAdmin: isAdmin}
}

// SearchComments implements CommentService.
//
// Visibility rule for a soft-deleted comment (deleted_at IS NOT NULL): a
// customer caller never sees the row at all (excluded at the SQL level, via
// excludeDeleted -- see CommentRepository.SearchComments' own doc comment on
// why that can't be a post-query filter); a non-admin internal caller sees
// the row with Content replaced by the literal string "[deleted]"; an admin
// caller sees the real content. A non-deleted comment is unaffected either
// way.
func (s *commentService) SearchComments(ctx context.Context, req domain.SearchCommentsRequest) (domain.SearchCommentsResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchCommentsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	var typeFilter *string
	if req.Filters != nil && req.Filters.Type != nil {
		enumType, ok := commentTypeToEnum[*req.Filters.Type]
		if !ok {
			return domain.SearchCommentsResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(*req.Filters.Type)}
		}
		typeFilter = &enumType
	}

	vis := s.resolveCommentCallerVisibility(ctx)

	rows, total, err := s.repo.SearchComments(ctx, req.ReferenceID, req.ReferenceType, typeFilter, vis.IsCustomer, req.Pagination)
	if err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	comments := make([]domain.Comment, 0, len(rows))
	for _, row := range rows {
		c := commentRowToDomain(row)
		if row.DeletedAt != nil && !vis.IsAdmin {
			c.Content = commentDeletedPlaceholder
		}
		comments = append(comments, c)
	}

	return domain.SearchCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

// CreateComment implements CommentService.
func (s *commentService) CreateComment(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateCommentResponse{}, err
	}
	enumType, ok := commentTypeToEnum[req.Type]
	if !ok || req.Type == domain.CommentTypeActivity {
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}

	// req.CreatedBy is a caller-supplied override, documented on
	// CreateCommentRequest as existing solely to attribute an AI-assistant
	// reply to the assistant rather than to the customer whose token relayed
	// it -- its only legitimate value is the "agent" sentinel. Anything else
	// is refused rather than trusted at face value: honoring an arbitrary
	// caller-supplied string here would let any caller attribute a comment
	// to anyone (identity spoofing), since comment.created_by is a free-text
	// column with no server-side verification of who it names. Every
	// ordinary call (no override) always has createdBy derived from the
	// caller's own authenticated token, never from the request body.
	createdBy := ""
	switch req.CreatedBy {
	case "":
		// fall through to token resolution below
	case commentCreatedByAgent:
		createdBy = commentCreatedByAgent
	default:
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: `createdBy must be "agent" or omitted`}
	}
	if createdBy == "" {
		token := middleware.UserIDTokenFromContext(ctx)
		if token == "" {
			return domain.CreateCommentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		user, err := s.userRepo.GetUserByEmail(ctx, email)
		if err != nil {
			return domain.CreateCommentResponse{}, err
		}
		createdBy = user.Email
	}

	row, err := s.repo.CreateComment(ctx, req.ReferenceID, req.ReferenceType, enumType, req.Content, createdBy)
	if err != nil {
		return domain.CreateCommentResponse{}, err
	}
	if s.imageLinker != nil {
		s.imageLinker.LinkFromContent(ctx, row.ID, row.WorkItemID, createdBy, req.Content)
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (snWriteback/snMirror are both nil otherwise -- see
	// NewCommentServiceWithSNWriteback's own doc comment). Postgres has
	// already committed by this point and is fully authoritative for the
	// comment -- this mirror's only job is making sure ServiceNow's copy of
	// the comment text exists too. req.Type == CommentTypeActivity can never
	// reach here (rejected above), so unlike case's comment mirror there is
	// no type to skip dispatch for.
	if s.snWriteback != nil {
		// req.CreatedBy is "" or "agent" here; ServiceNow resolves the author from the token otherwise.
		mirrorReq := req
		s.snWriteback.Dispatch(ctx, "comment", req.ReferenceID, "create",
			map[string]any{"referenceId": req.ReferenceID, "referenceType": req.ReferenceType, "type": req.Type, "content": req.Content},
			func(writeCtx context.Context) error {
				_, err := s.snMirror.CreateComment(writeCtx, mirrorReq)
				return err
			},
		)
	}

	return domain.CreateCommentResponse{
		Message: "Comment created successfully",
		Comment: domain.CaseCommentDetail{
			ID:        row.ID,
			CreatedOn: row.CreatedOn,
			CreatedBy: createdBy,
		},
	}, nil
}

// resolveCommentActor resolves the authenticated caller's email, recorded as
// the editor/deleter of record on UpdateComment/DeleteComment (and read back
// unauthorized on GetCommentEditHistory -- see that method's own doc
// comment). Unlike resolveCommentCallerVisibility (SearchComments'
// best-effort, never-fails resolution), a write operation requires a real
// identity: a missing token is an UnauthorizedError, a malformed one a
// ValidationError, and an email with no matching "user" row propagates
// GetUserByEmail's own NotFoundError -- the same posture CreateComment
// already takes for its own token resolution above.
//
// This service performs no author-or-role authorization check of its own for
// UpdateComment/DeleteComment/GetCommentEditHistory -- see GetComment's own
// doc comment for why that decision now belongs entirely to csm-portal-backend,
// the sole caller of this path (customer-portal-backend-v2 only ever creates
// comments, never edits/deletes/reads history). This function's only job is
// attribution, not authorization.
func (s *commentService) resolveCommentActor(ctx context.Context) (email string, err error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	email, err = emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

// GetComment returns a single comment by id, with no author/role check of
// its own, on purpose: this exists for csm-portal-backend to call before
// deciding whether to allow an edit/delete -- it needs to know the comment's
// author (CreatedBy) to enforce "cs_engineer may only touch their own
// comment, admin/comment_updater may touch any", and this service has no way
// to express that decision itself (it doesn't know csm-portal-backend's own
// Asgardeo role vocabulary, and dropped its own separate Postgres "admin"
// role check from this path as redundant and a source of confusion against
// it -- see resolveCommentActor's own doc comment). Exposing one comment's
// content this way is not a new disclosure: any internal caller who can see
// this comment at all already sees its full content via
// POST /comments/search, which this service already serves with no stricter
// per-comment gate than row-level security already provides (project
// membership, WORK_NOTE visibility -- migrations 0147/0175/0191).
func (s *commentService) GetComment(ctx context.Context, id string) (domain.Comment, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Comment{}, err
	}
	row, err := s.repo.GetCommentByID(ctx, id)
	if err != nil {
		return domain.Comment{}, err
	}
	return commentRowToDomain(row), nil
}

// UpdateComment implements CommentService. Deliberately NOT mirrored to
// ServiceNow under DATA_SOURCE=postgres-servicenow-dual-write, unlike
// CreateComment: ServiceNow's sys_journal_field is append-only, and
// snCommentSearchService.UpdateComment (the would-be mirror target) already
// unconditionally returns a ServiceUnavailableError documented as a
// permanent platform limitation, not a gap -- "stock ServiceNow does not let
// an agent edit or delete a journal entry either" (see
// commentEditDeleteUnsupportedOnSNMsg's own doc comment). Dispatching a
// mirror here would only ever record a guaranteed sn_writeback_failures row
// on every single edit, forever, which is not useful signal -- there is
// nothing an operator could ever fix on the ServiceNow side to make it
// succeed. This is the comment analogue of Part A's conversation finding:
// no real path exists, so no dead-end plumbing was built for it. Postgres
// remains fully authoritative for comment edits regardless.
func (s *commentService) UpdateComment(ctx context.Context, req domain.UpdateCommentRequest) (domain.UpdateCommentResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateCommentResponse{}, err
	}
	if req.Content == "" {
		return domain.UpdateCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}

	actorEmail, err := s.resolveCommentActor(ctx)
	if err != nil {
		return domain.UpdateCommentResponse{}, err
	}

	row, err := s.repo.UpdateComment(ctx, req.ID, req.Content, actorEmail)
	if err != nil {
		return domain.UpdateCommentResponse{}, err
	}
	if s.imageLinker != nil {
		s.imageLinker.LinkFromContent(ctx, row.ID, row.WorkItemID, actorEmail, req.Content)
	}

	return domain.UpdateCommentResponse{
		Message: "Comment updated successfully",
		Comment: commentRowToDomain(row),
	}, nil
}

// DeleteComment implements CommentService. Deliberately NOT mirrored to
// ServiceNow under DATA_SOURCE=postgres-servicenow-dual-write -- same
// reasoning as UpdateComment's own doc comment above.
func (s *commentService) DeleteComment(ctx context.Context, id string) error {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return err
	}

	actorEmail, err := s.resolveCommentActor(ctx)
	if err != nil {
		return err
	}

	return s.repo.SoftDeleteComment(ctx, id, actorEmail)
}

// GetCommentEditHistory implements CommentService. Performs no author/role
// check of its own, same as UpdateComment/DeleteComment -- see GetComment's
// own doc comment for why that decision belongs to csm-portal-backend now.
// Row-level security still limits comments and their edit history to project
// members and hides WORK_NOTE rows from external callers (migrations 0147,
// 0175, 0191); that boundary is unaffected by this change.
func (s *commentService) GetCommentEditHistory(ctx context.Context, id string) (domain.GetCommentEditHistoryResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.GetCommentEditHistoryResponse{}, err
	}

	if _, err := s.resolveCommentActor(ctx); err != nil {
		return domain.GetCommentEditHistoryResponse{}, err
	}

	// Confirms the comment exists at all, so a bad id is a 404 rather than a
	// silently empty history list.
	if _, err := s.repo.GetCommentByID(ctx, id); err != nil {
		return domain.GetCommentEditHistoryResponse{}, err
	}

	rows, err := s.repo.GetCommentEditHistory(ctx, id)
	if err != nil {
		return domain.GetCommentEditHistoryResponse{}, err
	}

	history := make([]domain.CommentEditHistoryEntry, 0, len(rows))
	for _, row := range rows {
		history = append(history, domain.CommentEditHistoryEntry{
			Body:     row.Body,
			EditedBy: row.EditedBy,
			EditedOn: row.EditedAt,
		})
	}
	return domain.GetCommentEditHistoryResponse{History: history}, nil
}
