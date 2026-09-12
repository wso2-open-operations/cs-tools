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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeJWTWithEmail builds an unsigned-but-well-formed JWT (3 base64url segments)
// whose payload carries the given email claim, matching what emailFromJWT reads.
func fakeJWTWithEmail(t *testing.T, email string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadBytes, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".sig"
}

func TestParseCaseFieldFilters_NamedFieldTranslations(t *testing.T) {
	callerEmail, callerErr := "jane.doe@example.com", error(nil)

	cases := []struct {
		name  string
		in    []domain.CaseFieldFilter
		check func(t *testing.T, p domain.ParsedCaseFilters)
	}{
		{
			name: "type in",
			in:   []domain.CaseFieldFilter{{Field: "type", Op: "in", Values: []string{"case", "engagement"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.Types) != 2 || p.Types[0] != "case" || p.Types[1] != "engagement" {
					t.Fatalf("Types = %v", p.Types)
				}
			},
		},
		{
			// "default_case" is the production customer-portal frontend's
			// actual value for this type (ServiceNow's own raw caseType wire
			// value, predating this service's "case" enum) — see
			// caseTypeAliases. It must normalize to "case" here, not pass
			// through as-is.
			name: "type in normalizes the default_case alias to case",
			in:   []domain.CaseFieldFilter{{Field: "type", Op: "in", Values: []string{"default_case", "service_request"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.Types) != 2 || p.Types[0] != "case" || p.Types[1] != "service_request" {
					t.Fatalf("Types = %v, want [case service_request]", p.Types)
				}
			},
		},
		{
			name: "accountId in maps to AccountIDs",
			in: []domain.CaseFieldFilter{{
				Field:  "accountId",
				Op:     "in",
				Values: []string{"00000000-0000-0000-0000-000000000000"},
			}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.AccountIDs) != 1 || p.AccountIDs[0] != "00000000-0000-0000-0000-000000000000" {
					t.Fatalf("AccountIDs = %v", p.AccountIDs)
				}
			},
		},
		{
			name: "tag in maps to Tags",
			in:   []domain.CaseFieldFilter{{Field: "tag", Op: "in", Values: []string{"patch"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.Tags) != 1 || p.Tags[0] != "patch" {
					t.Fatalf("Tags = %v", p.Tags)
				}
			},
		},
		{
			name: "tag notIn maps to ExcludeTags",
			in:   []domain.CaseFieldFilter{{Field: "tag", Op: "notIn", Values: []string{"patch"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.ExcludeTags) != 1 || p.ExcludeTags[0] != "patch" {
					t.Fatalf("ExcludeTags = %v", p.ExcludeTags)
				}
			},
		},
		{
			name: "state in maps to States",
			in:   []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open", "reopened"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.States) != 2 || p.States[0] != "open" || p.States[1] != "reopened" {
					t.Fatalf("States = %v", p.States)
				}
				if len(p.ExcludeStates) != 0 {
					t.Fatalf("ExcludeStates = %v, want empty", p.ExcludeStates)
				}
			},
		},
		{
			name: "state notIn maps to ExcludeStates",
			in:   []domain.CaseFieldFilter{{Field: "state", Op: "notIn", Values: []string{"awaiting_info", "solution_proposed", "closed"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				want := []domain.CaseState{"awaiting_info", "solution_proposed", "closed"}
				if len(p.ExcludeStates) != len(want) {
					t.Fatalf("ExcludeStates = %v, want %v", p.ExcludeStates, want)
				}
				for i, w := range want {
					if p.ExcludeStates[i] != w {
						t.Fatalf("ExcludeStates = %v, want %v", p.ExcludeStates, want)
					}
				}
				// notIn must never be folded into the positive allowlist: that
				// would silently invert the predicate's meaning.
				if len(p.States) != 0 {
					t.Fatalf("States = %v, want empty: notIn must not populate the in list", p.States)
				}
			},
		},
		{
			name: "state in and notIn are independent and may be combined",
			in: []domain.CaseFieldFilter{
				{Field: "state", Op: "in", Values: []string{"open", "work_in_progress"}},
				{Field: "state", Op: "notIn", Values: []string{"closed"}},
			},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.States) != 2 {
					t.Fatalf("States = %v, want 2 entries", p.States)
				}
				if len(p.ExcludeStates) != 1 || p.ExcludeStates[0] != "closed" {
					t.Fatalf("ExcludeStates = %v, want [closed]", p.ExcludeStates)
				}
			},
		},
		{
			name: "assignedUserId isEmpty maps to Unassigned",
			in:   []domain.CaseFieldFilter{{Field: "assignedUserId", Op: "isEmpty"}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.Unassigned {
					t.Fatalf("expected Unassigned = true")
				}
			},
		},
		{
			name: "resolutionNotes isEmpty maps to ResolutionNotesEmpty",
			in:   []domain.CaseFieldFilter{{Field: "resolutionNotes", Op: "isEmpty"}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.ResolutionNotesEmpty {
					t.Fatalf("expected ResolutionNotesEmpty = true")
				}
			},
		},
		{
			name: "createdBy in maps to literal CreatedBy list",
			in:   []domain.CaseFieldFilter{{Field: "createdBy", Op: "in", Values: []string{"a@example.com", "b@example.com"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.CreatedBy) != 2 {
					t.Fatalf("CreatedBy = %v", p.CreatedBy)
				}
				if p.CreatedByMe {
					t.Fatalf("expected CreatedByMe = false for a literal email list")
				}
			},
		},
		{
			name: "createdBy eq placeholder maps to CreatedByMe",
			in:   []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if !p.CreatedByMe {
					t.Fatalf("expected CreatedByMe = true")
				}
				if len(p.CreatedBy) != 0 {
					t.Fatalf("expected CreatedBy left empty (SN forwards CreatedByMe as a flag, not folded in), got %v", p.CreatedBy)
				}
			},
		},
		{
			name: "createdOn gte/lte map to StartCreatedDate/EndCreatedDate",
			in: []domain.CaseFieldFilter{
				{Field: "createdOn", Op: "gte", Values: []string{"2026-01-01"}},
				{Field: "createdOn", Op: "lte", Values: []string{"2026-01-31"}},
			},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.StartCreatedDate == nil || p.EndCreatedDate == nil {
					t.Fatalf("expected both StartCreatedDate and EndCreatedDate set")
				}
			},
		},
		{
			name: "projectOnboardingStatus in",
			in:   []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "in", Values: []string{"Completed"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.ProjectOnboardingStatuses) != 1 || p.ProjectOnboardingStatuses[0] != "Completed" {
					t.Fatalf("ProjectOnboardingStatuses = %v", p.ProjectOnboardingStatuses)
				}
			},
		},
		{
			name: "projectOnboardingStatus notIn",
			in:   []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "notIn", Values: []string{"In-Progress"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if len(p.ExcludeProjectOnboardingStatuses) != 1 || p.ExcludeProjectOnboardingStatuses[0] != "In-Progress" {
					t.Fatalf("ExcludeProjectOnboardingStatuses = %v", p.ExcludeProjectOnboardingStatuses)
				}
				if len(p.ProjectOnboardingStatuses) != 0 {
					t.Fatalf("ProjectOnboardingStatuses = %v, want empty", p.ProjectOnboardingStatuses)
				}
			},
		},
		{
			name: "parentId eq",
			in:   []domain.CaseFieldFilter{{Field: "parentId", Op: "eq", Values: []string{"00000000-0000-0000-0000-000000000000"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.ParentID == nil || *p.ParentID != "00000000-0000-0000-0000-000000000000" {
					t.Fatalf("ParentID = %v", p.ParentID)
				}
			},
		},
		{
			name: "number eq",
			in:   []domain.CaseFieldFilter{{Field: "number", Op: "eq", Values: []string{"CS0441174"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.Number == nil || *p.Number != "CS0441174" {
					t.Fatalf("Number = %v", p.Number)
				}
			},
		},
		{
			name: "internalId eq",
			in:   []domain.CaseFieldFilter{{Field: "internalId", Op: "eq", Values: []string{"12345"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.InternalID == nil || *p.InternalID != "12345" {
					t.Fatalf("InternalID = %v", p.InternalID)
				}
			},
		},
		{
			name: "slaBreached eq true maps to HasBreachedSLA",
			in:   []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"true"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.HasBreachedSLA == nil || !*p.HasBreachedSLA {
					t.Fatalf("HasBreachedSLA = %v, want pointer to true", p.HasBreachedSLA)
				}
			},
		},
		{
			name: "slaBreached eq false maps to HasBreachedSLA",
			in:   []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"false"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.HasBreachedSLA == nil || *p.HasBreachedSLA {
					t.Fatalf("HasBreachedSLA = %v, want pointer to false", p.HasBreachedSLA)
				}
			},
		},
		{
			name: "accountEscalationActive eq true maps to HasActiveAccountEscalation",
			in:   []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.HasActiveAccountEscalation == nil || !*p.HasActiveAccountEscalation {
					t.Fatalf("HasActiveAccountEscalation = %v, want pointer to true", p.HasActiveAccountEscalation)
				}
				// Distinct from the case-level escalation filter: setting
				// accountEscalationActive must not also populate
				// HasActiveEscalation.
				if p.HasActiveEscalation != nil {
					t.Fatalf("HasActiveEscalation = %v, want nil (accountEscalationActive is a distinct field)", p.HasActiveEscalation)
				}
			},
		},
		{
			name: "accountEscalationActive eq false maps to HasActiveAccountEscalation",
			in:   []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"false"}}},
			check: func(t *testing.T, p domain.ParsedCaseFilters) {
				if p.HasActiveAccountEscalation == nil || *p.HasActiveAccountEscalation {
					t.Fatalf("HasActiveAccountEscalation = %v, want pointer to false", p.HasActiveAccountEscalation)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseCaseFieldFilters(tc.in, callerEmail, callerErr, time.Now().UTC())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, p)
		})
	}
}

func TestParseCaseFieldFilters_Rejections(t *testing.T) {
	cases := []struct {
		name string
		in   []domain.CaseFieldFilter
	}{
		{name: "unsupported field", in: []domain.CaseFieldFilter{{Field: "bogus", Op: "in", Values: []string{"x"}}}},
		{name: "unsupported op", in: []domain.CaseFieldFilter{{Field: "type", Op: "bogus", Values: []string{"x"}}}},
		{name: "bad field/op combo", in: []domain.CaseFieldFilter{{Field: "type", Op: "eq", Values: []string{"case"}}}},
		{name: "in with no values", in: []domain.CaseFieldFilter{{Field: "type", Op: "in"}}},
		{name: "assignedUserId isNotEmpty unsupported", in: []domain.CaseFieldFilter{{Field: "assignedUserId", Op: "isNotEmpty"}}},
		{name: "resolutionNotes isNotEmpty unsupported", in: []domain.CaseFieldFilter{{Field: "resolutionNotes", Op: "isNotEmpty"}}},
		{name: "createdBy eq non-placeholder literal", in: []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{"someone@example.com"}}}},
		{name: "createdOn bad date format", in: []domain.CaseFieldFilter{{Field: "createdOn", Op: "gte", Values: []string{"not-a-date"}}}},
		{name: "number in unsupported", in: []domain.CaseFieldFilter{{Field: "number", Op: "in", Values: []string{"CS0441174"}}}},
		{name: "internalId in unsupported", in: []domain.CaseFieldFilter{{Field: "internalId", Op: "in", Values: []string{"12345"}}}},
		{name: "projectOnboardingStatus eq unsupported", in: []domain.CaseFieldFilter{{Field: "projectOnboardingStatus", Op: "eq", Values: []string{"Completed"}}}},
		{name: "accountId malformed UUID", in: []domain.CaseFieldFilter{{Field: "accountId", Op: "in", Values: []string{"not-a-uuid"}}}},
		{name: "slaBreached with unsupported op", in: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "in", Values: []string{"true"}}}},
		{name: "slaBreached with non-boolean value", in: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"yes"}}}},
		{name: "slaBreached with more than one value", in: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"true", "false"}}}},
		{name: "accountEscalationActive with unsupported op", in: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "isNotEmpty"}}},
		{name: "accountEscalationActive with non-boolean value", in: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"yes"}}}},
		{name: "accountEscalationActive with more than one value", in: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true", "false"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.in, "caller@example.com", nil, time.Now().UTC())
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestParseCaseFieldFilters_DateOnlyLteBoundIncludesWholeDay(t *testing.T) {
	filters := []domain.CaseFieldFilter{
		{Field: "createdOn", Op: "lte", Values: []string{"2026-01-31"}},
	}

	p, err := ParseCaseFieldFilters(filters, "caller@example.com", nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.EndCreatedDate == nil {
		t.Fatalf("expected EndCreatedDate to be set")
	}

	endOfDay := time.Date(2026, 1, 31, 23, 59, 59, 999999999, time.UTC)
	if !p.EndCreatedDate.Equal(endOfDay) {
		t.Fatalf("expected EndCreatedDate %v to equal the exact inclusive boundary %v", p.EndCreatedDate, endOfDay)
	}

	startOfNextDay := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !p.EndCreatedDate.Before(startOfNextDay) {
		t.Fatalf("expected EndCreatedDate %v to exclude 00:00:00 of the next day", p.EndCreatedDate)
	}
}

func TestParseCaseFieldFilters_CreatedByCurrentUser_RequiresCallerEmail(t *testing.T) {
	filters := []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}}}

	if _, err := ParseCaseFieldFilters(filters, "", nil, time.Now().UTC()); err == nil {
		t.Fatalf("expected an error when no caller email is available")
	} else if _, ok := err.(*apierror.UnauthorizedError); !ok {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}

	forwardedErr := &apierror.ValidationError{Msg: "x-user-id-token: malformed"}
	if _, err := ParseCaseFieldFilters(filters, "", forwardedErr, time.Now().UTC()); err != forwardedErr {
		t.Fatalf("expected the resolver's own error to be forwarded, got %v", err)
	}
}

// fixedNow is a fixed reference instant (a Saturday, no significance beyond
// being unambiguous) every resolveRelativeDate test below resolves against,
// so the expected outputs are exact rather than moving-target "relative to
// whenever this test happens to run."
var fixedNow = time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)

func TestResolveRelativeDate_Resolutions(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
	}{
		{"today", "__today__", "2026-08-15"},
		{"daysAgo 0", "__daysAgo:0__", "2026-08-15"},
		{"daysAgo 2", "__daysAgo:2__", "2026-08-13"},
		{"daysAgo 30 crosses month boundary", "__daysAgo:30__", "2026-07-16"},
		{"startOfMonth 0 (this month)", "__startOfMonth:0__", "2026-08-01"},
		{"startOfMonth -1 (last month)", "__startOfMonth:-1__", "2026-07-01"},
		{"startOfMonth -2 (month before last)", "__startOfMonth:-2__", "2026-06-01"},
		{"startOfMonth 1 (next month)", "__startOfMonth:1__", "2026-09-01"},
		{"endOfMonth 0 (this month, 31 days)", "__endOfMonth:0__", "2026-08-31"},
		{"endOfMonth 1 (next month, 30 days)", "__endOfMonth:1__", "2026-09-30"},
		{"startOfQuarter 0 (Q3: Jul-Sep)", "__startOfQuarter:0__", "2026-07-01"},
		{"startOfQuarter -1 (Q2: Apr-Jun)", "__startOfQuarter:-1__", "2026-04-01"},
		{"endOfQuarter 0 (Q3 ends Sep 30)", "__endOfQuarter:0__", "2026-09-30"},
		{"endOfQuarter 1 (Q4 ends Dec 31)", "__endOfQuarter:1__", "2026-12-31"},
		{"year rollover: startOfMonth 6 from August", "__startOfMonth:6__", "2027-02-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, matched, err := resolveRelativeDate(tc.value, fixedNow)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !matched {
				t.Fatalf("expected %q to be recognized as a relative-date placeholder", tc.value)
			}
			if resolved != tc.expected {
				t.Fatalf("resolveRelativeDate(%q) = %q, want %q", tc.value, resolved, tc.expected)
			}
		})
	}
}

func TestResolveRelativeDate_NotAPlaceholder(t *testing.T) {
	cases := []string{"2026-07-01", "2026-07-01T00:00:00Z", "__current_user_email__", "__bogus__", "__bogus:1__", "not-a-date-at-all"}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			resolved, matched, err := resolveRelativeDate(value, fixedNow)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched {
				t.Fatalf("expected %q not to be recognized as a relative-date placeholder, got resolved=%q", value, resolved)
			}
		})
	}
}

func TestResolveRelativeDate_Rejections(t *testing.T) {
	cases := []string{
		"__daysAgo__",        // missing required offset
		"__daysAgo:abc__",    // non-integer offset
		"__daysAgo:-1__",     // negative offset not allowed for daysAgo
		"__today:5__",        // today takes no argument
		"__startOfMonth__",   // missing required offset
		"__startOfQuarter__", // missing required offset
	}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			_, _, err := resolveRelativeDate(value, fixedNow)
			if err == nil {
				t.Fatalf("expected an error for %q, got nil", value)
			}
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestParseCaseFieldFilters_RelativeDatePlaceholders(t *testing.T) {
	filters := []domain.CaseFieldFilter{
		{Field: "createdOn", Op: "gte", Values: []string{"__daysAgo:30__"}},
		{Field: "createdOn", Op: "lte", Values: []string{"__today__"}},
	}

	p, err := ParseCaseFieldFilters(filters, "caller@example.com", nil, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.StartCreatedDate == nil || p.EndCreatedDate == nil {
		t.Fatalf("expected both StartCreatedDate and EndCreatedDate set")
	}

	wantStart := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	if !p.StartCreatedDate.Equal(wantStart) {
		t.Fatalf("StartCreatedDate = %v, want %v", p.StartCreatedDate, wantStart)
	}

	// lte on a relative-date placeholder still gets the same inclusive-of-
	// whole-day bump a literal YYYY-MM-DD lte value gets (see
	// TestParseCaseFieldFilters_DateOnlyLteBoundIncludesWholeDay) -- resolution
	// happens before that logic runs, not around it.
	wantEnd := time.Date(2026, time.August, 15, 23, 59, 59, 999999999, time.UTC)
	if !p.EndCreatedDate.Equal(wantEnd) {
		t.Fatalf("EndCreatedDate = %v, want %v (inclusive of the whole day)", p.EndCreatedDate, wantEnd)
	}
}

// An OR-group branch containing createdBy must fail with the "not supported
// inside an OR group" validation error (400). Before the caller-email sentinel
// was introduced, ParseCaseFieldFilterGroups passed an empty callerEmail, so
// the createdBy current-user filter short-circuited with an UnauthorizedError
// and an authenticated caller saw a misleading 401 for a merely-invalid request.
func TestParseCaseFieldFilterGroups_CreatedByRejectedAsValidationError(t *testing.T) {
	cases := []struct {
		name  string
		group []domain.CaseFieldFilter
	}{
		{
			name:  "createdBy eq current-user placeholder",
			group: []domain.CaseFieldFilter{{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}}},
		},
		{
			name:  "createdBy in literal emails",
			group: []domain.CaseFieldFilter{{Field: "createdBy", Op: "in", Values: []string{"someone@example.com"}}},
		},
		{
			name: "createdBy alongside a supported field",
			group: []domain.CaseFieldFilter{
				{Field: "state", Op: "in", Values: []string{"open"}},
				{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{{Filters: tc.group}})
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			var ue *apierror.UnauthorizedError
			if errors.As(err, &ue) {
				t.Fatalf("got *apierror.UnauthorizedError (401) %q, want a 400 validation error", ue.Msg)
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
			}
			const want = `anyOf: field "createdBy" is not supported inside an OR group`
			if ve.Msg != want {
				t.Errorf("Msg = %q, want %q", ve.Msg, want)
			}
		})
	}
}

// state+in is one of the fields an OR branch does model, but state+notIn is
// not: CaseFilterGroup has no exclusion field, so accepting it would drop the
// predicate and widen the branch's result set.
func TestParseCaseFieldFilterGroups_RejectsStateNotIn(t *testing.T) {
	branch := domain.CaseFilterBranch{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "notIn", Values: []string{"closed"}}},
	}
	_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{branch})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
	}
	const want = `anyOf: field "state" (notIn) is not supported inside an OR group`
	if ve.Msg != want {
		t.Errorf("Msg = %q, want %q", ve.Msg, want)
	}
}

// TestParseCaseFieldFilterGroups_RejectsSLAAndAccountEscalationFilters proves
// slaBreached and accountEscalationActive -- like the pre-existing escalation
// filter -- are rejected inside an OR-group branch: CaseFilterGroup does not
// model either field, so silently accepting them inside a branch would drop
// the predicate rather than apply it.
func TestParseCaseFieldFilterGroups_RejectsSLAAndAccountEscalationFilters(t *testing.T) {
	cases := []struct {
		name   string
		branch domain.CaseFilterBranch
		want   string
	}{
		{
			name:   "slaBreached",
			branch: domain.CaseFilterBranch{Filters: []domain.CaseFieldFilter{{Field: "slaBreached", Op: "eq", Values: []string{"true"}}}},
			want:   `anyOf: field "slaBreached" is not supported inside an OR group`,
		},
		{
			name:   "accountEscalationActive",
			branch: domain.CaseFilterBranch{Filters: []domain.CaseFieldFilter{{Field: "accountEscalationActive", Op: "eq", Values: []string{"true"}}}},
			want:   `anyOf: field "accountEscalationActive" is not supported inside an OR group`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{tc.branch})
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
			}
			if ve.Msg != tc.want {
				t.Errorf("Msg = %q, want %q", ve.Msg, tc.want)
			}
		})
	}
}

// state+in stays accepted inside a branch -- the rejection above must not
// have caught the positive form too.
func TestParseCaseFieldFilterGroups_AcceptsStateIn(t *testing.T) {
	branch := domain.CaseFilterBranch{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
	}
	groups, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{branch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 || len(groups[0].States) != 1 || groups[0].States[0] != "open" {
		t.Fatalf("groups = %+v, want one branch with States [open]", groups)
	}
}

// tag/excludeTags are supported inside an anyOf branch now that ServiceNow's
// CaseUtils (searchCases and groupCasesBy) accepts orGroups[].tags/
// excludeTags -- the Go-side rejection that used to gate this is gone.
func TestParseCaseFieldFilterGroups_AcceptsTagAndExcludeTags(t *testing.T) {
	branch := domain.CaseFilterBranch{
		Filters: []domain.CaseFieldFilter{
			{Field: "tag", Op: "in", Values: []string{"migration"}},
			{Field: "tag", Op: "notIn", Values: []string{"archived"}},
		},
	}
	groups, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{branch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if len(groups[0].Tags) != 1 || groups[0].Tags[0] != "migration" {
		t.Errorf("groups[0].Tags = %v, want [migration]", groups[0].Tags)
	}
	if len(groups[0].ExcludeTags) != 1 || groups[0].ExcludeTags[0] != "archived" {
		t.Errorf("groups[0].ExcludeTags = %v, want [archived]", groups[0].ExcludeTags)
	}
}

// A branch is parsed by ParseCaseFieldFilters, which roots every message it
// raises at the top-level "filters" path. Returned unchanged, that path is a
// lie inside an OR group: the array the client has to fix lives at
// anyOf[i].filters, and the public contract says so. The index has to be the
// offending branch's, not always zero.
func TestParseCaseFieldFilterGroups_BranchErrorsCarryTheAnyOfPath(t *testing.T) {
	valid := domain.CaseFilterBranch{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
	}

	cases := []struct {
		name     string
		branches []domain.CaseFilterBranch
		want     string
	}{
		{
			name: "invalid field in the first branch",
			branches: []domain.CaseFilterBranch{
				{Filters: []domain.CaseFieldFilter{{Field: "notAField", Op: "in", Values: []string{"x"}}}},
			},
			want: "anyOf[0].filters: unsupported field: notAField",
		},
		{
			name: "invalid op in the first branch",
			branches: []domain.CaseFilterBranch{
				{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "notAnOp", Values: []string{"open"}}}},
			},
			want: "anyOf[0].filters: unsupported op: notAnOp",
		},
		{
			name: "field/op combination the field does not support",
			branches: []domain.CaseFilterBranch{
				{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "eq", Values: []string{"open"}}}},
			},
			want: `anyOf[0].filters: field "state" does not support op "eq"`,
		},
		{
			name:     "invalid field in a later branch reports that branch's index",
			branches: []domain.CaseFilterBranch{valid, valid, {Filters: []domain.CaseFieldFilter{{Field: "notAField", Op: "in", Values: []string{"x"}}}}},
			want:     "anyOf[2].filters: unsupported field: notAField",
		},
		{
			name:     "invalid op in a later branch reports that branch's index",
			branches: []domain.CaseFilterBranch{valid, {Filters: []domain.CaseFieldFilter{{Field: "severity", Op: "notAnOp", Values: []string{"high"}}}}},
			want:     "anyOf[1].filters: unsupported op: notAnOp",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilterGroups(tc.branches)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
			}
			if ve.Msg != tc.want {
				t.Errorf("Msg = %q, want %q", ve.Msg, tc.want)
			}
		})
	}
}

// The sentinel caller email must never leak into a parsed group.
func TestParseCaseFieldFilterGroups_SupportedFieldsParse(t *testing.T) {
	groups, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{
		{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open", "closed"}}}},
		{Filters: []domain.CaseFieldFilter{{Field: "severity", Op: "in", Values: []string{"high"}}}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if len(groups[0].States) != 2 || groups[0].States[0] != domain.CaseState("open") {
		t.Errorf("groups[0].States = %v, want [open closed]", groups[0].States)
	}
	if len(groups[1].Severities) != 1 || groups[1].Severities[0] != domain.CaseSeverity("high") {
		t.Errorf("groups[1].Severities = %v, want [high]", groups[1].Severities)
	}
}

// An OR branch carrying no predicates constrains nothing, and because the
// branches are OR'd against each other one such branch widens the ENTIRE
// result set: every case matches, silently, with a 200 and no error anywhere.
// It has to be rejected at parse time. `"anyOf": [{}]` is valid JSON against
// the schema's own shape, so nothing upstream catches it either.
func TestParseCaseFieldFilterGroups_RejectsEmptyBranch(t *testing.T) {
	valid := domain.CaseFilterBranch{
		Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}},
	}

	cases := []struct {
		name     string
		branches []domain.CaseFilterBranch
		want     string
	}{
		{
			name:     `"anyOf": [{}] -- filters key absent entirely`,
			branches: []domain.CaseFilterBranch{{}},
			want:     "anyOf[0].filters: an OR branch must carry at least one filter predicate",
		},
		{
			name:     `"anyOf": [{"filters": []}] -- present but empty`,
			branches: []domain.CaseFilterBranch{{Filters: []domain.CaseFieldFilter{}}},
			want:     "anyOf[0].filters: an OR branch must carry at least one filter predicate",
		},
		{
			name:     "an empty branch alongside valid ones still reports its own index",
			branches: []domain.CaseFilterBranch{valid, valid, {}},
			want:     "anyOf[2].filters: an OR branch must carry at least one filter predicate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups, err := ParseCaseFieldFilterGroups(tc.branches)
			if err == nil {
				t.Fatalf("expected an error, got nil and %d group(s) -- an unconstrained branch was forwarded", len(groups))
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v (%T), want *apierror.ValidationError (400)", err, err)
			}
			if !strings.HasPrefix(ve.Msg, tc.want) {
				t.Errorf("Msg = %q, want prefix %q", ve.Msg, tc.want)
			}
		})
	}
}
