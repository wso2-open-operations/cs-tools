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

// Package playbooks writes the catalogue's PLG playbook answers section from
// the answer fields a request carries. The service holds no playbooks: PLG
// admins author them, the CSM portal backend reads the active ones with each
// generate request and sends their single choice and checklist tasks, and it
// checks every task and answer code the model writes against the same list.
package playbooks

import (
	"fmt"
	"strings"
)

// Field is one playbook task whose answers a PLG widget can filter or group
// by, as the model is told about it.
type Field struct {
	Code      string   `json:"code"`      // what playbookTask names, e.g. "ELIGIBILITY_ASSESSMENT_OUTCOME"
	Name      string   `json:"name"`      // display name, e.g. "Eligibility Assessment Outcome"
	ValueType string   `json:"valueType"` // "SINGLE_SELECT" or "CHECKLIST"
	Options   []Option `json:"options"`
}

// Option is one answer a task offers.
type Option struct {
	Code  string `json:"code"`  // what playbookAnswers names, e.g. "MONITOR"
	Label string `json:"label"` // display label, e.g. "Monitor"
}

var typeLabels = map[string]string{
	"SINGLE_SELECT": "one answer",
	"CHECKLIST":     "any number of answers",
}

// Section is the PLG playbook answers section for fields, or "" when there
// are none (a request without fields leaves the catalogue as it is).
func Section(fields []Field) string {
	var rows strings.Builder
	for _, f := range fields {
		answers := make([]string, len(f.Options))
		for i, o := range f.Options {
			answers[i] = fmt.Sprintf("`%s` %s", o.Code, o.Label)
		}
		fmt.Fprintf(&rows, "| %s | `%s` | %s | %s |\n", f.Name, f.Code, typeLabels[f.ValueType], strings.Join(answers, ", "))
	}
	if rows.Len() == 0 {
		return ""
	}
	return "## PLG playbook answers\n\n" +
		"These are the only playbook tasks a PLG widget can filter or group by. A task is named by its\n" +
		"**code** in `playbookTask`, and its answers by their **codes** in `playbookAnswers`; never write\n" +
		"a name or a label as a code.\n\n" +
		"| Task | Code | Answers per registration | Answers (code, label) |\n|---|---|---|---|\n" + rows.String()
}

// WithPlaybookAnswers is the catalogue followed by the PLG playbook answers
// section, or the catalogue as it is when fields is empty.
func WithPlaybookAnswers(catalogue string, fields []Field) string {
	section := Section(fields)
	if section == "" {
		return catalogue
	}
	return strings.TrimRight(catalogue, "\n") + "\n\n---\n\n" + section
}
