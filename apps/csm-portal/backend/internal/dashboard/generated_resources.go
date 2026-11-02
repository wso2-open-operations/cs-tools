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
	"regexp"
	"sort"
	"strings"
	"time"
)

// Besides the case family, a generated dashboard may hold widgets of the
// resourceTypes in generatedResourceRules. Their queries are not the case
// DSL, so each is checked here against what its own search accepts (checked
// live against entity-service on the Postgres data source), and only what a
// prompt can express without knowing ids: no account, project, group or
// user ids, the viewer ("__current_user__") only.

// valueKind is how one filter value list is checked.
type valueKind int

const (
	valuesEnum        valueKind = iota // every value in values
	valuesCurrentUser                  // exactly ["__current_user__"]
	valuesDate                         // each a date, a timestamp or a relative date token
	valuesBool                         // exactly one of "true", "false"
	valuesOneOf                        // exactly one value in values
	valuesScalarEnum                   // a flat key's single string, one of values
	valuesJSONBool                     // a flat key's JSON true / false
	valuesTeamName                     // a flat key's single team name or key (checked against the registry later)
	valuesRating                       // a flat key's JSON whole number 1 to 5
)

// valueRule checks a value list: a flat key's list, or a field filter's values.
type valueRule struct {
	kind   valueKind
	values []string // valuesEnum, valuesOneOf
}

// fieldRule is one field a "filters" list entry may name.
type fieldRule struct {
	ops   []string
	value valueRule
}

// resourceRules is what a generated widget of one resourceType may hold.
type resourceRules struct {
	noun       string               // in messages: "a time card widget", "is not a time card filter"
	shapes     []Shape              // allowed shapes
	groupBy    []string             // allowed groupBy.field values
	buckets    []string             // allowed groupBy.bucket values; with groupBy empty, a bucket is the only grouping
	flat       map[string]valueRule // allowed flat query keys
	dateKeys   map[string]bool      // flat keys that hold one date string, not a list
	fields     map[string]fieldRule // allowed "filters" list fields; nil means no list
	sortBy     []string             // allowed list sortBy fields; none means a list sets no sortBy
	slicesVia  string               // the grouping to suggest when groupBy is not allowed
	fixedDates bool                 // dateKeys take a plain date only: no relative date token (the webapp does not fill one in for this type)
}

var (
	opsShapes = []Shape{ShapeCount, ShapeList, ShapePie, ShapeBar}
	onlyMe    = valueRule{kind: valuesCurrentUser}
	aDate     = valueRule{kind: valuesDate}
	aBool     = valueRule{kind: valuesBool}
	createdOn = fieldRule{ops: []string{"gte", "lte"}, value: aDate}
	assignee  = fieldRule{ops: []string{"in"}, value: onlyMe}
)

// generatedResourceRules: the non-case resourceTypes a generated dashboard
// may use. Incident and problem states are upper case in the filters list;
// change request states are the flat, lower-case "states" (a "state" field
// filter is refused by that search). Server-side grouping is by state only:
// assignmentGroup is refused on the Postgres data source. Problems take no
// sortBy at all.
var generatedResourceRules = map[ResourceType]resourceRules{
	ResourceTimeCard: {
		noun:   "time card",
		shapes: opsShapes,
		flat: map[string]valueRule{
			"states":    {kind: valuesEnum, values: []string{"pending", "submitted", "approved", "rejected", "processed", "recalled"}},
			"userIds":   onlyMe,
			"startDate": aDate,
			"endDate":   aDate,
		},
		dateKeys:  map[string]bool{"startDate": true, "endDate": true},
		sortBy:    []string{"workDate", "updatedOn"},
		slicesVia: "group by state with slices",
	},
	ResourceIncident: {
		noun:    "incident",
		shapes:  opsShapes,
		groupBy: []string{"state"},
		flat: map[string]valueRule{
			"priorities": {kind: valuesEnum, values: []string{"CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"}},
		},
		fields: map[string]fieldRule{
			"state":          {ops: []string{"in"}, value: valueRule{kind: valuesEnum, values: []string{"NEW", "IN_PROGRESS", "ON_HOLD", "RESOLVED", "CLOSED", "CANCELLED"}}},
			"createdOn":      createdOn,
			"slaViolated":    {ops: []string{"eq"}, value: aBool},
			"madeSla":        {ops: []string{"eq"}, value: aBool},
			"assignedUserId": assignee,
		},
		sortBy: []string{"createdOn", "updatedOn", "openedOn"},
	},
	ResourceChangeRequest: {
		noun:    "change request",
		shapes:  opsShapes,
		groupBy: []string{"state"},
		flat: map[string]valueRule{
			"states":  {kind: valuesEnum, values: []string{"new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "canceled", "rollback"}},
			"impacts": {kind: valuesEnum, values: []string{"high", "medium", "low"}},
		},
		fields: map[string]fieldRule{
			"approval":       {ops: []string{"eq"}, value: valueRule{kind: valuesOneOf, values: []string{"not requested", "requested", "approved", "rejected"}}},
			"createdOn":      createdOn,
			"assignedUserId": assignee,
		},
		sortBy: []string{"createdOn", "updatedOn"},
	},
	// Accounts and projects take flat filters only and cannot group on the
	// server; neither search takes a widget sortBy (projects only sort by end
	// date). An account's abtTeam is a team name: the generator writes the
	// registry key and the team resolver turns it into the name.
	ResourceAccount: {
		noun:   "account",
		shapes: opsShapes,
		flat: map[string]valueRule{
			"lifeCycle":    {kind: valuesScalarEnum, values: []string{"Customer", "Partner", "Prospect", "Lost Customer", "Lost Prospect"}},
			"abtTeam":      {kind: valuesTeamName},
			"risks":        {kind: valuesEnum, values: []string{"hasNoGoLive", "hasGoneLive", "hasRecentCases", "noSupportCases6mo", "hasEolProduct", "hasAbandonedMigrations", "hasMigrationDelays", "hasRecentEscalations"}},
			"healthStatus": {kind: valuesScalarEnum, values: []string{"at_risk", "healthy", "to_be_reviewed"}},
			"hasProject":   {kind: valuesJSONBool},
			"active":       {kind: valuesJSONBool},
		},
		slicesVia: "group with slices",
	},
	ResourceProject: {
		noun:   "project",
		shapes: opsShapes,
		flat: map[string]valueRule{
			"onboardingStatus":     {kind: valuesEnum, values: []string{"Not-Started", "In-Progress", "Completed", "Not-Applicable"}},
			"closureStatus":        {kind: valuesScalarEnum, values: []string{"Open", "Suspended", "Restricted"}},
			"excludeClosureStates": {kind: valuesEnum, values: []string{"Open", "Suspended", "Restricted"}},
			"endDateFrom":          aDate,
			"endDateTo":            aDate,
		},
		dateKeys:  map[string]bool{"endDateFrom": true, "endDateTo": true},
		sortBy:    []string{"endDate"},
		slicesVia: "group with slices",
	},
	// Case feedback (CSAT): a flat rating and submission dates; grouped on the
	// server by a date bucket (bars of the average rating) or by rating /
	// reasons (counts), never by a field. The search takes no sortBy.
	ResourceCaseFeedback: {
		noun:   "case feedback",
		shapes: opsShapes,
		flat: map[string]valueRule{
			"rating":   {kind: valuesRating},
			"dateFrom": aDate,
			"dateTo":   aDate,
		},
		dateKeys:   map[string]bool{"dateFrom": true, "dateTo": true},
		fixedDates: true,
		buckets: []string{"day", "week", "month", "rating", "reasons_very_dissatisfied", "reasons_dissatisfied",
			"reasons_neutral", "reasons_satisfied", "reasons_very_satisfied"},
		slicesVia: "group with groupBy.bucket or with rating slices",
	},
	ResourceProblem: {
		noun:    "problem",
		shapes:  opsShapes,
		groupBy: []string{"state"},
		fields: map[string]fieldRule{
			"state":          {ops: []string{"in"}, value: valueRule{kind: valuesEnum, values: []string{"NEW", "ASSESS", "ROOT_CAUSE_ANALYSIS", "FIX_IN_PROGRESS", "RESOLVED", "CLOSED"}}},
			"assignedUserId": assignee,
		},
	},
}

// generatedRelativeDate is a relative date placeholder the webapp turns into
// a date before the request (resolveRelativeDateFilters).
var generatedRelativeDate = regexp.MustCompile(`^__(today|daysAgo|startOfMonth|endOfMonth|startOfQuarter|endOfQuarter)(:-?\d+)?__$`)

// checkGeneratedResourceWidget checks a generated widget whose resourceType
// is in generatedResourceRules: its shape, groupBy, query, every slice's
// query, and its sortBy.
func checkGeneratedResourceWidget(i int, w WidgetTemplate, rules resourceRules) error {
	at := fmt.Sprintf("widgets[%d] (%q)", i, w.ID)
	if !containsShape(rules.shapes, w.Shape) {
		return fmt.Errorf("%w: %s: a %s widget's shape must be %s, not %q", ErrGeneratedInvalid, at, rules.noun, orList(shapeNames(rules.shapes)), w.Shape)
	}
	if w.GroupBy != nil {
		switch {
		case len(rules.buckets) > 0:
			if w.GroupBy.Field != "" || !contains(rules.buckets, w.GroupBy.Bucket) {
				return fmt.Errorf("%w: %s.groupBy: a %s widget groups by bucket %s only", ErrGeneratedInvalid, at, rules.noun, orList(rules.buckets))
			}
		case len(rules.groupBy) == 0:
			return fmt.Errorf("%w: %s: a %s widget cannot use groupBy; %s", ErrGeneratedInvalid, at, rules.noun, rules.slicesVia)
		case w.GroupBy.Bucket != "" || !contains(rules.groupBy, w.GroupBy.Field):
			return fmt.Errorf("%w: %s.groupBy: a %s widget groups by %s only", ErrGeneratedInvalid, at, rules.noun, orList(rules.groupBy))
		}
	}

	if err := checkGeneratedQuery(at+".query", w.Query, rules); err != nil {
		return err
	}
	for j, sl := range w.Slices {
		if err := checkGeneratedQuery(fmt.Sprintf("%s.slices[%d].query", at, j), sl.Query, rules); err != nil {
			return err
		}
	}
	if len(w.SortBy) > 0 {
		if len(rules.sortBy) == 0 {
			return fmt.Errorf("%w: %s.sortBy: a %s list takes no sortBy", ErrGeneratedInvalid, at, rules.noun)
		}
		field, _ := w.SortBy["field"].(string)
		order, _ := w.SortBy["order"].(string)
		if !contains(rules.sortBy, field) || (order != "asc" && order != "desc") {
			return fmt.Errorf("%w: %s.sortBy: a %s list sorts by %s, asc or desc", ErrGeneratedInvalid, at, rules.noun, orList(rules.sortBy))
		}
	}
	return nil
}

// checkGeneratedQuery checks one query object: its flat keys and, when the
// type has one, its "filters" list.
func checkGeneratedQuery(at string, query map[string]any, rules resourceRules) error {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := query[k]
		if k == "filters" && rules.fields != nil {
			if err := checkGeneratedFieldFilters(at+".filters", v, rules); err != nil {
				return err
			}
			continue
		}
		rule, ok := rules.flat[k]
		if !ok {
			return fmt.Errorf("%w: %s: %q is not a %s filter (%s)", ErrGeneratedInvalid, at, k, rules.noun, strings.Join(allowedKeys(rules), ", "))
		}
		switch rule.kind {
		case valuesScalarEnum:
			if s, ok := v.(string); !ok || !contains(rule.values, s) {
				return fmt.Errorf("%w: %s.%s must be one of %s", ErrGeneratedInvalid, at, k, strings.Join(rule.values, ", "))
			}
			continue
		case valuesJSONBool:
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%w: %s.%s must be true or false", ErrGeneratedInvalid, at, k)
			}
			continue
		case valuesTeamName:
			if s, ok := v.(string); !ok || strings.TrimSpace(s) == "" {
				return fmt.Errorf("%w: %s.%s must be a team's key from the Teams section", ErrGeneratedInvalid, at, k)
			}
			continue
		case valuesRating:
			if n, ok := v.(float64); !ok || n != float64(int(n)) || n < 1 || n > 5 {
				return fmt.Errorf("%w: %s.%s must be a whole number from 1 to 5", ErrGeneratedInvalid, at, k)
			}
			continue
		}
		if rules.dateKeys[k] {
			s, ok := v.(string)
			if ok && rules.fixedDates && generatedRelativeDate.MatchString(s) {
				return fmt.Errorf("%w: %s.%s: a %s widget takes a plain date (YYYY-MM-DD), not a relative date", ErrGeneratedInvalid, at, k, rules.noun)
			}
			if !ok || !generatedDate(s) {
				return fmt.Errorf("%w: %s.%s must be a date (YYYY-MM-DD) or a relative date such as __daysAgo:30__", ErrGeneratedInvalid, at, k)
			}
			continue
		}
		values, ok := stringList(v)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be a list of strings", ErrGeneratedInvalid, at, k)
		}
		if err := checkGeneratedValues(at+"."+k, values, rule, rules.noun, k); err != nil {
			return err
		}
	}
	return nil
}

// checkGeneratedFieldFilters checks a "filters" list: each entry a
// {field, op, values} object naming an allowed field and op.
func checkGeneratedFieldFilters(at string, v any, rules resourceRules) error {
	entries, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%w: %s must be a list of {field, op, values}", ErrGeneratedInvalid, at)
	}
	for j, e := range entries {
		eat := fmt.Sprintf("%s[%d]", at, j)
		entry, ok := e.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s must be a {field, op, values} object", ErrGeneratedInvalid, eat)
		}
		for k := range entry {
			if k != "field" && k != "op" && k != "values" {
				return fmt.Errorf("%w: %s: unknown key %q", ErrGeneratedInvalid, eat, k)
			}
		}
		field, _ := entry["field"].(string)
		op, _ := entry["op"].(string)
		rule, ok := rules.fields[field]
		if !ok {
			names := make([]string, 0, len(rules.fields))
			for n := range rules.fields {
				names = append(names, n)
			}
			sort.Strings(names)
			return fmt.Errorf("%w: %s: field %q is not a %s filter field (%s)", ErrGeneratedInvalid, eat, field, rules.noun, strings.Join(names, ", "))
		}
		if !contains(rule.ops, op) {
			return fmt.Errorf("%w: %s: field %q takes op %s, not %q", ErrGeneratedInvalid, eat, field, orList(rule.ops), op)
		}
		values, ok := stringList(entry["values"])
		if !ok {
			return fmt.Errorf("%w: %s.values must be a list of strings", ErrGeneratedInvalid, eat)
		}
		if err := checkGeneratedValues(eat+".values", values, rule.value, rules.noun, field); err != nil {
			return err
		}
	}
	return nil
}

// checkGeneratedValues checks one filter's values against its rule: values
// from a fixed list, exactly one of a list, the current user only, one date
// (or relative date), or one boolean. The error names where (at) and what is
// allowed.
func checkGeneratedValues(at string, values []string, rule valueRule, noun, key string) error {
	switch rule.kind {
	case valuesEnum:
		if len(values) == 0 {
			return fmt.Errorf("%w: %s must be a non-empty list of %s", ErrGeneratedInvalid, at, key)
		}
		for _, v := range values {
			if !contains(rule.values, v) {
				return fmt.Errorf("%w: %s: %q is not a %s %s (%s)", ErrGeneratedInvalid, at, v, noun, singular(key), strings.Join(rule.values, ", "))
			}
		}
	case valuesOneOf:
		if len(values) != 1 || !contains(rule.values, values[0]) {
			return fmt.Errorf("%w: %s must be exactly one of %s", ErrGeneratedInvalid, at, strings.Join(rule.values, ", "))
		}
	case valuesCurrentUser:
		if len(values) != 1 || values[0] != "__current_user__" {
			return fmt.Errorf("%w: %s can only be [\"__current_user__\"] (the viewer's own)", ErrGeneratedInvalid, at)
		}
	case valuesDate:
		if len(values) != 1 || !generatedDate(values[0]) {
			return fmt.Errorf("%w: %s must be one date or a relative date such as __daysAgo:30__", ErrGeneratedInvalid, at)
		}
	case valuesBool:
		if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			return fmt.Errorf("%w: %s must be exactly one of \"true\", \"false\"", ErrGeneratedInvalid, at)
		}
	}
	return nil
}

// valueNames names one value of a list key in messages: "states" -> "state".
var valueNames = map[string]string{
	"states": "state", "priorities": "priority", "impacts": "impact", "risks": "risk",
	"onboardingStatus": "onboarding status", "excludeClosureStates": "closure state",
}

// singular is the readable name of one value of key (valueNames), for error
// messages; key itself when it has none.
func singular(key string) string {
	if name, ok := valueNames[key]; ok {
		return name
	}
	return key
}

// allowedKeys is the query keys a resource type accepts, sorted, with
// "filters" when it takes field filters.
func allowedKeys(rules resourceRules) []string {
	keys := make([]string, 0, len(rules.flat)+1)
	for k := range rules.flat {
		keys = append(keys, k)
	}
	if rules.fields != nil {
		keys = append(keys, "filters")
	}
	sort.Strings(keys)
	return keys
}

// generatedDate reports whether s is a date a generated filter may hold: a
// relative date placeholder, YYYY-MM-DD, or an RFC 3339 time.
func generatedDate(s string) bool {
	if generatedRelativeDate.MatchString(s) {
		return true
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

// stringList is v as a list of strings; false when it is not one.
func stringList(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// contains reports whether list holds s.
func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// containsShape reports whether list holds s.
func containsShape(list []Shape, s Shape) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// shapeNames is shapes as strings, for error messages.
func shapeNames(shapes []Shape) []string {
	out := make([]string, len(shapes))
	for i, s := range shapes {
		out[i] = string(s)
	}
	return out
}

// orList joins ["a","b","c"] as "a, b or c".
func orList(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}
