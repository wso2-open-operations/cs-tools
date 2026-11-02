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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	catalogVersionID = "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"
	catalogProductID = "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5c"
)

// fakeCatalogRepo records what the service asked of it.
type fakeCatalogRepo struct {
	repository.ProductCatalogRepository // unused methods panic

	exists         bool
	release, eol   *string
	applied        *repository.CatalogApply
	actor          string
	appliedIDs     []string
	storedDatesHit bool
}

func (r *fakeCatalogRepo) EntityExists(context.Context, string, string) (bool, error) {
	return r.exists, nil
}

func (r *fakeCatalogRepo) StoredVersionDates(context.Context, string) (*string, *string, bool, error) {
	r.storedDatesHit = true
	return r.release, r.eol, true, nil
}

func (r *fakeCatalogRepo) ApplyChange(_ context.Context, actor string, a repository.CatalogApply) ([]string, error) {
	r.actor, r.applied = actor, &a
	if r.appliedIDs != nil {
		return r.appliedIDs, nil
	}
	return []string{catalogVersionID}, nil
}

// fakeVersionRepo answers GetProductVersions with one row per id.
type fakeVersionRepo struct {
	repository.ProductVersionRepository
	search *domain.SearchProductVersionsRequest
}

func (r *fakeVersionRepo) GetProductVersions(_ context.Context, ids []string) ([]domain.ProductVersion, error) {
	out := make([]domain.ProductVersion, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.ProductVersion{ID: id})
	}
	return out, nil
}

func (r *fakeVersionRepo) SearchProductVersions(_ context.Context, req domain.SearchProductVersionsRequest) ([]domain.ProductVersion, int, error) {
	r.search = &req
	return []domain.ProductVersion{}, 0, nil
}

type fakeCatalogProductRepo struct {
	repository.ProductRepository
}

func (fakeCatalogProductRepo) GetProduct(_ context.Context, id string) (*domain.Product, error) {
	return &domain.Product{ID: id}, nil
}

func catalogCtx(roles ...string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "pm@wso2.com", Roles: roles})
}

func newCatalogService(repo *fakeCatalogRepo) ProductCatalogService {
	return newCatalogServiceWith(repo, &fakeVersionRepo{})
}

func newCatalogServiceWith(repo *fakeCatalogRepo, versions *fakeVersionRepo) ProductCatalogService {
	return NewProductCatalogService(repo, fakeCatalogProductRepo{}, versions, alwaysUnrestrictedAccess{},
		[]string{"cs_product_catalog_manager", "cs_admin"})
}

const manager = "cs_product_catalog_manager"

func TestProductCatalogChangesNeedTheManagerRole(t *testing.T) {
	repo := &fakeCatalogRepo{exists: true}
	_, err := newCatalogService(repo).UpdateProductVersion(catalogCtx("cs_engineer"), catalogVersionID, json.RawMessage(`{"description":"x"}`))
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("err = %v", err)
	}
	if repo.applied != nil {
		t.Fatal("repo was called")
	}
}

func TestProductCatalogAdminMayEdit(t *testing.T) {
	repo := &fakeCatalogRepo{exists: true}
	_, err := newCatalogService(repo).UpdateProductVersion(catalogCtx("cs_admin"), catalogVersionID, json.RawMessage(`{"description":"x"}`))
	if err != nil || repo.applied == nil || repo.actor != "pm@wso2.com" {
		t.Fatalf("applied=%v actor=%s err=%v", repo.applied, repo.actor, err)
	}
}

func TestProductCatalogUpdateOfAMissingEntityIsNotFound(t *testing.T) {
	repo := &fakeCatalogRepo{exists: false}
	_, err := newCatalogService(repo).UpdateProduct(catalogCtx(manager), catalogProductID, json.RawMessage(`{"description":"x"}`))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || repo.applied != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestProductCatalogInvalidChangeNeverReachesTheRepo(t *testing.T) {
	repo := &fakeCatalogRepo{exists: true}
	_, err := newCatalogService(repo).UpdateProductVersion(catalogCtx(manager), catalogVersionID, json.RawMessage(`{"bogus":true}`))
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != "bogus: unknown field for a product version" || repo.applied != nil {
		t.Fatalf("err=%v applied=%v", err, repo.applied)
	}
}

func TestProductCatalogRetireReturnsTheListingRows(t *testing.T) {
	other := "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5d"
	repo := &fakeCatalogRepo{exists: true, appliedIDs: []string{catalogVersionID, other}}
	resp, err := newCatalogService(repo).RetireProductVersion(catalogCtx(manager), catalogVersionID)
	if err != nil {
		t.Fatal(err)
	}
	if repo.applied.Operation != repository.CatalogOpRetire || len(resp.ProductVersions) != 2 {
		t.Fatalf("applied=%+v resp=%+v", repo.applied, resp)
	}
}

func TestProductCatalogCreateVersionTakesProductAndProfiles(t *testing.T) {
	repo := &fakeCatalogRepo{appliedIDs: []string{"a", "b"}}
	resp, err := newCatalogService(repo).CreateProductVersions(catalogCtx(manager), catalogProductID,
		json.RawMessage(`{"version":"4.6.0","profiles":["All In One"," Gateway ",""]}`))
	if err != nil {
		t.Fatal(err)
	}
	a := repo.applied
	if a.Operation != repository.CatalogOpCreate || len(a.Profiles) != 3 || a.Profiles[1] != "Gateway" || a.Profiles[2] != "" {
		t.Fatalf("applied = %+v", a)
	}
	cols := map[string]any{}
	for _, s := range a.Sets {
		cols[s.Column] = s.Value
	}
	if v, ok := cols["product_id"].(*string); !ok || *v != catalogProductID {
		t.Fatalf("product_id = %v", cols["product_id"])
	}
	if len(resp.ProductVersions) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestProductCatalogCreateVersionRejects(t *testing.T) {
	cases := map[string]string{
		"no profiles":           `{"version":"1.0.0"}`,
		"empty profiles":        `{"version":"1.0.0","profiles":[]}`,
		"repeated profile":      `{"version":"1.0.0","profiles":["Gateway","gateway"]}`,
		"deploymentProfile":     `{"version":"1.0.0","profiles":["A"],"deploymentProfile":"A"}`,
		"productId in the body": `{"version":"1.0.0","profiles":["A"],"productId":"` + catalogProductID + `"}`,
		"no version":            `{"profiles":["A"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeCatalogRepo{}
			_, err := newCatalogService(repo).CreateProductVersions(catalogCtx(manager), catalogProductID, json.RawMessage(body))
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || repo.applied != nil {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestPrepareCatalogChangeKeepsProfileAndPublicIDToTheRow(t *testing.T) {
	a, err := prepareCatalogChange(domain.CatalogEntityProductVersion, repository.CatalogOpUpdate, catalogVersionID,
		json.RawMessage(`{"deploymentProfile":"Key Manager","publicId":901,"supportEolDate":"2027-03-22"}`))
	if err != nil {
		t.Fatal(err)
	}
	row := map[string]bool{}
	for _, s := range a.RowSets {
		row[s.Column] = true
	}
	if !row["deployment_profile"] || !row["public_id"] || len(a.Sets) != 1 || a.Sets[0].Column != "support_eol_date" {
		t.Fatalf("sets=%+v rowSets=%+v", a.Sets, a.RowSets)
	}
}

func TestPrepareCatalogChangeRejectsBadChanges(t *testing.T) {
	cases := []struct {
		name               string
		entity, op, id, pl string
	}{
		{"bad id", "product_version", "UPDATE", "nope", `{"description":"x"}`},
		{"retire a product", "product", "RETIRE", catalogVersionID, ``},
		{"empty update", "product_version", "UPDATE", catalogVersionID, `{}`},
		{"unknown field", "product_version", "UPDATE", catalogVersionID, `{"bogus":1}`},
		{"create-only field on update", "product_version", "UPDATE", catalogVersionID, `{"version":"2.0.0"}`},
		{"missing required", "product_version", "CREATE", "", `{"version":"1.0.0","profiles":["A"]}`},
		{"bad status", "product_version", "UPDATE", catalogVersionID, `{"currentSupportStatus":"eol"}`},
		{"support type is not a field yet", "product_version", "UPDATE", catalogVersionID, `{"supportType":"lts"}`},
		{"bad date", "product_version", "UPDATE", catalogVersionID, `{"releaseDate":"2026-13-01"}`},
		{"eol before release", "product_version", "UPDATE", catalogVersionID, `{"releaseDate":"2026-05-01","supportEolDate":"2026-04-30"}`},
		{"null flag", "product_version", "UPDATE", catalogVersionID, `{"isPublic":null}`},
		{"version too long", "product_version", "CREATE", "", `{"productId":"` + catalogVersionID + `","version":"123456789012345678901","profiles":["A"]}`},
		{"negative public id", "product_version", "UPDATE", catalogVersionID, `{"publicId":-4}`},
		{"tags not strings", "product_version", "UPDATE", catalogVersionID, `{"tags":[1]}`},
		{"carbon unknown key", "product_version", "UPDATE", catalogVersionID, `{"carbon":{"kernel":"4.6.0"}}`},
		{"retired is not a field", "product_version", "UPDATE", catalogVersionID, `{"retired":false}`},
		{"profiles on update", "product_version", "UPDATE", catalogVersionID, `{"profiles":["A"]}`},
		{"empty line name", "release_line", "CREATE", "", `{"name":" "}`},
		{"bad product class", "product", "CREATE", "", `{"name":"X","class":"hardware"}`},
		{"body not an object", "product_version", "UPDATE", catalogVersionID, `[1]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := prepareCatalogChange(tc.entity, tc.op, tc.id, json.RawMessage(tc.pl))
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestPrepareCatalogChangeBuildsColumnAssignments(t *testing.T) {
	a, err := prepareCatalogChange(domain.CatalogEntityProductVersion, repository.CatalogOpUpdate, catalogVersionID, json.RawMessage(`{
		"currentSupportStatus": "Deprecated",
		"supportEolDate": "2027-03-22",
		"publicName": "  API Manager  ",
		"releaseLinks": "",
		"tags": null,
		"carbon": {"platformRelease": "Wilkinson", "kernelPatchNo": ""}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for _, s := range a.Sets {
		got[s.Column] = s.Value
	}
	if v := got["current_support_status"].(*string); *v != "deprecated" {
		t.Errorf("status = %q", *v)
	}
	if v := got["public_name"].(*string); *v != "API Manager" {
		t.Errorf("public name = %q", *v)
	}
	if v := got["release_links"].(*string); v != nil {
		t.Errorf("empty release links should clear, got %q", *v)
	}
	if v := got["tags"].([]string); len(v) != 0 {
		t.Errorf("null tags should be empty, got %v", v)
	}
	if !a.CarbonSet || a.Carbon == nil || *a.Carbon.PlatformRelease != "Wilkinson" || a.Carbon.KernelPatchNo != nil {
		t.Errorf("carbon = %+v", a.Carbon)
	}
	for _, s := range a.Sets {
		if s.Cast == "" || s.Column == "" {
			t.Errorf("set without column or cast: %+v", s)
		}
	}
}

func TestProductCatalogSearchAllVersions(t *testing.T) {
	versions := &fakeVersionRepo{}
	svc := newCatalogServiceWith(&fakeCatalogRepo{}, versions)
	_, err := svc.SearchAllProductVersions(catalogCtx(), domain.SearchAllProductVersionsRequest{
		SearchProductVersionsRequest: domain.SearchProductVersionsRequest{Feed: "umt"}})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("unknown feed: err = %v", err)
	}
	_, err = svc.SearchAllProductVersions(catalogCtx(), domain.SearchAllProductVersionsRequest{
		SearchProductVersionsRequest: domain.SearchProductVersionsRequest{Pagination: domain.Pagination{Limit: 1001}}})
	if !errors.As(err, &ve) {
		t.Fatalf("limit over 1000: err = %v", err)
	}
	_, err = svc.SearchAllProductVersions(catalogCtx(), domain.SearchAllProductVersionsRequest{
		ProductID:                    catalogProductID,
		SearchProductVersionsRequest: domain.SearchProductVersionsRequest{Feed: domain.CatalogFeedSupportMatrix, Pagination: domain.Pagination{Limit: 1000}}})
	if err != nil || versions.search == nil || versions.search.ProductID != catalogProductID || versions.search.Pagination.Limit != 1000 {
		t.Fatalf("err = %v search = %+v", err, versions.search)
	}
}

func TestProductCatalogUpdateChecksTheStoredDates(t *testing.T) {
	release, eol := "2026-11-01", "2030-01-31"
	cases := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{"EOL before the stored release date", `{"supportEolDate":"2020-01-01"}`, true},
		{"EOL after the stored release date", `{"supportEolDate":"2031-01-01"}`, false},
		{"release date after the stored EOL", `{"releaseDate":"2031-01-01"}`, true},
		{"clearing the EOL date", `{"supportEolDate":null}`, false},
		{"neither date", `{"currentSupportStatus":"deprecated"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeCatalogRepo{exists: true, release: &release, eol: &eol}
			_, err := newCatalogService(repo).UpdateProductVersion(catalogCtx(manager), catalogVersionID, json.RawMessage(tc.payload))
			var verr *apierror.ValidationError
			if tc.wantErr != errors.As(err, &verr) || (tc.wantErr && repo.applied != nil) {
				t.Fatalf("err = %v, applied = %v", err, repo.applied != nil)
			}
		})
	}
}
