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

package playbooks

import "testing"

var fields = []Field{
	{Code: "ELIGIBILITY_ASSESSMENT_OUTCOME", Name: "Eligibility Assessment Outcome", ValueType: "SINGLE_SELECT",
		Options: []Option{{"ELIGIBLE", "Eligible"}, {"MONITOR", "Monitor"}, {"EXCLUDED", "Excluded"}}},
	{Code: "DISQUALIFY_REASONS", Name: "Disqualify Reasons", ValueType: "CHECKLIST",
		Options: []Option{{"NO_ACTIVITY", "No activity"}, {"PERSONAL_EMAIL", "Personal email"}}},
}

const wantSection = "## PLG playbook answers\n\n" +
	"These are the only playbook tasks a PLG widget can filter or group by. A task is named by its\n" +
	"**code** in `playbookTask`, and its answers by their **codes** in `playbookAnswers`; never write\n" +
	"a name or a label as a code.\n\n" +
	"| Task | Code | Answers per registration | Answers (code, label) |\n|---|---|---|---|\n" +
	"| Eligibility Assessment Outcome | `ELIGIBILITY_ASSESSMENT_OUTCOME` | one answer | `ELIGIBLE` Eligible, `MONITOR` Monitor, `EXCLUDED` Excluded |\n" +
	"| Disqualify Reasons | `DISQUALIFY_REASONS` | any number of answers | `NO_ACTIVITY` No activity, `PERSONAL_EMAIL` Personal email |\n"

func TestSectionListsEveryTaskAndAnswer(t *testing.T) {
	if got := Section(fields); got != wantSection {
		t.Errorf("Section() =\n%s\nwant\n%s", got, wantSection)
	}
}

func TestNoFieldsIsNoSection(t *testing.T) {
	if got := Section(nil); got != "" {
		t.Errorf("Section(nil) = %q, want empty", got)
	}
	if got := WithPlaybookAnswers("catalogue\n", nil); got != "catalogue\n" {
		t.Errorf("WithPlaybookAnswers(no fields) = %q, want the catalogue unchanged", got)
	}
}

func TestWithPlaybookAnswersAppendsAfterASeparator(t *testing.T) {
	got := WithPlaybookAnswers("catalogue\n\n\n", fields)
	if want := "catalogue\n\n---\n\n" + wantSection; got != want {
		t.Errorf("WithPlaybookAnswers() =\n%q\nwant\n%q", got, want)
	}
}
