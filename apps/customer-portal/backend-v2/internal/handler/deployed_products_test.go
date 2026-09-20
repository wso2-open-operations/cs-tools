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

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type fakeDeployedProductEntity struct {
	gotSearch entity.SearchDeployedProductsRequest
	gotCreate entity.CreateDeployedProductRequest
	gotUpdate struct {
		id  string
		req entity.UpdateDeployedProductRequest
	}
}

func (f *fakeDeployedProductEntity) SearchDeployedProducts(_ context.Context, req entity.SearchDeployedProductsRequest) (entity.SearchDeployedProductsResponse, error) {
	f.gotSearch = req
	cores := "4"
	tps := "100.0"
	cat := "Integration"
	return entity.SearchDeployedProductsResponse{
		DeployedProducts: []entity.DeployedProductView{
			{
				ID:         "dp123456789012345678901234567890",
				Deployment: entity.EntityRef{ID: "dep1", Name: "Prod"},
				Product:    entity.ProductEntityRef{ID: "prod1", Name: "APIM"},
				Cores:      &cores,
				TPS:        &tps,
				Category:   &cat,
				CreatedOn:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedOn:  time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			},
		},
		Total:  1,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}

func (f *fakeDeployedProductEntity) CreateDeployedProduct(_ context.Context, req entity.CreateDeployedProductRequest) (entity.CreateDeployedProductResponse, error) {
	f.gotCreate = req
	return entity.CreateDeployedProductResponse{
		Message: "Created",
		DeployedProduct: entity.CreatedDeployedProduct{
			ID:        "dp-created-1",
			CreatedOn: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy: "user-1",
		},
	}, nil
}

func (f *fakeDeployedProductEntity) UpdateDeployedProduct(_ context.Context, id string, req entity.UpdateDeployedProductRequest) (entity.UpdateDeployedProductResponse, error) {
	f.gotUpdate.id = id
	f.gotUpdate.req = req
	return entity.UpdateDeployedProductResponse{
		Message: "Updated",
		DeployedProduct: entity.UpdatedDeployedProduct{
			ID:        id,
			UpdatedOn: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
			UpdatedBy: "user-1",
		},
	}, nil
}

func (f *fakeDeployedProductEntity) SearchDeployedProductMetrics(_ context.Context, _ string, _ entity.DeployedProductMetricsRequest) (entity.DeployedProductMetricsResponse, error) {
	return entity.DeployedProductMetricsResponse{}, nil
}

func (f *fakeDeployedProductEntity) SearchDeployedProductUsageCounts(_ context.Context, _ string, _ entity.DeployedProductUsageCountsRequest) (entity.DeployedProductUsageCountsResponse, error) {
	return entity.DeployedProductUsageCountsResponse{}, nil
}

func TestSearchDeployedProducts_AcceptsUUIDAndBareSysID(t *testing.T) {
	tests := []struct {
		name         string
		deploymentID string
		wantDashedID string
	}{
		{
			name:         "dashed UUID",
			deploymentID: "4e8431b1-1b8c-0310-0bb3-da47b04bcba6",
			wantDashedID: "4e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		},
		{
			name:         "bare 32-hex sysid",
			deploymentID: "4e8431b11b8c03100bb3da47b04bcba6",
			wantDashedID: "4e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeDeployedProductEntity{}
			mux := http.NewServeMux()
			mux.HandleFunc("POST /deployments/{deploymentId}/products/search", NewDeployedProductHandler(fake).SearchDeployedProducts)

			body := `{"pagination":{"limit":10,"offset":0},"filters":{"productCategories":["Integration"]}}`
			req := authedRequest(http.MethodPost, "/deployments/"+tc.deploymentID+"/products/search", body)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
			}

			if len(fake.gotSearch.DeploymentIDs) != 1 || fake.gotSearch.DeploymentIDs[0] != tc.wantDashedID {
				t.Errorf("got DeploymentIDs = %v, want [%s]", fake.gotSearch.DeploymentIDs, tc.wantDashedID)
			}
		})
	}
}

func TestSearchDeployedProducts_RejectsInvalidID(t *testing.T) {
	fake := &fakeDeployedProductEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployments/{deploymentId}/products/search", NewDeployedProductHandler(fake).SearchDeployedProducts)

	invalidIDs := []string{
		"not-a-valid-id",
		"4e8431b1-1b8c-0310-0bb3",
		"4e8431b11b8c03100bb3da47b04bcba6zz",
		"12345",
	}

	for _, id := range invalidIDs {
		w := httptest.NewRecorder()
		req := authedRequest(http.MethodPost, "/deployments/"+id+"/products/search", `{"pagination":{"limit":10,"offset":0}}`)
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("id %q: status = %d, want 400", id, w.Code)
		}
	}
}

func TestSearchDeployedProducts_RejectsUnauthenticated(t *testing.T) {
	fake := &fakeDeployedProductEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployments/{deploymentId}/products/search", NewDeployedProductHandler(fake).SearchDeployedProducts)

	req := httptest.NewRequest(http.MethodPost, "/deployments/4e8431b1-1b8c-0310-0bb3-da47b04bcba6/products/search", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// Identifiers must reach entity-service as canonical dashed UUIDs: it
// validates them with validateUUIDs and converts them to sysids itself, so
// sending a hyphen-stripped value made every create fail with
// "projectId contains invalid UUID".
func TestCreateDeployedProduct_NormalizesIdentifiersToDashedUUID(t *testing.T) {
	fake := &fakeDeployedProductEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployments/{deploymentId}/products", NewDeployedProductHandler(fake).CreateDeployedProduct)

	body := `{
		"productId": "5e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		"versionId": "6e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		"projectId": "7e8431b1-1b8c-0310-0bb3-da47b04bcba6"
	}`
	req := authedRequest(http.MethodPost, "/deployments/4e8431b1-1b8c-0310-0bb3-da47b04bcba6/products", body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", w.Code, w.Body.String())
	}

	if fake.gotCreate.DeploymentID != "4e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got DeploymentID = %q, want the dashed UUID", fake.gotCreate.DeploymentID)
	}
	if fake.gotCreate.ProductID != "5e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got ProductID = %q, want the dashed UUID", fake.gotCreate.ProductID)
	}
	if fake.gotCreate.ProjectID != "7e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got ProjectID = %q, want the dashed UUID", fake.gotCreate.ProjectID)
	}
}

func TestCreateDeployedProduct_RejectsInvalidBodyIdentifiers(t *testing.T) {
	fake := &fakeDeployedProductEntity{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /deployments/{deploymentId}/products", NewDeployedProductHandler(fake).CreateDeployedProduct)

	invalidBodies := []struct {
		name string
		body string
	}{
		{
			name: "malformed product id",
			body: `{"productId": "dp-created-1", "versionId": "6e8431b1-1b8c-0310-0bb3-da47b04bcba6", "projectId": "7e8431b1-1b8c-0310-0bb3-da47b04bcba6"}`,
		},
		{
			name: "malformed version id",
			body: `{"productId": "5e8431b1-1b8c-0310-0bb3-da47b04bcba6", "versionId": "invalid", "projectId": "7e8431b1-1b8c-0310-0bb3-da47b04bcba6"}`,
		},
		{
			name: "empty project id",
			body: `{"productId": "5e8431b1-1b8c-0310-0bb3-da47b04bcba6", "versionId": "6e8431b1-1b8c-0310-0bb3-da47b04bcba6", "projectId": ""}`,
		},
	}

	for _, tc := range invalidBodies {
		t.Run(tc.name, func(t *testing.T) {
			req := authedRequest(http.MethodPost, "/deployments/4e8431b1-1b8c-0310-0bb3-da47b04bcba6/products", tc.body)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", w.Code, w.Body.String())
			}
		})
	}
}

// Either id shape may arrive on the URL, and both must leave as the canonical
// dashed UUID — entity-service validates id and deploymentId with
// validateUUIDs on this route too, and converts them to sysids itself.
func TestPatchDeployedProduct_AcceptsBothIDShapes(t *testing.T) {
	tests := map[string]struct{ deploymentID, productID string }{
		"dashed uuids":  {"4e8431b1-1b8c-0310-0bb3-da47b04bcba6", "5e8431b1-1b8c-0310-0bb3-da47b04bcba6"},
		"bare sysids":   {"4e8431b11b8c03100bb3da47b04bcba6", "5e8431b11b8c03100bb3da47b04bcba6"},
		"mixed shapes":  {"4e8431b1-1b8c-0310-0bb3-da47b04bcba6", "5e8431b11b8c03100bb3da47b04bcba6"},
		"uppercase hex": {"4E8431B1-1B8C-0310-0BB3-DA47B04BCBA6", "5E8431B11B8C03100BB3DA47B04BCBA6"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeDeployedProductEntity{}
			mux := http.NewServeMux()
			mux.HandleFunc("PATCH /deployments/{deploymentId}/products/{id}", NewDeployedProductHandler(fake).PatchDeployedProduct)

			req := authedRequest(http.MethodPatch, "/deployments/"+tc.deploymentID+"/products/"+tc.productID, `{"cores": 8}`)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
			}
			if fake.gotUpdate.id != "5e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
				t.Errorf("got update id = %q, want the dashed UUID", fake.gotUpdate.id)
			}
			if fake.gotUpdate.req.DeploymentID == nil || *fake.gotUpdate.req.DeploymentID != "4e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
				t.Errorf("got DeploymentID = %v, want the dashed UUID", fake.gotUpdate.req.DeploymentID)
			}
		})
	}
}
