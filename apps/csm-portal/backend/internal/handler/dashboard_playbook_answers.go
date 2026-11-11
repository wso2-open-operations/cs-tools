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
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/csmai"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// errDashboardPlaybook marks a generated dashboard whose PLG playbook answer
// filter names a task or an answer no active playbook has. The handler
// answers it as it does errDashboardTeam: 422, with the reason.
var errDashboardPlaybook = errors.New("invalid playbook answer filter")

// playbookAnswerFields are the tasks a PLG widget can filter or group by
// answer: the active, non-bookend single choice and checklist tasks of the
// active playbooks, one per code (several products' playbooks ask the same
// task), sorted by code. A code's name and type are the first playbook's
// (in the list's order); its answers every playbook's, first seen first. A
// code holding a character the AI service's table cannot hold, or longer than
// maxPlaybookCode, is left out, and such a character in a name or label
// becomes a space and a name or label is cut at maxPlaybookText, so an
// admin's wording never fails a generate request or grows the prompt. For the same reason the list stops at the AI
// service's caps: the first maxPlaybookAnswerFields codes, each with its
// first maxPlaybookAnswerOptions answers; a filter on one left out is then
// refused by checkPlaybookAnswers (a 422 naming the known ones), not by the
// AI service.
func playbookAnswerFields(playbooks []plgdomain.Playbook) []csmai.PlaybookAnswerField {
	byCode := map[string]*csmai.PlaybookAnswerField{}
	var codes []string
	for _, pb := range playbooks {
		if !pb.Active {
			continue
		}
		for _, t := range pb.Tasks {
			if !t.Active || t.IsBookend || (t.ValueType != plgdomain.ValueSingleSelect && t.ValueType != plgdomain.ValueChecklist) {
				continue
			}
			code := strings.TrimSpace(t.Code)
			if code == "" || !fitsTable(code) {
				continue
			}
			f, ok := byCode[code]
			if !ok {
				f = &csmai.PlaybookAnswerField{Code: code, Name: tableText(t.Name, code), ValueType: string(t.ValueType)}
				byCode[code] = f
				codes = append(codes, code)
			}
			for _, o := range t.Options {
				oc := strings.TrimSpace(o.Code)
				if oc == "" || !fitsTable(oc) || hasOption(*f, oc) || len(f.Options) >= maxPlaybookAnswerOptions {
					continue
				}
				f.Options = append(f.Options, csmai.PlaybookAnswerOption{Code: oc, Label: tableText(o.Label, oc)})
			}
		}
	}
	sort.Strings(codes)
	fields := make([]csmai.PlaybookAnswerField, 0, len(codes))
	for _, c := range codes {
		if len(byCode[c].Options) > 0 && len(fields) < maxPlaybookAnswerFields {
			fields = append(fields, *byCode[c])
		}
	}
	return fields
}

// maxPlaybookAnswerFields and maxPlaybookAnswerOptions are the AI service's
// caps on the playbook answers of one request (maxPlaybookAnswers and
// maxPlaybookOptions in its handler); it refuses a request over either.
const (
	maxPlaybookAnswerFields  = 200
	maxPlaybookAnswerOptions = 100
)

// tableBreakers are the characters the AI service refuses in a playbook
// answer field (they would break its markdown table).
const tableBreakers = "|\n\r`"

// maxPlaybookCode and maxPlaybookText bound what one task or answer adds to
// the model's prompt: a code is matched as it is, so a longer one is left
// out; a name or label is only read, so a longer one is cut.
const (
	maxPlaybookCode = 100
	maxPlaybookText = 100
)

// fitsTable reports whether a task or answer code can be sent as it is.
func fitsTable(code string) bool {
	return !strings.ContainsAny(code, tableBreakers) && utf8.RuneCountInString(code) <= maxPlaybookCode
}

// tableText is s with every tableBreakers character a space, trimmed, cut at
// maxPlaybookText characters (ending in "…"), or fallback when that leaves
// nothing.
func tableText(s, fallback string) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if strings.ContainsRune(tableBreakers, r) {
			return ' '
		}
		return r
	}, s))
	if r := []rune(s); len(r) > maxPlaybookText {
		s = strings.TrimSpace(string(r[:maxPlaybookText-1])) + "…"
	}
	if s == "" {
		return fallback
	}
	return s
}

func hasOption(f csmai.PlaybookAnswerField, code string) bool {
	for _, o := range f.Options {
		if o.Code == code {
			return true
		}
	}
	return false
}

// checkPlaybookAnswers checks every PLG widget's playbook answer filter
// against fields: its playbookTask (a slice's own, or the widget's) names a
// field, and each of its playbookAnswers is one of that field's answers.
func checkPlaybookAnswers(d dashboard.Dashboard, fields []csmai.PlaybookAnswerField) error {
	byCode := make(map[string]csmai.PlaybookAnswerField, len(fields))
	for _, f := range fields {
		byCode[f.Code] = f
	}
	for i, w := range d.Widgets {
		if w.ResourceType != dashboard.ResourcePlgOrganization && w.ResourceType != dashboard.ResourcePlgRegistration {
			continue
		}
		where := fmt.Sprintf("widgets[%d] (%q)", i, w.ID)
		if err := checkPlaybookQuery(where, w.Query, nil, byCode, fields); err != nil {
			return err
		}
		for j, sl := range w.Slices {
			if err := checkPlaybookQuery(fmt.Sprintf("%s.slices[%d]", where, j), sl.Query, w.Query, byCode, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkPlaybookQuery checks one query's playbook answer filter; a slice's
// query takes the widget's (base) task, and the widget's answers, when it
// names none of its own, as the webapp merges a slice over its widget key by
// key. So a slice that names another task under the widget's answers is
// checked against its own task, not left to count nothing.
func checkPlaybookQuery(where string, query, base map[string]any, byCode map[string]csmai.PlaybookAnswerField, fields []csmai.PlaybookAnswerField) error {
	task, _ := query["playbookTask"].(string)
	if _, own := query["playbookTask"]; !own {
		task, _ = base["playbookTask"].(string)
	}
	task = strings.TrimSpace(task)
	if task == "" {
		return nil
	}
	f, ok := byCode[task]
	if !ok {
		known := make([]string, len(fields))
		for i, f := range fields {
			known[i] = f.Code
		}
		if len(known) == 0 {
			return fmt.Errorf("%w: %s: unknown playbook task %q (no playbook task can be filtered by now)", errDashboardPlaybook, where, task)
		}
		return fmt.Errorf("%w: %s: unknown playbook task %q (known tasks: %s)", errDashboardPlaybook, where, task, strings.Join(known, ", "))
	}
	answers, _ := query["playbookAnswers"].([]any)
	inherited := ""
	if _, own := query["playbookAnswers"]; !own {
		answers, _ = base["playbookAnswers"].([]any)
		inherited = " (the widget's playbookAnswers)"
	}
	for _, a := range answers {
		code, _ := a.(string)
		if !hasOption(f, strings.TrimSpace(code)) {
			names := make([]string, len(f.Options))
			for i, o := range f.Options {
				names[i] = o.Code
			}
			return fmt.Errorf("%w: %s: %q%s is not an answer to %s (answers: %s)", errDashboardPlaybook, where, code, inherited, task, strings.Join(names, ", "))
		}
	}
	return nil
}
