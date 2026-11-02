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
	"reflect"
	"strings"
	"testing"
)

// aiIntegrationService is integrations/csm-ai-integration-service, and
// serviceCatalogue its built-in dashboard catalogue, relative to this
// package's directory.
const (
	aiIntegrationService = "../../../../../integrations/csm-ai-integration-service"
	serviceCatalogue     = aiIntegrationService + "/internal/catalogue/catalogue.md"
)

// catalogueExample returns the catalogue's own worked example (section 10),
// the definition the model is shown as the target.
//
// The test is skipped only when the AI integration service is not in the
// checkout at all (this backend's change can be checked out before the
// service's is merged). With the service present, a missing or moved
// catalogue fails the test, so the drift check stays on.
func catalogueExample(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	// The catalogue lives with the model call, in the AI integration
	// service; its example must still pass this backend's own check, so the
	// two components cannot drift apart unnoticed.
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	text := string(raw)
	_, after, ok := strings.Cut(text, "## 10. A complete example")
	if !ok {
		t.Fatal("catalogue has no section 10 example")
	}
	_, after, ok = strings.Cut(after, "```json\n")
	if !ok {
		t.Fatal("catalogue section 10 has no json block")
	}
	example, _, ok := strings.Cut(after, "```")
	if !ok {
		t.Fatal("catalogue section 10 json block is not closed")
	}
	return example
}

func TestParseGenerated_TheCatalogueExampleIsValid(t *testing.T) {
	d, err := ParseGenerated([]byte(catalogueExample(t)))
	if err != nil {
		t.Fatalf("the catalogue's own example is rejected: %v", err)
	}
	if d.ID != GeneratedPlaceholderID {
		t.Errorf("ID = %q, want the placeholder %q", d.ID, GeneratedPlaceholderID)
	}
	if len(d.Widgets) == 0 {
		t.Fatal("no widgets")
	}
	for _, w := range d.Widgets {
		filters, _ := w.Query["filters"].([]any)
		found := false
		for _, f := range filters {
			if isImpliedTypeFilter(f, w.ResourceType) {
				found = true
			}
		}
		if !found {
			t.Errorf("widget %q: the implied type filter was not injected: %v", w.ID, w.Query)
		}
	}
}

// The same definition loaded as a dashboard file loads to the same dashboard,
// so the preview of a generated dashboard matches it once it is a file.
func TestParseGenerated_MatchesTheFileLoader(t *testing.T) {
	example := catalogueExample(t)
	generated, err := ParseGenerated([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	asFile := strings.Replace(example, "{", `{"id": "`+GeneratedPlaceholderID+`",`, 1)
	loaded, err := ParseDashboardsConfig("[" + asFile + "]")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := generated, loaded[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("generated and file-loaded dashboards differ\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseGenerated_ModelIDIsReplaced(t *testing.T) {
	d, err := ParseGenerated([]byte(strings.Replace(catalogueExample(t), "{", `{"id": "ai-something",`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != GeneratedPlaceholderID {
		t.Errorf("ID = %q, want %q", d.ID, GeneratedPlaceholderID)
	}
}

func TestParseGenerated_Rejects(t *testing.T) {
	widget := func(extra string) string {
		return `{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + extra + `]}`
	}
	count := `{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{}}`
	cases := []struct {
		name, raw, want string
	}{
		{"broken JSON", `{"displayName":`, "not valid JSON"},
		{"an array", `[{}]`, "must be a JSON object"},
		{"two objects", widget(count) + ` {}`, "data after the dashboard object"},
		{"unknown top-level field", strings.Replace(widget(count), "{", `{"owner":"x",`, 1), `unknown field "owner"`},
		{"misspelt widget field", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWith":3,"query":{}}`), `unknown field "widgets[0].gridWith"`},
		{"legacy widget filters key", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"filters":{}}`), `unknown field "widgets[0].filters"`},
		{"unknown column field", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"list","gridWidth":6,"query":{},"listLimit":5,"sortBy":{"field":"createdOn","order":"desc"},"columns":[{"key":"number"}]}`), `unknown field "widgets[0].columns[0].key"`},
		{"unknown slice field", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[{"label":"A","colour":"info","query":{}}]}`), `unknown field "widgets[0].slices[0].colour"`},
		{"restricted", strings.Replace(widget(count), "{", `{"restricted":false,`, 1), `"restricted" cannot be set`},
		{"filter presets", strings.Replace(widget(count), "{", `{"filterPresets":{},`, 1), `"filterPresets" cannot be set`},
		{"isDefault", strings.Replace(widget(count), `"isDefault":false`, `"isDefault":true`, 1), `"isDefault" must be false`},
		{"not a supported resource type", widget(`{"id":"w","displayName":"W","resourceType":"incident_task","shape":"count","gridWidth":3,"query":{}}`), `resourceType "incident_task" is not one of`},
		{"wrong value type", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":"3","query":{}}`), "invalid generated dashboard"},
		{"cs and team based", strings.Replace(widget(count), `"isTeamBased":false`, `"isTeamBased":true`, 1), "contradictory configuration"},
		{"no display name", strings.Replace(widget(count), `"displayName":"D",`, "", 1), `"displayName" is empty`},
		{"pie without slices", widget(`{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":{}}`), `needs either "slices" or "groupBy"`},
		{"grid too wide", widget(strings.Replace(count, `"gridWidth":3`, `"gridWidth":13`, 1)), `"gridWidth" is 13`},
		{"duplicate widget id", widget(count + "," + count), `duplicate widget id "w"`},
	}
	for _, tc := range cases {
		_, err := ParseGenerated([]byte(tc.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", tc.name, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "dashboard definitions: "+generatedSource) {
			t.Errorf("%s: err = %v, want the loader's file prefix trimmed", tc.name, err)
		}
	}
}
