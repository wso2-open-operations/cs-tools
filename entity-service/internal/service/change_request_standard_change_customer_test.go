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
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A standard change template is its customer's: a change request is raised from
// it only for one of that customer's projects, and on dual-write that is checked
// before the previous system is called (a refusal after it would strand a record
// there with no PostgreSQL row).

const (
	custExampleCorp = "00000000-0000-0000-0000-000000000301"
	custOtherCorp   = "00000000-0000-0000-0000-000000000302"
	projExampleCorp = "00000000-0000-0000-0000-000000000401"
	projOtherCorp   = "00000000-0000-0000-0000-000000000402"
	projNoCustomer  = "00000000-0000-0000-0000-000000000499"
	stdTemplateID   = "00000000-0000-4000-8000-0000000057d1"
	stdVersionID    = "00000000-0000-4000-8000-0000000057d2"
)

type fakeTemplateCatalogue struct {
	template domain.StandardChangeTemplate
	projects map[string]*domain.EntityRef
}

func (f fakeTemplateCatalogue) GetTemplate(_ context.Context, id string) (*domain.StandardChangeTemplate, error) {
	if !strings.EqualFold(id, f.template.ID) {
		return nil, nil
	}
	t := f.template
	return &t, nil
}

func (f fakeTemplateCatalogue) ProjectAccount(_ context.Context, projectID string) (*domain.EntityRef, error) {
	return f.projects[projectID], nil
}

func exampleCorpCatalogue(customer *domain.EntityRef) fakeTemplateCatalogue {
	low, impact := domain.ChangeRequestRiskLow, domain.ChangeRequestImpactLow
	return fakeTemplateCatalogue{
		template: domain.StandardChangeTemplate{
			ID: stdTemplateID, Number: "STD0000001", Name: "Rotate cluster keys",
			State: domain.StandardChangeTemplateStateActive, Customer: customer,
			CurrentVersion: &domain.StandardChangeTemplateVersion{
				ID: stdVersionID, TemplateID: stdTemplateID, Version: 1,
				StandardChangeContent: domain.StandardChangeContent{
					Subject:            stdp("Rotate keys on {{cluster}}"),
					ImplementationPlan: stdp("<p>Rotate on {{cluster}}</p>"),
					BackoutPlan:        stdp("<p>Restore</p>"),
					TestPlan:           stdp("<p>Check</p>"),
					Risk:               &low, Impact: &impact,
					Parameters: []domain.StandardChangeParameter{{Key: "cluster", Label: "Cluster", Type: "text", Required: true}},
				},
			},
		},
		projects: map[string]*domain.EntityRef{
			projExampleCorp: {ID: custExampleCorp, Name: "Example Corp"},
			projOtherCorp:   {ID: custOtherCorp, Name: "Other Corp"},
			projNoCustomer:  {},
		},
	}
}

func TestChangeRequestService_CreateFromTemplate_CustomerIsCheckedBeforeThePreviousSystem(t *testing.T) {
	standard := domain.ChangeRequestTypeStandard
	raise := func(project *string) domain.CreateChangeRequestRequest {
		return domain.CreateChangeRequestRequest{Type: &standard, ProjectID: project,
			StandardChangeTemplateID: stdp(stdTemplateID), StandardChangeParameters: map[string]string{"cluster": "aks-1"}}
	}
	exampleCorp := &domain.EntityRef{ID: custExampleCorp, Name: "Example Corp"}

	refusals := []struct {
		name     string
		customer *domain.EntityRef
		project  *string
		code     string
		words    string
	}{
		{"another customer's project", exampleCorp, stdp(projOtherCorp), apierror.CodeStandardChangeTemplateOtherCustomer, `"Rotate cluster keys" is Example Corp's and cannot be used for a project of Other Corp`},
		{"no project", exampleCorp, nil, apierror.CodeStandardChangeTemplateOtherCustomer, "projectId is required"},
		{"a blank project", exampleCorp, stdp("  "), apierror.CodeStandardChangeTemplateOtherCustomer, "projectId is required"},
		{"a project with no customer", exampleCorp, stdp(projNoCustomer), apierror.CodeStandardChangeTemplateOtherCustomer, "a project of no customer"},
		{"a template with no customer", nil, stdp(projExampleCorp), apierror.CodeStandardChangeTemplateUnavailable, "has no customer recorded"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			mirror := &stubMirrorChangeRequestService{
				createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
					t.Error("the previous system was called for a template of another customer")
					return domain.CreateChangeRequestResponse{}, nil
				},
			}
			svc := WithStandardChangeTemplates(NewChangeRequestServiceWithSNMirror(&stubChangeRequestRepo{}, stubUserRepo{}, mirror),
				exampleCorpCatalogue(tc.customer))
			_, err := svc.CreateChangeRequest(context.Background(), raise(tc.project))
			var ve *apierror.ValidationError
			var ce *apierror.ConflictError
			got := ""
			switch {
			case errors.As(err, &ve):
				got = ve.Code
			case errors.As(err, &ce):
				got = ce.Code
			default:
				t.Fatalf("err = %v (%T), want the code %s", err, err, tc.code)
			}
			if got != tc.code || !strings.Contains(err.Error(), tc.words) {
				t.Fatalf("err = %q (code %q), want code %q saying %q", err, got, tc.code, tc.words)
			}
		})
	}

	t.Run("an unknown project", func(t *testing.T) {
		svc := WithStandardChangeTemplates(NewChangeRequestServiceWithSNMirror(&stubChangeRequestRepo{}, stubUserRepo{}, &stubMirrorChangeRequestService{}),
			exampleCorpCatalogue(exampleCorp))
		_, err := svc.CreateChangeRequest(context.Background(), raise(stdp("00000000-0000-0000-0000-000000000488")))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.Contains(err.Error(), "does not refer to a project") {
			t.Fatalf("err = %v, want a 400 for an unknown project", err)
		}
		_, err = svc.CreateChangeRequest(context.Background(), raise(stdp("not-a-uuid")))
		if !errors.As(err, &ve) || !strings.Contains(err.Error(), "projectId must be a valid UUID") {
			t.Fatalf("err = %v, want a 400 for a malformed project id", err)
		}
	})

	t.Run("the template's own customer's project reaches the previous system", func(t *testing.T) {
		called := 0
		mirror := &stubMirrorChangeRequestService{
			createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
				called++
				if r.StandardChangeTemplateID != nil || r.ProjectID != nil {
					t.Errorf("the previous system was sent the template or the project: %+v", r)
				}
				resp := domain.CreateChangeRequestResponse{Message: "ok"}
				resp.ChangeRequest.ID, resp.ChangeRequest.Number = testUUID, "CHG0000001"
				return resp, nil
			},
		}
		var toPostgres domain.CreateChangeRequestRequest
		repo := &stubChangeRequestRepo{
			createChangeRequestFromServiceNow: func(_ context.Context, r domain.CreateChangeRequestRequest, _, _, _ string) (domain.CreateChangeRequestResponse, error) {
				toPostgres = r
				return domain.CreateChangeRequestResponse{Message: "ok"}, nil
			},
		}
		svc := WithStandardChangeTemplates(NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror), exampleCorpCatalogue(exampleCorp))
		if _, err := svc.CreateChangeRequest(context.Background(), raise(stdp(projExampleCorp))); err != nil {
			t.Fatalf("raise for the template's customer: %v", err)
		}
		if called != 1 || toPostgres.ProjectID == nil || *toPostgres.ProjectID != projExampleCorp ||
			toPostgres.StandardChangeTemplateVersionID == nil || *toPostgres.StandardChangeTemplateVersionID != stdVersionID {
			t.Fatalf("called %d, PostgreSQL got project %v version %v", called, toPostgres.ProjectID, toPostgres.StandardChangeTemplateVersionID)
		}
	})
}
