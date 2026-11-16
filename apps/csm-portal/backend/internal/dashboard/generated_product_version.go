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

// checkGeneratedProductVersions checks a generated case-family widget's
// product and productVersion filters and its grouping, which entity-service
// would otherwise refuse only when the tile loads: product is "in" (the
// products kept) or "notIn" (the products left out) with one or more names;
// productVersion is "in" with one or more versions, never in an anyOf branch,
// and only beside a product "in" filter naming exactly one product in the
// same search (a slice's, else the widget's, as the webapp merges a slice
// over the widget by field); a productVersion grouping needs that product in
// the widget's own query. A widget without them is not touched.
func checkGeneratedProductVersions(i int, w WidgetTemplate) error {
	at := fmt.Sprintf("widgets[%d] (%q)", i, w.ID)
	widgetProducts, _ := keptProducts(w.Query)
	if err := checkProductVersionQuery(at+".query", w.Query, widgetProducts); err != nil {
		return err
	}
	for j, sl := range w.Slices {
		products, ok := keptProducts(sl.Query)
		if !ok {
			products = widgetProducts
		}
		if err := checkProductVersionQuery(fmt.Sprintf("%s.slices[%d].query", at, j), sl.Query, products); err != nil {
			return err
		}
	}
	if w.GroupBy != nil && w.GroupBy.Field == "productVersion" && distinctCount(widgetProducts) != 1 {
		return fmt.Errorf("%w: %s.groupBy: productVersion needs a product filter naming exactly one product in the widget's query", ErrGeneratedInvalid, at)
	}
	return nil
}

// checkProductVersionQuery checks one query's product and productVersion
// entries; products are the products its search keeps (product "in").
func checkProductVersionQuery(at string, query map[string]any, products []string) error {
	branches, _ := query["anyOf"].([]any)
	for k, b := range branches {
		branch, _ := b.(map[string]any)
		if _, ok := queryFilterValues(branch, "productVersion"); ok {
			return fmt.Errorf("%w: %s.anyOf[%d]: productVersion cannot be in an anyOf branch", ErrGeneratedInvalid, at, k)
		}
	}
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		field, _ := entry["field"].(string)
		where := fmt.Sprintf("%s.filters[%d]", at, k)
		op, _ := entry["op"].(string)
		values, ok := stringList(entry["values"])
		if field == "product" && ((op != "in" && op != "notIn") || !ok || len(values) == 0) {
			return fmt.Errorf("%w: %s: product takes op \"in\" or \"notIn\" with one or more product names", ErrGeneratedInvalid, where)
		}
		if field != "productVersion" {
			continue
		}
		if op != "in" || !ok || len(values) == 0 {
			return fmt.Errorf("%w: %s: productVersion takes op \"in\" with one or more versions", ErrGeneratedInvalid, where)
		}
		if distinctCount(products) != 1 {
			return fmt.Errorf("%w: %s: productVersion needs a product filter naming exactly one product", ErrGeneratedInvalid, where)
		}
	}
	return nil
}

// queryFilterValues is every value of a query's filters on field, and whether
// it has one.
func queryFilterValues(query map[string]any, field string) ([]string, bool) {
	var out []string
	found := false
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		entry, _ := f.(map[string]any)
		if name, _ := entry["field"].(string); name != field {
			continue
		}
		found = true
		values, _ := stringList(entry["values"])
		out = append(out, values...)
	}
	return out, found
}

// keptProducts is the values of a query's product "in" filters (the products
// its search keeps), and whether it has a product filter of any op: a slice's
// product filters, whatever their op, replace its widget's, as the webapp
// merges a slice over the widget by field.
func keptProducts(query map[string]any) ([]string, bool) {
	_, found := queryFilterValues(query, "product")
	var kept []string
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		entry, _ := f.(map[string]any)
		field, _ := entry["field"].(string)
		op, _ := entry["op"].(string)
		if field == "product" && op == "in" {
			values, _ := stringList(entry["values"])
			kept = append(kept, values...)
		}
	}
	return kept, found
}

// distinctCount is how many different strings values holds.
func distinctCount(values []string) int {
	seen := map[string]bool{}
	for _, v := range values {
		seen[v] = true
	}
	return len(seen)
}
