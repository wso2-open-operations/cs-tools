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
)

type stubCSMOnlyItems struct {
	csmOnly bool
	err     error
}

func (s stubCSMOnlyItems) IsCatalogItemCSMOnly(context.Context, string) (bool, error) {
	return s.csmOnly, s.err
}

var (
	csmOnlyStaffScope      = AccessScope{Unrestricted: true}
	csmOnlyStaffViaCPScope = AccessScope{Unrestricted: true, ViaCustomerPortal: true}
	csmOnlyCustomerScope   = AccessScope{ProjectIDs: []string{"p1"}, ViaCustomerPortal: true}
)

func TestIsCSMPortalStaff(t *testing.T) {
	tests := []struct {
		name   string
		access AccessService
		want   bool
	}{
		{"CSM portal staff", stubAccess{scope: csmOnlyStaffScope}, true},
		{"staff via the customer portal", stubAccess{scope: csmOnlyStaffViaCPScope}, false},
		{"customer", stubAccess{scope: csmOnlyCustomerScope}, false},
		{"unresolvable caller", stubAccess{err: errors.New("no identity")}, false},
		{"no resolver", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCSMPortalStaff(context.Background(), tt.access); got != tt.want {
				t.Fatalf("isCSMPortalStaff = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateCSMOnlyCatalogItem(t *testing.T) {
	sr := domain.CreateCaseRequest{Type: "service_request", CatalogItemID: "item-1"}
	lookupErr := errors.New("db down")
	tests := []struct {
		name      string
		items     csmOnlyCatalogItems
		access    AccessService
		req       domain.CreateCaseRequest
		forbidden bool
		wantErr   error
	}{
		{name: "staff-only item, CSM portal staff", items: stubCSMOnlyItems{csmOnly: true}, access: stubAccess{scope: csmOnlyStaffScope}, req: sr},
		{name: "staff-only item, customer", items: stubCSMOnlyItems{csmOnly: true}, access: stubAccess{scope: csmOnlyCustomerScope}, req: sr, forbidden: true},
		{name: "staff-only item, staff via the customer portal", items: stubCSMOnlyItems{csmOnly: true}, access: stubAccess{scope: csmOnlyStaffViaCPScope}, req: sr, forbidden: true},
		{name: "staff-only item, unresolvable caller", items: stubCSMOnlyItems{csmOnly: true}, access: stubAccess{err: errors.New("no identity")}, req: sr, forbidden: true},
		{name: "ordinary item, customer", items: stubCSMOnlyItems{}, access: stubAccess{scope: csmOnlyCustomerScope}, req: sr},
		{name: "not a service request", items: stubCSMOnlyItems{csmOnly: true}, access: stubAccess{scope: csmOnlyCustomerScope}, req: domain.CreateCaseRequest{Type: "case"}},
		{name: "check not wired", items: nil, access: stubAccess{scope: csmOnlyCustomerScope}, req: sr},
		{name: "lookup fails", items: stubCSMOnlyItems{err: lookupErr}, access: stubAccess{scope: csmOnlyStaffScope}, req: sr, wantErr: lookupErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &caseService{access: tt.access}
			if tt.items != nil {
				s.csmOnlyItems = tt.items
			}
			err := s.validateCSMOnlyCatalogItem(context.Background(), tt.req)
			var forbidden *apierror.ForbiddenError
			switch {
			case tt.forbidden:
				if !errors.As(err, &forbidden) {
					t.Fatalf("err = %v, want ForbiddenError", err)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

func TestCatalogService_PassesCallerToRepo(t *testing.T) {
	ctx := context.Background()
	const productID = "00000000-0000-0000-0000-000000000001"
	const catalogID = "00000000-0000-0000-0000-000000000002"
	const itemID = "00000000-0000-0000-0000-000000000003"
	for _, tt := range []struct {
		name   string
		access AccessService
		want   bool
	}{
		{"CSM portal staff", stubAccess{scope: csmOnlyStaffScope}, true},
		{"customer", stubAccess{scope: csmOnlyCustomerScope}, false},
		{"no resolver", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubCatalogRepo{includeCSMOnly: !tt.want}
			svc := NewCatalogService(repo, tt.access)
			if _, err := svc.SearchCatalogs(ctx, domain.SearchCatalogsRequest{DeployedProductID: productID}); err != nil {
				t.Fatalf("SearchCatalogs: %v", err)
			}
			if repo.includeCSMOnly != tt.want {
				t.Fatalf("SearchCatalogs includeCSMOnly = %v, want %v", repo.includeCSMOnly, tt.want)
			}
			repo.includeCSMOnly = !tt.want
			if _, err := svc.GetCatalogItemVariables(ctx, catalogID, itemID); err != nil {
				t.Fatalf("GetCatalogItemVariables: %v", err)
			}
			if repo.includeCSMOnly != tt.want {
				t.Fatalf("GetCatalogItemVariables includeCSMOnly = %v, want %v", repo.includeCSMOnly, tt.want)
			}
		})
	}
}
