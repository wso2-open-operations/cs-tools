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
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type caseCommentTemplateService struct {
	repo     repository.CaseCommentTemplateRepository
	userRepo repository.UserRepository
	// csmPortalBackendClientID is CSM_PORTAL_BACKEND_CLIENT_ID: the shared
	// templates are written only through that client, which checks the admin
	// role. Empty means nobody can write them.
	csmPortalBackendClientID string
}

// NewCaseCommentTemplateService constructs a CaseCommentTemplateService.
// csmPortalBackendClientID is the only client allowed to write shared templates.
func NewCaseCommentTemplateService(repo repository.CaseCommentTemplateRepository, userRepo repository.UserRepository, csmPortalBackendClientID string) CaseCommentTemplateService {
	return &caseCommentTemplateService{repo: repo, userRepo: userRepo, csmPortalBackendClientID: csmPortalBackendClientID}
}

func (s *caseCommentTemplateService) List(ctx context.Context) (domain.CaseCommentTemplateList, error) {
	userID, _, err := s.caller(ctx)
	if err != nil {
		return domain.CaseCommentTemplateList{}, err
	}
	templates, err := s.repo.List(ctx, userID)
	if err != nil {
		return domain.CaseCommentTemplateList{}, err
	}
	if templates == nil {
		templates = []domain.CaseCommentTemplate{}
	}
	return domain.CaseCommentTemplateList{Templates: templates}, nil
}

func (s *caseCommentTemplateService) CreatePersonal(ctx context.Context, req domain.CreateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	userID, email, err := s.caller(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.create(ctx, &userID, email, req, domain.MaxPersonalCaseCommentTemplates)
}

func (s *caseCommentTemplateService) UpdatePersonal(ctx context.Context, id string, req domain.UpdateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	userID, email, err := s.caller(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.update(ctx, id, &userID, email, req)
}

func (s *caseCommentTemplateService) DeletePersonal(ctx context.Context, id string) error {
	userID, _, err := s.caller(ctx)
	if err != nil {
		return err
	}
	if err := validateCaseCommentTemplateID(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, &userID)
}

// The shared methods need a signed-in person (recorded as created_by /
// updated_by) AND the CSM portal backend's client: that backend admits only an
// admin (PermAdmin), and this service holds no portal role vocabulary to check
// the role itself. Any other internal caller, a person's own token included,
// is refused, the same rule as the portal banners and de-escalator grants.

func (s *caseCommentTemplateService) CreateShared(ctx context.Context, req domain.CreateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	email, err := s.sharedWriter(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.create(ctx, nil, email, req, domain.MaxSharedCaseCommentTemplates)
}

func (s *caseCommentTemplateService) UpdateShared(ctx context.Context, id string, req domain.UpdateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	email, err := s.sharedWriter(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.update(ctx, id, nil, email, req)
}

func (s *caseCommentTemplateService) DeleteShared(ctx context.Context, id string) error {
	if _, err := s.sharedWriter(ctx); err != nil {
		return err
	}
	if err := validateCaseCommentTemplateID(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, nil)
}

func (s *caseCommentTemplateService) create(ctx context.Context, ownerUserID *string, actorEmail string, req domain.CreateCaseCommentTemplateRequest, limit int) (domain.CaseCommentTemplate, error) {
	name, err := validateCaseCommentTemplateName(req.Name)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	if err := validateCaseCommentTemplateBody(req.Body); err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	if err := validateCaseCommentTemplateAppliesTo(req.AppliesTo); err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	req.Name = name
	// The cap is checked inside the insert's own transaction, under a lock per
	// owner, so two requests at once cannot both pass it.
	return s.repo.Create(ctx, ownerUserID, req, actorEmail, limit)
}

func (s *caseCommentTemplateService) update(ctx context.Context, id string, ownerUserID *string, actorEmail string, req domain.UpdateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	if err := validateCaseCommentTemplateID(id); err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	if req.Name == nil && req.Body == nil && req.AppliesTo == nil {
		return domain.CaseCommentTemplate{}, &apierror.ValidationError{Msg: "nothing to update: send name, body or appliesTo"}
	}
	if req.Name != nil {
		name, err := validateCaseCommentTemplateName(*req.Name)
		if err != nil {
			return domain.CaseCommentTemplate{}, err
		}
		req.Name = &name
	}
	if req.Body != nil {
		if err := validateCaseCommentTemplateBody(*req.Body); err != nil {
			return domain.CaseCommentTemplate{}, err
		}
	}
	if req.AppliesTo != nil {
		if err := validateCaseCommentTemplateAppliesTo(*req.AppliesTo); err != nil {
			return domain.CaseCommentTemplate{}, err
		}
	}
	return s.repo.Update(ctx, id, ownerUserID, req, actorEmail)
}

// caller is the CSM user behind the request's validated x-user-id-token: its
// user id (the personal owner) and email (the recorded editor). Never taken
// from the request body.
func (s *caseCommentTemplateService) caller(ctx context.Context) (userID, email string, err error) {
	email = auth.IdentityFromContext(ctx).UserEmail
	if email == "" {
		return "", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}
	return user.ID, email, nil
}

// sharedWriter is caller for the shared writes, plus the CSM portal backend
// client check described above the shared methods.
func (s *caseCommentTemplateService) sharedWriter(ctx context.Context) (string, error) {
	_, email, err := s.caller(ctx)
	if err != nil {
		return "", err
	}
	if s.csmPortalBackendClientID == "" || auth.IdentityFromContext(ctx).ClientID != s.csmPortalBackendClientID {
		return "", &apierror.ForbiddenError{Msg: "shared comment templates can only be changed through the CSM Portal"}
	}
	return email, nil
}

func validateCaseCommentTemplateID(id string) error {
	if !uuidPattern.MatchString(id) {
		return &apierror.ValidationError{Msg: "id must be a UUID"}
	}
	return nil
}

func validateCaseCommentTemplateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", &apierror.ValidationError{Msg: "name is required"}
	}
	if utf8.RuneCountInString(name) > domain.MaxCaseCommentTemplateNameLength {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf("name must be at most %d characters", domain.MaxCaseCommentTemplateNameLength)}
	}
	return name, nil
}

func validateCaseCommentTemplateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return &apierror.ValidationError{Msg: "body is required"}
	}
	if len(body) > domain.MaxCaseCommentTemplateBodyLength {
		return &apierror.ValidationError{Msg: fmt.Sprintf("body must be at most %d bytes", domain.MaxCaseCommentTemplateBodyLength)}
	}
	return nil
}

func validateCaseCommentTemplateAppliesTo(a domain.CommentTemplateAppliesTo) error {
	if !a.Valid() {
		return apierror.InvalidValue("appliesTo", string(a), "template type", []string{"COMMENT", "WORK_NOTE", "BOTH"})
	}
	return nil
}
