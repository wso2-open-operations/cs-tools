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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// currentUserFilterPlaceholder is the literal `values` entry that a
// createdBy+eq filter must carry to mean "the authenticated caller", mirroring
// the old createdByMe:true request field. There is no way to express "eq this
// literal email" today: a single literal creator email is already covered by
// createdBy+in with one value, so eq is reserved for this placeholder.
const currentUserFilterPlaceholder = "__current_user_email__"

// caseFilterFieldSet is the exact set of CaseFieldFilter.Field values accepted
// by case search. Anything else is rejected outright.
var caseFilterFieldSet = map[string]bool{
	"type": true, "state": true, "severity": true, "engagementType": true,
	"issueType": true, "workState": true, "tag": true, "projectId": true,
	"deploymentId": true, "assignedUserId": true, "createdBy": true,
	"createdOn": true, "updatedOn": true, "closedOn": true, "resolvedOn": true, "product": true,
	"projectOnboardingStatus": true, "projectType": true, "creTeam": true, "sreTeam": true,
	"resolutionNotes": true, "parentId": true, "taskSLABusinessElapsedPercent": true,
	"escalationLevel": true, "escalation": true, "number": true, "internalId": true,
	"accountId": true, "slaBreached": true, "accountEscalationActive": true,
}

// caseFilterOpSet is the exact set of CaseFieldFilter.Op values accepted by
// case search, independent of field. Field/op compatibility is enforced
// separately in ParseCaseFieldFilters.
var caseFilterOpSet = map[string]bool{
	"eq": true, "in": true, "notIn": true, "isEmpty": true, "isNotEmpty": true,
	"gte": true, "lte": true,
}

// requireCaseFilterValues rejects a filter entry whose op needs a non-empty
// values array but doesn't have one.
func requireCaseFilterValues(f domain.CaseFieldFilter) error {
	if len(f.Values) == 0 {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q requires a non-empty values array", f.Field, f.Op)}
	}
	return nil
}

// badCaseFilterCombo reports a field/op combination that is not supported.
func badCaseFilterCombo(f domain.CaseFieldFilter) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q does not support op %q", f.Field, f.Op)}
}

// relativeDatePlaceholderPattern matches the shape of every relative-date
// placeholder this package recognizes: __<name>__ (no argument, e.g. __today__)
// or __<name>:<argument>__ (e.g. __daysAgo:30__, __startOfMonth:-1__,
// __daysAgo:abc__). The argument capture is deliberately permissive (any
// run of non-underscore characters, not just digits) so a non-integer
// argument like "abc" still matches this shape -- letting
// parseRelativeDateArg reject it with a targeted "non-integer offset" error
// -- rather than silently falling through as "not a placeholder at all"
// just because it happened to fail a stricter \d+ match. A string that
// doesn't match this shape at all is not one of ours -- it falls through to
// the literal RFC3339/YYYY-MM-DD parse in parseCaseFilterDate, which
// rejects it with its own generic message. A string that DOES match this
// shape but names an unrecognized function gets the same fallthrough.
var relativeDatePlaceholderPattern = regexp.MustCompile(`^__([a-zA-Z]+)(?::([^_]*))?__$`)

// resolveRelativeDate resolves a filter value like "__daysAgo:2__" or
// "__startOfMonth:-1__" to a concrete "YYYY-MM-DD" string computed against
// now (always UTC, matching the rest of this pipeline's date handling --
// see formatSNDateTimeUTC). Returns matched=false, err=nil for a value that
// isn't one of these placeholders at all (a literal date, or garbage that
// parseCaseFilterDate's own validation will reject).
//
// Supported placeholders (N is a signed integer offset, 0 = current):
//   - __today__                 today's date
//   - __daysAgo:N__             today minus N days (N >= 0)
//   - __startOfMonth:N__        1st of the month N months from now
//   - __endOfMonth:N__          last day of the month N months from now
//   - __startOfQuarter:N__      1st of the quarter N quarters from now
//   - __endOfQuarter:N__        last day of the quarter N quarters from now
//
// Each resolves to a plain date (no time component); the caller (parseCaseFilterDate)
// applies the existing date-only handling -- including the lte
// inclusive-of-whole-day bump -- identically to a literal "YYYY-MM-DD" value.
func resolveRelativeDate(value string, now time.Time) (resolved string, matched bool, err error) {
	m := relativeDatePlaceholderPattern.FindStringSubmatch(value)
	if m == nil {
		return "", false, nil
	}
	name, argStr := m[1], m[2]
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch name {
	case "today":
		if argStr != "" {
			return "", false, &apierror.ValidationError{Msg: fmt.Sprintf("filters: %q does not take an argument", value)}
		}
		return today.Format("2006-01-02"), true, nil

	case "daysAgo":
		n, err := parseRelativeDateArg(value, argStr)
		if err != nil {
			return "", false, err
		}
		if n < 0 {
			return "", false, &apierror.ValidationError{Msg: fmt.Sprintf("filters: %q must have a non-negative offset", value)}
		}
		return today.AddDate(0, 0, -n).Format("2006-01-02"), true, nil

	case "startOfMonth":
		n, err := parseRelativeDateArg(value, argStr)
		if err != nil {
			return "", false, err
		}
		return startOfRelativeMonth(now, n).Format("2006-01-02"), true, nil

	case "endOfMonth":
		n, err := parseRelativeDateArg(value, argStr)
		if err != nil {
			return "", false, err
		}
		return startOfRelativeMonth(now, n+1).AddDate(0, 0, -1).Format("2006-01-02"), true, nil

	case "startOfQuarter":
		n, err := parseRelativeDateArg(value, argStr)
		if err != nil {
			return "", false, err
		}
		return startOfRelativeQuarter(now, n).Format("2006-01-02"), true, nil

	case "endOfQuarter":
		n, err := parseRelativeDateArg(value, argStr)
		if err != nil {
			return "", false, err
		}
		return startOfRelativeQuarter(now, n+1).AddDate(0, 0, -1).Format("2006-01-02"), true, nil

	default:
		// Shape matches (__name__ or __name:N__) but the name isn't one of
		// ours -- not a relative-date placeholder at all (e.g. some other
		// package's __current_x__ token used by mistake on a date field).
		// Let parseCaseFilterDate's literal-date parse reject it.
		return "", false, nil
	}
}

// parseRelativeDateArg parses a relativeDatePlaceholderPattern match's :N
// argument, rejecting a missing or non-integer offset with an error naming
// the full original placeholder value.
func parseRelativeDateArg(value, argStr string) (int, error) {
	if argStr == "" {
		return 0, &apierror.ValidationError{Msg: fmt.Sprintf("filters: %q requires a :N integer offset", value)}
	}
	n, err := strconv.Atoi(argStr)
	if err != nil {
		return 0, &apierror.ValidationError{Msg: fmt.Sprintf("filters: %q has a non-integer offset", value)}
	}
	return n, nil
}

// startOfRelativeMonth returns the 1st of the month n months from now's
// month (UTC, midnight), e.g. n=0 this month, n=-1 last month, n=1 next
// month. Relies on time.AddDate's month normalization for year rollover.
func startOfRelativeMonth(now time.Time, n int) time.Time {
	base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, n, 0)
}

// startOfRelativeQuarter returns the 1st of the quarter n quarters from now's
// quarter (UTC, midnight), e.g. n=0 this quarter, n=-1 last quarter.
func startOfRelativeQuarter(now time.Time, n int) time.Time {
	quarterStartMonth := ((int(now.Month())-1)/3)*3 + 1
	base := time.Date(now.Year(), time.Month(quarterStartMonth), 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, n*3, 0)
}

// parseCaseFilterDate parses a single filter value into a date/time. The
// value may be one of the relative-date placeholders resolveRelativeDate
// recognizes, a full RFC3339 timestamp, or a plain YYYY-MM-DD date
// (interpreted as UTC midnight) -- callers may reasonably send any of these
// for a date-range bound. now is the reference instant relative-date
// placeholders resolve against (production passes time.Now().UTC(); tests
// pass a fixed instant).
func parseCaseFilterDate(f domain.CaseFieldFilter, value string, now time.Time) (*time.Time, error) {
	if resolved, matched, err := resolveRelativeDate(value, now); err != nil {
		return nil, err
	} else if matched {
		value = resolved
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		// A date-only lte bound means "on or before that whole day".
		if f.Op == "lte" {
			t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
		}
		return &t, nil
	}
	return nil, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be an RFC3339 timestamp, YYYY-MM-DD date, or a recognized relative-date placeholder", f.Field, f.Op, value)}
}

// ParseCaseFieldFilters translates the case-search wire contract's generic
// filter array (domain.CaseFieldFilter) into domain.ParsedCaseFilters, the
// internal named-field representation both CaseService backends (ServiceNow
// and Postgres) and the Postgres repository build their queries/payloads
// from. This is the one place field/op validation and the field→op meaning
// table live; everything downstream keeps its pre-existing, already-verified
// per-field logic, just reading from ParsedCaseFilters instead of the old
// named request fields.
//
// callerEmail/callerEmailErr resolve the authenticated caller's identity
// (via the same JWT helper callers already use elsewhere in this package,
// e.g. case_service.go's prior createdByMe handling): pass a non-empty
// callerEmail when resolution succeeded, or callerEmailErr describing why it
// didn't. Both are only consulted if the array actually contains a
// createdBy+eq current-user filter, so callers can resolve them
// unconditionally without changing behavior for requests that don't need it.
//
// now is the reference instant createdOn/updatedOn/closedOn's relative-date
// placeholders (see resolveRelativeDate) resolve against; production callers
// pass time.Now().UTC(), tests pass a fixed instant.
func ParseCaseFieldFilters(filters []domain.CaseFieldFilter, callerEmail string, callerEmailErr error, now time.Time) (domain.ParsedCaseFilters, error) {
	var p domain.ParsedCaseFilters

	for _, f := range filters {
		if !caseFilterFieldSet[f.Field] {
			return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !caseFilterOpSet[f.Op] {
			return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "type":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.Types = append(p.Types, normalizeCaseType(v))
			}

		case "state":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				for _, v := range f.Values {
					p.States = append(p.States, domain.CaseState(v))
				}
			case "notIn":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				for _, v := range f.Values {
					p.ExcludeStates = append(p.ExcludeStates, domain.CaseState(v))
				}
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "severity":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.Severities = append(p.Severities, domain.CaseSeverity(v))
			}

		case "engagementType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.EngagementTypes = append(p.EngagementTypes, domain.EngagementType(v))
			}

		case "issueType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.IssueTypes = append(p.IssueTypes, domain.CaseIssueType(v))
			}

		case "workState":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			for _, v := range f.Values {
				p.WorkStates = append(p.WorkStates, domain.CaseWorkState(v))
			}

		case "tag":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.Tags = append(p.Tags, f.Values...)
			case "notIn":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.ExcludeTags = append(p.ExcludeTags, f.Values...)
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "projectId":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProjectIDs = append(p.ProjectIDs, f.Values...)

		case "deploymentId":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.DeploymentIDs = append(p.DeploymentIDs, f.Values...)

		case "assignedUserId":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.AssignedUserIDs = append(p.AssignedUserIDs, f.Values...)
			case "isEmpty":
				p.Unassigned = true
			default:
				// isNotEmpty (or anything else) has no prior equivalent: nothing
				// today expresses "assigned to someone specific but no one in
				// particular" -- reject rather than invent new behavior.
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "createdBy":
			switch f.Op {
			case "in":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				p.CreatedBy = append(p.CreatedBy, f.Values...)
			case "eq":
				if err := requireCaseFilterValues(f); err != nil {
					return domain.ParsedCaseFilters{}, err
				}
				if len(f.Values) != 1 || f.Values[0] != currentUserFilterPlaceholder {
					return domain.ParsedCaseFilters{}, &apierror.ValidationError{
						Msg: fmt.Sprintf("filters: createdBy eq only supports values: [%q] (the current-user placeholder); use op \"in\" for literal creator emails", currentUserFilterPlaceholder),
					}
				}
				if callerEmailErr != nil {
					return domain.ParsedCaseFilters{}, callerEmailErr
				}
				if callerEmail == "" {
					return domain.ParsedCaseFilters{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required for the createdBy current-user filter"}
				}
				p.CreatedByMe = true
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "createdOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0], now)
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.StartCreatedDate = t
			case "lte":
				p.EndCreatedDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "updatedOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0], now)
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.StartUpdatedDate = t
			case "lte":
				p.EndUpdatedDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "closedOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0], now)
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.ClosedStartDate = t
			case "lte":
				p.ClosedEndDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "resolvedOn":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			t, err := parseCaseFilterDate(f, f.Values[0], now)
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "gte":
				p.ResolvedStartDate = t
			case "lte":
				p.ResolvedEndDate = t
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "product":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProductNames = append(p.ProductNames, f.Values...)

		case "projectOnboardingStatus":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			switch f.Op {
			case "in":
				p.ProjectOnboardingStatuses = append(p.ProjectOnboardingStatuses, f.Values...)
			case "notIn":
				p.ExcludeProjectOnboardingStatuses = append(p.ExcludeProjectOnboardingStatuses, f.Values...)
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "projectType":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.ProjectTypeNames = append(p.ProjectTypeNames, f.Values...)

		case "creTeam":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.CreTeamIDs = append(p.CreTeamIDs, f.Values...)

		case "sreTeam":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.SreTeamIDs = append(p.SreTeamIDs, f.Values...)

		case "accountId":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if err := validateUUIDs("filters: accountId", f.Values); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.AccountIDs = append(p.AccountIDs, f.Values...)

		case "resolutionNotes":
			// isNotEmpty has no prior equivalent: false and omitted were
			// explicitly documented as identical for resolutionNotesEmpty, so
			// there is no distinct "resolution notes present" SN-layer
			// behavior to translate to -- reject rather than invent one.
			if f.Op != "isEmpty" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			p.ResolutionNotesEmpty = true

		case "parentId":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: parentId eq requires exactly one value"}
			}
			id := f.Values[0]
			p.ParentID = &id

		case "number":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: number eq requires exactly one value"}
			}
			number := f.Values[0]
			p.Number = &number

		case "internalId":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: internalId eq requires exactly one value"}
			}
			internalID := f.Values[0]
			p.InternalID = &internalID

		case "taskSLABusinessElapsedPercent":
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			pct, err := parseCaseFilterPercent(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if p.TaskSLAFilter == nil {
				p.TaskSLAFilter = &domain.TaskSLAFilter{}
			}
			switch f.Op {
			case "gte":
				p.TaskSLAFilter.MinBusinessElapsedPercent = &pct
			case "lte":
				p.TaskSLAFilter.MaxBusinessElapsedPercent = &pct
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "escalationLevel":
			if f.Op != "in" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.EscalationLevels = append(p.EscalationLevels, f.Values...)

		case "escalation":
			switch f.Op {
			case "isEmpty":
				noEscalation := false
				p.HasActiveEscalation = &noEscalation
			case "isNotEmpty":
				hasEscalation := true
				p.HasActiveEscalation = &hasEscalation
			default:
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}

		case "slaBreached":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: slaBreached eq requires exactly one value"}
			}
			b, err := parseCaseFilterBool(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.HasBreachedSLA = &b

		case "accountEscalationActive":
			if f.Op != "eq" {
				return domain.ParsedCaseFilters{}, badCaseFilterCombo(f)
			}
			if err := requireCaseFilterValues(f); err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			if len(f.Values) != 1 {
				return domain.ParsedCaseFilters{}, &apierror.ValidationError{Msg: "filters: accountEscalationActive eq requires exactly one value"}
			}
			b, err := parseCaseFilterBool(f, f.Values[0])
			if err != nil {
				return domain.ParsedCaseFilters{}, err
			}
			p.HasActiveAccountEscalation = &b
		}
	}

	return p, nil
}

// orGroupCallerEmailSentinel is the placeholder caller email
// ParseCaseFieldFilterGroups passes into ParseCaseFieldFilters. It exists only
// to keep the parse from failing early on the createdBy current-user filter, a
// field that is rejected for OR groups regardless; it never reaches a query or
// the wire. Not a routable address.
const orGroupCallerEmailSentinel = "or-group-validation@invalid"

// branchFilterErrorPrefix is the JSON path ParseCaseFieldFilters roots every
// validation message at. Inside an OR branch that path is wrong: the filter
// array lives at anyOf[i].filters, not at the top-level filters, and the
// public contract reports the location a client actually has to edit.
const branchFilterErrorPrefix = "filters:"

// relocateBranchFilterError rewrites a ParseCaseFieldFilters validation error
// raised while parsing branch index i so its path names the branch --
// "filters: unsupported field: x" becomes
// "anyOf[0].filters: unsupported field: x". The ValidationError type is
// preserved so the error still maps to a 400; an error that is not one of
// ours, or that is not rooted at "filters:", is returned untouched rather than
// having a path glued onto a message that never carried one.
func relocateBranchFilterError(err error, i int) error {
	var v *apierror.ValidationError
	if !errors.As(err, &v) || !strings.HasPrefix(v.Msg, branchFilterErrorPrefix) {
		return err
	}
	return &apierror.ValidationError{
		Msg: fmt.Sprintf("anyOf[%d].%s", i, v.Msg),
	}
}

// ParseCaseFieldFilterGroups translates SearchCasesFilters.AnyOf (each branch
// carrying its own independent CaseFieldFilter array) into
// domain.CaseFilterGroup entries, reusing ParseCaseFieldFilters's per-field
// parsing/validation for every branch. Only fields with a direct,
// non-subquery ServiceNow mapping are allowed inside a branch (see
// domain.CaseFilterGroup doc comment) -- a branch containing any other field
// is rejected with a validation error naming the unsupported field, rather
// than silently dropping it.
func ParseCaseFieldFilterGroups(branches []domain.CaseFilterBranch) ([]domain.CaseFilterGroup, error) {
	result := make([]domain.CaseFilterGroup, 0, len(branches))
	for i, branch := range branches {
		group := branch.Filters
		// A branch with no predicates constrains nothing, and the branches are
		// OR'd against each other -- so a single empty one widens the WHOLE
		// result set to every case, with a 200 and nothing in any log to show
		// for it. `"anyOf": [{}]` decodes cleanly into this, so the schema does
		// not catch it either (openapi.yaml now carries minItems: 1 for the
		// clients that do validate, but the server cannot rely on that).
		if len(group) == 0 {
			return nil, &apierror.ValidationError{
				Msg: fmt.Sprintf("anyOf[%d].filters: an OR branch must carry at least one filter predicate; an empty branch matches everything and would silently widen the whole result set", i),
			}
		}
		// now is irrelevant here: rejectUnsupportedOrGroupFields below rejects any
		// date-range field (createdOn/updatedOn/closedOn/resolvedOn) inside an OR-group branch,
		// so this call never actually resolves a relative-date placeholder.
		//
		// orGroupCallerEmailSentinel stands in for the caller's email so the
		// createdBy-eq-current-user branch parses instead of short-circuiting with
		// a 401: createdBy is not permitted inside an OR group at all, and the
		// caller deserves that 400 validation error, not a misleading
		// "not authenticated". The sentinel is never read -- parsing only sets
		// ParsedCaseFilters.CreatedByMe, which rejectUnsupportedOrGroupFields then
		// rejects, and CreatedBy/CreatedByMe are not copied into CaseFilterGroup.
		parsed, err := ParseCaseFieldFilters(group, orGroupCallerEmailSentinel, nil, time.Now().UTC())
		if err != nil {
			return nil, relocateBranchFilterError(err, i)
		}
		if err := rejectUnsupportedOrGroupFields(parsed); err != nil {
			return nil, err
		}
		result = append(result, domain.CaseFilterGroup{
			Types:            parsed.Types,
			States:           parsed.States,
			Severities:       parsed.Severities,
			EngagementTypes:  parsed.EngagementTypes,
			IssueTypes:       parsed.IssueTypes,
			WorkStates:       parsed.WorkStates,
			ProjectIDs:       parsed.ProjectIDs,
			DeploymentIDs:    parsed.DeploymentIDs,
			AssignedUserIDs:  parsed.AssignedUserIDs,
			EscalationLevels: parsed.EscalationLevels,
			Tags:             parsed.Tags,
			ExcludeTags:      parsed.ExcludeTags,
		})
	}
	return result, nil
}

// rejectUnsupportedOrGroupFields errors if parsed carries any field
// CaseFilterGroup does not model -- these remain usable only via the
// top-level (AND-only) Filters array, not inside an OR branch.
func rejectUnsupportedOrGroupFields(parsed domain.ParsedCaseFilters) error {
	switch {
	case parsed.ParentID != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"parentId\" is not supported inside an OR group"}
	case parsed.Number != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"number\" is not supported inside an OR group"}
	case parsed.InternalID != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"internalId\" is not supported inside an OR group"}
	case len(parsed.CreatedBy) > 0 || parsed.CreatedByMe:
		return &apierror.ValidationError{Msg: "anyOf: field \"createdBy\" is not supported inside an OR group"}
	case parsed.ClosedStartDate != nil || parsed.ClosedEndDate != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"closedOn\" is not supported inside an OR group"}
	case parsed.ResolvedStartDate != nil || parsed.ResolvedEndDate != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"resolvedOn\" is not supported inside an OR group"}
	case parsed.StartCreatedDate != nil || parsed.EndCreatedDate != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"createdOn\" is not supported inside an OR group"}
	case parsed.StartUpdatedDate != nil || parsed.EndUpdatedDate != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"updatedOn\" is not supported inside an OR group"}
	case len(parsed.ProductNames) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"product\" is not supported inside an OR group"}
	case len(parsed.ProjectOnboardingStatuses) > 0 || len(parsed.ExcludeProjectOnboardingStatuses) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"projectOnboardingStatus\" is not supported inside an OR group"}
	case len(parsed.ProjectTypeNames) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"projectType\" is not supported inside an OR group"}
	case len(parsed.CreTeamIDs) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"creTeam\" is not supported inside an OR group"}
	case len(parsed.SreTeamIDs) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"sreTeam\" is not supported inside an OR group"}
	case len(parsed.AccountIDs) > 0:
		return &apierror.ValidationError{Msg: "anyOf: field \"accountId\" is not supported inside an OR group"}
	case len(parsed.ExcludeStates) > 0:
		// state+in is supported inside a branch (CaseFilterGroup.States);
		// state+notIn is not modeled there.
		return &apierror.ValidationError{Msg: "anyOf: field \"state\" (notIn) is not supported inside an OR group"}
	case parsed.Unassigned:
		return &apierror.ValidationError{Msg: "anyOf: field \"assignedUserId\" (isEmpty) is not supported inside an OR group"}
	case parsed.ResolutionNotesEmpty:
		return &apierror.ValidationError{Msg: "anyOf: field \"resolutionNotes\" is not supported inside an OR group"}
	case parsed.TaskSLAFilter != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"taskSLABusinessElapsedPercent\" is not supported inside an OR group"}
	case parsed.HasActiveEscalation != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"escalation\" is not supported inside an OR group"}
	case parsed.HasBreachedSLA != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"slaBreached\" is not supported inside an OR group"}
	case parsed.HasActiveAccountEscalation != nil:
		return &apierror.ValidationError{Msg: "anyOf: field \"accountEscalationActive\" is not supported inside an OR group"}
	}
	return nil
}

// parseCaseFilterPercent parses a single filter value into a non-negative
// integer percentage, for fields like taskSLABusinessElapsedPercent. No upper
// bound: confirmed live against wso2sndev that a long-overdue, never-resolved
// SLA's businessElapsedPercent keeps climbing well past 100 (observed up to
// 45950) rather than capping there -- a value like 100 correctly means
// "breached", not "the maximum possible value".
func parseCaseFilterPercent(f domain.CaseFieldFilter, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be a non-negative integer", f.Field, f.Op, value)}
	}
	return n, nil
}

// parseCaseFilterBool mirrors incident_filters.go's parseIncidentFilterBool
// for case search's own plain-boolean eq fields (slaBreached,
// accountEscalationActive) -- unlike "escalation" (a reference-field
// isEmpty/isNotEmpty presence check), these are true booleans with an
// explicit true/false value on the wire, the same shape as the incident
// side's slaViolated. Only the exact lowercase "true"/"false" the OpenAPI
// contract documents are accepted.
func parseCaseFilterBool(f domain.CaseFieldFilter, value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, &apierror.ValidationError{Msg: fmt.Sprintf("filters: field %q op %q value %q must be a boolean", f.Field, f.Op, value)}
	}
}

// resolveCaseFilterCallerEmail resolves the authenticated caller's email from
// the request's forwarded x-user-id-token, for use as ParseCaseFieldFilters'
// callerEmail/callerEmailErr pair. Safe to call unconditionally: the result is
// only consulted by ParseCaseFieldFilters when the filter array actually
// contains a createdBy+eq current-user filter.
func resolveCaseFilterCallerEmail(token string) (string, error) {
	if token == "" {
		return "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required for the createdBy current-user filter"}
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return "", &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	return email, nil
}
