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
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestParseGenerated_AcceptsRegions: a region filter ("in" or "notIn") on a
// case-family widget, in its query or a slice's, and a grouping by region.
func TestParseGenerated_AcceptsRegions(t *testing.T) {
	for name, w := range map[string]string{
		"in":                        `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"region","op":"in","values":["APAC"]}]}}`,
		"notIn on security reports": `{"id":"w","displayName":"W","resourceType":"security_report_analysis","shape":"count","gridWidth":3,"query":{"filters":[{"field":"region","op":"notIn","values":["APAC","EMEA"]}]}}`,
		"a slice": `{"id":"w","displayName":"W","resourceType":"service_request","shape":"pie","gridWidth":6,"query":{},
			"slices":[{"label":"APAC","query":{"filters":[{"field":"region","op":"in","values":["APAC"]}]}}]}`,
		"grouped": `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{},"groupBy":{"field":"region","maxGroups":10}}`,
	} {
		if _, err := ParseGenerated([]byte(opsDashboard(w))); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestParseGenerated_RefusesRegionsItCannotRun: a region filter that is not
// "in" or "notIn" with regions, one in an anyOf branch, and one on a widget
// whose search has no region (an account widget).
func TestParseGenerated_RefusesRegionsItCannotRun(t *testing.T) {
	const takes = `region takes op "in" or "notIn" with one or more regions`
	for _, tc := range []struct{ name, widget, want string }{
		{"eq", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"region","op":"eq","values":["APAC"]}]}}`, takes},
		{"no regions", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"region","op":"in","values":[]}]}}`, takes},
		{"in an anyOf branch", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"anyOf":[
			{"filters":[{"field":"region","op":"in","values":["APAC"]}]},{"filters":[{"field":"state","op":"in","values":["open"]}]}]}}`, "anyOf[0]: region cannot be in an anyOf branch"},
		{"an account widget", `{"id":"w","displayName":"W","resourceType":"account","shape":"count","gridWidth":3,"query":{"region":"APAC"}}`, `"region" is not a account filter`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGenerated([]byte(opsDashboard(tc.widget)))
			if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want ErrGeneratedInvalid containing %q", err, tc.want)
			}
		})
	}
}

// TestParseGenerated_TheCatalogueRegionExamplesAreValid: every widget with a
// region that the catalogue shows the model (section 7: a grouping by region
// and a region filter) passes this backend's own check, so the catalogue
// cannot teach a widget the save would refuse.
func TestParseGenerated_TheCatalogueRegionExamplesAreValid(t *testing.T) {
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	_, section, ok := strings.Cut(string(raw), "## 7. Grouping")
	if !ok {
		t.Fatal("catalogue has no section 7")
	}
	section, _, _ = strings.Cut(section, "## 8.")
	grouped, filtered := 0, 0
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		rest = next
		if !strings.Contains(block, `"region"`) {
			continue
		}
		d, err := ParseGenerated([]byte(opsDashboard(block)))
		if err != nil {
			t.Errorf("section 7 region example is rejected: %v\n%s", err, block)
			continue
		}
		if g := d.Widgets[0].GroupBy; g != nil && g.Field == "region" {
			grouped++
		} else if _, ok := queryFilterValues(d.Widgets[0].Query, "region"); ok {
			filtered++
		}
	}
	if grouped == 0 || filtered == 0 {
		t.Errorf("section 7 has %d region groupings and %d region filters, want at least one of each", grouped, filtered)
	}
}
