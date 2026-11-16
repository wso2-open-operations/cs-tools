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

// testVersions are the fixture's versions by product id (testProducts'
// index): WSO2 API Manager (p0) out of order, with a 4.10.0 after 4.9.0;
// Identity Hub's two catalog rows (p4, p5) each with some of its versions,
// 2.0 beside 2.0.0; WSO2 Identity Server (p3) with none.
var testVersions = map[string][]string{
	"p0": {"4.3.1", "3.2.0", "4.2.0", "4.10.0", "4.3.0", "4.9.0"},
	"p4": {"1.0.0"},
	"p5": {"1.0.0", "2.0.0", "2.0"},
}

// versionDashboard is a generated dashboard of one count widget filtering on
// product and, when versions is not nil, productVersion with versions; and,
// when sliceVersions is given, a pie of that product with one slice per
// entry filtering on that version.
func versionDashboard(t *testing.T, product string, versions []string, sliceVersions ...string) dashboard.Dashboard {
	t.Helper()
	list := func(v ...string) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	query := `{"filters":[{"field":"product","op":"in","values":` + list(product) + `}`
	if versions != nil {
		query += `,{"field":"productVersion","op":"in","values":` + list(versions...) + `}`
	}
	query += `]}`
	widgets := []string{`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + query + `}`}
	if len(sliceVersions) > 0 {
		var slices []string
		for _, v := range sliceVersions {
			slices = append(slices, `{"label":"`+v+`","query":{"filters":[{"field":"productVersion","op":"in","values":`+list(v)+`}]}}`)
		}
		widgets = append(widgets, `{"id":"p","displayName":"P","resourceType":"case","shape":"pie","gridWidth":6,
			"query":{"filters":[{"field":"product","op":"in","values":`+list(product)+`}]},"slices":[`+strings.Join(slices, ",")+`]}`)
	}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// versionValues is the values of a query's productVersion filter.
func versionValues(t *testing.T, query map[string]any) string {
	t.Helper()
	for _, f := range query["filters"].([]any) {
		if e := f.(map[string]any); e["field"] == "productVersion" {
			return fmt.Sprint(e["values"])
		}
	}
	t.Fatalf("no productVersion filter in %v", query)
	return ""
}

// generateVersions runs WithDashboardProducts over d.
func generateVersions(search *fakeProductSearch, d dashboard.Dashboard) (GeneratedDashboard, error) {
	inner := &fakeGenerator{result: GeneratedDashboard{Dashboard: d, Reply: "Here it is."}}
	return WithDashboardProducts(inner, search).Generate(context.Background(), "versions", nil)
}

// TestWithDashboardProducts_ResolvesVersions: a version as written becomes
// the product's versions it names, in version order and without repeats: the
// exact version (over a longer one in its line), a line ("4.3", "4.x", "4", "v4.2.0"), across every catalog
// row of the product's name; a slice's version resolves against the widget's
// product; each product's versions are read once, page by page.
func TestWithDashboardProducts_ResolvesVersions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		product  string
		versions []string
		want     string
	}{
		{"exact", "API Manager", []string{"4.3.0"}, "[4.3.0]"},
		{"a line", "API Manager", []string{"4.3"}, "[4.3.0 4.3.1]"},
		{"a major line with x", "API Manager", []string{"4.x"}, "[4.2.0 4.3.0 4.3.1 4.9.0 4.10.0]"},
		{"a major line", "API Manager", []string{"4"}, "[4.2.0 4.3.0 4.3.1 4.9.0 4.10.0]"},
		{"a leading v and a repeat", "API Manager", []string{"v4.2.0", "4.2", "3.2.0"}, "[3.2.0 4.2.0]"},
		{"two catalog rows of one name", "Identity Hub", []string{"2", "1.0.0"}, "[1.0.0 2.0 2.0.0]"},
		{"the exact version over its line", "Identity Hub", []string{"2.0"}, "[2.0]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			search := &fakeProductSearch{names: testProducts, versions: testVersions}
			got, err := generateVersions(search, versionDashboard(t, tc.product, tc.versions))
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if v := versionValues(t, got.Dashboard.Widgets[0].Query); v != tc.want {
				t.Errorf("versions = %s, want %s", v, tc.want)
			}
		})
	}

	search := &fakeProductSearch{names: testProducts, versions: testVersions}
	got, err := generateVersions(search, versionDashboard(t, "API Manager", []string{"4.3"}, "4.2", "4.10"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for j, want := range []string{"[4.2.0]", "[4.10.0]"} {
		if v := versionValues(t, got.Dashboard.Widgets[1].Slices[j].Query); v != want {
			t.Errorf("slice %d versions = %s, want %s", j, v, want)
		}
	}
	if fmt.Sprint(search.versionCalls) != "[p0@0]" {
		t.Errorf("versions calls = %v, want WSO2 API Manager's versions read once", search.versionCalls)
	}

	// Past the first page: the 70th version is on the second.
	many := make([]string, productSearchLimit+20)
	for i := range many {
		many[i] = fmt.Sprintf("2.%d.0", i)
	}
	paged := &fakeProductSearch{names: testProducts, versions: map[string][]string{"p0": many}}
	got, err = generateVersions(paged, versionDashboard(t, "API Manager", []string{"2.69"}))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if v := versionValues(t, got.Dashboard.Widgets[0].Query); v != "[2.69.0]" || fmt.Sprint(paged.versionCalls) != "[p0@0 p0@50]" {
		t.Errorf("versions = %s after calls %v, want [2.69.0] after both pages", v, paged.versionCalls)
	}
}

// TestWithDashboardProducts_RefusesAVersionItCannotPlace: a version the
// product does not have is an errDashboardProduct naming the product and its
// newest versions; so is any version of a product with none recorded, and an
// empty version.
func TestWithDashboardProducts_RefusesAVersionItCannotPlace(t *testing.T) {
	for _, tc := range []struct {
		name     string
		product  string
		versions []string
		want     string
	}{
		{"no such version", "API Manager", []string{"3.9"}, `WSO2 API Manager has no version "3.9" (its versions: 4.2.0, 4.3.0, 4.3.1, 4.9.0, 4.10.0 and 1 older)`},
		{"no such line", "API Manager", []string{"5.x"}, `WSO2 API Manager has no version "5.x"`},
		{"a line is not a prefix of the text", "API Manager", []string{"4.1"}, `has no version "4.1"`},
		{"few versions", "Identity Hub", []string{"3"}, `Identity Hub has no version "3" (its versions: 1.0.0, 2.0, 2.0.0)`},
		{"none recorded", "Identity Server", []string{"7.0"}, `WSO2 Identity Server has no versions recorded, so "7.0" cannot be used`},
		{"empty", "API Manager", []string{" "}, "a product version filter has an empty version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			search := &fakeProductSearch{names: testProducts, versions: testVersions}
			_, err := generateVersions(search, versionDashboard(t, tc.product, tc.versions))
			if !errors.Is(err, errDashboardProduct) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want an errDashboardProduct containing %q", err, tc.want)
			}
		})
	}
}

// TestWithDashboardProducts_VersionLookupFails: a versions search that fails,
// or a product with more versions than are read, is an
// errDashboardProductLookup, never a version reported missing.
func TestWithDashboardProducts_VersionLookupFails(t *testing.T) {
	failing := &fakeProductSearch{names: testProducts, versions: testVersions, versionErr: errors.New("entity service down")}
	if _, err := generateVersions(failing, versionDashboard(t, "API Manager", []string{"4.3"})); !errors.Is(err, errDashboardProductLookup) {
		t.Errorf("failing search: err = %v, want errDashboardProductLookup", err)
	}

	many := make([]string, productVersionPages*productSearchLimit+1)
	for i := range many {
		many[i] = fmt.Sprintf("1.%d", i)
	}
	huge := &fakeProductSearch{names: testProducts, versions: map[string][]string{"p0": many}}
	if _, err := generateVersions(huge, versionDashboard(t, "API Manager", []string{"1.0"})); !errors.Is(err, errDashboardProductLookup) {
		t.Errorf("too many versions: err = %v, want errDashboardProductLookup", err)
	}
}

// TestWithDashboardProducts_NoVersionNoVersionSearch: a dashboard without a
// productVersion filter never reads versions.
func TestWithDashboardProducts_NoVersionNoVersionSearch(t *testing.T) {
	search := &fakeProductSearch{names: testProducts, versions: testVersions}
	if _, err := generateVersions(search, versionDashboard(t, "API Manager", nil)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(search.versionCalls) != 0 {
		t.Errorf("versions calls = %v, want none", search.versionCalls)
	}
}

// TestWithDashboardProducts_ResolvesProductsLeftOut: a product notIn filter's
// names become the products' own names, its op kept; a version beside it
// belongs to the product kept (in), even when the notIn filter comes first.
func TestWithDashboardProducts_ResolvesProductsLeftOut(t *testing.T) {
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
		`{"field":"product","op":"notIn","values":["identity server","api gateway"]},` +
		`{"field":"product","op":"in","values":["API Manager"]},{"field":"productVersion","op":"in","values":["4.3"]}]}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	search := &fakeProductSearch{names: testProducts, versions: testVersions}
	got, err := WithDashboardProducts(&fakeGenerator{result: GeneratedDashboard{Dashboard: d}}, search).Generate(context.Background(), "x", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	q := got.Dashboard.Widgets[0].Query
	var left, kept string
	for _, f := range q["filters"].([]any) {
		if e := f.(map[string]any); e["field"] == "product" {
			if e["op"] == "notIn" {
				left = fmt.Sprint(e["values"])
			} else {
				kept = fmt.Sprintf("%v %v", e["op"], e["values"])
			}
		}
	}
	if left != "[WSO2 Identity Server API Gateway]" || kept != "in [WSO2 API Manager]" {
		t.Errorf("left out %s, kept %s; want [WSO2 Identity Server API Gateway] and in [WSO2 API Manager]", left, kept)
	}
	if v := versionValues(t, q); v != "[4.3.0 4.3.1]" {
		t.Errorf("versions = %s, want [4.3.0 4.3.1] (WSO2 API Manager's)", v)
	}
	if fmt.Sprint(search.versionCalls) != "[p0@0]" {
		t.Errorf("versions searched %v, want only WSO2 API Manager's (p0)", search.versionCalls)
	}
}
