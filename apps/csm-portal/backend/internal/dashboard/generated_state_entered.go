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
	"fmt"
	"strings"
	"time"
)

// caseStates are the values entity-service's case search takes for a
// stateEntered filter (the case states, as for state).
var caseStates = []string{"open", "work_in_progress", "waiting_on_wso2", "awaiting_info", "solution_proposed", "reopened", "closed"}

// checkGeneratedStateEntered checks a generated case-family widget's
// stateEntered / stateEnteredOn filters ("cases that moved to a state in a
// window"), which entity-service would otherwise refuse only when the tile
// loads: stateEntered is "in" with case states, stateEnteredOn is "gte" or
// "lte" with one date, needs a stateEntered in the same search (a slice's
// filters merged over the widget's, as the webapp merges them), and neither
// sits in an anyOf branch. A window of two plain dates must not end before it
// starts (checkStateEnteredWindow). A widget without them is not touched.
func checkGeneratedStateEntered(i int, w WidgetTemplate) error {
	at := fmt.Sprintf("widgets[%d] (%q)", i, w.ID)
	base := filterFields(w.Query)
	if err := checkStateEnteredFilters(at+".query", w.Query, base, nil); err != nil {
		return err
	}
	if err := checkStateEnteredWindow(at+".query", w.Query); err != nil {
		return err
	}
	for j, sl := range w.Slices {
		sliceAt := fmt.Sprintf("%s.slices[%d].query", at, j)
		if err := checkStateEnteredFilters(sliceAt, sl.Query, filterFields(sl.Query), base); err != nil {
			return err
		}
		// The webapp merges a slice over the widget by field: a slice with
		// its own stateEnteredOn replaces the widget's whole window, and one
		// without keeps the widget's, checked above.
		if err := checkStateEnteredWindow(sliceAt, sl.Query); err != nil {
			return err
		}
	}
	return nil
}

// checkStateEnteredWindow refuses a stateEnteredOn window whose end is before
// its start, which entity-service would refuse when the tile loads. It reads
// the bounds as entity-service does: the last gte and the last lte win, and a
// date-only lte covers its whole day. A window with a relative date (or only
// one bound) is not compared: it moves with the day it is read.
func checkStateEnteredWindow(at string, query map[string]any) error {
	var start, end string
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		entry, _ := f.(map[string]any)
		if field, _ := entry["field"].(string); field != "stateEnteredOn" {
			continue
		}
		values, _ := stringList(entry["values"])
		if len(values) != 1 {
			continue
		}
		switch op, _ := entry["op"].(string); op {
		case "gte":
			start = values[0]
		case "lte":
			end = values[0]
		}
	}
	from, okFrom := literalBound(start, false)
	to, okTo := literalBound(end, true)
	if okFrom && okTo && to.Before(from) {
		return fmt.Errorf("%w: %s: stateEnteredOn ends (lte %s) before it starts (gte %s)", ErrGeneratedInvalid, at, end, start)
	}
	return nil
}

// literalBound parses a plain date or an RFC3339 instant; a date-only upper
// bound is the end of that day. ok is false for anything else, a relative
// date included.
func literalBound(s string, upper bool) (t time.Time, ok bool) {
	if d, err := time.Parse("2006-01-02", s); err == nil {
		if upper {
			return d.Add(24*time.Hour - time.Nanosecond), true
		}
		return d, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// checkStateEnteredFilters checks one query's stateEntered / stateEnteredOn
// entries; fields are its own filter fields, base the widget's (for a
// slice), so a slice's window may lean on the widget's stateEntered.
func checkStateEnteredFilters(at string, query map[string]any, fields, base map[string]bool) error {
	branches, _ := query["anyOf"].([]any)
	for k, b := range branches {
		branch, _ := b.(map[string]any)
		for field := range filterFields(branch) {
			if field == "stateEntered" || field == "stateEnteredOn" {
				return fmt.Errorf("%w: %s.anyOf[%d]: %s cannot be in an anyOf branch", ErrGeneratedInvalid, at, k, field)
			}
		}
	}
	filters, _ := query["filters"].([]any)
	for k, f := range filters {
		entry, _ := f.(map[string]any)
		field, _ := entry["field"].(string)
		op, _ := entry["op"].(string)
		values, _ := stringList(entry["values"])
		where := fmt.Sprintf("%s.filters[%d]", at, k)
		switch field {
		case "stateEntered":
			if op != "in" || len(values) == 0 {
				return fmt.Errorf("%w: %s: stateEntered takes op \"in\" with one or more case states", ErrGeneratedInvalid, where)
			}
			for _, v := range values {
				if !contains(caseStates, v) {
					return fmt.Errorf("%w: %s: %q is not a case state (%s)", ErrGeneratedInvalid, where, v, strings.Join(caseStates, ", "))
				}
			}
		case "stateEnteredOn":
			if (op != "gte" && op != "lte") || len(values) != 1 || !generatedDate(values[0]) {
				return fmt.Errorf("%w: %s: stateEnteredOn takes op \"gte\" or \"lte\" with one date or a relative date such as __today__", ErrGeneratedInvalid, where)
			}
			if !fields["stateEntered"] && !base["stateEntered"] {
				return fmt.Errorf("%w: %s: stateEnteredOn needs a stateEntered filter (the states the cases moved to)", ErrGeneratedInvalid, where)
			}
		}
	}
	return nil
}

// filterFields is the set of field names in a query's (or an anyOf
// branch's) "filters" list.
func filterFields(query map[string]any) map[string]bool {
	out := map[string]bool{}
	filters, _ := query["filters"].([]any)
	for _, f := range filters {
		entry, _ := f.(map[string]any)
		if field, ok := entry["field"].(string); ok {
			out[field] = true
		}
	}
	return out
}
