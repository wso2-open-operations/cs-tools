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

// TestParseGenerated_TheCatalogueProductExamplesAreValid: every case widget
// grouped by product that the catalogue shows the model (section 7) passes
// this backend's own check, so the catalogue cannot teach a widget the save
// would refuse.
func TestParseGenerated_TheCatalogueProductExamplesAreValid(t *testing.T) {
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
	var widgets []string
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		if strings.Contains(block, `"groupBy": { "field": "product",`) && strings.Contains(block, `"resourceType"`) {
			widgets = append(widgets, block)
		}
		rest = next
	}
	if len(widgets) == 0 {
		t.Fatal("found no product grouping example in section 7")
	}
	for i, w := range widgets {
		d, err := ParseGenerated([]byte(opsDashboard(w)))
		if err != nil {
			t.Errorf("section 7 product example %d is rejected: %v\n%s", i+1, err, w)
			continue
		}
		if g := d.Widgets[0].GroupBy; g == nil || g.Field != "product" {
			t.Errorf("section 7 product example %d groupBy = %+v, want product", i+1, g)
		}
	}
}
