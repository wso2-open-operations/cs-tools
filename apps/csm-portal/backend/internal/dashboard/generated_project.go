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

import "fmt"

// namedFilterValues is, for each filter field naming records as the user
// wrote them (the handler's WithDashboardProjects / WithDashboardAccounts
// turn them into projectId / accountId filters right after generation, and
// WithDashboardRegions checks the regions), what its values are.
var namedFilterValues = []struct{ field, values string }{
	{"project", "project names or keys"},
	{"account", "account names or numbers"},
	{"region", "regions"},
}

// checkGeneratedNamedFilters checks a generated case-family widget's project,
// account and region filters: each is "in" or "notIn" with one or more names, and
// only in a query's top-level filters, never in an anyOf branch. A widget
// without one is not touched.
func checkGeneratedNamedFilters(i int, w WidgetTemplate) error {
	at := fmt.Sprintf("widgets[%d] (%q)", i, w.ID)
	if err := checkNamedFilterQuery(at+".query", w.Query); err != nil {
		return err
	}
	for j, sl := range w.Slices {
		if err := checkNamedFilterQuery(fmt.Sprintf("%s.slices[%d].query", at, j), sl.Query); err != nil {
			return err
		}
	}
	return nil
}

// checkNamedFilterQuery checks one query's project, account and region
// entries.
func checkNamedFilterQuery(at string, query map[string]any) error {
	branches, _ := query["anyOf"].([]any)
	for k, b := range branches {
		branch, _ := b.(map[string]any)
		for _, n := range namedFilterValues {
			if _, ok := queryFilterValues(branch, n.field); ok {
				return fmt.Errorf("%w: %s.anyOf[%d]: %s cannot be in an anyOf branch", ErrGeneratedInvalid, at, k, n.field)
			}
		}
	}
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		field, _ := entry["field"].(string)
		for _, n := range namedFilterValues {
			if field != n.field {
				continue
			}
			op, _ := entry["op"].(string)
			values, ok := stringList(entry["values"])
			if (op != "in" && op != "notIn") || !ok || len(values) == 0 {
				return fmt.Errorf("%w: %s.filters[%d]: %s takes op \"in\" or \"notIn\" with one or more %s", ErrGeneratedInvalid, at, k, field, n.values)
			}
		}
	}
	return nil
}
