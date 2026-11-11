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

// projectQuery is a filters list with one project filter on the given names.
func projectQuery(names ...string) string {
	return `{"filters":[{"field":"project","op":"in","values":["` + strings.Join(names, `","`) + `"]}]}`
}

// TestParseGenerated_AcceptsProjects: a project filter in a widget's query
// and in its slices, projects left out (notIn) beside one kept in, and a
// grouping by project.
func TestParseGenerated_AcceptsProjects(t *testing.T) {
	for name, w := range map[string]string{
		"a count": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + projectQuery("EXCORP-PROJ-1", "Summit") + `}`,
		"slices":  `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[{"label":"A","query":` + projectQuery("EXCORP-PROJ-1") + `},{"label":"B","query":` + projectQuery("Summit") + `}]}`,
		"projects left out": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"project","op":"notIn","values":["EXCORP-PROJ-1","Summit"]},{"field":"project","op":"in","values":["Quartz Labs Production"]}]}}`,
		"accounts kept and left out": `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[
			{"field":"account","op":"in","values":["Example Corp"]},{"field":"account","op":"notIn","values":["ACC0002"]}]}}`,
		"grouped by project": `{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{"filters":[{"field":"state","op":"in","values":["open"]}]},
			"groupBy":{"field":"project","maxGroups":10}}`,
	} {
		if _, err := ParseGenerated([]byte(opsDashboard(w))); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestParseGenerated_RefusesProjectsItCannotResolve: a project filter that is
// not "in" or "notIn" with names, or one in an anyOf branch, in a widget or a
// slice.
func TestParseGenerated_RefusesProjectsItCannotResolve(t *testing.T) {
	const shape = `project takes op "in" or "notIn" with one or more project names or keys`
	for _, tc := range []struct {
		name, widget, want string
	}{
		{"eq", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"project","op":"eq","values":["Summit"]}]}}`, "query.filters[0]: " + shape},
		{"no values", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"project","op":"in","values":[]}]}}`, shape},
		{"not names", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"project","op":"in","values":[1]}]}}`, shape},
		{"in a slice", `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[{"label":"A","query":{"filters":[{"field":"project","op":"isEmpty"}]}}]}`, "slices[0].query.filters[0]: " + shape},
		{"account eq", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"account","op":"eq","values":["Example Corp"]}]}}`,
			`query.filters[0]: account takes op "in" or "notIn" with one or more account names or numbers`},
		{"an account in an anyOf branch", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"anyOf":[{"filters":[{"field":"account","op":"in","values":["Example Corp"]}]},{"filters":[{"field":"state","op":"in","values":["open"]}]}]}}`,
			"query.anyOf[0]: account cannot be in an anyOf branch"},
		{"in an anyOf branch", `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"anyOf":[` + projectQuery("Summit") + `,{"filters":[{"field":"state","op":"in","values":["open"]}]}]}}`, "query.anyOf[0]: project cannot be in an anyOf branch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGenerated([]byte(opsDashboard(tc.widget)))
			if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want ErrGeneratedInvalid containing %q", err, tc.want)
			}
		})
	}
}

// TestParseGenerated_TheCatalogueProjectExamplesAreValid: every case widget
// with a project that the catalogue shows the model (section 7: a grouping by
// project and a project filter) passes this backend's own check, so the
// catalogue cannot teach a widget the save would refuse.
func TestParseGenerated_TheCatalogueProjectExamplesAreValid(t *testing.T) {
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
		if !strings.Contains(block, `"project"`) || !strings.Contains(block, `"resourceType": "case"`) {
			continue
		}
		d, err := ParseGenerated([]byte(opsDashboard(block)))
		if err != nil {
			t.Errorf("section 7 project example is rejected: %v\n%s", err, block)
			continue
		}
		if g := d.Widgets[0].GroupBy; g != nil && g.Field == "project" {
			grouped++
		} else if _, ok := queryFilterValues(d.Widgets[0].Query, "project"); ok {
			filtered++
		}
	}
	if grouped == 0 || filtered == 0 {
		t.Errorf("section 7 has %d project groupings and %d project filters, want at least one of each", grouped, filtered)
	}
}
