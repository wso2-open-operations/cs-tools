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
	"sort"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// errDashboardProduct marks a generated product filter naming no product, or
// several, or a productVersion filter naming no version of its product: the
// handler's 422 with the reason (generate_product).
var errDashboardProduct = errors.New("unusable product filter")

// errDashboardProductLookup marks a product or product version search that
// failed while the names were being checked: the handler's 502.
var errDashboardProductLookup = errors.New("product lookup failed")

// errMsgGenerateProductLookup is errDashboardProductLookup's message.
const errMsgGenerateProductLookup = "The product names in the dashboard couldn't be checked. Try again."

// productSearchLimit is the page of a products search one name is checked
// against: the entity service's own cap.
const productSearchLimit = 50

// productNamesShown is how many candidates a "matches several products"
// reason lists before "and N more", and how many versions a "has no version"
// reason lists.
const productNamesShown = 5

// productVersionPages is how many pages of productSearchLimit versions one
// product's versions are read in at most; a product with more is a lookup
// failure rather than a version wrongly reported missing.
const productVersionPages = 10

// productSearcher is the entity service's POST /products/search and POST
// /products/{id}/versions/search (entity.CustomerEntityClient.SearchProducts
// and SearchProductVersions), the searches the cases list's product picker
// uses.
type productSearcher interface {
	SearchProducts(ctx context.Context, body []byte) ([]byte, error)
	SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
}

// productResolvingGenerator turns the product names in what another
// generator produced into the products' own names, once, right after
// generation: the case search's product filter matches a name exactly, so
// "API Manager" would find nothing where the product is "WSO2 API Manager".
// From then on the dashboard holds real names, so the generate reply, Save,
// My Dashboards and Shared Dashboards need nothing more.
type productResolvingGenerator struct {
	inner    DashboardGenerator
	products productSearcher
}

// WithDashboardProducts wraps a generator so that each value of a "product"
// filter ("in" or "notIn") becomes the name of the one product it names
// (resolveProductName),
// and each value of a "productVersion" filter the versions of that product it
// names (resolveVersion); a value naming no product, several, or no version
// is an errDashboardProduct.
func WithDashboardProducts(inner DashboardGenerator, products productSearcher) DashboardGenerator {
	return &productResolvingGenerator{inner: inner, products: products}
}

// Generate asks the wrapped generator for the dashboard, then resolves its
// product and product version filters.
func (g *productResolvingGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	result, err := g.inner.Generate(ctx, prompt, current)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	r := productResolver{search: g.products, names: map[string]string{}, ids: map[string][]string{}, versions: map[string][]string{}}
	if err := r.resolveDashboard(ctx, &result.Dashboard); err != nil {
		return GeneratedDashboard{}, err
	}
	return result, nil
}

// productResolver resolves the product names and versions of one generated
// dashboard, searching each distinct name, and each product's versions, once.
type productResolver struct {
	search   productSearcher
	names    map[string]string   // a name as written, lower-cased -> the product's name
	ids      map[string][]string // a product's name -> the ids of the products of that name
	versions map[string][]string // a product's name -> its versions, in version order
}

// resolveDashboard resolves every widget's and slice's product filters in
// place, then their productVersion filters against the resolved product (a
// slice's own, else the widget's, as the webapp merges them). ParseGenerated
// has already made sure each version filter's search names one product.
func (r productResolver) resolveDashboard(ctx context.Context, d *dashboard.Dashboard) error {
	for i := range d.Widgets {
		w := &d.Widgets[i]
		where := fmt.Sprintf("widgets[%d]", i)
		if err := r.resolveQuery(ctx, w.Query, where); err != nil {
			return err
		}
		for j := range w.Slices {
			if err := r.resolveQuery(ctx, w.Slices[j].Query, fmt.Sprintf("%s.slices[%d]", where, j)); err != nil {
				return err
			}
		}
	}
	for i := range d.Widgets {
		w := &d.Widgets[i]
		where := fmt.Sprintf("widgets[%d]", i)
		product := queryProduct(w.Query)
		if err := r.resolveVersions(ctx, w.Query, product, where); err != nil {
			return err
		}
		for j := range w.Slices {
			sliceProduct := queryProduct(w.Slices[j].Query)
			if sliceProduct == "" {
				sliceProduct = product
			}
			if err := r.resolveVersions(ctx, w.Slices[j].Query, sliceProduct, fmt.Sprintf("%s.slices[%d]", where, j)); err != nil {
				return err
			}
		}
	}
	return nil
}

// queryProduct is the first value of a query's product "in" filter (a
// product it keeps), "" if it has none; a product "notIn" filter leaves
// products out, so a version never belongs to it.
func queryProduct(query map[string]any) string {
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		entry, _ := f.(map[string]any)
		field, _ := entry["field"].(string)
		op, _ := entry["op"].(string)
		if field != "product" || op != "in" {
			continue
		}
		values, _ := entry["values"].([]any)
		if len(values) > 0 {
			name, _ := values[0].(string)
			return name
		}
	}
	return ""
}

// resolveVersions resolves the values of a query's "productVersion" filters
// against product's versions; any other filter is left as it is.
func (r productResolver) resolveVersions(ctx context.Context, query map[string]any, product, where string) error {
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		if field, _ := entry["field"].(string); field != "productVersion" {
			continue
		}
		at := fmt.Sprintf("%s.filters[%d]", where, k)
		if product == "" {
			return fmt.Errorf("%w: %s: a product version needs the product it belongs to", errDashboardProduct, at)
		}
		known, err := r.productVersions(ctx, product, at)
		if err != nil {
			return err
		}
		values, _ := entry["values"].([]any)
		var resolved []string
		for _, v := range values {
			s, ok := v.(string)
			if !ok {
				continue
			}
			matched, err := resolveVersion(product, s, known, at)
			if err != nil {
				return err
			}
			resolved = append(resolved, matched...)
		}
		resolved = uniqueVersions(resolved)
		out := make([]any, len(resolved))
		for i, v := range resolved {
			out[i] = v
		}
		entry["values"] = out
	}
	return nil
}

// productVersions is every version of the products named product, in version
// order, read once per dashboard.
func (r productResolver) productVersions(ctx context.Context, product, at string) ([]string, error) {
	if known, ok := r.versions[product]; ok {
		return known, nil
	}
	var all []string
	for _, id := range r.ids[product] {
		for page := 0; ; page++ {
			if page == productVersionPages {
				return nil, fmt.Errorf("%w: %s: %s has more than %d versions", errDashboardProductLookup, at, product, productVersionPages*productSearchLimit)
			}
			body, err := json.Marshal(map[string]any{"pagination": map[string]int{"limit": productSearchLimit, "offset": page * productSearchLimit}})
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %v", errDashboardProductLookup, at, err)
			}
			raw, err := r.search.SearchProductVersions(ctx, id, body)
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %v", errDashboardProductLookup, at, err)
			}
			var res struct {
				ProductVersions []struct {
					Version string `json:"version"`
				} `json:"productVersions"`
				HasMore bool `json:"hasMore"`
			}
			if err := json.Unmarshal(raw, &res); err != nil {
				return nil, fmt.Errorf("%w: %s: read the product versions search: %v", errDashboardProductLookup, at, err)
			}
			for _, v := range res.ProductVersions {
				if v := strings.TrimSpace(v.Version); v != "" {
					all = append(all, v)
				}
			}
			if !res.HasMore {
				break
			}
		}
	}
	all = uniqueVersions(all)
	r.versions[product] = all
	return all, nil
}

// resolveVersion is the versions of product a version as written means: the
// version of exactly that text, ignoring case and a leading "v"; otherwise
// every version in that line ("4.3" -> 4.3.0, 4.3.1; "4.x" or "4" -> every
// 4.*). A version naming none of known is an errDashboardProduct listing the
// product's versions.
func resolveVersion(product, value string, known []string, at string) ([]string, error) {
	written := strings.TrimSpace(value)
	if written == "" {
		return nil, fmt.Errorf("%w: %s: a product version filter has an empty version", errDashboardProduct, at)
	}
	line := written
	if len(line) > 1 && (line[0] == 'v' || line[0] == 'V') && line[1] >= '0' && line[1] <= '9' {
		line = line[1:]
	}
	wildcard := false
	for {
		trimmed := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(line, ".x"), ".X"), ".*")
		if trimmed == line {
			break
		}
		line, wildcard = trimmed, true
	}
	if !wildcard {
		for _, v := range known {
			if strings.EqualFold(v, line) {
				return []string{v}, nil
			}
		}
	}
	var matched []string
	for _, v := range known {
		if strings.EqualFold(v, line) || strings.HasPrefix(strings.ToLower(v), strings.ToLower(line)+".") {
			matched = append(matched, v)
		}
	}
	if len(matched) > 0 {
		return matched, nil
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("%w: %s: %s has no versions recorded, so %q cannot be used", errDashboardProduct, at, product, written)
	}
	shown, more := known, ""
	if len(known) > productNamesShown {
		shown = known[len(known)-productNamesShown:]
		more = fmt.Sprintf(" and %d older", len(known)-productNamesShown)
	}
	return nil, fmt.Errorf("%w: %s: %s has no version %q (its versions: %s%s)", errDashboardProduct, at, product, written, strings.Join(shown, ", "), more)
}

// uniqueVersions is versions without repeats, in version order.
func uniqueVersions(versions []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range versions {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return versionLess(out[i], out[j]) })
	return out
}

// versionLess orders versions part by part ("4.10.0" after "4.9.0"),
// numerically where both parts are numbers, otherwise as text.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] == pb[i] {
			continue
		}
		na, errA := strconv.Atoi(pa[i])
		nb, errB := strconv.Atoi(pb[i])
		if errA == nil && errB == nil {
			return na < nb
		}
		return pa[i] < pb[i]
	}
	return len(pa) < len(pb)
}

// resolveQuery resolves the values of a query's "product" filters (the case
// search's field / op / values list); any other filter is left as it is.
func (r productResolver) resolveQuery(ctx context.Context, query map[string]any, where string) error {
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		if field, _ := entry["field"].(string); field != "product" {
			continue
		}
		values, _ := entry["values"].([]any)
		resolved := make([]any, 0, len(values))
		seen := map[string]bool{}
		for _, v := range values {
			s, ok := v.(string)
			if !ok {
				resolved = append(resolved, v)
				continue
			}
			name, err := r.resolveProductName(ctx, s, fmt.Sprintf("%s.filters[%d]", where, k))
			if err != nil {
				return err
			}
			if !seen[name] {
				seen[name] = true
				resolved = append(resolved, name)
			}
		}
		entry["values"] = resolved
	}
	return nil
}

// resolveProductName is the product a name means: the product of exactly
// that name, ignoring case, if there is one; otherwise the only product whose
// name contains it ("API Manager" -> "WSO2 API Manager"). A name contained in
// several products' names, or in none, is an errDashboardProduct saying so.
func (r productResolver) resolveProductName(ctx context.Context, value, where string) (string, error) {
	written := strings.TrimSpace(value)
	key := strings.ToLower(written)
	if name, ok := r.names[key]; ok {
		return name, nil
	}
	if written == "" {
		return "", fmt.Errorf("%w: %s: a product filter has an empty name", errDashboardProduct, where)
	}
	body, err := json.Marshal(map[string]any{
		"searchQuery": written,
		"pagination":  map[string]int{"limit": productSearchLimit, "offset": 0},
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", errDashboardProductLookup, where, err)
	}
	raw, err := r.search.SearchProducts(ctx, body)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", errDashboardProductLookup, where, err)
	}
	var res struct {
		Products []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"products"`
		HasMore bool `json:"hasMore"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", fmt.Errorf("%w: %s: read the products search: %v", errDashboardProductLookup, where, err)
	}
	var names []string
	ids := map[string][]string{}
	for _, p := range res.Products {
		n := strings.TrimSpace(p.Name)
		if n == "" {
			continue
		}
		if _, seen := ids[n]; !seen {
			names = append(names, n)
		}
		ids[n] = append(ids[n], p.ID)
	}
	for _, n := range names {
		if strings.EqualFold(n, written) {
			r.names[key], r.ids[n] = n, ids[n]
			return n, nil
		}
	}
	switch {
	case len(names) == 1 && !res.HasMore:
		r.names[key], r.ids[names[0]] = names[0], ids[names[0]]
		return names[0], nil
	case len(names) == 0:
		return "", fmt.Errorf("%w: %s: no product is named like %q", errDashboardProduct, where, written)
	}
	shown := names
	more := ""
	if len(names) > productNamesShown {
		shown = names[:productNamesShown]
		more = fmt.Sprintf(" and %d more", len(names)-productNamesShown)
	}
	if res.HasMore {
		more = " and more"
	}
	return "", fmt.Errorf("%w: %s: %q matches several products (%s%s); name the one you mean",
		errDashboardProduct, where, written, strings.Join(shown, ", "), more)
}
