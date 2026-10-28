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
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// standardChangeService serves the standard change catalogue and its proposals.
// Every method is for internal (WSO2) callers only; a write also needs the
// caller's own user token, because who did it decides what they may do (the
// proposer edits, a CAB member decides, and nobody decides their own proposal).
type standardChangeService struct {
	repo   repository.StandardChangeRepository
	access AccessService
}

// NewStandardChangeService constructs a StandardChangeService.
func NewStandardChangeService(repo repository.StandardChangeRepository, access AccessService) StandardChangeService {
	return &standardChangeService{repo: repo, access: access}
}

const standardChangeForbidden = "standard change templates are available to WSO2 staff only"

var standardChangeTemplateStates = map[domain.StandardChangeTemplateState]string{
	domain.StandardChangeTemplateStateActive:    "ACTIVE",
	domain.StandardChangeTemplateStateSuspended: "SUSPENDED",
	domain.StandardChangeTemplateStateRetired:   "RETIRED",
}

var standardChangeProposalStates = map[domain.StandardChangeProposalState]string{
	domain.StandardChangeProposalStateDraft:     "DRAFT",
	domain.StandardChangeProposalStateCABReview: "CAB_REVIEW",
	domain.StandardChangeProposalStateApproved:  "APPROVED",
	domain.StandardChangeProposalStateRejected:  "REJECTED",
	domain.StandardChangeProposalStateWithdrawn: "WITHDRAWN",
}

var standardChangeProposalKinds = map[domain.StandardChangeProposalKind]string{
	domain.StandardChangeProposalKindNew:    "NEW",
	domain.StandardChangeProposalKindModify: "MODIFY",
	domain.StandardChangeProposalKindRetire: "RETIRE",
}

// internal checks the caller is WSO2 staff and returns their email ("" for a
// machine caller, which may read but not write).
func (s *standardChangeService) internal(ctx context.Context) (string, error) {
	if err := RequireInternalCaller(ctx, s.access, standardChangeForbidden); err != nil {
		return "", err
	}
	return strings.TrimSpace(auth.IdentityFromContext(ctx).UserEmail), nil
}

// actor is internal plus "a person, not a machine": every write needs one.
func (s *standardChangeService) actor(ctx context.Context) (string, error) {
	email, err := s.internal(ctx)
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	return email, nil
}

func (s *standardChangeService) SearchTemplates(ctx context.Context, req domain.SearchStandardChangeTemplatesRequest) (domain.SearchStandardChangeTemplatesResponse, error) {
	if _, err := s.internal(ctx); err != nil {
		return domain.SearchStandardChangeTemplatesResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchStandardChangeTemplatesResponse{}, err
	}
	states := []string{"ACTIVE", "SUSPENDED"}
	if len(req.States) > 0 {
		states = states[:0]
		for _, st := range req.States {
			v, ok := standardChangeTemplateStates[st]
			if !ok {
				return domain.SearchStandardChangeTemplatesResponse{}, &apierror.ValidationError{Msg: "states must be active, suspended or retired"}
			}
			states = append(states, v)
		}
	}
	q := ""
	if req.Query != nil {
		q = *req.Query
	}
	list, total, err := s.repo.SearchTemplates(ctx, states, q, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchStandardChangeTemplatesResponse{}, err
	}
	return domain.SearchStandardChangeTemplatesResponse{Templates: list, Total: total, Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}, nil
}

func (s *standardChangeService) GetTemplate(ctx context.Context, id string) (domain.StandardChangeTemplate, error) {
	if _, err := s.internal(ctx); err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	t, err := s.repo.GetTemplate(ctx, id)
	if err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	if t == nil {
		return domain.StandardChangeTemplate{}, &apierror.NotFoundError{Msg: "standard change template not found"}
	}
	return *t, nil
}

func (s *standardChangeService) GetTemplateVersion(ctx context.Context, templateID, versionID string) (domain.StandardChangeTemplateVersion, error) {
	if _, err := s.internal(ctx); err != nil {
		return domain.StandardChangeTemplateVersion{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": templateID, "versionId": versionID}); err != nil {
		return domain.StandardChangeTemplateVersion{}, err
	}
	v, err := s.repo.GetVersion(ctx, versionID)
	if err != nil {
		return domain.StandardChangeTemplateVersion{}, err
	}
	if v == nil || !strings.EqualFold(v.TemplateID, templateID) {
		return domain.StandardChangeTemplateVersion{}, &apierror.NotFoundError{Msg: "standard change template version not found"}
	}
	return *v, nil
}

// mayManageTemplate: the template's owner or a CAB member.
func (s *standardChangeService) mayManageTemplate(ctx context.Context, actor, id string) error {
	t, err := s.GetTemplate(ctx, id)
	if err != nil {
		return err
	}
	if t.Owner != nil && strings.EqualFold(t.Owner.Email, actor) {
		return nil
	}
	cab, err := s.repo.IsCABMember(ctx, actor)
	if err != nil {
		return err
	}
	if !cab {
		return &apierror.ForbiddenError{Msg: "only the template's owner or a member of the CAB can do this"}
	}
	return nil
}

func (s *standardChangeService) SuspendTemplate(ctx context.Context, id string) (domain.StandardChangeTemplate, error) {
	return s.moveTemplate(ctx, id, []string{"ACTIVE"}, "SUSPENDED")
}

func (s *standardChangeService) ResumeTemplate(ctx context.Context, id string) (domain.StandardChangeTemplate, error) {
	return s.moveTemplate(ctx, id, []string{"SUSPENDED"}, "ACTIVE")
}

func (s *standardChangeService) moveTemplate(ctx context.Context, id string, from []string, to string) (domain.StandardChangeTemplate, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	if err := s.mayManageTemplate(ctx, actor, id); err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	t, err := s.repo.SetTemplateState(ctx, id, actor, from, to)
	if err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	return *t, nil
}

func (s *standardChangeService) ClearTemplateReview(ctx context.Context, id string) (domain.StandardChangeTemplate, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	if err := s.mayManageTemplate(ctx, actor, id); err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	t, err := s.repo.ClearReviewRequired(ctx, id, actor)
	if err != nil {
		return domain.StandardChangeTemplate{}, err
	}
	return *t, nil
}

func (s *standardChangeService) CreateProposal(ctx context.Context, req domain.CreateStandardChangeProposalRequest) (domain.StandardChangeProposal, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if _, ok := standardChangeProposalKinds[req.Kind]; !ok {
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "kind must be new, modify or retire"}
	}
	ids := map[string]string{}
	if req.TemplateID != nil {
		ids["templateId"] = *req.TemplateID
	}
	if req.SourceChangeRequestID != nil {
		ids["sourceChangeRequestId"] = *req.SourceChangeRequestID
	}
	if req.CopyFromProposalID != nil {
		ids["copyFromProposalId"] = *req.CopyFromProposalID
	}
	if err := checkUUIDFields(ids); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	switch req.Kind {
	case domain.StandardChangeProposalKindNew:
		if req.TemplateID != nil {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "templateId is for a modification or a retirement; a new template has none yet"}
		}
		if req.SourceChangeRequestID != nil && req.CopyFromProposalID != nil {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "start from a change request or copy a proposal, not both"}
		}
	default:
		if req.TemplateID == nil {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "templateId is required to modify or retire a template"}
		}
		if req.SourceChangeRequestID != nil || req.CopyFromProposalID != nil {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "a modification or retirement starts from the template; sourceChangeRequestId and copyFromProposalId are for a new template"}
		}
	}
	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if utf8.RuneCountInString(name) > maxStandardChangeName {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "name must be at most 255 characters"}
		}
	}
	id, err := s.repo.CreateProposal(ctx, actor, repository.StandardChangeProposalCreate{
		Kind: req.Kind, Name: name, TemplateID: lowerPtr(req.TemplateID),
		SourceChangeRequestID: lowerPtr(req.SourceChangeRequestID), CopyFromProposalID: lowerPtr(req.CopyFromProposalID),
	})
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, actor, id)
}

func lowerPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.ToLower(strings.TrimSpace(*s))
	return &v
}

// proposalFor reads a proposal and works out what the caller may do with it.
func (s *standardChangeService) proposalFor(ctx context.Context, viewer, id string) (domain.StandardChangeProposal, error) {
	p, err := s.repo.GetProposal(ctx, id)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if p == nil {
		return domain.StandardChangeProposal{}, &apierror.NotFoundError{Msg: "standard change proposal not found"}
	}
	mine := viewer != "" && p.ProposedBy != nil && strings.EqualFold(p.ProposedBy.Email, viewer)
	p.CanEdit = mine && p.State == domain.StandardChangeProposalStateDraft
	p.CanSubmit = p.CanEdit
	p.CanWithdraw = mine && (p.State == domain.StandardChangeProposalStateDraft || p.State == domain.StandardChangeProposalStateCABReview)
	for i := range p.Approvers {
		a := &p.Approvers[i]
		a.CanDecide = !mine && viewer != "" && p.State == domain.StandardChangeProposalStateCABReview &&
			a.State == "REQUESTED" && a.Approver != nil && strings.EqualFold(a.Approver.Email, viewer)
	}
	return *p, nil
}

func (s *standardChangeService) GetProposal(ctx context.Context, id string) (domain.StandardChangeProposal, error) {
	viewer, err := s.internal(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, viewer, id)
}

func (s *standardChangeService) SearchProposals(ctx context.Context, req domain.SearchStandardChangeProposalsRequest) (domain.SearchStandardChangeProposalsResponse, error) {
	viewer, err := s.internal(ctx)
	if err != nil {
		return domain.SearchStandardChangeProposalsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchStandardChangeProposalsResponse{}, err
	}
	f := repository.StandardChangeProposalFilter{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset}
	for _, st := range req.States {
		v, ok := standardChangeProposalStates[st]
		if !ok {
			return domain.SearchStandardChangeProposalsResponse{}, &apierror.ValidationError{Msg: "states must be draft, cab_review, approved, rejected or withdrawn"}
		}
		f.States = append(f.States, v)
	}
	for _, k := range req.Kinds {
		v, ok := standardChangeProposalKinds[k]
		if !ok {
			return domain.SearchStandardChangeProposalsResponse{}, &apierror.ValidationError{Msg: "kinds must be new, modify or retire"}
		}
		f.Kinds = append(f.Kinds, v)
	}
	if req.TemplateID != nil {
		if err := checkUUIDFields(map[string]string{"templateId": *req.TemplateID}); err != nil {
			return domain.SearchStandardChangeProposalsResponse{}, err
		}
		f.TemplateID = lowerPtr(req.TemplateID)
	}
	if req.ProposedByMe || req.AwaitingMyDecision {
		if viewer == "" {
			return domain.SearchStandardChangeProposalsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		if req.ProposedByMe {
			f.ProposedByEmail = viewer
		}
		if req.AwaitingMyDecision {
			f.AwaitingEmail = viewer
		}
	}
	if req.Query != nil {
		f.Query = *req.Query
	}
	list, total, err := s.repo.SearchProposals(ctx, f)
	if err != nil {
		return domain.SearchStandardChangeProposalsResponse{}, err
	}
	return domain.SearchStandardChangeProposalsResponse{Proposals: list, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

var validStandardChangeCategory = map[domain.ChangeRequestCategory]bool{
	domain.ChangeRequestCategoryHardware: true, domain.ChangeRequestCategorySoftware: true,
	domain.ChangeRequestCategoryService: true, domain.ChangeRequestCategorySystemSoftware: true,
	domain.ChangeRequestCategoryApplicationsSoftware: true, domain.ChangeRequestCategoryNetwork: true,
	domain.ChangeRequestCategoryTelecom: true, domain.ChangeRequestCategoryDocumentation: true,
	domain.ChangeRequestCategoryOther: true, domain.ChangeRequestCategoryRegularReleaseCloud: true,
	domain.ChangeRequestCategoryHotfixReleaseCloud: true, domain.ChangeRequestCategoryDevOps: true,
	domain.ChangeRequestCategoryCloudComputing: true,
}

func (s *standardChangeService) UpdateProposal(ctx context.Context, id string, req domain.UpdateStandardChangeProposalRequest) (domain.StandardChangeProposal, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || utf8.RuneCountInString(n) > maxStandardChangeName {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "name must be 1 to 255 characters"}
		}
		req.Name = &n
	}
	if req.Subject != nil {
		n := strings.TrimSpace(*req.Subject)
		if utf8.RuneCountInString(n) > maxStandardChangeName {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "subject must be at most 255 characters"}
		}
		req.Subject = &n
	}
	if req.Category != nil && *req.Category != "" && !validStandardChangeCategory[*req.Category] {
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "category is not a change request category"}
	}
	if req.Impact != nil && *req.Impact != "" && !validChangeRequestImpact[*req.Impact] {
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "impact must be one of high, medium or low"}
	}
	if req.Risk != nil && *req.Risk != "" && !validChangeRequestRisk[*req.Risk] {
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "risk must be one of high, moderate or low"}
	}
	if req.Parameters != nil {
		params, err := normalizeStandardChangeParameters(*req.Parameters)
		if err != nil {
			return domain.StandardChangeProposal{}, err
		}
		req.Parameters = &params
	}
	if req.EvidenceChangeRequestIDs != nil {
		if len(*req.EvidenceChangeRequestIDs) > 100 {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "a proposal can cite at most 100 change requests as evidence"}
		}
		for i, e := range *req.EvidenceChangeRequestIDs {
			if err := checkUUIDFields(map[string]string{"evidenceChangeRequestIds": e}); err != nil {
				return domain.StandardChangeProposal{}, err
			}
			(*req.EvidenceChangeRequestIDs)[i] = strings.ToLower(e)
		}
	}
	if err := s.repo.UpdateProposal(ctx, id, actor, req); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, actor, id)
}

func (s *standardChangeService) SubmitProposal(ctx context.Context, id string) (domain.StandardChangeProposal, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := s.repo.SubmitProposal(ctx, id, actor, checkStandardChangeProposalComplete); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, actor, id)
}

func (s *standardChangeService) WithdrawProposal(ctx context.Context, id string) (domain.StandardChangeProposal, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := s.repo.WithdrawProposal(ctx, id, actor); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, actor, id)
}

func (s *standardChangeService) DecideProposal(ctx context.Context, id string, req domain.DecideStandardChangeProposalRequest) (domain.StandardChangeProposal, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return domain.StandardChangeProposal{}, err
	}
	if err := checkUUIDFields(map[string]string{"id": id}); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	var approve bool
	switch strings.ToLower(strings.TrimSpace(req.Decision)) {
	case "approved":
		approve = true
	case "rejected":
	default:
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: `decision must be "approved" or "rejected"`}
	}
	var comment *string
	if req.Comment != nil {
		c := strings.TrimSpace(*req.Comment)
		if utf8.RuneCountInString(c) > 4000 {
			return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "comment must be at most 4000 characters"}
		}
		if c != "" {
			comment = &c
		}
	}
	if !approve && comment == nil {
		return domain.StandardChangeProposal{}, &apierror.ValidationError{Msg: "a rejection needs a comment saying why, for the proposal's record"}
	}
	if err := s.repo.DecideProposal(ctx, id, actor, approve, comment); err != nil {
		return domain.StandardChangeProposal{}, err
	}
	return s.proposalFor(ctx, actor, id)
}

// checkUUIDFields refuses any named value that is not a UUID, naming the field.
func checkUUIDFields(fields map[string]string) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !validate.IsUUID(strings.TrimSpace(fields[k])) {
			return &apierror.ValidationError{Msg: k + " must be a valid UUID"}
		}
	}
	return nil
}
