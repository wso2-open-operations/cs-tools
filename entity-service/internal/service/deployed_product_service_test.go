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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubDeployedProductRepo is a minimal repository.DeployedProductRepository
// whose SearchDeployedProducts panics if called: tests using it prove the
// PostgreSQL-backed service rejects an unsupported ProductCategories filter
// before ever reaching the repository, not merely that the repository
// ignores it.
type stubDeployedProductRepo struct {
	searchDeployedProducts              func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error)
	searchProjectsByProductVersion      func(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error)
	createDeployedProductFromServiceNow func(ctx context.Context, req domain.CreateDeployedProductRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployedProduct, error)
	createDeployedProduct               func(ctx context.Context, req domain.CreateDeployedProductRequest, createdBy string) (domain.CreatedDeployedProduct, error)
	updateDeployedProductFields         func(ctx context.Context, req domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error)
	getDeployedProductCategory          func(ctx context.Context, id string) (*string, error)
}

func (s *stubDeployedProductRepo) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
	if s.searchDeployedProducts != nil {
		return s.searchDeployedProducts(ctx, req)
	}
	panic("SearchDeployedProducts called unexpectedly: the productCategories rejection should have short-circuited before reaching the repository")
}

func (s *stubDeployedProductRepo) SearchDeployedProductMetrics(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductMetricsResponse, error) {
	panic("SearchDeployedProductMetrics not stubbed")
}

func (s *stubDeployedProductRepo) SearchDeployedProductUsageCounts(ctx context.Context, id, deploymentID, startDate, endDate string) (domain.DeployedProductUsageCountsResponse, error) {
	panic("SearchDeployedProductUsageCounts not stubbed")
}

func (s *stubDeployedProductRepo) SearchProjectsByProductVersion(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error) {
	if s.searchProjectsByProductVersion != nil {
		return s.searchProjectsByProductVersion(ctx, req, excludeClosureStates, excludeSubscriptionTypes)
	}
	panic("SearchProjectsByProductVersion called unexpectedly")
}

func (s *stubDeployedProductRepo) CreateDeployedProductFromServiceNow(ctx context.Context, req domain.CreateDeployedProductRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployedProduct, error) {
	if s.createDeployedProductFromServiceNow == nil {
		panic("stubDeployedProductRepo: CreateDeployedProductFromServiceNow not set")
	}
	return s.createDeployedProductFromServiceNow(ctx, req, id, number, createdBy, createdOn)
}

func (s *stubDeployedProductRepo) CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest, createdBy string) (domain.CreatedDeployedProduct, error) {
	if s.createDeployedProduct == nil {
		panic("stubDeployedProductRepo: CreateDeployedProduct not set")
	}
	return s.createDeployedProduct(ctx, req, createdBy)
}

func (s *stubDeployedProductRepo) UpdateDeployedProductFields(ctx context.Context, req domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
	if s.updateDeployedProductFields == nil {
		panic("stubDeployedProductRepo: UpdateDeployedProductFields not set")
	}
	return s.updateDeployedProductFields(ctx, req, updatedBy)
}

func (s *stubDeployedProductRepo) GetDeployedProductCategory(ctx context.Context, id string) (*string, error) {
	if s.getDeployedProductCategory == nil {
		panic("stubDeployedProductRepo: GetDeployedProductCategory not set")
	}
	return s.getDeployedProductCategory(ctx, id)
}

// stubMirrorDeployedProductService implements both the full
// DeployedProductService interface (so it satisfies deployedProductService's
// snMirror field type) and the narrow deployedProductSNCreator interface
// createDeployedProductSNFirst actually type-asserts against -- mirroring
// stubMirrorDeploymentService's shape in deployment_service_test.go.
type stubMirrorDeployedProductService struct {
	createDeployedProductSNFirstDetailsFn func(context.Context, domain.CreateDeployedProductRequest) (id, number, createdBy string, createdOn time.Time, err error)
	updateDeployedProduct                 func(context.Context, domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error)
}

func (m *stubMirrorDeployedProductService) SearchDeployedProducts(context.Context, domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error) {
	panic("stubMirrorDeployedProductService: SearchDeployedProducts not exercised by these tests")
}

func (m *stubMirrorDeployedProductService) SearchProjectsByProductVersion(context.Context, domain.SearchProjectsByProductVersionRequest) (domain.SearchProjectsByProductVersionResponse, error) {
	panic("stubMirrorDeployedProductService: SearchProjectsByProductVersion not exercised by these tests")
}

func (m *stubMirrorDeployedProductService) CreateDeployedProduct(context.Context, domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	panic("stubMirrorDeployedProductService: CreateDeployedProduct not exercised by these tests -- createDeployedProductSNFirst calls createDeployedProductSNFirstDetails instead")
}

func (m *stubMirrorDeployedProductService) UpdateDeployedProduct(ctx context.Context, req domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
	if m.updateDeployedProduct == nil {
		panic("stubMirrorDeployedProductService: UpdateDeployedProduct not set")
	}
	return m.updateDeployedProduct(ctx, req)
}

func (m *stubMirrorDeployedProductService) SearchDeployedProductMetrics(context.Context, string, domain.DeployedProductMetricsRequest) (domain.DeployedProductMetricsResponse, error) {
	panic("stubMirrorDeployedProductService: SearchDeployedProductMetrics not exercised by these tests")
}

func (m *stubMirrorDeployedProductService) SearchDeployedProductUsageCounts(context.Context, string, domain.DeployedProductUsageCountsRequest) (domain.DeployedProductUsageCountsResponse, error) {
	panic("stubMirrorDeployedProductService: SearchDeployedProductUsageCounts not exercised by these tests")
}

func (m *stubMirrorDeployedProductService) createDeployedProductSNFirstDetails(ctx context.Context, req domain.CreateDeployedProductRequest) (id, number, createdBy string, createdOn time.Time, err error) {
	if m.createDeployedProductSNFirstDetailsFn == nil {
		panic("stubMirrorDeployedProductService: createDeployedProductSNFirstDetailsFn not set")
	}
	return m.createDeployedProductSNFirstDetailsFn(ctx, req)
}

func validCreateDeployedProductRequest() domain.CreateDeployedProductRequest {
	return domain.CreateDeployedProductRequest{
		ProjectID:    testDeploymentUUID,
		DeploymentID: testDeployedProdID,
		ProductID:    testDeploymentUUID,
		VersionID:    testDeployedProdID,
	}
}

// A plain Postgres data source (no SN mirror wired at all) still rejects
// CreateDeployedProduct, which is SN-first by design.
func TestDeployedProductService_CreateNotSupportedOnPlainPostgres(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	if _, err := svc.CreateDeployedProduct(context.Background(), validCreateDeployedProductRequest()); err == nil {
		t.Error("CreateDeployedProduct: expected an error on the plain Postgres data source")
	}
}

// On plain PostgreSQL (no mirror wired) every update branch reaches the
// repository with the validated request and the caller's email.
func TestDeployedProductService_UpdateDeployedProduct_PlainPostgres(t *testing.T) {
	cores := 8
	tps := 12.5
	category := "pdp"
	deploymentID := testDeployedProdID
	cases := []struct {
		name string
		req  domain.UpdateDeployedProductRequest
	}{
		{"cores", domain.UpdateDeployedProductRequest{Cores: &cores}},
		{"tps", domain.UpdateDeployedProductRequest{TPS: &tps}},
		{"description", domain.UpdateDeployedProductRequest{Description: json.RawMessage(`{"type":"doc"}`)}},
		{"updates", domain.UpdateDeployedProductRequest{Updates: []domain.ProductUpdateEntry{{UpdateLevel: 3, Date: "2026-01-15"}}}},
		{"category", domain.UpdateDeployedProductRequest{Category: &category}},
		{"deactivate", domain.UpdateDeployedProductRequest{Active: boolPtr(false)}},
		{"scoped to deployment", domain.UpdateDeployedProductRequest{Cores: &cores, DeploymentID: &deploymentID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotReq domain.UpdateDeployedProductRequest
			var gotBy string
			calls := 0
			repo := &stubDeployedProductRepo{
				updateDeployedProductFields: func(_ context.Context, r domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
					calls++
					gotReq, gotBy = r, updatedBy
					return domain.UpdatedDeployedProduct{ID: r.ID, UpdatedBy: updatedBy}, nil
				},
			}
			svc := NewDeployedProductService(repo)
			req := tc.req
			req.ID = testDeploymentUUID

			ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
			resp, err := svc.UpdateDeployedProduct(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 1 {
				t.Fatalf("repo called %d times, want 1", calls)
			}
			if !reflect.DeepEqual(gotReq, req) {
				t.Errorf("repo got %+v, want %+v", gotReq, req)
			}
			if gotBy != "jane.doe@example.com" || resp.DeployedProduct.UpdatedBy != gotBy {
				t.Errorf("updatedBy = %q / %q", gotBy, resp.DeployedProduct.UpdatedBy)
			}
		})
	}
}

// Plain PostgreSQL runs the same validation as the dual-write path and never
// reaches the repository with an invalid request.
func TestDeployedProductService_UpdateDeployedProduct_PlainPostgres_Validation(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{}) // repo panics if called
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.UpdateDeployedProduct(ctx, domain.UpdateDeployedProductRequest{ID: testDeploymentUUID})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
	if _, err := svc.UpdateDeployedProduct(context.Background(), domain.UpdateDeployedProductRequest{ID: testDeploymentUUID, Active: boolPtr(false)}); err == nil {
		t.Error("expected an error when x-user-id-token is missing")
	}
}

// If ServiceNow never accepts the deployed product, nothing must be written
// to Postgres at all -- stubDeployedProductRepo panics if
// CreateDeployedProductFromServiceNow is called, which is exactly the
// assertion.
func TestDeployedProductService_CreateDeployedProduct_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorDeployedProductService{
		createDeployedProductSNFirstDetailsFn: func(context.Context, domain.CreateDeployedProductRequest) (string, string, string, time.Time, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return "", "", "", time.Time{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubDeployedProductRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	_, err := svc.CreateDeployedProduct(context.Background(), validCreateDeployedProductRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the deployed product")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// On ServiceNow success, the Postgres insert must use EXACTLY the
// id/number/createdBy/createdOn ServiceNow returned -- not anything
// generated locally -- so both systems agree on identity from the moment
// the Postgres row exists.
func TestDeployedProductService_CreateDeployedProduct_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID        = "44444444-4444-4444-4444-444444444444"
		snNumber    = "DP000099001"
		snCreatedBy = "jane.doe@example.com"
	)
	createdOn := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mirror := &stubMirrorDeployedProductService{
		createDeployedProductSNFirstDetailsFn: func(context.Context, domain.CreateDeployedProductRequest) (string, string, string, time.Time, error) {
			return snID, snNumber, snCreatedBy, createdOn, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotCreatedBy string
	var gotCreatedOn time.Time
	repo := &stubDeployedProductRepo{
		createDeployedProductFromServiceNow: func(_ context.Context, req domain.CreateDeployedProductRequest, id, number, createdBy string, createdOn time.Time) (domain.CreatedDeployedProduct, error) {
			mu.Lock()
			gotID, gotNumber, gotCreatedBy, gotCreatedOn = id, number, createdBy, createdOn
			mu.Unlock()
			return domain.CreatedDeployedProduct{ID: id, CreatedBy: createdBy, CreatedOn: createdOn}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	resp, err := svc.CreateDeployedProduct(context.Background(), validCreateDeployedProductRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotCreatedBy != snCreatedBy || !gotCreatedOn.Equal(createdOn) {
		t.Errorf("CreateDeployedProductFromServiceNow got (%q, %q, %q, %v), want (%q, %q, %q, %v)",
			gotID, gotNumber, gotCreatedBy, gotCreatedOn, snID, snNumber, snCreatedBy, createdOn)
	}
	if resp.DeployedProduct.ID != snID {
		t.Errorf("response DeployedProduct.ID = %q, want %q", resp.DeployedProduct.ID, snID)
	}
}

// UpdateDeployedProduct writes Postgres first and dispatches the ServiceNow
// mirror asynchronously afterward -- the mirror call must still happen, just
// not block the response.
func TestDeployedProductService_UpdateDeployedProduct_PostgresFirstThenAsyncMirror(t *testing.T) {
	cores := 8
	req := domain.UpdateDeployedProductRequest{ID: testDeploymentUUID, Cores: &cores}

	called := make(chan struct{})
	var mu sync.Mutex
	var gotReq domain.UpdateDeployedProductRequest
	mirror := &stubMirrorDeployedProductService{
		updateDeployedProduct: func(_ context.Context, r domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
			mu.Lock()
			gotReq = r
			mu.Unlock()
			close(called)
			return domain.UpdateDeployedProductResponse{}, nil
		},
	}
	repo := &stubDeployedProductRepo{
		updateDeployedProductFields: func(_ context.Context, r domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
			return domain.UpdatedDeployedProduct{ID: r.ID, UpdatedBy: updatedBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateDeployedProduct(ctx, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateDeployedProduct was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotReq.ID != req.ID || gotReq.Cores == nil || *gotReq.Cores != cores {
		t.Errorf("mirror got %+v, want ID=%q Cores=%d", gotReq, req.ID, cores)
	}
}

// A failed async mirror write must not fail the update itself -- Postgres
// already committed, and the caller already has a 200 by the time the
// mirror even runs.
func TestDeployedProductService_UpdateDeployedProduct_MirrorFailureDoesNotFailTheUpdate(t *testing.T) {
	cores := 8
	req := domain.UpdateDeployedProductRequest{ID: testDeploymentUUID, Cores: &cores}

	called := make(chan struct{})
	mirror := &stubMirrorDeployedProductService{
		updateDeployedProduct: func(context.Context, domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
			close(called)
			return domain.UpdateDeployedProductResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubDeployedProductRepo{
		updateDeployedProductFields: func(_ context.Context, r domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
			return domain.UpdatedDeployedProduct{ID: r.ID, UpdatedBy: updatedBy}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateDeployedProduct(ctx, req); err != nil {
		t.Fatalf("unexpected error from UpdateDeployedProduct despite Postgres succeeding: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateDeployedProduct was never called")
	}
}

// TestDeployedProductService_UpdateDeployedProduct_RejectsInvalidRequest proves the
// dual-write path calls the same validateUpdateDeployedProductRequest checks
// the ServiceNow-only path always enforced, rather than writing an
// otherwise-rejected request straight to Postgres.
func TestDeployedProductService_UpdateDeployedProduct_RejectsInvalidRequest(t *testing.T) {
	mirror := &stubMirrorDeployedProductService{}
	repo := &stubDeployedProductRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	// Neither a detail field nor Active is set.
	_, err := svc.UpdateDeployedProduct(context.Background(), domain.UpdateDeployedProductRequest{ID: testDeploymentUUID})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestDeployedProductService_UpdateDeployedProduct_RejectsInvalidCategory proves
// an unrecognized category value is rejected with a ValidationError before
// ever reaching the repository, rather than surfacing as a raw Postgres
// enum-cast error.
func TestDeployedProductService_UpdateDeployedProduct_RejectsInvalidCategory(t *testing.T) {
	mirror := &stubMirrorDeployedProductService{}
	repo := &stubDeployedProductRepo{}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	bogus := "not-a-real-category"
	_, err := svc.UpdateDeployedProduct(context.Background(), domain.UpdateDeployedProductRequest{ID: testDeploymentUUID, Category: &bogus})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestDeployedProductService_UpdateDeployedProduct_CategoryAloneIsSufficient
// proves a request setting only Category (no cores/tps/description/updates)
// satisfies the "at least one field" check and reaches the repository --
// this is the whole point of the fix: a caller categorizing an old,
// otherwise-unchanged deployed product shouldn't have to touch an unrelated
// field just to satisfy this validation.
func TestDeployedProductService_UpdateDeployedProduct_CategoryAloneIsSufficient(t *testing.T) {
	mirror := &stubMirrorDeployedProductService{
		updateDeployedProduct: func(context.Context, domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
			return domain.UpdateDeployedProductResponse{}, nil
		},
	}
	var gotCategory *string
	repo := &stubDeployedProductRepo{
		updateDeployedProductFields: func(ctx context.Context, req domain.UpdateDeployedProductRequest, updatedBy string) (domain.UpdatedDeployedProduct, error) {
			gotCategory = req.Category
			return domain.UpdatedDeployedProduct{ID: req.ID}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := NewDeployedProductServiceWithSNWriteback(repo, dispatcher, mirror)

	ms := "ms"
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	if _, err := svc.UpdateDeployedProduct(ctx, domain.UpdateDeployedProductRequest{ID: testDeploymentUUID, Category: &ms}); err != nil {
		t.Fatalf("UpdateDeployedProduct: %v", err)
	}
	if gotCategory == nil || *gotCategory != "ms" {
		t.Fatalf("expected Category %q to reach the repository, got %v", "ms", gotCategory)
	}
}

// TestDeployedProductService_SearchDeployedProducts_ForwardsProductCategories
// guards against a regression CodeRabbit caught on PR #2189: once
// SearchDeployedProducts stopped delegating to ServiceNow under dual-write
// (see that method's own doc comment), ProductCategories had nowhere to go
// but the Postgres repository -- the category predicate is implemented
// there (deployed_product_repo.go), so the service must forward the filter
// rather than reject it.
func TestDeployedProductService_SearchDeployedProducts_ForwardsProductCategories(t *testing.T) {
	var gotCategories []string
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			gotCategories = req.ProductCategories
			return nil, 0, nil
		},
	})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{
		ProductCategories: []string{"pdp"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(gotCategories) != 1 || gotCategories[0] != "pdp" {
		t.Fatalf("expected ProductCategories [\"pdp\"] to reach the repository, got %v", gotCategories)
	}
}

func TestDeployedProductService_SearchDeployedProducts_NoProductCategoriesReachesRepository(t *testing.T) {
	called := false
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(ctx context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			called = true
			return nil, 0, nil
		},
	})

	_, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected SearchDeployedProducts to reach the repository when ProductCategories is empty")
	}
}

func TestDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductID(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "not-a-uuid",
		ProductVersionID: "22222222-2222-2222-2222-222222222222",
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func TestDeployedProductService_SearchProjectsByProductVersion_RejectsInvalidProductVersionID(t *testing.T) {
	svc := NewDeployedProductService(&stubDeployedProductRepo{})

	_, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "11111111-1111-1111-1111-111111111111",
		ProductVersionID: "not-a-uuid",
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestDeployedProductService_SearchProjectsByProductVersion_AppliesMandatoryExclusions
// proves the service passes the SAME package-level mandatory exclusion
// policy the ServiceNow implementation uses (mandatoryExcludeClosureStates/
// mandatoryExcludeSubscriptionTypes) through to the repository, rather than
// leaving it to the caller or silently dropping it -- the whole point of
// this being a fixed policy, not a request field.
func TestDeployedProductService_SearchProjectsByProductVersion_AppliesMandatoryExclusions(t *testing.T) {
	var gotClosureStates []string
	var gotSubscriptionTypes []domain.SubscriptionType
	var gotReq domain.SearchProjectsByProductVersionRequest

	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchProjectsByProductVersion: func(ctx context.Context, req domain.SearchProjectsByProductVersionRequest, excludeClosureStates []string, excludeSubscriptionTypes []domain.SubscriptionType) ([]domain.EntityRef, int, error) {
			gotReq = req
			gotClosureStates = excludeClosureStates
			gotSubscriptionTypes = excludeSubscriptionTypes
			return []domain.EntityRef{{ID: "p1", Name: "Project One"}}, 1, nil
		},
	})

	resp, err := svc.SearchProjectsByProductVersion(context.Background(), domain.SearchProjectsByProductVersionRequest{
		ProductID:        "11111111-1111-1111-1111-111111111111",
		ProductVersionID: "22222222-2222-2222-2222-222222222222",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.Projects) != 1 || resp.Projects[0].ID != "p1" {
		t.Fatalf("expected the repository's project to pass through, got %+v", resp.Projects)
	}
	if resp.Total != 1 || resp.Limit != gotReq.Pagination.Limit || resp.Offset != gotReq.Pagination.Offset {
		t.Fatalf("expected pagination fields to echo the normalized request, got %+v (req pagination %+v)", resp, gotReq.Pagination)
	}
	if len(gotClosureStates) != len(mandatoryExcludeClosureStates) {
		t.Fatalf("expected mandatoryExcludeClosureStates to be passed through, got %v", gotClosureStates)
	}
	if len(gotSubscriptionTypes) != len(mandatoryExcludeSubscriptionTypes) {
		t.Fatalf("expected mandatoryExcludeSubscriptionTypes to be passed through, got %v", gotSubscriptionTypes)
	}
}

func TestDeployedProductService_SearchDeployedProducts_ProjectSfIDs(t *testing.T) {
	var gotSfIDs []string
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(_ context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			gotSfIDs = req.ProjectSfIDs
			return nil, 0, nil
		},
	})

	if _, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{ProjectSfIDs: []string{"SF001"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSfIDs) != 1 || gotSfIDs[0] != "SF001" {
		t.Errorf("repository did not receive projectSfIds: got %v", gotSfIDs)
	}

	for name, ids := range map[string][]string{"empty": {""}, "blank": {"  "}, "padded": {"SF001 "}} {
		if _, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{ProjectSfIDs: ids}); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

// active reaches the repository as sent; nil when omitted, so the repository's
// default of active-only applies.
func TestDeployedProductService_SearchDeployedProducts_ActiveFilter(t *testing.T) {
	var gotReq domain.SearchDeployedProductsRequest
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		searchDeployedProducts: func(_ context.Context, req domain.SearchDeployedProductsRequest) ([]domain.DeployedProductView, int, error) {
			gotReq = req
			return nil, 0, nil
		},
	})
	no := false

	if _, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{}); err != nil || gotReq.Active != nil {
		t.Errorf("omitted active: err=%v active=%v, want nil", err, gotReq.Active)
	}
	if _, err := svc.SearchDeployedProducts(context.Background(), domain.SearchDeployedProductsRequest{Active: &no}); err != nil || gotReq.Active == nil || *gotReq.Active {
		t.Errorf("active=false did not reach the repository: err=%v active=%v", err, gotReq.Active)
	}
}

// On the plain PostgreSQL data source a deployed product is created in
// Postgres, by the caller (from x-user-id-token), with the request passed on
// unchanged. It used to be refused outright with a 400.
func TestDeployedProductService_CreateDeployedProduct_PostgresCreatesTheRow(t *testing.T) {
	cores := 4
	category := "ms"
	req := domain.CreateDeployedProductRequest{
		ProjectID: testProjectUUID, DeploymentID: testDeploymentUUID,
		ProductID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
		Cores: &cores, Category: &category,
	}
	var gotReq domain.CreateDeployedProductRequest
	var gotBy string
	svc := NewDeployedProductService(&stubDeployedProductRepo{
		createDeployedProduct: func(_ context.Context, r domain.CreateDeployedProductRequest, createdBy string) (domain.CreatedDeployedProduct, error) {
			gotReq, gotBy = r, createdBy
			return domain.CreatedDeployedProduct{ID: "33333333-3333-4333-8333-333333333333", CreatedBy: createdBy}, nil
		},
	})

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := svc.CreateDeployedProduct(ctx, req)
	if err != nil {
		t.Fatalf("CreateDeployedProduct: %v", err)
	}
	if gotBy != "jane.doe@example.com" {
		t.Errorf("createdBy = %q, want the caller's email", gotBy)
	}
	if gotReq.DeploymentID != req.DeploymentID || gotReq.VersionID != req.VersionID || gotReq.Cores == nil || *gotReq.Cores != 4 || gotReq.Category == nil || *gotReq.Category != "ms" {
		t.Errorf("repository got %+v, want the request unchanged", gotReq)
	}
	if resp.DeployedProduct.ID != "33333333-3333-4333-8333-333333333333" || resp.Message == "" {
		t.Errorf("response = %+v", resp)
	}
}

// Bad input is refused before the repository is reached (the stub panics if
// it is called), and a call with no user token is unauthorized.
func TestDeployedProductService_CreateDeployedProduct_PostgresRefusesBadInput(t *testing.T) {
	valid := domain.CreateDeployedProductRequest{
		ProjectID: testProjectUUID, DeploymentID: testDeploymentUUID,
		ProductID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
	}
	badCategory := "nope"
	cases := map[string]func(r *domain.CreateDeployedProductRequest){
		"projectId":    func(r *domain.CreateDeployedProductRequest) { r.ProjectID = "x" },
		"deploymentId": func(r *domain.CreateDeployedProductRequest) { r.DeploymentID = "" },
		"productId":    func(r *domain.CreateDeployedProductRequest) { r.ProductID = "not-a-uuid" },
		"versionId":    func(r *domain.CreateDeployedProductRequest) { r.VersionID = "x" },
		"category":     func(r *domain.CreateDeployedProductRequest) { r.Category = &badCategory },
	}
	svc := NewDeployedProductService(&stubDeployedProductRepo{})
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for name, mutate := range cases {
		req := valid
		mutate(&req)
		var ve *apierror.ValidationError
		if _, err := svc.CreateDeployedProduct(ctx, req); !errors.As(err, &ve) {
			t.Errorf("%s: want a ValidationError, got %v", name, err)
		}
	}

	var ue *apierror.UnauthorizedError
	if _, err := svc.CreateDeployedProduct(context.Background(), valid); !errors.As(err, &ue) {
		t.Errorf("no user token: want an UnauthorizedError, got %v", err)
	}
}
