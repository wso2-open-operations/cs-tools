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

package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestBuildEntityCreateDeployedProductRequest_SendsDashedUUIDs is the
// regression test for "projectId contains invalid UUID" on
// POST /deployments/{deploymentId}/products. Every identifier must leave this
// backend in the canonical dashed form whichever shape it arrived in:
// entity-service validates all four with validateUUIDs and converts them to
// sysids itself, so a hyphen-stripped value was rejected before any deployed
// product could be created.
func TestBuildEntityCreateDeployedProductRequest_SendsDashedUUIDs(t *testing.T) {
	const (
		dashedProject    = "a1b2c3d4-e5f6-0718-293a-4b5c6d7e8f90"
		dashlessProject  = "a1b2c3d4e5f60718293a4b5c6d7e8f90"
		dashlessDeploy   = "0f1e2d3c4b5a69788796a5b4c3d2e1f0"
		dashedDeployment = "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"
	)

	got := BuildEntityCreateDeployedProductRequest(dashlessDeploy, DeployedProductCreateRequest{
		ProjectID: dashlessProject,
		ProductID: "5e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		VersionID: "6E8431B1-1B8C-0310-0BB3-DA47B04BCBA6",
	})

	if got.ProjectID != dashedProject {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, dashedProject)
	}
	if got.DeploymentID != dashedDeployment {
		t.Errorf("DeploymentID = %q, want %q", got.DeploymentID, dashedDeployment)
	}
	if got.ProductID != "5e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("ProductID = %q, want the dashed form unchanged", got.ProductID)
	}
	if got.VersionID != "6e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("VersionID = %q, want the lowercased dashed form", got.VersionID)
	}
}

// TestBuildEntityUpdateDeployedProductRequest_SendsDashedUUIDs covers the same
// direction on PATCH, whose id and deploymentId entity-service validates the
// same way.
func TestBuildEntityUpdateDeployedProductRequest_SendsDashedUUIDs(t *testing.T) {
	got := BuildEntityUpdateDeployedProductRequest(
		"4e8431b11b8c03100bb3da47b04bcba6",
		"0f1e2d3c4b5a69788796a5b4c3d2e1f0",
		DeployedProductUpdateRequest{},
	)

	if got.ID != "4e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("ID = %q, want the dashed form", got.ID)
	}
	if got.DeploymentID == nil || *got.DeploymentID != "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0" {
		t.Errorf("DeploymentID = %v, want the dashed form", got.DeploymentID)
	}
}

func TestBuildEntitySearchDeployedProductsRequest(t *testing.T) {
	t.Run("converts dashed UUID to canonical dashed UUID and sets root deploymentIds without filters", func(t *testing.T) {
		req := DeployedProductSearchRequest{
			Pagination: entity.Pagination{Limit: 10, Offset: 0},
			Filters: &DeployedProductSearchFilters{
				ProductCategories: []string{"Integration"},
			},
		}
		got := BuildEntitySearchDeployedProductsRequest("4e8431b1-1b8c-0310-0bb3-da47b04bcba6", req)

		expectedDashedID := "4e8431b1-1b8c-0310-0bb3-da47b04bcba6"
		if len(got.DeploymentIDs) != 1 || got.DeploymentIDs[0] != expectedDashedID {
			t.Errorf("got DeploymentIDs = %v, want [%s]", got.DeploymentIDs, expectedDashedID)
		}

		data, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if _, hasFilters := raw["filters"]; hasFilters {
			t.Errorf("expected no filters object in serialized json, got: %v", raw)
		}
		deps, ok := raw["deploymentIds"].([]any)
		if !ok || len(deps) != 1 || deps[0] != expectedDashedID {
			t.Errorf("expected root deploymentIds to contain %s, got: %v", expectedDashedID, raw["deploymentIds"])
		}
	})

	t.Run("bare sysid normalized to dashed UUID at root", func(t *testing.T) {
		req := DeployedProductSearchRequest{
			Pagination: entity.Pagination{Limit: 20, Offset: 10},
		}
		got := BuildEntitySearchDeployedProductsRequest("4e8431b11b8c03100bb3da47b04bcba6", req)
		expectedDashedID := "4e8431b1-1b8c-0310-0bb3-da47b04bcba6"
		if len(got.DeploymentIDs) != 1 || got.DeploymentIDs[0] != expectedDashedID {
			t.Errorf("got DeploymentIDs = %v, want [%s]", got.DeploymentIDs, expectedDashedID)
		}
	})
}

func TestUnmarshalSearchDeployedProductsResponse_UpstreamBallerinaPayload(t *testing.T) {
	rawJSON := `{
		"deployedProducts": [
			{
				"id": "dp123456789012345678901234567890",
				"deployment": {"id": "d1", "name": "Prod"},
				"product": {"id": "p1", "name": "APIM"},
				"version": {
					"id": "v1",
					"name": "4.2.0",
					"releasedOn": "2023-01-15",
					"endOfLifeOn": "2026-01-15"
				},
				"cores": 8,
				"tps": 250.5,
				"category": {
					"id": "cat1",
					"name": "API Management"
				},
				"createdOn": "2024-02-10 09:30:00",
				"updatedOn": "2024-02-11 10:00:00"
			}
		],
		"totalRecords": 1,
		"offset": 0,
		"limit": 10
	}`

	var entityResp entity.SearchDeployedProductsResponse
	if err := json.Unmarshal([]byte(rawJSON), &entityResp); err != nil {
		t.Fatalf("failed to unmarshal upstream ballerina response: %v", err)
	}

	if entityResp.Total != 1 {
		t.Errorf("entityResp.Total = %d, want 1", entityResp.Total)
	}
	if len(entityResp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(entityResp.DeployedProducts))
	}
	dp := entityResp.DeployedProducts[0]
	if dp.Cores == nil || *dp.Cores != "8" {
		t.Errorf("dp.Cores = %v, want '8'", dp.Cores)
	}
	if dp.TPS == nil || *dp.TPS != "250.5" {
		t.Errorf("dp.TPS = %v, want '250.5'", dp.TPS)
	}
	if dp.Category == nil || *dp.Category != "API Management" {
		t.Errorf("dp.Category = %v, want 'API Management'", dp.Category)
	}
	if dp.Version == nil || dp.Version.ReleasedDate == nil {
		t.Fatalf("expected version releasedDate to be parsed")
	}
	expectedReleased := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	if !dp.Version.ReleasedDate.Equal(expectedReleased) {
		t.Errorf("version.ReleasedDate = %v, want %v", dp.Version.ReleasedDate, expectedReleased)
	}

	// Now map to portal response
	portalResp := MapSearchDeployedProducts(entityResp)
	if portalResp.TotalRecords != 1 {
		t.Errorf("portalResp.TotalRecords = %d, want 1", portalResp.TotalRecords)
	}
	if len(portalResp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 portal deployed product, got %d", len(portalResp.DeployedProducts))
	}
	summary := portalResp.DeployedProducts[0]
	if summary.Cores == nil || *summary.Cores != 8 {
		t.Errorf("summary.Cores = %v, want 8", summary.Cores)
	}
	if summary.TPS == nil || *summary.TPS != 250.5 {
		t.Errorf("summary.TPS = %v, want 250.5", summary.TPS)
	}
	if summary.Category == nil || *summary.Category != "API Management" {
		t.Errorf("summary.Category = %v, want 'API Management'", summary.Category)
	}
}

func TestBuildEntityCreateDeployedProductRequest(t *testing.T) {
	cores := 4
	tps := 100.0
	desc := "test product"
	req := DeployedProductCreateRequest{
		ProductID:   "5e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		VersionID:   "6e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		ProjectID:   "7e8431b1-1b8c-0310-0bb3-da47b04bcba6",
		Cores:       &cores,
		TPS:         &tps,
		Description: &desc,
	}
	got := BuildEntityCreateDeployedProductRequest("4e8431b1-1b8c-0310-0bb3-da47b04bcba6", req)

	if got.DeploymentID != "4e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got DeploymentID = %q, want %q", got.DeploymentID, "4e8431b1-1b8c-0310-0bb3-da47b04bcba6")
	}
	if got.ProductID != "5e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got ProductID = %q, want %q", got.ProductID, "5e8431b1-1b8c-0310-0bb3-da47b04bcba6")
	}
	if got.VersionID != "6e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got VersionID = %q, want %q", got.VersionID, "6e8431b1-1b8c-0310-0bb3-da47b04bcba6")
	}
	if got.ProjectID != "7e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got ProjectID = %q, want %q", got.ProjectID, "7e8431b1-1b8c-0310-0bb3-da47b04bcba6")
	}
}

func TestBuildEntityUpdateDeployedProductRequest(t *testing.T) {
	cores := 2
	req := DeployedProductUpdateRequest{
		Cores: &cores,
	}
	got := BuildEntityUpdateDeployedProductRequest("1e8431b1-1b8c-0310-0bb3-da47b04bcba6", "2e8431b1-1b8c-0310-0bb3-da47b04bcba6", req)

	if got.ID != "1e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got ID = %q, want %q", got.ID, "1e8431b1-1b8c-0310-0bb3-da47b04bcba6")
	}
	if got.DeploymentID == nil || *got.DeploymentID != "2e8431b1-1b8c-0310-0bb3-da47b04bcba6" {
		t.Errorf("got DeploymentID = %v, want 2e8431b1-1b8c-0310-0bb3-da47b04bcba6", got.DeploymentID)
	}
}

func TestDeployedProductDecoders_PropagateTimestampErrors(t *testing.T) {
	t.Run("invalid createdOn in DeployedProductView returns error", func(t *testing.T) {
		raw := `{"id":"1","createdOn":"invalid-time","updatedOn":"2024-01-01"}`
		var dp entity.DeployedProductView
		if err := json.Unmarshal([]byte(raw), &dp); err == nil {
			t.Error("expected error unmarshaling malformed createdOn, got nil")
		}
	})

	t.Run("invalid updatedOn in DeployedProductView returns error", func(t *testing.T) {
		raw := `{"id":"1","createdOn":"2024-01-01","updatedOn":"invalid-time"}`
		var dp entity.DeployedProductView
		if err := json.Unmarshal([]byte(raw), &dp); err == nil {
			t.Error("expected error unmarshaling malformed updatedOn, got nil")
		}
	})

	t.Run("invalid createdOn in CreatedDeployedProduct returns error", func(t *testing.T) {
		raw := `{"id":"1","createdOn":"invalid-time"}`
		var cp entity.CreatedDeployedProduct
		if err := json.Unmarshal([]byte(raw), &cp); err == nil {
			t.Error("expected error unmarshaling malformed createdOn, got nil")
		}
	})

	t.Run("invalid updatedOn in UpdatedDeployedProduct returns error", func(t *testing.T) {
		raw := `{"id":"1","updatedOn":"invalid-time"}`
		var up entity.UpdatedDeployedProduct
		if err := json.Unmarshal([]byte(raw), &up); err == nil {
			t.Error("expected error unmarshaling malformed updatedOn, got nil")
		}
	})

	t.Run("invalid releasedOn in DeployedProductVersionRef returns error", func(t *testing.T) {
		raw := `{"id":"1","name":"v1","releasedOn":"invalid-time"}`
		var vr entity.DeployedProductVersionRef
		if err := json.Unmarshal([]byte(raw), &vr); err == nil {
			t.Error("expected error unmarshaling malformed releasedOn, got nil")
		}
	})

	t.Run("nil or empty dates are allowed", func(t *testing.T) {
		raw := `{"id":"1","name":"v1","releasedOn":null,"endOfLifeOn":""}`
		var vr entity.DeployedProductVersionRef
		if err := json.Unmarshal([]byte(raw), &vr); err != nil {
			t.Fatalf("expected nil/empty dates to unmarshal cleanly, got: %v", err)
		}
		if vr.ReleasedDate != nil {
			t.Errorf("expected ReleasedDate to be nil, got: %v", vr.ReleasedDate)
		}
		if vr.SupportEoLDate != nil {
			t.Errorf("expected SupportEoLDate to be nil, got: %v", vr.SupportEoLDate)
		}
	})
}
