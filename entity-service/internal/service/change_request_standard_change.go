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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// standardChangeTemplateReader is what creating a change request from a standard
// change template needs of the catalogue.
type standardChangeTemplateReader interface {
	GetTemplate(ctx context.Context, id string) (*domain.StandardChangeTemplate, error)
}

// WithStandardChangeTemplates makes CreateChangeRequest raise standard changes
// from the standard change catalogue: a standard change then needs a template,
// and the template's current version (with the parameters filled in) is what the
// change request says. svc must be what a NewChangeRequestService* constructor
// returned; any other value is returned unchanged.
func WithStandardChangeTemplates(svc ChangeRequestService, templates standardChangeTemplateReader) ChangeRequestService {
	if s, ok := svc.(*changeRequestService); ok && templates != nil {
		s.standardChanges = templates
	}
	return svc
}

// prepareStandardChange checks a create against the standard change catalogue and,
// for one raised from a template, fills in what the template specifies: the
// rendered content, the version raised from and the parameter values as stored.
// Without a catalogue (WithStandardChangeTemplates not called) a create that
// names no template is returned as it is.
func (s *changeRequestService) prepareStandardChange(ctx context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestRequest, error) {
	standard := req.Type != nil && *req.Type == domain.ChangeRequestTypeStandard
	if req.StandardChangeTemplateID == nil {
		if req.StandardChangeTemplateVersionID != nil || len(req.StandardChangeParameters) > 0 {
			return req, &apierror.ValidationError{Msg: "standardChangeTemplateVersionId and standardChangeParameters need standardChangeTemplateId"}
		}
		if standard && s.standardChanges != nil {
			return req, &apierror.ValidationError{Code: apierror.CodeChangeRequestStandardTemplateField,
				Msg: "a standard change is raised from an approved standard change template: send standardChangeTemplateId (see the Standard Changes catalogue)"}
		}
		return req, nil
	}
	if s.standardChanges == nil {
		return req, &apierror.ValidationError{Msg: "standard change templates are not available on this data source"}
	}
	if req.Type == nil {
		t := domain.ChangeRequestTypeStandard
		req.Type = &t
	} else if !standard {
		return req, &apierror.ValidationError{Code: apierror.CodeChangeRequestStandardTemplateField,
			Msg: fmt.Sprintf("standardChangeTemplateId needs type standard, not %q", *req.Type)}
	}
	ids := map[string]string{"standardChangeTemplateId": *req.StandardChangeTemplateID}
	if req.StandardChangeTemplateVersionID != nil {
		ids["standardChangeTemplateVersionId"] = *req.StandardChangeTemplateVersionID
	}
	for field, id := range ids {
		if !validate.IsUUID(id) {
			return req, &apierror.ValidationError{Msg: field + " must be a valid UUID"}
		}
	}
	if sent := standardTemplateFieldsSent(req); len(sent) > 0 {
		return req, &apierror.ValidationError{Code: apierror.CodeChangeRequestStandardTemplateField,
			Msg: strings.Join(sent, ", ") + " cannot be set: the standard change template specifies them; send only its parameters (standardChangeParameters) and the fields it does not specify"}
	}

	tpl, err := s.standardChanges.GetTemplate(ctx, strings.ToLower(*req.StandardChangeTemplateID))
	if err != nil {
		return req, err
	}
	if tpl == nil {
		return req, &apierror.NotFoundError{Msg: "standard change template not found"}
	}
	switch {
	case tpl.State == domain.StandardChangeTemplateStateSuspended:
		return req, &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("the change request cannot be raised: the standard change template %q is suspended", tpl.Name)}
	case tpl.State != domain.StandardChangeTemplateStateActive || tpl.CurrentVersion == nil:
		return req, &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("the change request cannot be raised: the standard change template %q has been retired", tpl.Name)}
	case req.StandardChangeTemplateVersionID != nil && !strings.EqualFold(*req.StandardChangeTemplateVersionID, tpl.CurrentVersion.ID):
		return req, &apierror.ConflictError{Code: apierror.CodeStandardChangeTemplateUnavailable,
			Msg: fmt.Sprintf("the standard change template %q has been revised (now version %d) since you opened it; read it again", tpl.Name, tpl.CurrentVersion.Version)}
	}

	content, stored, err := renderStandardChange(*tpl.CurrentVersion, req.StandardChangeParameters)
	if err != nil {
		return req, err
	}
	req.Subject = derefString(content.Subject)
	req.Description = content.Description
	req.Justification = content.Justification
	req.ImplementationPlan = content.ImplementationPlan
	req.RiskImpactAnalysis = content.RiskImpactAnalysis
	req.BackoutPlan = content.BackoutPlan
	req.TestPlan = content.TestPlan
	req.Category = content.Category
	req.Impact = content.Impact
	req.Risk = content.Risk
	req.RollbackDurationText = content.RollbackDurationText
	req.IsPlanningVisibleToCustomers = &content.IsPlanningVisibleToCustomers
	req.CustomerApprovalRequired = &content.CustomerApprovalRequired
	req.CustomerReviewRequired = &content.CustomerReviewRequired
	versionID := tpl.CurrentVersion.ID
	req.StandardChangeTemplateVersionID = &versionID
	req.StandardChangeParameters = stored
	return req, nil
}

// refuseBecomingStandard keeps a change request that was not raised from a
// template from becoming a standard change by a PATCH, while the catalogue is in
// use: a standard change is raised from a template. A staff caller only (a
// customer may not set the type at all, which the repository refuses with its own
// words). Resending standard on a change that already is one is accepted. The
// opposite direction (a templated change leaving standard) is the repository's
// (checkStandardChangeTypeEdit).
func (s *changeRequestService) refuseBecomingStandard(ctx context.Context, id string, requested *domain.ChangeRequestType) error {
	if s.standardChanges == nil || requested == nil || *requested != domain.ChangeRequestTypeStandard || repository.IsExternalCaller(ctx) {
		return nil
	}
	cr, err := s.repo.GetChangeRequestByID(ctx, id)
	if err != nil {
		return err
	}
	if cr.StandardChange != nil || (cr.Type != nil && strings.EqualFold(*cr.Type, string(domain.ChangeRequestTypeStandard))) {
		return nil
	}
	return &apierror.ValidationError{Code: apierror.CodeChangeRequestStandardTemplateField,
		Msg: "type cannot be changed to standard: a standard change is raised from a standard change template; raise a new change request from the template"}
}

// standardTemplateFieldsSent names the fields of a create that a template
// specifies and the caller nevertheless sent. The deviation from the
// implementation plan is among them: a new change request raised from a template
// follows it, so it has nothing to carry over.
func standardTemplateFieldsSent(req domain.CreateChangeRequestRequest) []string {
	var sent []string
	add := func(name string, set bool) {
		if set {
			sent = append(sent, name)
		}
	}
	add("subject", strings.TrimSpace(req.Subject) != "")
	add("description", req.Description != nil)
	add("justification", req.Justification != nil)
	add("implementationPlan", req.ImplementationPlan != nil)
	add("riskImpactAnalysis", req.RiskImpactAnalysis != nil)
	add("backoutPlan", req.BackoutPlan != nil)
	add("testPlan", req.TestPlan != nil)
	add("category", req.Category != nil)
	add("impact", req.Impact != nil)
	add("risk", req.Risk != nil)
	add("rollbackDurationText", req.RollbackDurationText != nil)
	add("isPlanningVisibleToCustomers", req.IsPlanningVisibleToCustomers != nil)
	add("customerApprovalRequired", req.CustomerApprovalRequired != nil)
	add("customerReviewRequired", req.CustomerReviewRequired != nil)
	add("implementationDeviation", req.ImplementationDeviation != nil)
	return sent
}
