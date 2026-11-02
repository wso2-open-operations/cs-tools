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
	"strings"
	"testing"
)

func opsDashboard(widgets ...string) string {
	return `{"displayName":"Operations","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

func TestParseGenerated_AcceptsOperationsWidgets(t *testing.T) {
	raw := opsDashboard(
		`{"id":"p1-open","displayName":"Critical Open Incidents","resourceType":"incident","shape":"count","gridWidth":3,
		  "query":{"priorities":["CRITICAL"],"filters":[{"field":"state","op":"in","values":["NEW","IN_PROGRESS","ON_HOLD"]},
		  {"field":"createdOn","op":"gte","values":["__daysAgo:30__"]},{"field":"slaViolated","op":"eq","values":["true"]}]}}`,
		`{"id":"my-incidents","displayName":"My Incidents","resourceType":"incident","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"openedOn","order":"desc"},"query":{"filters":[{"field":"assignedUserId","op":"in","values":["__current_user__"]}]}}`,
		`{"id":"incidents-by-state","displayName":"Incidents by State","resourceType":"incident","shape":"pie","gridWidth":6,
		  "query":{},"groupBy":{"field":"state","maxGroups":8}}`,
		`{"id":"crs","displayName":"Change Requests Awaiting Approval","resourceType":"change_request","shape":"count","gridWidth":3,
		  "query":{"states":["assess","authorize"],"impacts":["high"],"filters":[{"field":"approval","op":"eq","values":["requested"]},
		  {"field":"createdOn","op":"gte","values":["__startOfMonth:0__"]}]}}`,
		`{"id":"crs-by-state","displayName":"Change Requests by State","resourceType":"change_request","shape":"bar","gridWidth":6,
		  "query":{},"slices":[{"label":"New","query":{"states":["new"]}},{"label":"Scheduled","query":{"states":["scheduled"]}}]}`,
		`{"id":"problems","displayName":"Open Problems","resourceType":"problem","shape":"list","gridWidth":6,"listLimit":5,
		  "query":{"filters":[{"field":"state","op":"in","values":["NEW","ASSESS","ROOT_CAUSE_ANALYSIS","FIX_IN_PROGRESS"]}]}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if len(d.Widgets) != 6 {
		t.Fatalf("widgets = %d, want 6", len(d.Widgets))
	}
}

func TestParseGenerated_RejectsOperationsWidgetsItCannotRun(t *testing.T) {
	inc := func(shape, rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"incident","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cr := func(rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"change_request","shape":"count","gridWidth":3` + rest + `}`)
	}
	pr := func(shape, rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"problem","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"incident line", inc("line", `,"query":{},"days":30`), "shape must be count, list, pie or bar"},
		{"incident by assignment group", inc("bar", `,"query":{},"groupBy":{"field":"assignmentGroup","maxGroups":10}`), "groups by state only"},
		{"incident lower-case state", inc("count", `,"query":{"filters":[{"field":"state","op":"in","values":["new"]}]}`), `"new" is not a incident state`},
		{"incident lower-case priority", inc("count", `,"query":{"priorities":["critical"]}`), `"critical" is not a incident priority`},
		{"incident by group id", inc("count", `,"query":{"filters":[{"field":"assignmentGroupId","op":"in","values":["5e636a12-0000-4000-8000-000000000001"]}]}`), `field "assignmentGroupId" is not a incident filter field`},
		{"incident someone else", inc("count", `,"query":{"filters":[{"field":"assignedUserId","op":"in","values":["5e636a12-0000-4000-8000-000000000001"]}]}`), `can only be ["__current_user__"]`},
		{"incident wrong op", inc("count", `,"query":{"filters":[{"field":"createdOn","op":"in","values":["__today__"]}]}`), `field "createdOn" takes op gte or lte`},
		{"incident bad date", inc("count", `,"query":{"filters":[{"field":"createdOn","op":"gte","values":["last week"]}]}`), "must be one date or a relative date"},
		{"incident sla not boolean", inc("count", `,"query":{"filters":[{"field":"slaViolated","op":"eq","values":["yes"]}]}`), `exactly one of "true", "false"`},
		{"incident unknown key", inc("count", `,"query":{"states":["NEW"]}`), `"states" is not a incident filter (filters, priorities)`},
		{"incident list sort", inc("list", `,"query":{},"listLimit":5,"sortBy":{"field":"severity","order":"desc"}`), "sorts by createdOn, updatedOn or openedOn"},
		{"change request state as a field", cr(`,"query":{"filters":[{"field":"state","op":"in","values":["new"]}]}`), `field "state" is not a change request filter field`},
		{"change request upper-case state", cr(`,"query":{"states":["NEW"]}`), `"NEW" is not a change request state`},
		{"change request two approvals", cr(`,"query":{"filters":[{"field":"approval","op":"eq","values":["requested","approved"]}]}`), "must be exactly one of not requested"},
		{"change request closed dates", cr(`,"query":{"closedStartDate":"2026-01-01T00:00:00Z"}`), `"closedStartDate" is not a change request filter`},
		{"problem sorted list", pr("list", `,"query":{},"listLimit":5,"sortBy":{"field":"createdOn","order":"desc"}`), "a problem list takes no sortBy"},
		{"problem slice lower-case", pr("pie", `,"query":{},"slices":[{"label":"New","query":{"filters":[{"field":"state","op":"in","values":["new"]}]}}]`), `slices[0].query.filters[0].values: "new" is not a problem state`},
		{"problem anyOf", pr("count", `,"query":{"anyOf":[]}`), `"anyOf" is not a problem filter (filters)`},
		{"problem filter entry with extra key", pr("count", `,"query":{"filters":[{"field":"state","op":"in","values":["NEW"],"label":"x"}]}`), `unknown key "label"`},
	}
	for _, c := range cases {
		_, err := ParseGenerated([]byte(c.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", c.name, err, c.want)
		}
	}
}

func TestParseGenerated_AcceptsAccountAndProjectWidgets(t *testing.T) {
	raw := opsDashboard(
		`{"id":"customers","displayName":"Customer Accounts","resourceType":"account","shape":"count","gridWidth":3,
		  "query":{"lifeCycle":"Customer","hasProject":true}}`,
		`{"id":"rigel-risk","displayName":"Rigel: Accounts at Risk","resourceType":"account","shape":"list","gridWidth":6,"listLimit":5,
		  "query":{"abtTeam":"rigel","healthStatus":"at_risk"}}`,
		`{"id":"risks","displayName":"Accounts by Risk","resourceType":"account","shape":"bar","gridWidth":6,"query":{"active":true},
		  "slices":[{"label":"EOL product","query":{"risks":["hasEolProduct"]}},{"label":"No go-live","query":{"risks":["hasNoGoLive"]}}]}`,
		`{"id":"onboarding","displayName":"Projects in Onboarding","resourceType":"project","shape":"count","gridWidth":3,
		  "query":{"onboardingStatus":["In-Progress"],"excludeClosureStates":["Restricted","Suspended"]}}`,
		`{"id":"ending","displayName":"Projects Ending This Quarter","resourceType":"project","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"endDate","order":"asc"},"query":{"endDateFrom":"__startOfQuarter:0__","endDateTo":"__endOfQuarter:0__","closureStatus":"Open"}}`,
	)
	if _, err := ParseGenerated([]byte(raw)); err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
}

func TestParseGenerated_RejectsAccountAndProjectWidgetsItCannotRun(t *testing.T) {
	acc := func(shape, rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"account","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	proj := func(shape, rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"project","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"account lower-case life cycle", acc("count", `,"query":{"lifeCycle":"customer"}`), "lifeCycle must be one of Customer"},
		{"account life cycle as a list", acc("count", `,"query":{"lifeCycle":["Customer"]}`), "lifeCycle must be one of Customer"},
		{"account boolean as a string", acc("count", `,"query":{"hasProject":"true"}`), "hasProject must be true or false"},
		{"account unknown risk", acc("count", `,"query":{"risks":["hasNoCases"]}`), `"hasNoCases" is not a account risk`},
		{"account by owner", acc("count", `,"query":{"ownerEmail":"jane.doe@example.com"}`), `"ownerEmail" is not a account filter`},
		{"account by sub-region", acc("count", `,"query":{"subRegion":["ASIA"]}`), `"subRegion" is not a account filter`},
		{"account blank team", acc("count", `,"query":{"abtTeam":" "}`), "abtTeam must be a team's key"},
		{"account group by", acc("bar", `,"query":{},"groupBy":{"field":"lifeCycle","maxGroups":5}`), "cannot use groupBy"},
		{"account sorted list", acc("list", `,"query":{},"listLimit":5,"sortBy":{"field":"name","order":"asc"}`), "a account list takes no sortBy"},
		{"project upper-case status", proj("count", `,"query":{"onboardingStatus":["IN_PROGRESS"]}`), `"IN_PROGRESS" is not a project onboarding status`},
		{"project by account", proj("count", `,"query":{"accountId":"5e636a12-0000-4000-8000-000000000001"}`), `"accountId" is not a project filter`},
		{"project bad end date", proj("count", `,"query":{"endDateTo":"next month"}`), "endDateTo must be a date"},
		{"project closure as a list", proj("count", `,"query":{"closureStatus":["Open"]}`), "closureStatus must be one of Open"},
		{"project other sort", proj("list", `,"query":{},"listLimit":5,"sortBy":{"field":"name","order":"asc"}`), "a project list sorts by endDate"},
	}
	for _, c := range cases {
		_, err := ParseGenerated([]byte(c.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", c.name, err, c.want)
		}
	}
}

func TestParseGenerated_AcceptsCaseFeedbackWidgets(t *testing.T) {
	raw := opsDashboard(
		`{"id":"responses","displayName":"Feedback Responses This Quarter","resourceType":"case_feedback","shape":"count","gridWidth":3,
		  "query":{"dateFrom":"2026-07-01","dateTo":"2026-09-30"}}`,
		`{"id":"trend","displayName":"CSAT by Month","resourceType":"case_feedback","shape":"bar","gridWidth":6,
		  "query":{},"groupBy":{"bucket":"month"}}`,
		`{"id":"by-rating","displayName":"Feedback by Rating","resourceType":"case_feedback","shape":"pie","gridWidth":6,
		  "query":{},"groupBy":{"bucket":"rating"}}`,
		`{"id":"why","displayName":"Why Customers Were Dissatisfied","resourceType":"case_feedback","shape":"bar","gridWidth":6,
		  "query":{},"groupBy":{"bucket":"reasons_dissatisfied"}}`,
		`{"id":"very-dissatisfied","displayName":"Very Dissatisfied Feedback","resourceType":"case_feedback","shape":"list","gridWidth":6,
		  "listLimit":5,"query":{"rating":1}}`,
		`{"id":"split","displayName":"Low vs High Ratings","resourceType":"case_feedback","shape":"pie","gridWidth":6,"query":{},
		  "slices":[{"label":"Very dissatisfied","query":{"rating":1}},{"label":"Very satisfied","query":{"rating":5}}]}`,
	)
	if _, err := ParseGenerated([]byte(raw)); err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
}

func TestParseGenerated_RejectsCaseFeedbackWidgetsItCannotRun(t *testing.T) {
	fb := func(shape, rest string) string {
		return opsDashboard(`{"id":"w","displayName":"W","resourceType":"case_feedback","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"group by a field", fb("bar", `,"query":{},"groupBy":{"field":"account","maxGroups":10}`), "groups by bucket day, week, month, rating"},
		{"an unknown bucket", fb("bar", `,"query":{},"groupBy":{"bucket":"quarter"}`), "groups by bucket"},
		{"line", fb("line", `,"query":{},"days":30`), "shape must be count, list, pie or bar"},
		{"rating as a string", fb("count", `,"query":{"rating":"5"}`), "rating must be a whole number from 1 to 5"},
		{"rating out of range", fb("count", `,"query":{"rating":6}`), "rating must be a whole number from 1 to 5"},
		{"by account", fb("count", `,"query":{"accountIds":["5e636a12-0000-4000-8000-000000000001"]}`), `"accountIds" is not a case feedback filter`},
		{"a bad date", fb("count", `,"query":{"dateFrom":"last week"}`), "dateFrom must be a date"},
		{"the date picker's placeholder", fb("count", `,"query":{"dateFrom":"__dateRangeFrom__"}`), "dateFrom must be a date"},
		{"a relative date (not filled in for feedback yet)", fb("count", `,"query":{"dateFrom":"__daysAgo:90__"}`), "takes a plain date (YYYY-MM-DD), not a relative date"},
		{"a sorted list", fb("list", `,"query":{},"listLimit":5,"sortBy":{"field":"submittedAt","order":"desc"}`), "a case feedback list takes no sortBy"},
	}
	for _, c := range cases {
		_, err := ParseGenerated([]byte(c.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", c.name, err, c.want)
		}
	}
}
