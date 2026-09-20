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

// The abbreviation and the update history are the two halves the Updates page
// needs: the abbreviation says which catalogue to read ("wso2am"), the history
// says which level the deployment is already on. Either one missing makes a
// deployed product unmatchable, which is exactly the state these tests exist to
// stop recurring.

func abbrevPtr(s string) *string { return &s }

func sampleDeployedProduct() entity.SearchDeployedProductsResponse {
	return entity.SearchDeployedProductsResponse{
		DeployedProducts: []entity.DeployedProductView{
			{
				ID:         "3db55042-3b1b-4310-3e1e-088aa4e45a67",
				Deployment: entity.EntityRef{ID: "dep-1", Name: "Primary Production"},
				Product: entity.ProductEntityRef{
					ID:           "prod-1",
					Name:         "WSO2 API Manager",
					Abbreviation: abbrevPtr("wso2am"),
				},
				Description: abbrevPtr("API"),
				Updates: []entity.ProductUpdateEntry{
					{UpdateLevel: 70, Date: "2026-05-07", Details: abbrevPtr("initial update")},
				},
				CreatedOn: time.Date(2026, 9, 14, 9, 13, 53, 0, time.UTC),
				UpdatedOn: time.Date(2026, 9, 14, 9, 13, 53, 0, time.UTC),
			},
		},
		Total: 1, Limit: 10, Offset: 0,
	}
}

// TestMapSearchDeployedProducts_ExposesAbbreviation is the regression guard:
// the product-updates service keys its catalogue by this value, so dropping it
// leaves every deployed product unmatchable against its update levels.
func TestMapSearchDeployedProducts_ExposesAbbreviation(t *testing.T) {
	out := MapSearchDeployedProducts(sampleDeployedProduct())

	if len(out.DeployedProducts) != 1 {
		t.Fatalf("got %d products, want 1", len(out.DeployedProducts))
	}
	p := out.DeployedProducts[0].Product
	if p == nil {
		t.Fatal("product reference was dropped")
	}
	if p.Abbreviation == nil {
		t.Fatal("abbreviation was dropped — the update catalogue cannot be matched without it")
	}
	if *p.Abbreviation != "wso2am" {
		t.Errorf("abbreviation = %q, want %q", *p.Abbreviation, "wso2am")
	}
	if p.Label != "WSO2 API Manager" {
		t.Errorf("label = %q, want the display name", p.Label)
	}
}

// TestMapSearchDeployedProducts_ExposesUpdateHistory covers the other half.
func TestMapSearchDeployedProducts_ExposesUpdateHistory(t *testing.T) {
	out := MapSearchDeployedProducts(sampleDeployedProduct())

	updates := out.DeployedProducts[0].Updates
	if len(updates) != 1 {
		t.Fatalf("got %d update entries, want 1", len(updates))
	}
	if updates[0].UpdateLevel != 70 {
		t.Errorf("updateLevel = %d, want 70", updates[0].UpdateLevel)
	}
	// The date must survive as the upstream's date-only string, not be
	// reformatted into a timestamp — the catalogue compares them as-is.
	if updates[0].Date != "2026-05-07" {
		t.Errorf("date = %q, want the upstream date-only string", updates[0].Date)
	}
}

// TestMapSearchDeployedProducts_ExposesDescription guards the third field of
// this class. The Manage Products dialog prefills its description editor from
// this value and diffs against it to decide whether to send a change, so an
// absent description made an existing one invisible in the UI. The v1 Ballerina
// backend has always returned it.
func TestMapSearchDeployedProducts_ExposesDescription(t *testing.T) {
	out := MapSearchDeployedProducts(sampleDeployedProduct())

	got := out.DeployedProducts[0].Description
	if got == nil {
		t.Fatal("description was dropped")
	}
	if *got != "API" {
		t.Errorf("description = %q, want %q", *got, "API")
	}
}

// TestMapSearchDeployedProducts_SerialisedKeys pins the JSON field names the
// frontend will read. A struct-tag typo would otherwise pass every assertion
// above while shipping an unusable response.
func TestMapSearchDeployedProducts_SerialisedKeys(t *testing.T) {
	out := MapSearchDeployedProducts(sampleDeployedProduct())

	encoded, err := json.Marshal(out.DeployedProducts[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Description *string `json:"description"`
		Product     *struct {
			ID           string  `json:"id"`
			Label        string  `json:"label"`
			Abbreviation *string `json:"abbreviation"`
		} `json:"product"`
		Updates []struct {
			UpdateLevel int    `json:"updateLevel"`
			Date        string `json:"date"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Product == nil || decoded.Product.Abbreviation == nil {
		t.Fatalf("product.abbreviation absent from the wire form: %s", encoded)
	}
	if *decoded.Product.Abbreviation != "wso2am" {
		t.Errorf("product.abbreviation = %q, want wso2am", *decoded.Product.Abbreviation)
	}
	if decoded.Product.Label != "WSO2 API Manager" {
		t.Errorf("product.label = %q — the key must stay `label`, not `name`", decoded.Product.Label)
	}
	if len(decoded.Updates) != 1 || decoded.Updates[0].UpdateLevel != 70 {
		t.Errorf("updates absent or wrong in the wire form: %s", encoded)
	}
	if decoded.Description == nil || *decoded.Description != "API" {
		t.Errorf("description absent from the wire form: %s", encoded)
	}
}

// TestMapSearchDeployedProducts_OmitsAbsentOptionalFields covers the Postgres
// data source, whose products table has no abbreviation column, and a product
// with no recorded updates. Neither may serialise as an empty value the
// frontend would have to special-case.
func TestMapSearchDeployedProducts_OmitsAbsentOptionalFields(t *testing.T) {
	in := sampleDeployedProduct()
	in.DeployedProducts[0].Product.Abbreviation = nil
	in.DeployedProducts[0].Updates = nil
	in.DeployedProducts[0].Description = nil

	out := MapSearchDeployedProducts(in)
	encoded, err := json.Marshal(out.DeployedProducts[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := raw["updates"]; present {
		t.Errorf("updates should be omitted when there is no history: %s", encoded)
	}
	product, _ := raw["product"].(map[string]any)
	if _, present := product["abbreviation"]; present {
		t.Errorf("abbreviation should be omitted when the data source has none: %s", encoded)
	}
	if _, present := raw["description"]; present {
		t.Errorf("description should be omitted when there is none: %s", encoded)
	}
}

// TestDeployedProductViewUnmarshal_DecodesBothFields guards the hand-written
// UnmarshalJSON on entity.DeployedProductView: it decodes into a shadow struct,
// so a field added to the view but not to that struct is silently dropped.
func TestDeployedProductViewUnmarshal_DecodesBothFields(t *testing.T) {
	const upstream = `{
		"id": "dp-1",
		"deployment": {"id": "dep-1", "name": "Primary Production"},
		"product": {"id": "prod-1", "name": "WSO2 Identity Server", "abbreviation": "wso2is"},
		"description": "API",
		"cores": 2,
		"tps": 100.0,
		"updates": [{"updateLevel": 70, "date": "2026-05-07", "details": "initial update"}],
		"createdOn": "2026-09-14T09:13:53Z",
		"updatedOn": "2026-09-14T09:13:53Z"
	}`

	var view entity.DeployedProductView
	if err := json.Unmarshal([]byte(upstream), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if view.Product.Abbreviation == nil || *view.Product.Abbreviation != "wso2is" {
		t.Errorf("abbreviation not decoded: %+v", view.Product)
	}
	if len(view.Updates) != 1 || view.Updates[0].UpdateLevel != 70 {
		t.Errorf("updates not decoded: %+v", view.Updates)
	}
	if view.Description == nil || *view.Description != "API" {
		t.Errorf("description not decoded: %+v", view.Description)
	}
}
