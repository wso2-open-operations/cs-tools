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

package dashboard

import (
	"errors"
	"strings"
	"testing"
)

// versionFilters is a filters list of a product filter on products (none
// when empty) and a productVersion filter on versions (none when nil).
func versionFilters(products []string, versions ...string) string {
	quote := func(v []string) string {
		if len(v) == 0 {
			return "[]"
		}
		return `["` + strings.Join(v, `","`) + `"]`
	}
	var entries []string
	if len(products) > 0 {
		entries = append(entries, `{"field":"product","op":"in","values":`+quote(products)+`}`)
	}
	if versions != nil {
		entries = append(entries, `{"field":"productVersion","op":"in","values":`+quote(versions)+`}`)
	}
	return `{"filters":[` + strings.Join(entries, ",") + `]}`
}

var apim = []string{"WSO2 API Manager"}

// TestParseGenerated_AcceptsProductVersions: a version beside one product in
// the widget's query; a slice's version with the widget's product, or with
// its own; a grouping by version of one product; one product named twice;
// products left out (notIn), alone or beside a version of one product kept.
func TestParseGenerated_AcceptsProductVersions(t *testing.T) {
	for name, w := range map[string]string{
		"a count": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + versionFilters(apim, "4.3") + `}`,
		"the product named twice": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` +
			versionFilters([]string{"WSO2 API Manager", "WSO2 API Manager"}, "4.3") + `}`,
		"slices on the widget's product": `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":` + versionFilters(apim) + `,
			"slices":[{"label":"4.2","query":` + versionFilters(nil, "4.2") + `},{"label":"4.3","query":` + versionFilters(nil, "4.3") + `}]}`,
		"slices with their own product": `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{},
			"slices":[{"label":"APIM 4","query":` + versionFilters(apim, "4") + `},{"label":"IS 7","query":` + versionFilters([]string{"WSO2 Identity Server"}, "7") + `}]}`,
		"grouped by version": `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":` + versionFilters(apim) + `,
			"groupBy":{"field":"productVersion","maxGroups":10}}`,
		"products left out": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"product","op":"notIn","values":["WSO2 API Manager","WSO2 Identity Server"]}]}}`,
		"a version beside one product kept and one left out": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"product","op":"notIn","values":["API Gateway"]},` + strings.TrimPrefix(versionFilters(apim, "4.3"), `{"filters":[`) + `}`,
	} {
		if _, err := ParseGenerated([]byte(opsDashboard(w))); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestParseGenerated_RefusesProductVersionsItCannotRun: a version without
// one product kept (none, two, two on the widget a slice falls back to, or
// only products left out, also in a slice whose product filter replaces the
// widget's), a version filter that is not "in" with versions, a version in an
// anyOf branch, a version grouping without one product in the widget's
// query, and a product filter that is not "in" or "notIn" with names.
func TestParseGenerated_RefusesProductVersionsItCannotRun(t *testing.T) {
	const needsOne = "productVersion needs a product filter naming exactly one product"
	two := []string{"WSO2 API Manager", "WSO2 Identity Server"}
	for _, tc := range []struct {
		name, widget, want string
	}{
		{"no product", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + versionFilters(nil, "4.3") + `}`, needsOne},
		{"two products", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + versionFilters(two, "4.3") + `}`, needsOne},
		{"a slice on two products", `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":` + versionFilters(two) + `,
			"slices":[{"label":"4.3","query":` + versionFilters(nil, "4.3") + `}]}`, "slices[0].query.filters[0]: " + needsOne},
		{"notIn", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"product","op":"in","values":["WSO2 API Manager"]},{"field":"productVersion","op":"notIn","values":["4.3"]}]}}`, `productVersion takes op "in" with one or more versions`},
		{"no versions", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + versionFilters(apim, []string{}...) + `}`, `productVersion takes op "in" with one or more versions`},
		{"in an anyOf branch", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"product","op":"in","values":["WSO2 API Manager"]}],"anyOf":[` + versionFilters(nil, "4.3") + `,{"filters":[{"field":"state","op":"in","values":["open"]}]}]}}`, "anyOf[0]: productVersion cannot be in an anyOf branch"},
		{"a version beside a product left out only", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"product","op":"notIn","values":["WSO2 API Manager"]},{"field":"productVersion","op":"in","values":["4.3"]}]}}`, needsOne},
		{"a slice whose product left out replaces the widget's", `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":` + versionFilters(apim) + `,
			"slices":[{"label":"4.3","query":{"filters":[{"field":"product","op":"notIn","values":["API Gateway"]},{"field":"productVersion","op":"in","values":["4.3"]}]}}]}`, "slices[0].query.filters[1]: " + needsOne},
		{"grouped by version beside a product left out", `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{"filters":[{"field":"product","op":"notIn","values":["WSO2 API Manager"]}]},
			"groupBy":{"field":"productVersion","maxGroups":10}}`, "groupBy: productVersion needs a product filter naming exactly one product"},
		{"product eq", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"product","op":"eq","values":["WSO2 API Manager"]}]}}`, `product takes op "in" or "notIn" with one or more product names`},
		{"product without names", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"product","op":"notIn","values":[]}]}}`, `product takes op "in" or "notIn" with one or more product names`},
		{"grouped without a product", `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{},
			"groupBy":{"field":"productVersion","maxGroups":10}}`, "groupBy: productVersion needs a product filter naming exactly one product"},
		{"grouped across two products", `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":` + versionFilters(two) + `,
			"groupBy":{"field":"productVersion","maxGroups":10}}`, "groupBy: productVersion needs a product filter naming exactly one product"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGenerated([]byte(opsDashboard(tc.widget)))
			if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want ErrGeneratedInvalid containing %q", err, tc.want)
			}
		})
	}
}
