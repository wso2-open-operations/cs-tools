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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
	valuesText                         // free text: 1 to maxGeneratedTextValues values, each 1 to maxGeneratedTextLength characters
	valuesCode                         // a flat key's single string, 1 to maxGeneratedTextLength characters
	valuesEmail                        // 1 to maxGeneratedTextValues email addresses, each at most maxGeneratedEmailLength characters
)

// The most values a valuesText list holds, and the longest one (entity-service
// refuses a PLG country name over 100 characters).
const (
	maxGeneratedTextValues = 20
	maxGeneratedTextLength = 100
)

// maxGeneratedEmailLength is the longest email address a valuesEmail list
// holds (as entity-service's PLG owner email filter takes).
const maxGeneratedEmailLength = 254

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
	groupByOn  []Shape              // the shapes a groupBy may be on; none means any
	exclusive  [][2]string          // flat keys never set together in one query (with a slice's added), the second a JSON boolean set true
	needs      map[string]string    // flat key -> the flat key a query holding it must hold too (with a slice's added)
	groupNeeds map[string]string    // groupBy.field -> the flat key the widget's own query must hold
	fixedDates bool                 // dateKeys take a plain date only: no relative date token (the webapp does not fill one in for this type)
	dayWindow  *dayWindowRule       // groupBy.field "day": the date keys that bound it and its longest window
	maxGroups  int                  // the largest groupBy.maxGroups the read takes (0 is its default); none means no bound here
}

// dayWindowRule is a "day" grouping's window: both keys must be set, and the
// window at most maxDays long, on every day (see checkDayWindow).
type dayWindowRule struct {
	startKey, endKey string
	maxDays          int
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
	// Time cards group on the server (POST /time-cards/aggregate) by state,
	// user, project or day, counting cards or summing hours (groupBy.measure).
	// A "day" grouping returns one bucket per day of its startDate..endDate
	// window, which entity-service caps at 31 days.
	ResourceTimeCard: {
		noun:    "time card",
		shapes:  opsShapes,
		groupBy: []string{"state", "user", "project", "day"},
		flat: map[string]valueRule{
			"states":    {kind: valuesEnum, values: []string{"pending", "submitted", "approved", "rejected", "processed", "recalled"}},
			"userIds":   onlyMe,
			"startDate": aDate,
			"endDate":   aDate,
		},
		dateKeys:  map[string]bool{"startDate": true, "endDate": true},
		sortBy:    []string{"workDate", "updatedOn"},
		dayWindow: &dayWindowRule{startKey: "startDate", endKey: "endDate", maxDays: 31},
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
	// PLG organisations and registrations (POST /plg/organizations/search,
	// /plg/registrations/search) take the PLG list's flat filters, and only
	// those a prompt can name: products, lifecycle stages, subscription
	// tiers, the registration dates, "unowned" and the viewer's own (ownerIds
	// exactly ["__current_user__"], which the webapp resolves to the viewer's
	// user id, the id an owner is; no other owner ids), no free
	// text). A pie or bar uses slices (one search per slice, each slice's
	// query held to the same filters); organisations may instead group by
	// owner on the server (POST /plg/organizations/aggregate, "Unowned" its
	// own group), and either type by country (/plg/organizations/aggregate,
	// /plg/registrations/aggregate, "Unknown" its own group), on a pie or bar
	// only. A country filter is countryNames (names as written, matched
	// ignoring case) or countryUnknown, never both in one query. A playbook
	// answer filter names its task (playbookTask, a task code) and then the
	// answers (playbookAnswers, option codes) or playbookUnanswered, never
	// both; registrations may group by their answer to the query's
	// playbookTask (groupBy playbookAnswer, /plg/registrations/aggregate, "Not
	// answered" its own group), on a pie or bar only. The codes are checked
	// against the live playbooks after generation. The
	// organisation search sorts by name or registration date; the
	// registration search has one order and takes no sortBy.
	// Registrations may also be a line: registrations per day, week or month
	// (POST /plg/registrations/volume-trend), with the case line's days or
	// fixed window and chart, checked as for any definition; never series.
	ResourcePlgOrganization: {
		noun:      "PLG organisation",
		shapes:    plgShapes,
		groupBy:   []string{"owner", "country"},
		flat:      plgFilters,
		dateKeys:  plgDateKeys,
		sortBy:    []string{"name", "registeredOn"},
		slicesVia: "group with slices",
		groupByOn: plgGroupByShapes,
		exclusive: plgExclusive,
		needs:     plgNeeds,
		maxGroups: maxPlgGroups,
	},
	ResourcePlgRegistration: {
		noun:       "PLG registration",
		shapes:     plgRegistrationShapes,
		groupBy:    []string{"country", "playbookAnswer"},
		flat:       withRule(plgFilters, "unacknowledgedOnly", valueRule{kind: valuesJSONBool}),
		dateKeys:   plgDateKeys,
		slicesVia:  "group with slices",
		groupByOn:  plgGroupByShapes,
		exclusive:  plgExclusive,
		needs:      plgNeeds,
		groupNeeds: map[string]string{"playbookAnswer": "playbookTask"},
		maxGroups:  maxPlgGroups,
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

// The PLG widgets' shared rules: the values are the PLG domain's own
// (plg/domain ValidLifecycleStage, ValidSubscriptionTier) and the products
// seeded in plg_product.
var (
	plgShapes  = []Shape{ShapeCount, ShapeList, ShapePie, ShapeBar}
	plgFilters = map[string]valueRule{
		"productCodes":       {kind: valuesEnum, values: []string{"IAM", "API_PLATFORM", "INTEGRATION_PLATFORM", "AGENT_PLATFORM", "ENG_PLATFORM"}},
		"lifecycleStages":    {kind: valuesEnum, values: []string{"REGISTRATION", "PLG_CS_ELIGIBLE", "FIRST_VALUE_ACHIEVED", "ACTIVATED", "COMMERCIAL", "ABANDONED"}},
		"subscriptionTiers":  {kind: valuesEnum, values: []string{"FREE", "TRIAL", "TRIAL_EXTENDED", "PAYG"}},
		"registeredFrom":     aDate,
		"registeredTo":       aDate,
		"unowned":            {kind: valuesJSONBool},
		"ownerIds":           onlyMe,
		"ownerEmails":        {kind: valuesEmail},
		"countryNames":       {kind: valuesText},
		"countryUnknown":     {kind: valuesJSONBool},
		"playbookTask":       {kind: valuesCode},
		"playbookAnswers":    {kind: valuesText},
		"playbookUnanswered": {kind: valuesJSONBool},
	}
	plgDateKeys      = map[string]bool{"registeredFrom": true, "registeredTo": true}
	plgGroupByShapes = []Shape{ShapePie, ShapeBar}
	plgExclusive     = [][2]string{{"countryNames", "countryUnknown"}, {"playbookAnswers", "playbookUnanswered"}, {"ownerEmails", "unowned"}}
	plgNeeds         = map[string]string{"playbookAnswers": "playbookTask", "playbookUnanswered": "playbookTask"}
)

// maxPlgGroups is the largest maxGroups entity-service's PLG aggregates take
// (maxPlgAggregateGroups there); a larger one is refused when the tile loads.
const maxPlgGroups = 50

// plgRegistrationShapes are plgShapes and the registration trend.
var plgRegistrationShapes = []Shape{ShapeCount, ShapeList, ShapePie, ShapeBar, ShapeLine}

// withRule is rules plus one more key, as a new map.
func withRule(rules map[string]valueRule, key string, rule valueRule) map[string]valueRule {
	out := make(map[string]valueRule, len(rules)+1)
	for k, v := range rules {
		out[k] = v
	}
	out[key] = rule
	return out
}

// generatedRelativeDate is a relative date placeholder the webapp turns into
// a date before the request (resolveRelativeDateFilters).
var generatedRelativeDate = regexp.MustCompile(`^__(today|daysAgo|startOfWeek|endOfWeek|startOfMonth|endOfMonth|startOfQuarter|endOfQuarter|startOfYear|endOfYear)(:-?\d+)?__$`)

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
		if len(rules.groupByOn) > 0 && !containsShape(rules.groupByOn, w.Shape) {
			return fmt.Errorf("%w: %s.groupBy: a %s widget groups only as a %s, not a %q", ErrGeneratedInvalid, at, rules.noun, orList(shapeNames(rules.groupByOn)), w.Shape)
		}
		if need, ok := rules.groupNeeds[w.GroupBy.Field]; ok {
			if _, has := w.Query[need]; !has {
				return fmt.Errorf("%w: %s.groupBy: grouping by %s needs query.%s", ErrGeneratedInvalid, at, w.GroupBy.Field, need)
			}
		}
		if rules.maxGroups > 0 && (w.GroupBy.MaxGroups < 0 || w.GroupBy.MaxGroups > rules.maxGroups) {
			return fmt.Errorf("%w: %s.groupBy.maxGroups is %d; it must be between 1 and %d (or omitted for the default)", ErrGeneratedInvalid, at, w.GroupBy.MaxGroups, rules.maxGroups)
		}
		if w.GroupBy.Field == "day" && rules.dayWindow != nil {
			if err := checkDayWindow(at, w.Query, *rules.dayWindow); err != nil {
				return err
			}
		}
	}

	if err := checkGeneratedQuery(at+".query", w.Query, rules); err != nil {
		return err
	}
	if err := checkExclusive(at+".query", w.Query, nil, rules); err != nil {
		return err
	}
	if err := checkNeeds(at+".query", w.Query, nil, rules); err != nil {
		return err
	}
	for j, sl := range w.Slices {
		sat := fmt.Sprintf("%s.slices[%d].query", at, j)
		if err := checkGeneratedQuery(sat, sl.Query, rules); err != nil {
			return err
		}
		if err := checkExclusive(sat, w.Query, sl.Query, rules); err != nil {
			return err
		}
		if err := checkNeeds(sat, w.Query, sl.Query, rules); err != nil {
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

// checkExclusive refuses a query (the widget's, with a slice's added when
// slice is set) that holds both keys of one of rules.exclusive, the second
// set true: no record matches both.
func checkExclusive(at string, query, slice map[string]any, rules resourceRules) error {
	for _, pair := range rules.exclusive {
		_, inQuery := query[pair[0]]
		_, inSlice := slice[pair[0]]
		on := query[pair[1]] == true || slice[pair[1]] == true
		if (inQuery || inSlice) && on {
			return fmt.Errorf("%w: %s: %q and %q cannot be combined (with a slice's, the widget's own query counts too)", ErrGeneratedInvalid, at, pair[0], pair[1])
		}
	}
	return nil
}

// checkNeeds refuses a query (the widget's, with a slice's added when slice
// is set) that holds a key of rules.needs without the key it needs.
func checkNeeds(at string, query, slice map[string]any, rules resourceRules) error {
	keys := make([]string, 0, len(rules.needs))
	for k := range rules.needs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		need := rules.needs[k]
		_, inQuery := query[k]
		_, inSlice := slice[k]
		_, hasQuery := query[need]
		_, hasSlice := slice[need]
		if (inQuery || inSlice) && !hasQuery && !hasSlice {
			return fmt.Errorf("%w: %s: %q needs %q (with a slice's, the widget's own query counts too)", ErrGeneratedInvalid, at, k, need)
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
		case valuesCode:
			if s, ok := v.(string); !ok || utf8.RuneCountInString(strings.TrimSpace(s)) == 0 || utf8.RuneCountInString(strings.TrimSpace(s)) > maxGeneratedTextLength {
				return fmt.Errorf("%w: %s.%s must be one code of 1 to %d characters", ErrGeneratedInvalid, at, k, maxGeneratedTextLength)
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
	case valuesEmail:
		if len(values) == 0 || len(values) > maxGeneratedTextValues {
			return fmt.Errorf("%w: %s must be a list of 1 to %d email addresses", ErrGeneratedInvalid, at, maxGeneratedTextValues)
		}
		for _, v := range values {
			v = strings.TrimSpace(v)
			if local, host, ok := strings.Cut(v, "@"); !ok || local == "" || host == "" || utf8.RuneCountInString(v) > maxGeneratedEmailLength {
				return fmt.Errorf("%w: %s: each value must be an email address of at most %d characters", ErrGeneratedInvalid, at, maxGeneratedEmailLength)
			}
		}
	case valuesText:
		if len(values) == 0 || len(values) > maxGeneratedTextValues {
			return fmt.Errorf("%w: %s must be a list of 1 to %d values", ErrGeneratedInvalid, at, maxGeneratedTextValues)
		}
		for _, v := range values {
			if n := utf8.RuneCountInString(strings.TrimSpace(v)); n == 0 || n > maxGeneratedTextLength {
				return fmt.Errorf("%w: %s: each value must be 1 to %d characters", ErrGeneratedInvalid, at, maxGeneratedTextLength)
			}
		}
	}
	return nil
}

// valueNames names one value of a list key in messages: "states" -> "state".
var valueNames = map[string]string{
	"states": "state", "priorities": "priority", "impacts": "impact", "risks": "risk",
	"onboardingStatus": "onboarding status", "excludeClosureStates": "closure state",
	"productCodes": "product", "lifecycleStages": "lifecycle stage", "subscriptionTiers": "subscription tier",
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

// checkDayWindow checks a "day" grouping's window: both date keys set, and
// either two plain dates at most rule.maxDays apart (both days counted), or
// two relative dates at most that long on every day. A saved dashboard is
// checked again each time it is read, so "on every day" is what keeps one
// that passes today from failing next month: __startOfQuarter:0__ to
// __today__ is short at a quarter's start and too long by its end. A plain
// date with a relative one grows (or shrinks below zero) day by day, so that
// pair is refused outright. The query's other keys are already checked.
func checkDayWindow(at string, query map[string]any, rule dayWindowRule) error {
	start, _ := query[rule.startKey].(string)
	end, _ := query[rule.endKey].(string)
	if start == "" || end == "" {
		return fmt.Errorf("%w: %s: a time card widget grouped by day needs query.%s and query.%s (at most %d days)",
			ErrGeneratedInvalid, at, rule.startKey, rule.endKey, rule.maxDays)
	}
	tooLong := fmt.Errorf("%w: %s: a time card widget grouped by day covers at most %d days, from query.%s to query.%s",
		ErrGeneratedInvalid, at, rule.maxDays, rule.startKey, rule.endKey)
	startRel, endRel := generatedRelativeDate.MatchString(start), generatedRelativeDate.MatchString(end)
	if startRel != endRel {
		return fmt.Errorf("%w: %s: a time card widget grouped by day takes two plain dates or two relative dates for query.%s and query.%s, not one of each",
			ErrGeneratedInvalid, at, rule.startKey, rule.endKey)
	}
	if !startRel {
		s, errS := time.Parse("2006-01-02", start)
		e, errE := time.Parse("2006-01-02", end)
		if errS != nil || errE != nil {
			return fmt.Errorf("%w: %s: a time card widget grouped by day takes plain dates (YYYY-MM-DD) for query.%s and query.%s",
				ErrGeneratedInvalid, at, rule.startKey, rule.endKey)
		}
		if days := windowDays(s, e); days < 1 || days > rule.maxDays {
			return tooLong
		}
		return nil
	}
	// The years are a sample of calendar shapes, not a limit: every day of
	// 2024-2027 (a leap year and three others) meets every month length,
	// quarter length and weekday a relative date can, and a window's length
	// depends only on those, so a pass holds for any later year too. Walking
	// every day (about 1,460 date steps, under a millisecond per widget) is
	// simpler to trust than a hand-picked list of dates.
	for day := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC); day.Year() < 2028; day = day.AddDate(0, 0, 1) {
		s, okS := relativeDate(start, day)
		e, okE := relativeDate(end, day)
		if !okS || !okE {
			return fmt.Errorf("%w: %s: query.%s or query.%s is not a relative date the dashboard can resolve",
				ErrGeneratedInvalid, at, rule.startKey, rule.endKey)
		}
		if days := windowDays(s, e); days < 1 || days > rule.maxDays {
			return tooLong
		}
	}
	return nil
}

// windowDays is how many days start..end covers, both counted; 0 or less when
// end is before start.
func windowDays(start, end time.Time) int {
	return int(end.Sub(start).Hours()/24) + 1
}

// relativeDate resolves a relative date token on the given day, as the
// webapp's resolveRelativeDatePlaceholderToDate does: __today__ (no
// argument), __daysAgo:N__ (N >= 0), and __startOfWeek:N__, __endOfWeek:N__,
// __startOfMonth:N__, __endOfMonth:N__, __startOfQuarter:N__,
// __endOfQuarter:N__, __startOfYear:N__, __endOfYear:N__ (N weeks / months /
// quarters / years from the current one; the argument is required). A week
// runs Monday to Sunday. false for a token the webapp leaves unresolved,
// and for an argument beyond maxRelativeDateOffset.
func relativeDate(token string, today time.Time) (time.Time, bool) {
	r, ok := parseRelativeDate(token)
	if !ok {
		return time.Time{}, false
	}
	return r.on(today), true
}

// relativeDateToken is a relative date token parsed once, so it can be
// resolved on many days (relativeLinePoints) without parsing it again.
type relativeDateToken struct {
	name string
	arg  int
}

// parseRelativeDate parses a token relativeDate resolves; false for one it
// does not.
func parseRelativeDate(token string) (relativeDateToken, bool) {
	m := relativeDateParts.FindStringSubmatch(token)
	if m == nil {
		return relativeDateToken{}, false
	}
	name, arg, hasArg := m[1], 0, m[2] != ""
	if hasArg {
		n, err := strconv.Atoi(m[2])
		if err != nil || n > maxRelativeDateOffset || n < -maxRelativeDateOffset {
			return relativeDateToken{}, false
		}
		arg = n
	}
	switch name {
	case "today":
		if hasArg {
			return relativeDateToken{}, false
		}
	case "daysAgo":
		if !hasArg || arg < 0 {
			return relativeDateToken{}, false
		}
	case "startOfMonth", "endOfMonth", "startOfQuarter", "endOfQuarter",
		"startOfWeek", "endOfWeek", "startOfYear", "endOfYear":
		if !hasArg {
			return relativeDateToken{}, false
		}
	default:
		return relativeDateToken{}, false
	}
	return relativeDateToken{name: name, arg: arg}, true
}

// on is the day the token names, counted from today.
func (r relativeDateToken) on(today time.Time) time.Time {
	arg := r.arg
	switch r.name {
	case "daysAgo":
		return today.AddDate(0, 0, -arg)
	case "startOfMonth", "endOfMonth":
		monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
		if r.name == "startOfMonth" {
			return monthStart.AddDate(0, arg, 0)
		}
		return monthStart.AddDate(0, arg+1, -1)
	case "startOfQuarter", "endOfQuarter":
		quarterStart := time.Date(today.Year(), today.Month()-(today.Month()-1)%3, 1, 0, 0, 0, 0, time.UTC)
		if r.name == "startOfQuarter" {
			return quarterStart.AddDate(0, 3*arg, 0)
		}
		return quarterStart.AddDate(0, 3*(arg+1), -1)
	case "startOfWeek", "endOfWeek":
		// time.Weekday: Sunday is 0, so Sunday is 6 days after Monday.
		weekStart := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
		if r.name == "startOfWeek" {
			return weekStart.AddDate(0, 0, 7*arg)
		}
		return weekStart.AddDate(0, 0, 7*arg+6)
	case "startOfYear", "endOfYear":
		yearStart := time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		if r.name == "startOfYear" {
			return yearStart.AddDate(arg, 0, 0)
		}
		return yearStart.AddDate(arg+1, 0, -1)
	}
	return today // "today"
}

// maxRelativeDateOffset bounds a relative date token's argument, as
// entity-service's resolveRelativeDate and the webapp's resolver do. Without
// it a huge argument overflows the date arithmetic and wraps to a date that
// looks valid (__startOfYear:9223372036854775807__ gave 2024-12-31). Every
// argument beyond it lands outside the years 0000-9999, which entity-service
// rejects, so the bound refuses nothing that used to resolve there.
const maxRelativeDateOffset = 4_000_000

// relativeDateParts splits a generatedRelativeDate token into its name and
// optional argument.
var relativeDateParts = regexp.MustCompile(`^__([a-zA-Z]+)(?::(-?\d+))?__$`)

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
