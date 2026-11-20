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
}

// NewCaseCommentTemplateService constructs a CaseCommentTemplateService.
func NewCaseCommentTemplateService(repo repository.CaseCommentTemplateRepository, userRepo repository.UserRepository) CaseCommentTemplateService {
	return &caseCommentTemplateService{repo: repo, userRepo: userRepo}
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

// The shared methods still require a user token: the editor is recorded as
// created_by/updated_by. Who may write them (an admin) is decided by the CSM
// portal backend, which holds the portal role vocabulary this service does not.

func (s *caseCommentTemplateService) CreateShared(ctx context.Context, req domain.CreateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	_, email, err := s.caller(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.create(ctx, nil, email, req, domain.MaxSharedCaseCommentTemplates)
}

func (s *caseCommentTemplateService) UpdateShared(ctx context.Context, id string, req domain.UpdateCaseCommentTemplateRequest) (domain.CaseCommentTemplate, error) {
	_, email, err := s.caller(ctx)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	return s.update(ctx, id, nil, email, req)
}

func (s *caseCommentTemplateService) DeleteShared(ctx context.Context, id string) error {
	if _, _, err := s.caller(ctx); err != nil {
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
	n, err := s.repo.Count(ctx, ownerUserID)
	if err != nil {
		return domain.CaseCommentTemplate{}, err
	}
	if n >= limit {
		return domain.CaseCommentTemplate{}, &apierror.ValidationError{Msg: fmt.Sprintf("at most %d templates are allowed here; delete one first", limit)}
	}
	req.Name = name
	return s.repo.Create(ctx, ownerUserID, req, actorEmail)
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
