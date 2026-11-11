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
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// namedSearchLimit is a page of the search one name is checked against: the
// entity service's own cap.
const namedSearchLimit = 50

// namedSearchPages caps how many pages one name reads. An exact code is
// taken as soon as it is seen; an exact name only once every page has been
// read, since another record of that name may be on a later one. A name that
// matches more records than these pages hold is answered as matching several.
const namedSearchPages = 10

// namedShown is how many candidates a "matches several" reason lists before
// "and N more".
const namedShown = 5

// namedRecord is one result of a search by name: a project (code: its key)
// or an account (code: its number).
type namedRecord struct {
	ID, Code, Name string
}

// label is how a reason names the record: its name and code, or its code
// alone when it has no name of its own.
func (r namedRecord) label() string {
	name := strings.TrimSpace(r.Name)
	if name == "" || strings.EqualFold(name, r.Code) {
		return r.Code
	}
	return fmt.Sprintf("%s (%s)", name, r.Code)
}

// namedKind is one kind of record a generated case filter names as the user
// wrote it and the case search filters by id: the filter's field, the id
// field it becomes, the nouns its reasons use, its two errors, and the
// search (by name or code, among the records the caller can see): one page
// of at most namedSearchLimit, from offset.
type namedKind struct {
	field, idField   string
	noun, plural     string
	unusable, lookup error
	search           func(ctx context.Context, written string, offset int) (records []namedRecord, hasMore bool, err error)
}

// namedResolvingGenerator turns the records named in what another generator
// produced into their ids, once, right after generation: the case search
// filters them by id, which the model never knows. From then on the dashboard
// holds ids, so the generate reply, Save, My Dashboards and Shared Dashboards
// need nothing more, and a renamed record still matches.
type namedResolvingGenerator struct {
	inner DashboardGenerator
	kind  namedKind
}

// Generate asks the wrapped generator for the dashboard, then resolves its
// filters on kind.field.
func (g *namedResolvingGenerator) Generate(ctx context.Context, prompt string, current json.RawMessage) (GeneratedDashboard, error) {
	result, err := g.inner.Generate(ctx, prompt, current)
	if err != nil {
		return GeneratedDashboard{}, err
	}
	r := namedResolver{kind: g.kind, ids: map[string]string{}}
	if err := r.resolveDashboard(ctx, &result.Dashboard); err != nil {
		return GeneratedDashboard{}, err
	}
	return result, nil
}

// namedResolver resolves one generated dashboard's filters on one kind,
// searching each distinct value once.
type namedResolver struct {
	kind namedKind
	ids  map[string]string // a value as written, lower-cased -> its record's id
}

// resolveDashboard resolves every widget's and slice's filters in place.
// ParseGenerated has already made sure each is "in" or "notIn" with names,
// in a query's top-level filters.
func (r namedResolver) resolveDashboard(ctx context.Context, d *dashboard.Dashboard) error {
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

// resolveQuery turns each of a query's filters on kind.field into a filter
// on kind.idField, with its op, on the ids of the records its values name,
// each id once; any other filter is left as it is.
func (r namedResolver) resolveQuery(ctx context.Context, query map[string]any, where string) error {
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		if field, _ := entry["field"].(string); field != r.kind.field {
			continue
		}
		values, _ := entry["values"].([]any)
		resolved := make([]any, 0, len(values))
		seen := map[string]bool{}
		for _, v := range values {
			s, _ := v.(string)
			id, err := r.resolveValue(ctx, s, fmt.Sprintf("%s.filters[%d]", where, k))
			if err != nil {
				return err
			}
			if !seen[id] {
				seen[id] = true
				resolved = append(resolved, id)
			}
		}
		entry["field"], entry["values"] = r.kind.idField, resolved
	}
	return nil
}

// resolveValue is the id of the record a value names: the record with exactly
// that code, ignoring case, if there is one; else the one record with exactly
// that name; else the only record the search finds. The search is read page
// by page (namedSearchPages). A value naming none, or several (two records of
// one name, or a value the search finds in several, or in more than those
// pages hold), is a kind.unusable saying so; a failed search is a kind.lookup.
func (r namedResolver) resolveValue(ctx context.Context, value, where string) (string, error) {
	k := r.kind
	written := strings.TrimSpace(value)
	key := strings.ToLower(written)
	if id, ok := r.ids[key]; ok {
		return id, nil
	}
	if written == "" {
		return "", fmt.Errorf("%w: %s: a %s filter has an empty name", k.unusable, where, k.noun)
	}
	var records, named []namedRecord
	hasMore := true
	for page := 0; page < namedSearchPages && hasMore; page++ {
		var got []namedRecord
		var err error
		got, hasMore, err = k.search(ctx, written, len(records))
		if err != nil {
			return "", fmt.Errorf("%w: %s: %v", k.lookup, where, err)
		}
		for _, rec := range got {
			if strings.EqualFold(strings.TrimSpace(rec.Code), written) {
				r.ids[key] = rec.ID
				return rec.ID, nil
			}
			if strings.EqualFold(strings.TrimSpace(rec.Name), written) {
				named = append(named, rec)
			}
		}
		records = append(records, got...)
		if len(got) == 0 {
			break
		}
	}
	candidates, more := records, hasMore
	switch {
	case len(named) == 1 && !hasMore:
		r.ids[key] = named[0].ID
		return named[0].ID, nil
	case len(named) > 1:
		candidates, more = named, false
	case len(records) == 1 && !hasMore:
		r.ids[key] = records[0].ID
		return records[0].ID, nil
	case len(records) == 0:
		return "", fmt.Errorf("%w: %s: no %s is named like %q", k.unusable, where, k.noun, written)
	}
	var shown []string
	for _, rec := range candidates[:min(len(candidates), namedShown)] {
		shown = append(shown, rec.label())
	}
	tail := ""
	if more {
		tail = " and more"
	} else if len(candidates) > namedShown {
		tail = fmt.Sprintf(" and %d more", len(candidates)-namedShown)
	}
	return "", fmt.Errorf("%w: %s: %q matches several %s (%s%s); name the one you mean",
		k.unusable, where, written, k.plural, strings.Join(shown, ", "), tail)
}
