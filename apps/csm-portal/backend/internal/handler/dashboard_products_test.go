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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// fakeProductSearch answers POST /products/search as the entity service
// does: every product whose name contains the search query, ignoring case,
// one page of at most limit, each product's id "p<its index in names>". It
// answers POST /products/{id}/versions/search with versions[id], one page of
// the request's limit at its offset. It records each query and versions call.
type fakeProductSearch struct {
	names        []string
	limit        int
	err          error
	queries      []string
	versions     map[string][]string
	versionErr   error
	versionCalls []string
}

// SearchProductVersions implements productSearcher.
func (f *fakeProductSearch) SearchProductVersions(_ context.Context, productID string, body []byte) ([]byte, error) {
	var req struct {
		Pagination struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.versionCalls = append(f.versionCalls, fmt.Sprintf("%s@%d", productID, req.Pagination.Offset))
	if f.versionErr != nil {
		return nil, f.versionErr
	}
	all := f.versions[productID]
	start := min(req.Pagination.Offset, len(all))
	end := min(start+req.Pagination.Limit, len(all))
	page, hasMore := all[start:end], end < len(all)
	var out []map[string]string
	for _, v := range page {
		out = append(out, map[string]string{"productId": productID, "version": v})
	}
	return json.Marshal(map[string]any{"productVersions": out, "hasMore": hasMore})
}

// SearchProducts implements productSearcher.
func (f *fakeProductSearch) SearchProducts(_ context.Context, body []byte) ([]byte, error) {
	var req struct {
		SearchQuery string `json:"searchQuery"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.queries = append(f.queries, req.SearchQuery)
	if f.err != nil {
		return nil, f.err
	}
	var products []map[string]string
	for i, n := range f.names {
		if strings.Contains(strings.ToLower(n), strings.ToLower(req.SearchQuery)) {
			products = append(products, map[string]string{"id": fmt.Sprintf("p%d", i), "name": n})
		}
	}
	hasMore := false
	if f.limit > 0 && len(products) > f.limit {
		products, hasMore = products[:f.limit], true
	}
	return json.Marshal(map[string]any{"products": products, "hasMore": hasMore})
}

// testProducts is the fixture catalogue: one name that contains another
// ("API Gateway" in "API Gateway Plus"), and three Identity products.
var testProducts = []string{"WSO2 API Manager", "API Gateway", "API Gateway Plus", "WSO2 Identity Server", "Identity Hub", "Identity Hub"}

// productDashboard is a generated dashboard whose first widget filters on the
// given product names and whose second is a pie with one slice per
// sliceProducts entry, as the model would write them.
func productDashboard(t *testing.T, products []string, sliceProducts ...string) dashboard.Dashboard {
	t.Helper()
	q := func(names ...string) string {
		b, _ := json.Marshal(names)
		return `{"filters":[{"field":"state","op":"in","values":["open"]},{"field":"product","op":"in","values":` + string(b) + `}]}`
	}
	widgets := []string{`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + q(products...) + `}`}
	if len(sliceProducts) > 0 {
		var slices []string
		for _, p := range sliceProducts {
			slices = append(slices, `{"label":"`+p+`","query":`+q(p)+`}`)
		}
		widgets = append(widgets, `{"id":"p","displayName":"P","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[`+strings.Join(slices, ",")+`]}`)
	}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// productValues is the values of a query's product filter.
func productValues(t *testing.T, query map[string]any) []any {
	t.Helper()
	for _, f := range query["filters"].([]any) {
		if e := f.(map[string]any); e["field"] == "product" {
			return e["values"].([]any)
		}
	}
	t.Fatalf("no product filter in %v", query)
	return nil
}

// TestWithDashboardProducts_ResolvesTheGeneratedDashboard: a name as the
// user wrote it becomes the product's own name, in a widget and in its
// slices; the exact name wins over a longer one that contains it; a name
// given twice, or two names of one product, become one value; each distinct
// name is searched once; everything else is as generated.
func TestWithDashboardProducts_ResolvesTheGeneratedDashboard(t *testing.T) {
	search := &fakeProductSearch{names: testProducts}
	inner := &fakeGenerator{result: GeneratedDashboard{
		Dashboard: productDashboard(t, []string{"API Manager", "wso2 api manager", "api gateway"}, "API Manager", "identity server"),
		Reply:     "Here it is.",
	}}
	got, err := WithDashboardProducts(inner, search).Generate(context.Background(), "api manager cases", json.RawMessage(`{"id":"x"}`))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inner.gotPrompt != "api manager cases" || string(inner.gotCurrent) != `{"id":"x"}` || got.Reply != "Here it is." {
		t.Errorf("inner got (%q, %s), reply %q", inner.gotPrompt, inner.gotCurrent, got.Reply)
	}
	if v := fmt.Sprint(productValues(t, got.Dashboard.Widgets[0].Query)); v != "[WSO2 API Manager API Gateway]" {
		t.Errorf("widget products = %s, want [WSO2 API Manager API Gateway]", v)
	}
	for j, want := range []string{"[WSO2 API Manager]", "[WSO2 Identity Server]"} {
		if v := fmt.Sprint(productValues(t, got.Dashboard.Widgets[1].Slices[j].Query)); v != want {
			t.Errorf("slice %d products = %s, want %s", j, v, want)
		}
	}
	if q, _ := json.Marshal(got.Dashboard.Widgets[0].Query); !strings.Contains(string(q), `{"field":"state","op":"in","values":["open"]}`) || !strings.Contains(string(q), `"field":"type"`) {
		t.Errorf("other filters changed: %s", q)
	}
	if fmt.Sprint(search.queries) != "[API Manager wso2 api manager api gateway identity server]" {
		t.Errorf("searched %q, want each distinct name once", search.queries)
	}
}

// TestWithDashboardProducts_RefusesANameItCannotPlace: a name no product's
// name contains, or that several contain, is an errDashboardProduct whose
// message (as the chat shows it) says which, listing at most five
// candidates; a failed search is an errDashboardProductLookup.
func TestWithDashboardProducts_RefusesANameItCannotPlace(t *testing.T) {
	many := []string{"Cloud A", "Cloud B", "Cloud C", "Cloud D", "Cloud E", "Cloud F", "Cloud G"}
	for _, tc := range []struct {
		name, product string
		search        *fakeProductSearch
		want          string
	}{
		{"no product", "Orion", &fakeProductSearch{names: testProducts}, `No product is named like "Orion".`},
		{"several products", "Identity", &fakeProductSearch{names: testProducts},
			`"Identity" matches several products (WSO2 Identity Server, Identity Hub); name the one you mean.`},
		{"more than five", "Cloud", &fakeProductSearch{names: many},
			`"Cloud" matches several products (Cloud A, Cloud B, Cloud C, Cloud D, Cloud E and 2 more); name the one you mean.`},
		{"more than one page", "Cloud", &fakeProductSearch{names: many, limit: 3},
			`"Cloud" matches several products (Cloud A, Cloud B, Cloud C and more); name the one you mean.`},
		{"one name on a page with more", "Cloud", &fakeProductSearch{names: many, limit: 1},
			`"Cloud" matches several products (Cloud A and more); name the one you mean.`},
		{"an empty name", " ", &fakeProductSearch{names: testProducts}, "A product filter has an empty name."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gen := WithDashboardProducts(&fakeGenerator{result: GeneratedDashboard{Dashboard: productDashboard(t, []string{tc.product})}}, tc.search)
			_, err := gen.Generate(context.Background(), "x", nil)
			if !errors.Is(err, errDashboardProduct) {
				t.Fatalf("err = %v, want errDashboardProduct", err)
			}
			if msg := dashboardTeamMessage(err); msg != tc.want {
				t.Errorf("message = %q, want %q", msg, tc.want)
			}
		})
	}

	gen := WithDashboardProducts(&fakeGenerator{result: GeneratedDashboard{Dashboard: productDashboard(t, []string{"API Manager"})}},
		&fakeProductSearch{err: errors.New("status 500")})
	if _, err := gen.Generate(context.Background(), "x", nil); !errors.Is(err, errDashboardProductLookup) {
		t.Errorf("a failed search: err = %v, want errDashboardProductLookup", err)
	}
}

// TestWithDashboardProducts_LeavesOtherDashboardsAlone: a dashboard with no
// product filter is not searched for, and a generator error passes through.
func TestWithDashboardProducts_LeavesOtherDashboardsAlone(t *testing.T) {
	search := &fakeProductSearch{names: testProducts}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{},"groupBy":{"field":"product","maxGroups":10}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if _, err := WithDashboardProducts(&fakeGenerator{result: GeneratedDashboard{Dashboard: d}}, search).Generate(context.Background(), "x", nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(search.queries) != 0 {
		t.Errorf("searched %q for a dashboard without a product filter", search.queries)
	}
	if _, err := WithDashboardProducts(&fakeGenerator{err: errGenerateRefused}, search).Generate(context.Background(), "x", nil); !errors.Is(err, errGenerateRefused) {
		t.Errorf("err = %v, want errGenerateRefused", err)
	}
}
