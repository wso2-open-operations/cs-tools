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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

func answerTask(code, name string, valueType plgdomain.TaskValueType, options ...string) plgdomain.PlaybookTask {
	t := plgdomain.PlaybookTask{Code: code, Name: name, ValueType: valueType, Active: true}
	for i := 0; i+1 < len(options); i += 2 {
		t.Options = append(t.Options, plgdomain.ChecklistOption{Code: options[i], Label: options[i+1]})
	}
	return t
}

// testPlaybooks: IAM's and API Platform's validation playbooks both ask the
// outcome (API Platform's with one more answer), and only IAM's the email
// classification; an inactive playbook, an inactive task, a bookend, a
// number or text task and a task with no answers are not answer fields.
var testPlaybooks = []plgdomain.Playbook{
	{Name: "IAM validation", Active: true, Tasks: []plgdomain.PlaybookTask{
		{Code: "INITIATE_PLAYBOOK", Name: "Initiate", ValueType: plgdomain.ValueBoolean, Active: true, IsBookend: true},
		answerTask("REGISTERED_EMAIL_CLASSIFICATION", "Registered Email Classification", plgdomain.ValueSingleSelect,
			"CORPORATE", "Corporate", "DISPOSABLE", "Disposable"),
		answerTask("ELIGIBILITY_ASSESSMENT_OUTCOME", "Eligibility Assessment Outcome", plgdomain.ValueSingleSelect,
			"ELIGIBLE", "Eligible", "MONITOR", "Monitor"),
		answerTask("INITIAL_SESSION_DURATION_MINUTES", "Initial Session Duration", plgdomain.ValueNumber),
		// Never in entity-service (only choice tasks carry options), but the
		// AI service would refuse a field of any other type.
		answerTask("ONBOARDING_EXIT_POINT", "Onboarding Exit Point", plgdomain.ValueString, "A", "A", "B", "B"),
	}},
	{Name: "API Platform validation", Active: true, Tasks: []plgdomain.PlaybookTask{
		answerTask("ELIGIBILITY_ASSESSMENT_OUTCOME", "Outcome (API)", plgdomain.ValueSingleSelect,
			"MONITOR", "Monitor", "EXCLUDED", "Excluded"),
		{Code: "OLD_REASONS", Name: "Old reasons", ValueType: plgdomain.ValueChecklist, Active: false,
			Options: []plgdomain.ChecklistOption{{Code: "A", Label: "A"}, {Code: "B", Label: "B"}}},
		answerTask("DISQUALIFY_REASONS", "Disqualify | reasons", plgdomain.ValueChecklist,
			"NO_ACTIVITY", "No\nactivity", "BAD|CODE", "Bad", "PERSONAL_EMAIL", "`Personal` email"),
		answerTask("EMPTY_CHOICE", "Empty", plgdomain.ValueSingleSelect),
	}},
	{Name: "Retired", Active: false, Tasks: []plgdomain.PlaybookTask{
		answerTask("RETIRED_TASK", "Retired", plgdomain.ValueSingleSelect, "X", "X", "Y", "Y"),
	}},
}

func TestPlaybookAnswerFields_TheActiveChoiceTasksOncePerCode(t *testing.T) {
	want := []csmai.PlaybookAnswerField{
		{Code: "DISQUALIFY_REASONS", Name: "Disqualify   reasons", ValueType: "CHECKLIST", Options: []csmai.PlaybookAnswerOption{
			{Code: "NO_ACTIVITY", Label: "No activity"}, {Code: "PERSONAL_EMAIL", Label: "Personal  email"}}},
		{Code: "ELIGIBILITY_ASSESSMENT_OUTCOME", Name: "Eligibility Assessment Outcome", ValueType: "SINGLE_SELECT", Options: []csmai.PlaybookAnswerOption{
			{Code: "ELIGIBLE", Label: "Eligible"}, {Code: "MONITOR", Label: "Monitor"}, {Code: "EXCLUDED", Label: "Excluded"}}},
		{Code: "REGISTERED_EMAIL_CLASSIFICATION", Name: "Registered Email Classification", ValueType: "SINGLE_SELECT", Options: []csmai.PlaybookAnswerOption{
			{Code: "CORPORATE", Label: "Corporate"}, {Code: "DISPOSABLE", Label: "Disposable"}}},
	}
	if got := playbookAnswerFields(testPlaybooks); !reflect.DeepEqual(got, want) {
		t.Errorf("playbookAnswerFields =\n%+v\nwant\n%+v", got, want)
	}
	if got := playbookAnswerFields(nil); len(got) != 0 {
		t.Errorf("playbookAnswerFields(nil) = %+v, want none", got)
	}
}

// TestPlaybookAnswerFields_StopAtTheAIServicesCaps: 201 tasks of 101
// answers each send the first 200 codes with their first 100 answers, and a
// filter on a code or an answer left out is refused here (a 422), not by the
// AI service.
func TestPlaybookAnswerFields_StopAtTheAIServicesCaps(t *testing.T) {
	var tasks []plgdomain.PlaybookTask
	for i := 0; i < 201; i++ {
		var options []string
		for j := 0; j < 101; j++ {
			options = append(options, fmt.Sprintf("A%03d", j), fmt.Sprintf("Answer %d", j))
		}
		tasks = append(tasks, answerTask(fmt.Sprintf("T%03d", i), fmt.Sprintf("Task %d", i), plgdomain.ValueSingleSelect, options...))
	}
	fields := playbookAnswerFields([]plgdomain.Playbook{{Name: "Big", Active: true, Tasks: tasks}})
	if len(fields) != 200 || fields[0].Code != "T000" || fields[199].Code != "T199" {
		t.Fatalf("got %d fields (%s to %s), want T000 to T199", len(fields), fields[0].Code, fields[len(fields)-1].Code)
	}
	for _, f := range fields {
		if len(f.Options) != 100 || f.Options[99].Code != "A099" {
			t.Fatalf("%s: %d answers, want A000 to A099", f.Code, len(f.Options))
		}
	}

	for _, c := range []struct{ name, query string }{
		{"the 201st task", `{"playbookTask":"T200","playbookAnswers":["A000"]}`},
		{"the 101st answer", `{"playbookTask":"T000","playbookAnswers":["A100"]}`},
	} {
		d, err := dashboard.ParseGenerated([]byte(plgGenerated(
			`{"id":"w","displayName":"W","resourceType":"plg_registration","shape":"count","gridWidth":6,"query":` + c.query + `}`)))
		if err != nil {
			t.Fatalf("%s: ParseGenerated: %v", c.name, err)
		}
		if err := checkPlaybookAnswers(d, fields); !errors.Is(err, errDashboardPlaybook) {
			t.Errorf("%s: checkPlaybookAnswers = %v, want errDashboardPlaybook", c.name, err)
		}
	}
	d, err := dashboard.ParseGenerated([]byte(plgGenerated(
		`{"id":"w","displayName":"W","resourceType":"plg_registration","shape":"count","gridWidth":6,"query":{"playbookTask":"T199","playbookAnswers":["A099"]}}`)))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if err := checkPlaybookAnswers(d, fields); err != nil {
		t.Errorf("the last task and answer kept: %v", err)
	}
}

func plgGenerated(widgets ...string) string {
	return `{"displayName":"PLG","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

func TestCheckPlaybookAnswers(t *testing.T) {
	fields := playbookAnswerFields(testPlaybooks)
	reg := func(rest string) string {
		return `{"id":"w","displayName":"W","resourceType":"plg_registration","gridWidth":6,` + rest + `}`
	}
	for _, c := range []struct {
		name, widget, want string // want "" for accepted
	}{
		{"an answer", reg(`"shape":"count","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["EXCLUDED"]}`), ""},
		{"a grouping", reg(`"shape":"bar","query":{"playbookTask":"REGISTERED_EMAIL_CLASSIFICATION"},"groupBy":{"field":"playbookAnswer"}`), ""},
		{"not answered", reg(`"shape":"count","query":{"playbookTask":"DISQUALIFY_REASONS","playbookUnanswered":true}`), ""},
		{"slices under the widget's task", reg(`"shape":"pie","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"},
			"slices":[{"label":"M","query":{"playbookAnswers":["MONITOR"]}},{"label":"E","query":{"playbookAnswers":["ELIGIBLE"]}}]`), ""},
		{"slices with their own tasks", reg(`"shape":"bar","query":{},
			"slices":[{"label":"M","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["MONITOR"]}},
			          {"label":"D","query":{"playbookTask":"REGISTERED_EMAIL_CLASSIFICATION","playbookAnswers":["DISPOSABLE"]}}]`), ""},
		{"an unknown task", reg(`"shape":"count","query":{"playbookTask":"OUTCOME","playbookAnswers":["MONITOR"]}`),
			`unknown playbook task "OUTCOME" (known tasks: DISQUALIFY_REASONS, ELIGIBILITY_ASSESSMENT_OUTCOME, REGISTERED_EMAIL_CLASSIFICATION)`},
		{"a number task", reg(`"shape":"bar","query":{"playbookTask":"INITIAL_SESSION_DURATION_MINUTES"},"groupBy":{"field":"playbookAnswer"}`),
			`unknown playbook task "INITIAL_SESSION_DURATION_MINUTES"`},
		{"an inactive playbook's task", reg(`"shape":"count","query":{"playbookTask":"RETIRED_TASK","playbookAnswers":["X"]}`), `unknown playbook task "RETIRED_TASK"`},
		{"an unknown answer", reg(`"shape":"count","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["MONITOR","Excluded"]}`),
			`"Excluded" is not an answer to ELIGIBILITY_ASSESSMENT_OUTCOME (answers: ELIGIBLE, MONITOR, EXCLUDED)`},
		{"another task's answer in a slice", reg(`"shape":"pie","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"},
			"slices":[{"label":"M","query":{"playbookAnswers":["MONITOR"]}},{"label":"D","query":{"playbookAnswers":["DISPOSABLE"]}}]`),
			`slices[1]: "DISPOSABLE" is not an answer to ELIGIBILITY_ASSESSMENT_OUTCOME`},
		{"an organisation's", `{"id":"o","displayName":"O","resourceType":"plg_organization","shape":"count","gridWidth":6,
			"query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["MAYBE"]}}`, `"MAYBE" is not an answer`},
	} {
		d, err := dashboard.ParseGenerated([]byte(plgGenerated(c.widget)))
		if err != nil {
			t.Fatalf("%s: ParseGenerated: %v", c.name, err)
		}
		err = checkPlaybookAnswers(d, fields)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: err = %v, want accepted", c.name, err)
		case c.want != "" && (!errors.Is(err, errDashboardPlaybook) || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: err = %v, want errDashboardPlaybook mentioning %q", c.name, err, c.want)
		}
	}

	d, err := dashboard.ParseGenerated([]byte(plgGenerated(reg(`"shape":"count","query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["MONITOR"]}`))))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkPlaybookAnswers(d, nil); !errors.Is(err, errDashboardPlaybook) || !strings.Contains(err.Error(), "no playbook task can be filtered by now") {
		t.Errorf("with no fields: err = %v, want errDashboardPlaybook saying none can be", err)
	}
}

const outcomeWidget = `{"id":"by-outcome","displayName":"By Outcome","resourceType":"plg_registration","shape":"bar","gridWidth":6,
"query":{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"},"groupBy":{"field":"playbookAnswer"}}`

func TestServiceGenerator_SendsThePlaybookAnswersAndChecksTheReply(t *testing.T) {
	list := func(context.Context) ([]plgdomain.Playbook, error) { return testPlaybooks, nil }

	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(plgGenerated(outcomeWidget))}}}
	got, err := WithPlaybooks(NewServiceDashboardGenerator(ai, nil), list).Generate(context.Background(), "p", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !reflect.DeepEqual(ai.gotReq.PlaybookAnswers, playbookAnswerFields(testPlaybooks)) || got.Dashboard.Widgets[0].ID != "by-outcome" {
		t.Errorf("sent %+v, got %+v", ai.gotReq.PlaybookAnswers, got.Dashboard)
	}

	unknown := strings.Replace(outcomeWidget, "ELIGIBILITY_ASSESSMENT_OUTCOME", "OUTCOME", 1)
	ai = &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(plgGenerated(unknown))}}}
	if _, err := WithPlaybooks(NewServiceDashboardGenerator(ai, nil), list).Generate(context.Background(), "p", nil); !errors.Is(err, errDashboardPlaybook) {
		t.Errorf("an unknown task: err = %v, want errDashboardPlaybook", err)
	}
}

// When the playbooks cannot be read, the model is sent none, and a playbook
// answer filter in its reply is refused rather than saved unchecked.
func TestServiceGenerator_NoPlaybooksSendsNoneAndRefusesTheirFilters(t *testing.T) {
	failing := func(context.Context) ([]plgdomain.Playbook, error) { return nil, errors.New("entity-service down") }

	ai := &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(validGenerated)}}}
	if _, err := WithPlaybooks(NewServiceDashboardGenerator(ai, nil), failing).Generate(context.Background(), "p", nil); err != nil {
		t.Fatalf("a dashboard with no PLG answer filter: %v", err)
	}
	if len(ai.gotReq.PlaybookAnswers) != 0 {
		t.Errorf("sent %+v, want no playbook answers", ai.gotReq.PlaybookAnswers)
	}
	if b, _ := json.Marshal(ai.gotReq); strings.Contains(string(b), "playbookAnswers") {
		t.Errorf("request %s holds playbookAnswers; want it left out", b)
	}

	ai = &fakeAIClient{res: csmai.DashboardResult{Candidates: []json.RawMessage{json.RawMessage(plgGenerated(outcomeWidget))}}}
	if _, err := WithPlaybooks(NewServiceDashboardGenerator(ai, nil), failing).Generate(context.Background(), "p", nil); !errors.Is(err, errDashboardPlaybook) {
		t.Errorf("a playbook answer filter with no playbooks: err = %v, want errDashboardPlaybook", err)
	}
}
