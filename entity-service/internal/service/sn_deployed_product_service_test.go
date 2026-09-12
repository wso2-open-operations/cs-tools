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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var (
	testDeployedProductSysid       = sysid32('1')
	testDeployedProductDeploySysid = sysid32('2')
	testDeployedProductProdSysid   = sysid32('3')
	testDeployedProductCatSysid    = sysid32('4')
)

// TestSNDeployedProductService_SearchDeployedProducts_MapsCategoryFromReferenceObject
// guards against reintroducing a plain-string decode for the SN response's
// "category" field. It is a ReferenceTableItem ({id, name, ...}), not a
// string -- decoding it into *string broke every deployed-products search
// with "json: cannot unmarshal object into Go struct field ...category of
// type string".
func TestSNDeployedProductService_SearchDeployedProducts_MapsCategoryFromReferenceObject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"category":   map[string]any{"id": testDeployedProductCatSysid, "name": "Middleware"},
					"createdOn":  "2026-01-01 00:00:00",
					"updatedOn":  "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(resp.DeployedProducts))
	}
	got := resp.DeployedProducts[0]
	if got.Category == nil || *got.Category != "Middleware" {
		t.Fatalf("expected category %q, got %v", "Middleware", got.Category)
	}
}

func TestSNDeployedProductService_SearchDeployedProducts_NilCategoryStaysNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"category":   nil,
					"createdOn":  "2026-01-01 00:00:00",
					"updatedOn":  "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(resp.DeployedProducts))
	}
	if resp.DeployedProducts[0].Category != nil {
		t.Fatalf("expected nil category, got %v", *resp.DeployedProducts[0].Category)
	}
}

// TestSNDeployedProductService_SearchDeployedProducts_CoresTPSNumericAndUpdatesPopulated
// guards the removal of the old fmt.Sprintf-based cores/tps stringification: SN's wire
// type decodes them as numeric (*int / *float64) already, so SearchDeployedProducts must
// pass them straight through instead of re-stringifying, and the updates array must be
// mapped from the SN response into domain.ProductUpdateEntry.
func TestSNDeployedProductService_SearchDeployedProducts_CoresTPSNumericAndUpdatesPopulated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"cores":      4,
					"tps":        100.5,
					"category":   nil,
					"updates": []map[string]any{
						{"updateLevel": 3, "date": "2026-02-01", "details": "Applied patch level 3"},
						{"updateLevel": 2, "date": "2026-01-01", "details": nil},
					},
					"createdOn": "2026-01-01 00:00:00",
					"updatedOn": "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.DeployedProducts) != 1 {
		t.Fatalf("expected 1 deployed product, got %d", len(resp.DeployedProducts))
	}
	got := resp.DeployedProducts[0]

	if got.Cores == nil || *got.Cores != 4 {
		t.Fatalf("Cores = %v, want *int(4)", got.Cores)
	}
	if got.TPS == nil || *got.TPS != 100.5 {
		t.Fatalf("TPS = %v, want *float64(100.5)", got.TPS)
	}

	if len(got.Updates) != 2 {
		t.Fatalf("expected 2 update-history entries, got %d", len(got.Updates))
	}
	if got.Updates[0].UpdateLevel != 3 || got.Updates[0].Date != "2026-02-01" {
		t.Fatalf("Updates[0] = %+v, want {UpdateLevel:3 Date:2026-02-01 ...}", got.Updates[0])
	}
	if got.Updates[0].Details == nil || *got.Updates[0].Details != "Applied patch level 3" {
		t.Fatalf("Updates[0].Details = %v, want non-nil %q", got.Updates[0].Details, "Applied patch level 3")
	}
	if got.Updates[1].UpdateLevel != 2 || got.Updates[1].Details != nil {
		t.Fatalf("Updates[1] = %+v, want {UpdateLevel:2 Details:nil}", got.Updates[1])
	}
}

// TestSNDeployedProductService_SearchDeployedProducts_NilUpdatesStaysNil confirms an
// absent "updates" field on the wire produces a nil (not empty-non-nil) domain slice.
func TestSNDeployedProductService_SearchDeployedProducts_NilUpdatesStaysNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployedProducts": []map[string]any{
				{
					"id":         testDeployedProductSysid,
					"deployment": map[string]any{"id": testDeployedProductDeploySysid, "name": "Production"},
					"product":    map[string]any{"id": testDeployedProductProdSysid, "name": "API Manager"},
					"createdOn":  "2026-01-01 00:00:00",
					"updatedOn":  "2026-01-02 00:00:00",
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 20,
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	resp, err := svc.SearchDeployedProducts(contextWithUserIDToken("token"), domain.SearchDeployedProductsRequest{
		Pagination: domain.Pagination{Limit: 20, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeployedProducts[0].Updates != nil {
		t.Fatalf("expected nil Updates, got %+v", resp.DeployedProducts[0].Updates)
	}
}

// TestSNDeployedProductService_UpdateDeployedProduct_UpdatesRoundTrip proves a request
// carrying a non-empty Updates array is forwarded to SN as the whole-array-replace payload,
// and the mocked SN response's echoed "updates" array comes back through in the domain
// response.
func TestSNDeployedProductService_UpdateDeployedProduct_UpdatesRoundTrip(t *testing.T) {
	productUUID := sysidToUUID(testDeployedProductSysid)
	details := "Applied patch level 5"

	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/"+testDeployedProductSysid, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "updated",
			"deployedProduct": map[string]any{
				"id":        testDeployedProductSysid,
				"updatedOn": "2026-02-01 00:00:00",
				"updatedBy": "jane.doe@example.com",
				"updates": []map[string]any{
					{"updateLevel": 5, "date": "2026-02-01", "details": details},
				},
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	req := domain.UpdateDeployedProductRequest{
		ID: productUUID,
		Updates: []domain.ProductUpdateEntry{
			{UpdateLevel: 4, Date: "2026-01-01", Details: nil},
			{UpdateLevel: 5, Date: "2026-02-01", Details: &details},
		},
	}

	resp, err := svc.UpdateDeployedProduct(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert the outbound payload carried the full Updates array, not just a delta.
	rawUpdates, ok := gotBody["updates"].([]any)
	if !ok || len(rawUpdates) != 2 {
		t.Fatalf("outbound payload updates = %v, want a 2-element array", gotBody["updates"])
	}
	first, _ := rawUpdates[0].(map[string]any)
	if first["updateLevel"] != float64(4) || first["date"] != "2026-01-01" {
		t.Fatalf("outbound payload updates[0] = %v, want {updateLevel:4 date:2026-01-01}", first)
	}

	if len(resp.DeployedProduct.Updates) != 1 {
		t.Fatalf("expected 1 echoed update entry, got %d", len(resp.DeployedProduct.Updates))
	}
	if resp.DeployedProduct.Updates[0].UpdateLevel != 5 {
		t.Fatalf("echoed Updates[0].UpdateLevel = %d, want 5", resp.DeployedProduct.Updates[0].UpdateLevel)
	}
	if resp.DeployedProduct.Updates[0].Details == nil || *resp.DeployedProduct.Updates[0].Details != details {
		t.Fatalf("echoed Updates[0].Details = %v, want %q", resp.DeployedProduct.Updates[0].Details, details)
	}
}

// TestSNDeployedProductService_UpdateDeployedProduct_UpdatesAloneSatisfiesDetailFields
// proves a request carrying only Updates (no cores/tps/description) is accepted as a
// valid "detail fields" update rather than rejected as an empty request.
func TestSNDeployedProductService_UpdateDeployedProduct_UpdatesAloneSatisfiesDetailFields(t *testing.T) {
	productUUID := sysidToUUID(testDeployedProductSysid)

	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/"+testDeployedProductSysid, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "updated",
			"deployedProduct": map[string]any{
				"id":        testDeployedProductSysid,
				"updatedOn": "2026-02-01 00:00:00",
				"updatedBy": "jane.doe@example.com",
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	req := domain.UpdateDeployedProductRequest{
		ID:      productUUID,
		Updates: []domain.ProductUpdateEntry{{UpdateLevel: 1, Date: "2026-01-01"}},
	}
	if _, err := svc.UpdateDeployedProduct(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSNDeployedProductService_UpdateDeployedProduct_EmptyUpdatesArrayClearsHistory proves
// a request carrying Updates set to a non-nil empty slice (the caller's way of clearing all
// update history) actually serialises as "updates":[] on the wire. Before this fix, the wire
// field's plain (non-pointer) slice type meant encoding/json's omitempty dropped an empty
// slice from the outgoing JSON regardless of nil-ness, so an explicit clear silently no-opped.
func TestSNDeployedProductService_UpdateDeployedProduct_EmptyUpdatesArrayClearsHistory(t *testing.T) {
	productUUID := sysidToUUID(testDeployedProductSysid)

	var gotBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/deployed-products/"+testDeployedProductSysid, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "updated",
			"deployedProduct": map[string]any{
				"id":        testDeployedProductSysid,
				"updatedOn": "2026-02-01 00:00:00",
				"updatedBy": "jane.doe@example.com",
			},
		})
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowDeployedProductService(client)

	req := domain.UpdateDeployedProductRequest{
		ID:      productUUID,
		Updates: []domain.ProductUpdateEntry{},
	}
	if _, err := svc.UpdateDeployedProduct(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotUpdates, hasKey := gotBody["updates"]
	if !hasKey {
		t.Fatalf("outbound payload has no %q key, want an explicit empty array (body: %v)", "updates", gotBody)
	}
	arr, ok := gotUpdates.([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("outbound payload updates = %v (%T), want []", gotUpdates, gotUpdates)
	}
}

// TestSNDeployedProductService_UpdateDeployedProduct_ValidatesUpdateEntries proves each
// entry in Updates is validated before the request ever reaches the backing service: a
// negative updateLevel and a malformed date are both rejected, and a well-formed array
// passes through untouched.
func TestSNDeployedProductService_UpdateDeployedProduct_ValidatesUpdateEntries(t *testing.T) {
	productUUID := sysidToUUID(testDeployedProductSysid)

	cases := []struct {
		name      string
		updates   []domain.ProductUpdateEntry
		wantValid bool
	}{
		{
			name:      "negative updateLevel rejected",
			updates:   []domain.ProductUpdateEntry{{UpdateLevel: -1, Date: "2026-01-01"}},
			wantValid: false,
		},
		{
			name:      "malformed date rejected",
			updates:   []domain.ProductUpdateEntry{{UpdateLevel: 1, Date: "01/01/2026"}},
			wantValid: false,
		},
		{
			name:      "valid entries pass through unchanged",
			updates:   []domain.ProductUpdateEntry{{UpdateLevel: 1, Date: "2026-01-01"}},
			wantValid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.wantValid {
					t.Fatalf("unexpected call to backing service for an invalid request")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"message": "updated",
					"deployedProduct": map[string]any{
						"id":        testDeployedProductSysid,
						"updatedOn": "2026-02-01 00:00:00",
						"updatedBy": "jane.doe@example.com",
					},
				})
			}))
			svc := NewServiceNowDeployedProductService(client)

			req := domain.UpdateDeployedProductRequest{ID: productUUID, Updates: tc.updates}
			_, err := svc.UpdateDeployedProduct(contextWithUserIDToken("token"), req)

			if tc.wantValid {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("error = %v (%T), want ValidationError", err, err)
			}
		})
	}
}
