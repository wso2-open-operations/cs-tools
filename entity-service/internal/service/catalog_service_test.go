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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubCatalogRepo struct {
	searchCatalogsCalled             bool
	getCatalogItemVariablesCalled    bool
	getCatalogItemVariablesErr       error
	getCatalogItemVariablesVariables []domain.CatalogItemVariable
	includeCSMOnly                   bool
}

func (s *stubCatalogRepo) SearchCatalogs(ctx context.Context, deployedProductID string, includeCSMOnly bool, pagination domain.Pagination) ([]domain.Catalog, int, error) {
	s.searchCatalogsCalled = true
	s.includeCSMOnly = includeCSMOnly
	return nil, 0, nil
}

func (s *stubCatalogRepo) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string, includeCSMOnly bool) ([]domain.CatalogItemVariable, error) {
	s.getCatalogItemVariablesCalled = true
	s.includeCSMOnly = includeCSMOnly
	return s.getCatalogItemVariablesVariables, s.getCatalogItemVariablesErr
}

func (s *stubCatalogRepo) IsCatalogItemCSMOnly(ctx context.Context, catalogItemID string) (bool, error) {
	return false, nil
}

type stubCatalogMirror struct {
	searchCatalogsCalled          bool
	getCatalogItemVariablesCalled bool
	getCatalogItemVariablesResp   domain.GetCatalogItemVariablesResponse
}

func (s *stubCatalogMirror) SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	s.searchCatalogsCalled = true
	return domain.SearchCatalogsResponse{}, nil
}

func (s *stubCatalogMirror) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error) {
	s.getCatalogItemVariablesCalled = true
	return s.getCatalogItemVariablesResp, nil
}

// TestCatalogService_GetCatalogItemVariables_UsesSNMirrorWhenSet guards
// against the dual-write regression found live: a catalog item id returned
// by SearchCatalogs (ServiceNow-sourced under dual-write) was then looked up
// against Postgres' own catalog_item_category table, which has no row for
// it, so every item 404'd with "catalog item not found in this catalog" --
// SR creation's request-type step was unusable under dual-write regardless
// of which item was picked. GetCatalogItemVariables must follow the same
// snMirror SearchCatalogs already does, not stay on Postgres unconditionally.
func TestCatalogService_GetCatalogItemVariables_UsesSNMirrorWhenSet(t *testing.T) {
	repo := &stubCatalogRepo{}
	mirror := &stubCatalogMirror{}
	svc := NewCatalogServiceWithSNFallback(repo, mirror)

	_, err := svc.GetCatalogItemVariables(context.Background(), testUUID, testDeploymentUUID)
	if err != nil {
		t.Fatalf("GetCatalogItemVariables: %v", err)
	}
	if !mirror.getCatalogItemVariablesCalled {
		t.Error("expected GetCatalogItemVariables to delegate to the SN mirror, it didn't")
	}
	if repo.getCatalogItemVariablesCalled {
		t.Error("expected GetCatalogItemVariables NOT to touch the Postgres repo when an SN mirror is set")
	}
}

// TestCatalogService_SearchCatalogs_UsesSNMirrorWhenSet pins the existing,
// already-correct behavior alongside the new GetCatalogItemVariables test
// above, so the two can't silently drift apart again.
func TestCatalogService_SearchCatalogs_UsesSNMirrorWhenSet(t *testing.T) {
	repo := &stubCatalogRepo{}
	mirror := &stubCatalogMirror{}
	svc := NewCatalogServiceWithSNFallback(repo, mirror)

	_, err := svc.SearchCatalogs(context.Background(), domain.SearchCatalogsRequest{
		DeployedProductID: testUUID,
	})
	if err != nil {
		t.Fatalf("SearchCatalogs: %v", err)
	}
	if !mirror.searchCatalogsCalled {
		t.Error("expected SearchCatalogs to delegate to the SN mirror, it didn't")
	}
	if repo.searchCatalogsCalled {
		t.Error("expected SearchCatalogs NOT to touch the Postgres repo when an SN mirror is set")
	}
}

// TestCatalogService_GetCatalogItemVariables_UsesRepoWhenNoMirror proves the
// plain `postgres` data source (no snMirror) is unaffected by the fix above
// -- it keeps reading Postgres directly, same as before.
func TestCatalogService_GetCatalogItemVariables_UsesRepoWhenNoMirror(t *testing.T) {
	fieldName := "field1"
	repo := &stubCatalogRepo{
		getCatalogItemVariablesVariables: []domain.CatalogItemVariable{{ID: "v1", Name: &fieldName}},
	}
	svc := NewCatalogService(repo, nil)

	resp, err := svc.GetCatalogItemVariables(context.Background(), testUUID, testDeploymentUUID)
	if err != nil {
		t.Fatalf("GetCatalogItemVariables: %v", err)
	}
	if !repo.getCatalogItemVariablesCalled {
		t.Error("expected GetCatalogItemVariables to read the Postgres repo when no SN mirror is set")
	}
	if len(resp.Variables) != 1 || resp.Variables[0].ID != "v1" {
		t.Errorf("Variables = %+v, want the repo's own result passed through", resp.Variables)
	}
}
