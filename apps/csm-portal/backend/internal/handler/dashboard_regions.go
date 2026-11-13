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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// errDashboardRegion marks a generated region filter naming a region no
// case's account has: the handler's 422 with the reason (generate_region).
var errDashboardRegion = errors.New("unusable region filter")

// errDashboardRegionLookup marks a region lookup that failed while the
// regions in a generated dashboard were being checked: the handler's 502.
var errDashboardRegionLookup = errors.New("region lookup failed")

// errMsgGenerateRegionLookup is errDashboardRegionLookup's message.
const errMsgGenerateRegionLookup = "The regions in the dashboard couldn't be checked. Try again."

// regionListGroups is the maxGroups of the aggregate the regions are read
// from: far more than the regions accounts hold, so a reply with others left
// over is a lookup failure rather than a region wrongly reported missing.
const regionListGroups = 1000

// caseAggregator is the entity service's POST /cases/aggregate
// (entity.CustomerEntityClient.AggregateCases), the same aggregate a case
// widget grouped by region runs.
type caseAggregator interface {
	AggregateCases(ctx context.Context, body []byte) ([]byte, error)
}

// caseRegion is one region of a case's account: Key as the case search's
// region filter compares it (no surrounding spaces, lower-cased), Label as
// the account record writes it.
type caseRegion struct {
	Key   string
	Label string
}

// listCaseRegions returns the regions of the accounts of the cases the caller
// can see, largest first, from one aggregate of every case by region. A
// region no visible case has is not listed.
func listCaseRegions(ctx context.Context, cases caseAggregator) ([]caseRegion, error) {
	body, err := json.Marshal(map[string]any{"groupBy": "region", "maxGroups": regionListGroups})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errDashboardRegionLookup, err)
	}
	raw, err := cases.AggregateCases(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errDashboardRegionLookup, err)
	}
	var resp struct {
		Groups []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
		} `json:"groups"`
		OthersCount int `json:"othersCount"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("%w: read the cases aggregate: %v", errDashboardRegionLookup, err)
	}
	if resp.OthersCount > 0 {
		return nil, fmt.Errorf("%w: more than %d regions", errDashboardRegionLookup, regionListGroups)
	}
	regions := make([]caseRegion, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		label := g.Label
		if label == "" {
			label = g.Key
		}
		regions = append(regions, caseRegion{Key: g.Key, Label: label})
	}
	return regions, nil
}

// regionResolvingGenerator checks the regions named in what another generator
// produced, once, right after generation: each value of a "region" filter
// ("in" or "notIn") becomes the region as an account writes it ("apac" and
// "APAC " become "APAC"), and a value no case's account has is refused rather
// than left to count nothing (in) or everything (notIn).
type regionResolvingGenerator struct {
	inner DashboardGenerator
	cases caseAggregator
}

// WithDashboardRegions wraps a generator so that each value of a "region"
// filter is matched, ignoring case and surrounding spaces, against the
// regions of the cases the caller can see (listCaseRegions, read only when a
// region filter is there); a value matching none is an errDashboardRegion.
func WithDashboardRegions(inner DashboardGenerator, cases caseAggregator) DashboardGenerator {
	return &regionResolvingGenerator{inner: inner, cases: cases}
}

// Generate asks the wrapped generator for the dashboard, then resolves its
// region filters.
func (g *regionResolvingGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	result, err := g.inner.Generate(ctx, prompt, current)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	r := &regionResolver{cases: g.cases}
	if err := r.resolveDashboard(ctx, &result.Dashboard); err != nil {
		return GeneratedDashboard{}, err
	}
	return result, nil
}

// regionResolver resolves one generated dashboard's region filters, reading
// the regions once, at the first region filter.
type regionResolver struct {
	cases   caseAggregator
	regions []caseRegion // nil until read
	byKey   map[string]string
}

// resolveDashboard resolves every widget's and slice's region filters in
// place. ParseGenerated has already made sure each is "in" or "notIn" with
// values, in a query's top-level filters, and only on a case-family widget.
func (r *regionResolver) resolveDashboard(ctx context.Context, d *dashboard.Dashboard) error {
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
	return nil
}

// resolveQuery replaces the values of a query's region filters with the
// regions they name, each once; any other filter is left as it is.
func (r *regionResolver) resolveQuery(ctx context.Context, query map[string]any, where string) error {
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		if field, _ := entry["field"].(string); field != "region" {
			continue
		}
		at := fmt.Sprintf("%s.filters[%d]", where, k)
		if err := r.load(ctx, at); err != nil {
			return err
		}
		values, _ := entry["values"].([]any)
		resolved := make([]any, 0, len(values))
		seen := map[string]bool{}
		for _, v := range values {
			s, _ := v.(string)
			key := strings.ToLower(strings.TrimSpace(s))
			label, ok := r.byKey[key]
			if !ok {
				return fmt.Errorf("%w: %s: %s", errDashboardRegion, at, r.unknown(s))
			}
			if !seen[key] {
				seen[key] = true
				resolved = append(resolved, label)
			}
		}
		entry["values"] = resolved
	}
	return nil
}

// load reads the regions, once.
func (r *regionResolver) load(ctx context.Context, at string) error {
	if r.byKey != nil {
		return nil
	}
	regions, err := listCaseRegions(ctx, r.cases)
	if err != nil {
		return fmt.Errorf("%w: %s", err, at)
	}
	r.regions, r.byKey = regions, make(map[string]string, len(regions))
	for _, region := range regions {
		r.byKey[region.Key] = region.Label
	}
	return nil
}

// unknown is the reason a value names no region, with the regions there are.
func (r *regionResolver) unknown(value string) string {
	written := strings.TrimSpace(value)
	if len(r.regions) == 0 {
		return fmt.Sprintf("no case's account has a region yet, so %q cannot be filtered on", written)
	}
	labels := make([]string, len(r.regions))
	for i, region := range r.regions {
		labels[i] = region.Label
	}
	return fmt.Sprintf("no case's account has the region %q; the regions are %s", written, strings.Join(labels, ", "))
}
