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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func categoryPtr(s string) *string { return &s }

// TestCaseService_CreateCase_RejectsDeployedProductCategoryOutsideAllowList
// covers the SR path (service_request, matching ProjectFeatures.SrProductCategories):
// a project type restricted to ["ms","pc"], a deployed product with a
// different category ("cl") -- must be rejected before repo.CreateCase is
// ever reached (no createCase stub set -- a call would panic).
func TestCaseService_CreateCase_RejectsDeployedProductCategoryOutsideAllowList(t *testing.T) {
	referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
		SrProductCategories: []string{"MS", "PC"},
	}}
	deployedProductRepo := &stubDeployedProductRepo{
		getDeployedProductCategory: func(context.Context, string) (*string, error) {
			return categoryPtr("cl"), nil
		},
	}
	svc := WithProductCategoryEnforcement(
		NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		referenceDataRepo, deployedProductRepo,
	)

	req := validServiceRequestCreateCaseRequest()
	req.CreatedBy = "system"
	_, err := svc.CreateCase(context.Background(), req)

	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("CreateCase err = %v, want *apierror.ValidationError", err)
	}
}

// TestCaseService_CreateCase_RejectsNilDeployedProductCategoryWhenRestricted
// is the fail-closed case explicitly chosen over fail-open: a deployed
// product with NO category set at all must still be rejected once its
// project type restricts the request's type, even though
// SearchDeployedProducts' own read-side filter treats a NULL category as a
// wildcard match -- that precedent deliberately does not carry over to this
// creation-time enforcement (see validateDeployedProductCategoryForType's
// own doc comment).
func TestCaseService_CreateCase_RejectsNilDeployedProductCategoryWhenRestricted(t *testing.T) {
	referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
		SrProductCategories: []string{"MS", "PC"},
	}}
	deployedProductRepo := &stubDeployedProductRepo{
		getDeployedProductCategory: func(context.Context, string) (*string, error) {
			return nil, nil
		},
	}
	svc := WithProductCategoryEnforcement(
		NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		referenceDataRepo, deployedProductRepo,
	)

	req := validServiceRequestCreateCaseRequest()
	req.CreatedBy = "system"
	_, err := svc.CreateCase(context.Background(), req)

	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("CreateCase err = %v, want *apierror.ValidationError", err)
	}
}

// TestCaseService_CreateCase_AllowsDeployedProductCategoryWithinAllowList is
// the matching success case: the deployed product's category is one of the
// project type's own allow-listed values, so CreateCase proceeds through to
// the repository exactly as it would with no allow-list at all.
func TestCaseService_CreateCase_AllowsDeployedProductCategoryWithinAllowList(t *testing.T) {
	referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
		SrProductCategories: []string{"MS", "PC"},
	}}
	deployedProductRepo := &stubDeployedProductRepo{
		getDeployedProductCategory: func(context.Context, string) (*string, error) {
			return categoryPtr("ms"), nil
		},
	}
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1", Number: "CS-PORTAL-000001", CreatedBy: req.CreatedBy}, nil
		},
	}
	svc := WithProductCategoryEnforcement(
		NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		referenceDataRepo, deployedProductRepo,
	)

	req := validServiceRequestCreateCaseRequest()
	req.CreatedBy = "system"
	resp, err := svc.CreateCase(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
	if resp.Case.ID != "case-1" {
		t.Errorf("Case.ID = %q, want %q", resp.Case.ID, "case-1")
	}
}

// TestCaseService_CreateCase_NoAllowListConfiguredSkipsCheck covers a
// project type with no entry for the request's own type (the matrix's "N/A"
// columns, e.g. a plain "Subscription" project type) -- an empty/nil
// allow-list is unrestricted, so a deployed product with no category at all
// must still be accepted.
func TestCaseService_CreateCase_NoAllowListConfiguredSkipsCheck(t *testing.T) {
	referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
		// SrProductCategories deliberately empty/nil.
	}}
	deployedProductRepo := &stubDeployedProductRepo{
		getDeployedProductCategory: func(context.Context, string) (*string, error) {
			return nil, nil
		},
	}
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1", CreatedBy: req.CreatedBy}, nil
		},
	}
	svc := WithProductCategoryEnforcement(
		NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		referenceDataRepo, deployedProductRepo,
	)

	req := validServiceRequestCreateCaseRequest()
	req.CreatedBy = "system"
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
}

// TestCaseService_CreateCase_TypeNotCaseOrServiceRequestSkipsCheck confirms
// the check only ever applies to "case"/"service_request" -- an engagement
// against the same restricted project type is unaffected, since the matrix
// has no "Default Engagement Creation Product Categories" column at all.
func TestCaseService_CreateCase_TypeNotCaseOrServiceRequestSkipsCheck(t *testing.T) {
	referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
		DefaultCaseProductCategories: []string{"CL"},
		SrProductCategories:          []string{"PDP"},
	}}
	deployedProductRepo := &stubDeployedProductRepo{
		getDeployedProductCategory: func(context.Context, string) (*string, error) {
			t.Fatal("GetDeployedProductCategory should not be called for an engagement")
			return nil, nil
		},
	}
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "eng-1", CreatedBy: req.CreatedBy}, nil
		},
	}
	svc := WithProductCategoryEnforcement(
		NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		referenceDataRepo, deployedProductRepo,
	)

	req := validEngagementCreateCaseRequest()
	req.CreatedBy = "system"
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
}

// TestCaseService_CreateCase_EnforcementNotWiredSkipsCheck confirms every
// existing CreateCase caller that never calls WithProductCategoryEnforcement
// (every test and routes.go branch that predates this check) keeps behaving
// exactly as before: a deployed product with no category is accepted
// regardless of project type, since the check is nil-safe to omit entirely.
func TestCaseService_CreateCase_EnforcementNotWiredSkipsCheck(t *testing.T) {
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1", CreatedBy: req.CreatedBy}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	req := validServiceRequestCreateCaseRequest()
	req.CreatedBy = "system"
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
}

// TestCaseService_CreateCase_CsmSrProductCategoriesForCSMPortalCallers covers
// migration 0264's csm_sr_product_categories: WSO2 staff raising a service
// request outside the customer portal may use a CL product under Cloud
// Support; a customer (and staff signed in to the customer portal) still
// gets sr_product_categories only.
func TestCaseService_CreateCase_CsmSrProductCategoriesForCSMPortalCallers(t *testing.T) {
	cases := []struct {
		name      string
		scope     AccessScope
		scopeErr  error
		csmCats   []string
		wantAllow bool
	}{
		{"csm portal staff, CL allowed", AccessScope{Unrestricted: true, HasInternalAccess: true}, nil, []string{"PDP", "CL"}, true},
		{"customer, CL refused", AccessScope{}, nil, []string{"PDP", "CL"}, false},
		{"staff via customer portal, CL refused", AccessScope{Unrestricted: true, ViaCustomerPortal: true}, nil, []string{"PDP", "CL"}, false},
		{"unresolvable scope, CL refused", AccessScope{}, errors.New("no identity"), []string{"PDP", "CL"}, false},
		{"no csm list, CL refused for staff", AccessScope{Unrestricted: true}, nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			referenceDataRepo := &fakeReferenceDataRepo{projectType: &repository.ProjectTypeRow{
				Name:                   "Cloud Support",
				SrProductCategories:    []string{"PDP"},
				CsmSrProductCategories: tc.csmCats,
			}}
			deployedProductRepo := &stubDeployedProductRepo{
				getDeployedProductCategory: func(context.Context, string) (*string, error) {
					return categoryPtr("cl"), nil
				},
			}
			repo := &stubCaseRepo{
				createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
					return domain.Case{ID: "case-1", CreatedBy: req.CreatedBy}, nil
				},
			}
			svc := WithProductCategoryEnforcement(
				NewCaseService(repo, stubUserRepo{}, nil, stubAccess{scope: tc.scope, err: tc.scopeErr}, nil),
				referenceDataRepo, deployedProductRepo,
			)

			req := validServiceRequestCreateCaseRequest()
			req.CreatedBy = "system"
			_, err := svc.CreateCase(context.Background(), req)

			if tc.wantAllow {
				if err != nil {
					t.Fatalf("CreateCase: %v, want allowed", err)
				}
				return
			}
			var validationErr *apierror.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("CreateCase err = %v, want *apierror.ValidationError", err)
			}
		})
	}
}
