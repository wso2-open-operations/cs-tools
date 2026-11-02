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

package service

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// This file holds the pure rules of a standard change template: what a parameter
// declaration may look like, what a proposal must contain before the CAB sees it,
// and how a version is rendered into a change request with the values it is raised
// with. No I/O, so every rule is unit-tested on its own.

const (
	maxStandardChangeParameters      = 30
	maxStandardChangeParameterLabel  = 120
	maxStandardChangeParameterHelp   = 500
	maxStandardChangeSelectOptions   = 50
	maxStandardChangeSelectOption    = 200
	maxStandardChangeTextValue       = 500
	maxStandardChangeTextareaValue   = 4000
	maxStandardChangeName            = 255
	maxStandardChangeRenderedSubject = 512 // work_item.subject
	maxStandardChangeRollbackText    = 255 // change_request.rollback_duration
	maxStandardChangeSuspendReason   = 1000
)

var (
	standardChangeParameterKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)
	// standardChangePlaceholder finds {{ key }} references; spaces inside the
	// braces are allowed. A reference whose key does not have the key shape is
	// not a placeholder at all and is left as text.
	standardChangePlaceholder = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]{0,49})\s*\}\}`)
)

var validStandardChangeParameterType = map[domain.StandardChangeParameterType]bool{
	domain.StandardChangeParameterTypeText:     true,
	domain.StandardChangeParameterTypeTextarea: true,
	domain.StandardChangeParameterTypeNumber:   true,
	domain.StandardChangeParameterTypeDate:     true,
	domain.StandardChangeParameterTypeSelect:   true,
}

// normalizeStandardChangeParameters trims a declaration list and checks it: keys
// unique and key-shaped, labels present, types known, options only (and always)
// on a select. It returns the cleaned list.
func normalizeStandardChangeParameters(in []domain.StandardChangeParameter) ([]domain.StandardChangeParameter, error) {
	if len(in) > maxStandardChangeParameters {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("a template can declare at most %d parameters", maxStandardChangeParameters)}
	}
	out := make([]domain.StandardChangeParameter, 0, len(in))
	seen := map[string]bool{}
	for i, p := range in {
		p.Key = strings.TrimSpace(p.Key)
		p.Label = strings.TrimSpace(p.Label)
		where := fmt.Sprintf("parameters[%d]", i)
		if !standardChangeParameterKey.MatchString(p.Key) {
			return nil, &apierror.ValidationError{Msg: where + ".key must start with a lower-case letter and hold only lower-case letters, digits and underscores (at most 50)"}
		}
		if seen[p.Key] {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q is declared twice", p.Key)}
		}
		seen[p.Key] = true
		if p.Label == "" || utf8.RuneCountInString(p.Label) > maxStandardChangeParameterLabel {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q needs a label of at most %d characters", p.Key, maxStandardChangeParameterLabel)}
		}
		if !validStandardChangeParameterType[p.Type] {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: type must be one of text, textarea, number, date or select", p.Key)}
		}
		if p.HelpText != nil {
			h := strings.TrimSpace(*p.HelpText)
			if h == "" {
				p.HelpText = nil
			} else if utf8.RuneCountInString(h) > maxStandardChangeParameterHelp {
				return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: help text must be at most %d characters", p.Key, maxStandardChangeParameterHelp)}
			} else {
				p.HelpText = &h
			}
		}
		if p.Type == domain.StandardChangeParameterTypeSelect {
			if len(p.Options) == 0 || len(p.Options) > maxStandardChangeSelectOptions {
				return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: a select needs 1 to %d options", p.Key, maxStandardChangeSelectOptions)}
			}
			opts := make([]string, 0, len(p.Options))
			seenOpt := map[string]bool{}
			for _, o := range p.Options {
				o = strings.TrimSpace(o)
				if o == "" || utf8.RuneCountInString(o) > maxStandardChangeSelectOption {
					return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: every option needs 1 to %d characters", p.Key, maxStandardChangeSelectOption)}
				}
				if seenOpt[o] {
					return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: option %q is listed twice", p.Key, o)}
				}
				seenOpt[o] = true
				opts = append(opts, o)
			}
			p.Options = opts
		} else if len(p.Options) > 0 {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("parameter %q: only a select takes options", p.Key)}
		}
		out = append(out, p)
	}
	return out, nil
}

// standardChangeTextFields lists the content's text fields with their API name,
// whether they are rich text (HTML), and a pointer to each.
func standardChangeTextFields(c *domain.StandardChangeContent) []struct {
	name string
	html bool
	v    **string
} {
	return []struct {
		name string
		html bool
		v    **string
	}{
		{"subject", false, &c.Subject},
		{"description", true, &c.Description},
		{"justification", true, &c.Justification},
		{"implementationPlan", true, &c.ImplementationPlan},
		{"riskImpactAnalysis", true, &c.RiskImpactAnalysis},
		{"backoutPlan", true, &c.BackoutPlan},
		{"testPlan", true, &c.TestPlan},
		{"rollbackDurationText", false, &c.RollbackDurationText},
	}
}

// standardChangePlaceholders returns the keys the content refers to, sorted.
func standardChangePlaceholders(c domain.StandardChangeContent) []string {
	seen := map[string]bool{}
	for _, f := range standardChangeTextFields(&c) {
		if *f.v == nil {
			continue
		}
		for _, m := range standardChangePlaceholder.FindAllStringSubmatch(**f.v, -1) {
			seen[m[1]] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func blankText(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

func blankRich(s *string) bool { return s == nil || repository.IsBlankRichText(*s) }

// checkStandardChangeProposalComplete is what a proposal must hold before the CAB
// is asked: everything a change request raised from it needs and cannot supply
// itself. A retirement needs only its rationale.
func checkStandardChangeProposalComplete(p domain.StandardChangeProposal) error {
	var missing []string
	if strings.TrimSpace(p.Name) == "" {
		missing = append(missing, "name")
	}
	if blankRich(p.Rationale) {
		missing = append(missing, "rationale")
	}
	// A new template belongs to one customer, which the proposal names. A
	// modification or retirement has its template's.
	if p.Kind == domain.StandardChangeProposalKindNew && (p.Customer == nil || strings.TrimSpace(p.Customer.ID) == "") {
		missing = append(missing, "customer (accountId)")
	}
	if p.Kind != domain.StandardChangeProposalKindRetire {
		c := p.StandardChangeContent
		if blankText(c.Subject) {
			missing = append(missing, "subject")
		}
		if blankRich(c.ImplementationPlan) {
			missing = append(missing, "implementationPlan")
		}
		if blankRich(c.BackoutPlan) {
			missing = append(missing, "backoutPlan")
		}
		if blankRich(c.TestPlan) {
			missing = append(missing, "testPlan")
		}
		if c.Impact == nil {
			missing = append(missing, "impact")
		}
		if c.Risk == nil {
			missing = append(missing, "risk")
		}
	}
	if len(missing) > 0 {
		return &apierror.ValidationError{Msg: "the proposal is not complete; it still needs: " + strings.Join(missing, ", ")}
	}
	if p.Kind == domain.StandardChangeProposalKindRetire {
		return nil
	}
	c := p.StandardChangeContent
	if *c.Risk == domain.ChangeRequestRiskHigh {
		return &apierror.ValidationError{Msg: "a standard change is pre-authorised, so it cannot be high risk: lower the risk, or raise this change as a normal change"}
	}
	if c.Subject != nil && utf8.RuneCountInString(*c.Subject) > maxStandardChangeName {
		return &apierror.ValidationError{Msg: fmt.Sprintf("subject must be at most %d characters", maxStandardChangeName)}
	}
	if c.RollbackDurationText != nil && utf8.RuneCountInString(*c.RollbackDurationText) > maxStandardChangeRollbackText {
		return &apierror.ValidationError{Msg: fmt.Sprintf("rollbackDurationText must be at most %d characters", maxStandardChangeRollbackText)}
	}
	params, err := normalizeStandardChangeParameters(c.Parameters)
	if err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, p := range params {
		declared[p.Key] = true
	}
	var unknown []string
	for _, k := range standardChangePlaceholders(c) {
		if !declared[k] {
			unknown = append(unknown, "{{"+k+"}}")
		}
	}
	if len(unknown) > 0 {
		return &apierror.ValidationError{Msg: "the template refers to parameters it does not declare: " + strings.Join(unknown, ", ")}
	}
	return nil
}

// normalizeStandardChangeValue checks one value against its declaration and
// returns it as stored ("" when an optional value is not given).
func normalizeStandardChangeValue(p domain.StandardChangeParameter, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		if p.Required {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) is required", p.Label, p.Key)}
		}
		return "", nil
	}
	switch p.Type {
	case domain.StandardChangeParameterTypeText:
		if strings.ContainsAny(v, "\r\n") {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be a single line", p.Label, p.Key)}
		}
		if utf8.RuneCountInString(v) > maxStandardChangeTextValue {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be at most %d characters", p.Label, p.Key, maxStandardChangeTextValue)}
		}
	case domain.StandardChangeParameterTypeTextarea:
		if utf8.RuneCountInString(v) > maxStandardChangeTextareaValue {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be at most %d characters", p.Label, p.Key, maxStandardChangeTextareaValue)}
		}
	case domain.StandardChangeParameterTypeNumber:
		if f, err := strconv.ParseFloat(v, 64); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be a number", p.Label, p.Key)}
		}
	case domain.StandardChangeParameterTypeDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be a date, YYYY-MM-DD", p.Label, p.Key)}
		}
	case domain.StandardChangeParameterTypeSelect:
		ok := false
		for _, o := range p.Options {
			if o == v {
				ok = true
				break
			}
		}
		if !ok {
			return "", &apierror.ValidationError{Msg: fmt.Sprintf("%s (%s) must be one of: %s", p.Label, p.Key, strings.Join(p.Options, ", "))}
		}
	}
	return v, nil
}

// renderStandardChange checks the values a change request is raised with against
// the version's parameters and returns the version's content with every
// placeholder replaced, plus the values as stored. In the rich-text fields a value
// is HTML-escaped (a textarea's line breaks become <br>), so a value can never
// inject markup into the approved plan; in the plain fields it is put in as typed.
func renderStandardChange(v domain.StandardChangeTemplateVersion, values map[string]string) (domain.StandardChangeContent, map[string]string, error) {
	declared := map[string]domain.StandardChangeParameter{}
	for _, p := range v.Parameters {
		declared[p.Key] = p
	}
	var unknown []string
	for k := range values {
		if _, ok := declared[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return domain.StandardChangeContent{}, nil, &apierror.ValidationError{Msg: "standardChangeParameters names parameters the template does not declare: " + strings.Join(unknown, ", ")}
	}
	stored := map[string]string{}
	for _, p := range v.Parameters {
		val, err := normalizeStandardChangeValue(p, values[p.Key])
		if err != nil {
			return domain.StandardChangeContent{}, nil, err
		}
		stored[p.Key] = val
	}
	out := v.StandardChangeContent
	for _, f := range standardChangeTextFields(&out) {
		if *f.v == nil {
			continue
		}
		asHTML := f.html
		rendered := standardChangePlaceholder.ReplaceAllStringFunc(**f.v, func(m string) string {
			key := standardChangePlaceholder.FindStringSubmatch(m)[1]
			val, ok := stored[key]
			if !ok {
				return m
			}
			if !asHTML {
				return strings.Join(strings.Fields(val), " ")
			}
			escaped := html.EscapeString(val)
			if declared[key].Type == domain.StandardChangeParameterTypeTextarea {
				escaped = strings.ReplaceAll(strings.ReplaceAll(escaped, "\r\n", "\n"), "\n", "<br>")
			}
			return escaped
		})
		*f.v = &rendered
	}
	if out.Subject != nil && utf8.RuneCountInString(*out.Subject) > maxStandardChangeRenderedSubject {
		return domain.StandardChangeContent{}, nil, &apierror.ValidationError{Msg: fmt.Sprintf("the title this template makes with these values is longer than %d characters: shorten the values", maxStandardChangeRenderedSubject)}
	}
	if out.RollbackDurationText != nil && utf8.RuneCountInString(*out.RollbackDurationText) > maxStandardChangeRollbackText {
		return domain.StandardChangeContent{}, nil, &apierror.ValidationError{Msg: fmt.Sprintf("the rollback duration this template makes with these values is longer than %d characters: shorten the values", maxStandardChangeRollbackText)}
	}
	return out, stored, nil
}

// normalizeSuspendReason trims a suspension reason: nil when blank, a
// ValidationError when longer than maxStandardChangeSuspendReason characters.
func normalizeSuspendReason(reason string) (*string, error) {
	r := strings.TrimSpace(reason)
	if r == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(r) > maxStandardChangeSuspendReason {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("reason must be at most %d characters", maxStandardChangeSuspendReason)}
	}
	return &r, nil
}
