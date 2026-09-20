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

import "testing"

// strPtr lives in case_detail_fields_test.go — shared across this package's tests.

// snClassLabel is what entity-service passes through from ServiceNow, and
// portalClassValue is what the frontend sends (PRODUCT_CLASS.PRODUCT_MODEL in
// productConstants.ts). They are the same class in two spellings — the whole
// point of normalizeProductClass.
const (
	snClassLabel     = "Product Model"
	portalClassValue = "product_model"
)

// TestFilterProductsByClass_PortalValueMatchesServiceNowLabel is the
// regression test for GET /products?class=product_model returning
// `"products": []` against a page that was entirely product_model: the filter
// compared with strings.EqualFold, which folds case but not the space in
// "Product Model" against the underscore in "product_model", so every item
// was discarded while totalRecords still reported the unfiltered count.
func TestFilterProductsByClass_PortalValueMatchesServiceNowLabel(t *testing.T) {
	page := SearchProductsResponse{
		Products: []ProductSummary{
			{ID: "1", Name: "WSO2 API Manager", Class: strPtr(snClassLabel)},
			{ID: "2", Name: "WSO2 Identity Server", Class: strPtr(snClassLabel)},
		},
		TotalRecords: 41,
		Limit:        10,
		Offset:       0,
		HasMore:      true,
	}

	got := FilterProductsByClass(page, portalClassValue)

	if len(got.Products) != 2 {
		t.Fatalf("products = %d, want 2 — the frontend's %q must match ServiceNow's %q",
			len(got.Products), portalClassValue, snClassLabel)
	}
	// Pagination metadata describes entity-service's own unfiltered page and
	// is deliberately left untouched (see FilterProductsByClass's doc comment).
	if got.TotalRecords != 41 || got.Limit != 10 || got.Offset != 0 || !got.HasMore {
		t.Errorf("pagination metadata was modified: %+v", got)
	}
}

// TestFilterProductsByClass_Spellings pins every spelling that must resolve to
// the same class, and the ones that must not.
func TestFilterProductsByClass_Spellings(t *testing.T) {
	tests := map[string]struct {
		itemClass *string
		want      string
		kept      bool
	}{
		"portal value vs SN label":    {itemClass: strPtr("Product Model"), want: "product_model", kept: true},
		"SN label vs SN label":        {itemClass: strPtr("Product Model"), want: "Product Model", kept: true},
		"portal value both sides":     {itemClass: strPtr("product_model"), want: "product_model", kept: true},
		"mixed case and spacing":      {itemClass: strPtr("PRODUCT model"), want: "Product_Model", kept: true},
		"surrounding whitespace":      {itemClass: strPtr("  Product Model  "), want: "product_model", kept: true},
		"different class is excluded": {itemClass: strPtr("Service Offering"), want: "product_model", kept: false},
		"substring is not a match":    {itemClass: strPtr("Product"), want: "product_model", kept: false},
		"nil class is excluded":       {itemClass: nil, want: "product_model", kept: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			page := SearchProductsResponse{Products: []ProductSummary{{ID: "1", Class: tc.itemClass}}}

			got := FilterProductsByClass(page, tc.want)

			if kept := len(got.Products) == 1; kept != tc.kept {
				t.Fatalf("kept = %v, want %v (item class %v, requested %q)",
					kept, tc.kept, derefOrNil(tc.itemClass), tc.want)
			}
		})
	}
}

// TestFilterProductsByClass_NoClassRequested confirms the unfiltered path is
// untouched: an absent class query param returns the page as-is, including
// items whose own Class is nil.
func TestFilterProductsByClass_NoClassRequested(t *testing.T) {
	page := SearchProductsResponse{
		Products: []ProductSummary{
			{ID: "1", Class: strPtr(snClassLabel)},
			{ID: "2", Class: nil},
		},
		TotalRecords: 2,
	}

	got := FilterProductsByClass(page, "")

	if len(got.Products) != 2 {
		t.Fatalf("products = %d, want 2 — an empty class filter must not drop anything", len(got.Products))
	}
}

func derefOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
