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
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// incidentStateToEnum maps domain.IncidentState to incident_state_enum's
// real labels (migration 0058) -- identity for every value except
// "canceled", which the enum spells with one L ('CANCELED') where
// domain.IncidentStateCancelled has two ("CANCELLED").
func incidentStateToEnum(s domain.IncidentState) string {
	if s == domain.IncidentStateCancelled {
		return "CANCELED"
	}
	return string(s)
}

// incidentStateBySNKey is snIncidentStateKeyMap read the other way: ServiceNow's
// raw numeric incident state to the domain state, for the one filter that
// arrives as raw keys ("incidentStateKeys"). A key outside it (4, 5, ...) has
// no state on this data source.
var incidentStateBySNKey = func() map[int]domain.IncidentState {
	m := make(map[int]domain.IncidentState, len(snIncidentStateKeyMap))
	for state, key := range snIncidentStateKeyMap {
		m[key] = state
	}
	return m
}()

// incidentStatesFromSNKeys maps the raw `incident_state` keys of an
// "incidentStateKeys" filter onto incident_state_enum labels, preserving the
// order given and dropping repeats. An unknown key is rejected rather than
// skipped: a skipped key narrows nothing and an "in" list that ends up empty
// would read as "no state filter" to the repository, silently widening the
// result.
func incidentStatesFromSNKeys(f domain.IncidentFieldFilter) ([]string, error) {
	var states []string
	seen := make(map[string]bool, len(f.Values))
	for _, v := range f.Values {
		raw, err := parseIncidentFilterNonNegativeIntString(f, v)
		if err != nil {
			return nil, err
		}
		key, _ := strconv.Atoi(raw) // validated as a non-negative integer above
		state, ok := incidentStateBySNKey[key]
		if !ok {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf(
				"filters: field %q value %q has no equivalent incident state on this data source (known keys: 1 New, 2 In Progress, 3 On Hold, 6 Resolved, 7 Closed, 8 Canceled)",
				f.Field, v)}
		}
		label := incidentStateToEnum(state)
		if !seen[label] {
			seen[label] = true
			states = append(states, label)
		}
	}
	return states, nil
}

// incidentResolutionCodeToEnum maps domain.IncidentResolutionCode to
// incident_resolution_code_enum's real labels (migration 0058). Two differ:
// the enum spells "Solved (Work Around)" 'SOLVED_WORK_AROUND' and "Not
// Actionable Alert" 'NOT_ACTIONABLE_ALERT'. Any other value (the domain type
// is a plain string, unchecked at decode) returns ok=false.
func incidentResolutionCodeToEnum(c domain.IncidentResolutionCode) (string, bool) {
	switch c {
	case domain.IncidentResolutionCodeSolvedWorkaround:
		return "SOLVED_WORK_AROUND", true
	case domain.IncidentResolutionCodeNotActionable:
		return "NOT_ACTIONABLE_ALERT", true
	case domain.IncidentResolutionCodeSolvedPermanently, domain.IncidentResolutionCodeNotSolvedNotReproducible,
		domain.IncidentResolutionCodeFalseAlarm, domain.IncidentResolutionCodeDuplicate:
		return string(c), true
	default:
		return "", false
	}
}

// incidentLifecycleUpdateFromRequest validates and maps the state-transition
// fields of an UPDATE (state, assignedEngineerId, resolutionCode,
// resolutionNotes, resolvedById) to their Postgres form. ok reports whether
// any of them was set. No field is required by any state here -- In Progress
// in particular needs no assignee or assignment group, matching ServiceNow;
// the Resolved/Closed resolution requirement is checked by the repository,
// which can see what is already on the record.
func incidentLifecycleUpdateFromRequest(req domain.UpdateIncidentRequest) (u repository.IncidentLifecycleUpdate, ok bool, err error) {
	if req.State != nil {
		if !validIncidentState[*req.State] {
			return u, false, &apierror.ValidationError{Msg: "invalid state: " + string(*req.State)}
		}
		v := incidentStateToEnum(*req.State)
		u.State = &v
		ok = true
	}
	if req.ResolutionCode != nil {
		v, valid := incidentResolutionCodeToEnum(*req.ResolutionCode)
		if !valid {
			return u, false, &apierror.ValidationError{Msg: "invalid resolutionCode: " + string(*req.ResolutionCode)}
		}
		u.ResolutionCode = &v
		ok = true
	}
	for field, val := range map[string]*string{"assignedEngineerId": req.AssignedEngineerID, "resolvedById": req.ResolvedByID} {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return u, false, err
			}
		}
	}
	if req.AssignedEngineerID != nil {
		u.AssignedEngineerID = req.AssignedEngineerID
		ok = true
	}
	if req.ResolvedByID != nil {
		u.ResolvedByID = req.ResolvedByID
		ok = true
	}
	if req.ResolutionNotes != nil {
		u.ResolutionNotes = req.ResolutionNotes
		ok = true
	}
	return u, ok, nil
}

// incidentRecordUpdateFromRequest validates and maps the record's own fields
// of an UPDATE (everything the portal's edit dialog sends beyond the state
// transition) onto u, with the checks snIncidentService.UpdateIncident makes
// on the same request. ok reports whether any of them was set.
func incidentRecordUpdateFromRequest(req domain.UpdateIncidentRequest, u *repository.IncidentLifecycleUpdate) (ok bool, err error) {
	if req.Priority != nil && !validIncidentPriority[*req.Priority] {
		return false, &apierror.ValidationError{Msg: "invalid priority: " + string(*req.Priority)}
	}
	if req.Category != nil && !validIncidentCategory[*req.Category] {
		return false, &apierror.ValidationError{Msg: "invalid category: " + string(*req.Category)}
	}
	if req.Subcategory != nil && !validIncidentSubcategory[*req.Subcategory] {
		return false, &apierror.ValidationError{Msg: "invalid subcategory: " + string(*req.Subcategory)}
	}
	if req.ContactType != nil && !validIncidentContactType[*req.ContactType] {
		return false, &apierror.ValidationError{Msg: "invalid contactType: " + string(*req.ContactType)}
	}
	if req.Impact != nil && !validIncidentImpact[*req.Impact] {
		return false, &apierror.ValidationError{Msg: "invalid impact: " + string(*req.Impact)}
	}
	if req.Urgency != nil && !validIncidentUrgency[*req.Urgency] {
		return false, &apierror.ValidationError{Msg: "invalid urgency: " + string(*req.Urgency)}
	}
	if req.Environment != nil && *req.Environment != nil && len([]rune(**req.Environment)) > 40 {
		// ServiceNow's u_enviroment is 40 characters, as is incident.environment.
		return false, &apierror.ValidationError{Msg: "environment must not exceed 40 characters"}
	}
	for field, val := range map[string]*string{
		"parentId":            req.ParentID,
		"parentIncidentId":    req.ParentIncidentID,
		"assignmentGroupId":   req.AssignmentGroupID,
		"serviceId":           req.ServiceID,
		"serviceOfferingId":   req.ServiceOfferingID,
		"configurationItemId": req.ConfigurationItemID,
		"changeRequestId":     req.ChangeRequestID,
		"problemId":           req.ProblemID,
		"causedById":          req.CausedByID,
	} {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return false, err
			}
		}
	}

	if req.WatchList != nil {
		if err := validateUUIDs("watchList", *req.WatchList); err != nil {
			return false, err
		}
	}

	label := func(v string) *string { return &v }
	u.Subject = req.Subject
	u.ParentID = req.ParentID
	u.AssignmentGroupID = req.AssignmentGroupID
	u.ServiceID = req.ServiceID
	u.ServiceOfferingID = req.ServiceOfferingID
	u.ConfigurationItemID = req.ConfigurationItemID
	u.ChangeRequestID = req.ChangeRequestID
	u.ProblemID = req.ProblemID
	u.CausedByID = req.CausedByID
	u.ParentIncidentID = req.ParentIncidentID
	u.IncidentReport = req.IncidentReport
	u.Environment = req.Environment
	u.WatchList = req.WatchList
	if req.Category != nil {
		u.Category = label(string(*req.Category))
	}
	if req.Subcategory != nil {
		u.SubcategoryValue = label(snIncidentSubcategoryKeyMap[*req.Subcategory])
	}
	if req.ContactType != nil {
		// The repository maps it to incident_contact_type_enum, as on create.
		u.ContactType = label(string(*req.ContactType))
	}
	if req.Impact != nil {
		u.Impact = label(string(*req.Impact))
	}
	if req.Urgency != nil {
		u.Urgency = label(string(*req.Urgency))
	}
	if req.Priority != nil {
		u.Priority = label(string(*req.Priority))
	}
	u.PriorityFor = priorityFromImpactUrgency

	ok = req.Subject != nil || req.ParentID != nil || req.AssignmentGroupID != nil ||
		req.Category != nil || req.Subcategory != nil || req.ContactType != nil ||
		req.Impact != nil || req.Urgency != nil || req.Priority != nil ||
		req.ServiceID != nil || req.ServiceOfferingID != nil || req.ConfigurationItemID != nil ||
		req.ChangeRequestID != nil || req.ProblemID != nil || req.CausedByID != nil ||
		req.ParentIncidentID != nil || req.IncidentReport != nil || req.Environment != nil ||
		req.WatchList != nil
	return ok, nil
}

// incidentPriorityToEnum maps domain.IncidentPriority to
// incident_priority_enum's real labels. incident_priority_enum has no
// 'PLANNING' label at all (only CRITICAL/HIGH/MODERATE/LOW), so
// IncidentPriorityPlanning returns ok=false rather than a miscast value --
// callers must reject it with a ValidationError, not silently drop or bind
// it.
func incidentPriorityToEnum(p domain.IncidentPriority) (string, bool) {
	switch p {
	case domain.IncidentPriorityCritical, domain.IncidentPriorityHigh, domain.IncidentPriorityModerate, domain.IncidentPriorityLow:
		return string(p), true
	default:
		// Rejects IncidentPriorityPlanning (no incident_priority_enum
		// equivalent) and any other value JSON decoding let through --
		// domain.IncidentPriority is a plain string type with no decode-time
		// validation, so a caller-supplied value outside the enum's actual
		// four labels must be caught here, not left to fail as a raw
		// Postgres enum-cast error.
		return "", false
	}
}

// parseIncidentFieldFiltersPostgres translates SearchIncidentsFilters for
// the Postgres data source: reuses incident_filters.go's field/op
// allow-lists and its date-parsing/boolean-parsing helpers (both
// data-source-agnostic), but "state" values are mapped through
// incidentStateToEnum directly rather than ParseIncidentFieldFilters' own
// SN-raw-integer translation (parsedIncidentFilters.StateKeys). Also maps
// req.Filters.Priorities (a separate, top-level field, not part of the
// generic Filters array) the same way.
//
// "incidentStateKeys" is ServiceNow's legacy `incident_state` field, which
// this schema has no column for: there is only `state`. The keys are mapped
// onto it (1 New, 2 In Progress, ...), the closest thing this data source
// has, and ANDed with any "state" filter, exactly as the two filters combine
// on ServiceNow. Dropping it instead (as this parser did) answered 200 with
// every incident of every state for a request that asked only for the open
// ones.
func parseIncidentFieldFiltersPostgres(f domain.SearchIncidentsFilters, now time.Time) (priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, err error) {
	var stateKeyStates []string
	stateKeysSupplied := false

	for _, p := range f.Priorities {
		enumValue, ok := incidentPriorityToEnum(p)
		if !ok {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters.priorities contains a value with no equivalent on this data source: " + string(p)}
		}
		priorities = append(priorities, enumValue)
	}

	for _, f := range f.Filters {
		if !incidentFilterFieldSet[f.Field] {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported field: " + f.Field}
		}
		if !incidentFilterOpSet[f.Op] {
			return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: unsupported op: " + f.Op}
		}

		switch f.Field {
		case "state":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			for _, v := range f.Values {
				state := domain.IncidentState(v)
				if !validIncidentState[state] {
					return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: state contains invalid value: " + v}
				}
				states = append(states, incidentStateToEnum(state))
			}
		case "assignmentGroupId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: assignmentGroupId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			// Applied by parseIncidentListFiltersPostgres.
		case "category", "updatedOn":
			// Parsed and applied by parseIncidentListFiltersPostgres.
		case "businessServiceId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: businessServiceId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			serviceIDs = append(serviceIDs, f.Values...)
		case "assignedUserId":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			if err := validateUUIDs("filters: assignedUserId", f.Values); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			assignedUserIDs = append(assignedUserIDs, f.Values...)
		case "createdOn":
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			t, err := parseIncidentFilterDate(f, f.Values[0], now)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			switch f.Op {
			case "gte":
				createdStartDate = t
			case "lte":
				createdEndDate = t
			default:
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
		case "slaViolated":
			if f.Op != "eq" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if len(f.Values) != 1 {
				return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: slaViolated eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			slaViolated = &b
		case "madeSla":
			if f.Op != "eq" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if len(f.Values) != 1 {
				return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: madeSla eq requires exactly one value"}
			}
			b, err := parseIncidentFilterBool(f, f.Values[0])
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			madeSla = &b
		case "incidentStateKeys":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			mapped, err := incidentStatesFromSNKeys(f)
			if err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			stateKeysSupplied = true
			for _, label := range mapped {
				stateKeyStates = appendUniqueString(stateKeyStates, label)
			}
		case "productName":
			if f.Op != "in" {
				return nil, nil, nil, nil, nil, nil, nil, nil, badIncidentFilterCombo(f)
			}
			if err := requireIncidentFilterValues(f); err != nil {
				return nil, nil, nil, nil, nil, nil, nil, nil, err
			}
			// Accepted, validated, but never applied -- no confirmed
			// product-name-to-service mapping on this data source.
		}
	}
	if stateKeysSupplied {
		if len(states) == 0 {
			states = stateKeyStates
		} else {
			// Both filters constrain the same column, so the incident must
			// satisfy both: keep the states they share.
			states = intersectStrings(states, stateKeyStates)
			if len(states) == 0 {
				// An empty list would mean "no state filter", the opposite of
				// "no incident can match".
				return nil, nil, nil, nil, nil, nil, nil, nil, &apierror.ValidationError{Msg: "filters: state and incidentStateKeys select no state in common"}
			}
		}
	}
	return priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate, nil
}

// parseIncidentListFiltersPostgres parses the incident list filters this data
// source applies beyond parseIncidentFieldFiltersPostgres's tuple -- the
// assignment group (work_item.assignment_group_id), the category and the
// last-updated window -- into the request's internal fields, where
// incidentWhereClause reads them. The ServiceNow data source does not take
// "category" / "updatedOn" (ParseIncidentFieldFilters rejects them there).
func parseIncidentListFiltersPostgres(f *domain.SearchIncidentsFilters, now time.Time) error {
	for _, ff := range f.Filters {
		switch ff.Field {
		case "assignmentGroupId":
			if ff.Op != "in" {
				return badIncidentFilterCombo(ff)
			}
			if err := requireIncidentFilterValues(ff); err != nil {
				return err
			}
			if err := validateUUIDs("filters: assignmentGroupId", ff.Values); err != nil {
				return err
			}
			f.AssignmentGroupIDs = append(f.AssignmentGroupIDs, ff.Values...)
		case "category":
			if ff.Op != "in" {
				return badIncidentFilterCombo(ff)
			}
			if err := requireIncidentFilterValues(ff); err != nil {
				return err
			}
			for _, v := range ff.Values {
				f.Categories = append(f.Categories, strings.ToUpper(strings.TrimSpace(v)))
			}
		case "updatedOn":
			if err := requireIncidentFilterValues(ff); err != nil {
				return err
			}
			t, err := parseIncidentFilterDate(ff, ff.Values[0], now)
			if err != nil {
				return err
			}
			switch ff.Op {
			case "gte":
				f.UpdatedStart = t
			case "lte":
				f.UpdatedEnd = t
			default:
				return badIncidentFilterCombo(ff)
			}
		}
	}
	if f.UpdatedStart != nil && f.UpdatedEnd != nil && f.UpdatedEnd.Before(*f.UpdatedStart) {
		return &apierror.ValidationError{Msg: "filters: updatedOn lte bound must not be before its gte bound"}
	}
	return nil
}

// appendUniqueString appends v to list unless it is already there.
func appendUniqueString(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

// intersectStrings returns the elements of a that are also in b, in a's order.
func intersectStrings(a, b []string) []string {
	var out []string
	for _, v := range a {
		for _, w := range b {
			if v == w {
				out = appendUniqueString(out, v)
				break
			}
		}
	}
	return out
}

type incidentService struct {
	repo repository.IncidentRepository
	// userRepo is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// -- only needed there, to resolve UpdateIncident's caller identity for
	// comment.created_by (see resolveActor).
	userRepo repository.UserRepository
	// snMirror is nil in every mode except DATA_SOURCE=postgres-servicenow-dual-write
	// (config.DataSourcePostgresServiceNowDualWrite) -- see
	// NewIncidentServiceWithSNMirror's own doc comment. When set, CreateIncident
	// delegates to createIncidentSNFirst instead of the plain Postgres path's
	// ServiceUnavailableError below, mirroring caseService's identical
	// snMirror-gated branch for CreateCase. UpdateIncident also uses it, as the
	// target of its async ServiceNow mirror dispatch (see that method's own
	// doc comment) once snWriteback below is set.
	snMirror IncidentService
	// workaroundProblems creates the workaround problem when UpdateIncident
	// resolves an incident as Solved (Workaround); dual-write only, set with
	// WithWorkaroundProblemCreator (see workaround_problem.go).
	workaroundProblems WorkaroundProblemCreator
	// handoffIssues file the GitHub issue a specialist handoff opens, by
	// credential name; a product whose credential has no client still hands
	// off and reports that no issue was filed. Set with
	// WithHandoffIssueCreators.
	handoffIssues SpecialistHandoffIssueClients
	// handoffConfig routes specialist handoffs; nil hands off nothing
	// (WithSpecialistHandoffConfig).
	handoffConfig *SpecialistHandoffConfig
	// handoffPortalBaseURL is the CSM portal's address, for the incident link
	// in a handoff's GitHub issue; empty shows the number unlinked
	// (WithHandoffPortalBaseURL).
	handoffPortalBaseURL string
	// handoffNow is the clock a handoff's GitHub issue reads; nil is
	// time.Now. Tests set it.
	handoffNow func() time.Time
	// snWriteback is nil in every mode except
	// DATA_SOURCE=postgres-servicenow-dual-write, same convention as
	// caseService's identical field -- see NewCaseServiceWithSNWriteback's
	// own doc comment. Set only via NewIncidentServiceWithSNMirror. Backs
	// UpdateIncident's best-effort async ServiceNow mirror write.
	snWriteback *SNWritebackDispatcher
	// eventPublisher publishes incident.created after a create has committed
	// to Postgres: createIncidentPortal (plain Postgres) and
	// createIncidentSNFirst (dual-write) both do. createIncidentSNFirst
	// publishes it itself, after CreateIncidentFromServiceNow
	// succeeds -- the mirror IncidentService above is always constructed
	// with its own publisher=nil in this mode, specifically so it never
	// publishes prematurely (before the Postgres insert this mode's reads
	// actually depend on has even been attempted). See
	// publishIncidentCreatedEvent's doc comment for the full reasoning.
	eventPublisher EventPublisherService
	// defaultServiceID is INCIDENT_DEFAULT_SERVICE_ID: the service whose
	// support group an incident gets when its own service has none
	// (resolveAssignmentGroup). Empty: such an incident is created unassigned.
	// Set with WithIncidentDefaultService.
	defaultServiceID string
	// updatable is set by the constructors whose instance may write an incident update
	// (NewIncidentServiceWithPublisher, NewIncidentServiceWithSNMirror). NewIncidentService stays
	// read-only for updates whatever it is given.
	updatable bool
	// slaSearchDelegate: see slaDelegate. When set (SLA_DATA_SOURCE=servicenow),
	// an SLA-shaped search or aggregate is answered by it instead of the
	// Postgres sla_live-backed query, exactly as caseService does.
	slaSearchDelegate slaDelegate[IncidentService]
	// slaEngine registers/revises/evaluates this engine's own source='CSM'
	// SLA clocks for incidents -- see SLAEngineService.RegisterIncidentClocks'
	// own doc comment. nil-safe to omit (CSM_SLA_ENGINE_ENABLED=false, or no
	// database configured at all): every call site below checks for nil
	// first. Set post-construction via WithIncidentSLAEngine, for BOTH the
	// plain-Postgres (NewIncidentServiceWithPublisher) and dual-write
	// (NewIncidentServiceWithSNMirror) constructors -- deliberately not a
	// constructor parameter on either, unlike caseService's slaEngine (which
	// NewCaseServiceWithSNWriteback does take directly): several existing
	// tests already construct NewIncidentServiceWithSNMirror positionally,
	// and the decorator shape (routes.go calls WithIncidentSLAEngine once,
	// after either branch) avoids changing that constructor's signature at
	// all while still reaching both paths.
	slaEngine SLAEngineService
}

// WithIncidentSLAEngine attaches the CSM-native SLA clock engine to an
// already-constructed IncidentService -- *incidentService (built via
// NewIncidentServiceWithPublisher or NewIncidentServiceWithSNMirror) or
// *snIncidentService (built via NewServiceNowIncidentService; never the
// mirror instance NewServiceNowIncidentMirrorService builds -- see that
// struct field's own doc comment). A single decorator handling both types,
// rather than a constructor parameter on either, because both constructors
// already have several existing positional-argument test call sites;
// mirrors WithSLAEngine's identical reasoning for caseService otherwise.
// Without this, s.slaEngine is always nil, so an incident never gets SLA
// clocks registered at creation or revised on a priority change. A no-op if
// svc is neither known concrete type; slaEngine may itself be nil (engine
// disabled, or no database), the normal "off" state.
func WithIncidentSLAEngine(svc IncidentService, slaEngine SLAEngineService) IncidentService {
	switch is := svc.(type) {
	case *incidentService:
		is.slaEngine = slaEngine
	case *snIncidentService:
		is.slaEngine = slaEngine
	}
	return svc
}

// setSLADelegate implements slaDelegateAcceptor[IncidentService].
func (s *incidentService) setSLADelegate(delegate IncidentService) {
	s.slaSearchDelegate = slaDelegate[IncidentService]{backend: delegate, set: delegate != nil}
}

// NewIncidentService constructs an IncidentService backed by Postgres.
func NewIncidentService(repo repository.IncidentRepository, eventPublisher EventPublisherService) IncidentService {
	return &incidentService{repo: repo, eventPublisher: eventPublisher}
}

// NewIncidentServiceWithPublisher is NewIncidentService for DATA_SOURCE=postgres when the platform
// creates incidents itself, with no ServiceNow behind it: CreateIncident publishes incident.created once
// the Postgres insert commits (the event the call-escalation ladders start from) and keeps the create's
// notes as comments, and UpdateIncident writes the engineer's claim (state, assignee, resolution) and
// work notes and comments. A create is attributed to the forwarded user, else the calling client's id
// (actorOf); userRepo resolves a forwarded end-user token for UpdateIncident, whose notes from a
// service caller with none are incidentSystemActorEmail.
func NewIncidentServiceWithPublisher(repo repository.IncidentRepository, userRepo repository.UserRepository, eventPublisher EventPublisherService) IncidentService {
	return &incidentService{repo: repo, userRepo: userRepo, eventPublisher: eventPublisher, updatable: true}
}

// NewIncidentServiceWithSNMirror is NewIncidentService plus the wiring
// DATA_SOURCE=postgres-servicenow-dual-write needs for incident CREATE and
// UPDATE: a synchronous, ServiceNow-first creation path (createIncidentSNFirst,
// identical reasoning to caseService.createCaseSNFirst's: a Postgres-first
// async create could leave a permanent orphan), and a Postgres-first,
// async-mirrored update path scoped to WorkNotes/AdditionalComments only
// (see UpdateIncident's own doc comment).
//
// mirror is the ServiceNow-backed IncidentService (from
// NewServiceNowIncidentService) whose CreateIncident performs the real
// ServiceNow POST, including its own side effects (publishIncidentCreated),
// and whose UpdateIncident is what UpdateIncident's async mirror dispatch
// calls. It is never made the active IncidentService here -- reads always
// stay on Postgres in this mode.
//
// dispatcher is the same *SNWritebackDispatcher instance case's own
// DATA_SOURCE=postgres-servicenow-dual-write wiring already constructs
// (routes.go) -- shared, not a second dispatcher, since a dispatcher is just
// a fixed background worker pool plus one sn_writeback_failures repository,
// nothing incident-specific about it.
func NewIncidentServiceWithSNMirror(repo repository.IncidentRepository, userRepo repository.UserRepository, mirror IncidentService, eventPublisher EventPublisherService, dispatcher *SNWritebackDispatcher) IncidentService {
	return &incidentService{repo: repo, userRepo: userRepo, snMirror: mirror, eventPublisher: eventPublisher, snWriteback: dispatcher, updatable: true}
}

// incidentSystemActorEmail is UpdateIncident's comment.created_by fallback
// when no end-user identity is forwarded -- see resolveActor's doc comment
// for why this differs from caseService.resolveActor (which this was
// otherwise copied from) in refusing to fall back at all.
const incidentSystemActorEmail = "system-m2m@wso2.com"

// resolveActor resolves the calling actor's email for
// comment.created_by -- from the request's forwarded end-user JWT
// (x-user-id-token) when present, exactly like caseService.resolveActor
// (case_service.go), duplicated here rather than factored out since
// incidentService and caseService share no common base type to hang it on.
//
// Deliberately DIFFERENT from caseService.resolveActor in one respect: this
// falls back to incidentSystemActorEmail instead of a 401 when no token is
// forwarded, rather than requiring one unconditionally. comment.created_by
// (migration 0040) is a free-text VARCHAR with no FK to a real user row
// (see CreateIncidentComment's own doc comment) -- there is no schema reason
// to require a resolvable platform user here. This matters concretely: the
// M2M pipeline this whole UpdateIncident extension exists to unblock
// (a machine client -> csm-integration-service, both M2M-only, forwarding
// no end-user token by design) would otherwise trade the
// original unconditional 503 for an unconditional 401 -- fixing nothing.
// Case's comment endpoints are reached by real logged-in portal users, so a
// hard requirement is correct there; this one is also reached by
// server-to-server automation with no end user in the loop at all, so it
// is not.
func (s *incidentService) resolveActor(ctx context.Context) (domain.User, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.User{Email: incidentSystemActorEmail}, nil
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return domain.User{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
	}
	if s.userRepo == nil {
		// NewIncidentService carries no user repository; comment.created_by is free text, so the email will do.
		return domain.User{Email: email}, nil
	}
	return s.userRepo.GetUserByEmail(ctx, email)
}

// SearchIncidents implements IncidentService.
func (s *incidentService) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := validateUUIDs("filters.parentIds", req.Filters.ParentIDs); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, err :=
		parseIncidentFieldFiltersPostgres(req.Filters, time.Now().UTC())
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := parseIncidentListFiltersPostgres(&req.Filters, time.Now().UTC()); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := parseIncidentAnyOfPostgres(&req.Filters); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	slaViolated, taskSLA, err := incidentSLAFilters(req.Filters.Filters, slaViolated)
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	// Incident search has no caller scoping (every caller is internal).
	if delegate, ok := s.slaSearchDelegate.route(true, slaViolated != nil || taskSLA != nil); ok {
		return delegate.SearchIncidents(ctx, req)
	}
	req.Filters.ParsedTaskSLA = taskSLA

	views, total, err := s.repo.SearchIncidents(ctx, req, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd)
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	return domain.SearchIncidentsResponse{
		Incidents: views,
		Total:     total,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}

// AggregateIncidents implements IncidentService.
func (s *incidentService) AggregateIncidents(ctx context.Context, req domain.AggregateIncidentsRequest) (domain.AggregateResponse, error) {
	if !validIncidentAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, err :=
		parseIncidentFieldFiltersPostgres(req.Filters, time.Now().UTC())
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := parseIncidentListFiltersPostgres(&req.Filters, time.Now().UTC()); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := parseIncidentAnyOfPostgres(&req.Filters); err != nil {
		return domain.AggregateResponse{}, err
	}

	slaViolated, taskSLA, err := incidentSLAFilters(req.Filters.Filters, slaViolated)
	if err != nil {
		return domain.AggregateResponse{}, err
	}
	if delegate, ok := s.slaSearchDelegate.route(true, slaViolated != nil || taskSLA != nil); ok {
		return delegate.AggregateIncidents(ctx, req)
	}
	searchReq := domain.SearchIncidentsRequest{Filters: req.Filters}
	searchReq.Filters.ParsedTaskSLA = taskSLA
	return s.repo.AggregateIncidents(ctx, searchReq, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, req.GroupBy, req.MaxGroups)
}

// GetIncidentByID implements IncidentService.
func (s *incidentService) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentView{}, err
	}
	return s.incidentView(ctx, id)
}

// SearchIncidentActivities implements IncidentService.
func (s *incidentService) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) (domain.SearchIncidentActivitiesResponse, error) {
	if err := validateUUIDs("incidentId", []string{req.IncidentID}); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	activity, total, err := s.repo.SearchIncidentActivities(ctx, req)
	if err != nil {
		return domain.SearchIncidentActivitiesResponse{}, err
	}

	return domain.SearchIncidentActivitiesResponse{
		Activity: activity,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		// The webapp pages until hasMore is false; without it a feed stopped at its first page.
		HasMore: req.Pagination.Offset+len(activity) < total,
	}, nil
}

// CreateIncident implements IncidentService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createIncidentSNFirst -- see that method's own doc comment.
// Otherwise it is createIncidentPortal, the native Postgres create.
func (s *incidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	// Both create paths validate the same way; doing it first keeps a bad request from reaching the group lookup and the runbook catalog.
	if err := validateCreateIncidentRequest(req); err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	decision, err := resolveAssignmentGroup(ctx, req, s.defaultServiceID, s.assignmentGroupLookups())
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	req = withAssignmentGroup(req, decision)
	if s.snMirror != nil {
		// Every field is written to Postgres too, as the plain create writes it
		// (CreateIncidentFromServiceNow): configuration item, subcategory,
		// assigned engineer and watch list included.
		return s.createIncidentSNFirst(ctx, req)
	}
	return s.createIncidentPortal(ctx, req)
}

// incidentSubcategoryValue is a create request's subcategory as
// incident_subcategory.value spells it (ServiceNow's choice value), or nil.
func incidentSubcategoryValue(req domain.CreateIncidentRequest) *string {
	if req.Subcategory == nil {
		return nil
	}
	v := snIncidentSubcategoryKeyMap[*req.Subcategory]
	return &v
}

// assignmentGroupLookups reads services and groups from Postgres for
// resolveAssignmentGroup (see that function for the rule). In dual-write mode
// this is the only lookup: the ServiceNow mirror sends the group chosen here
// and never looks one up itself.
func (s *incidentService) assignmentGroupLookups() assignmentGroupLookups {
	if s.repo == nil {
		// Only a test builds one without a repository; it finds no service and no group.
		return assignmentGroupLookups{
			supportGroupOf: func(context.Context, string) (repository.ServiceSupportGroup, error) {
				return repository.ServiceSupportGroup{}, nil
			},
			isSupportGroup: func(context.Context, string) (bool, error) { return false, nil },
		}
	}
	return assignmentGroupLookups{
		supportGroupOf: s.repo.SupportGroupOfService,
		isSupportGroup: s.repo.IsSupportGroup,
	}
}

// GetIncidentCreateDefaults implements IncidentService.
func (s *incidentService) GetIncidentCreateDefaults(ctx context.Context) (domain.IncidentCreateDefaults, error) {
	return incidentCreateDefaults(ctx, s.defaultServiceID, s.assignmentGroupLookups())
}

// createIncidentPortal implements CreateIncident's plain-Postgres path
// (s.snMirror == nil, no ServiceNow at all). It reproduces ServiceNow's
// IncidentUtils.createIncident, the script include behind the ServiceNow
// path's POST:
//
//   - state New (the column default) and priority from impact x urgency
//     (incidentPriorityFor);
//   - every optional field IncidentUtils sets when present, including
//     subcategory, assigned engineer, configuration item and watch list;
//   - additionalComments and workNotes written as journal entries (COMMENT /
//     WORK_NOTE rows) once the record exists, not as fields on it;
//   - incident.created published after the insert commits, the event the
//     ServiceNow path publishes and csm-notification-service consumes.
//
// The number is the portal's own (next_portal_work_item_number, migration
// 0140), as for every other type created natively; INC numbers come with the
// native numbering cutover (migration 0180).
//
// The actor is the caller's validated identity (actorOf): the user's email,
// or the client id of a machine caller such as alert ingestion, which has no
// user token to forward.
func (s *incidentService) createIncidentPortal(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	if err := validateCreateIncidentRequest(req); err != nil {
		return domain.CreateIncidentResponse{}, err
	}

	priority := incidentPriorityFor(req.Impact, req.Urgency)
	resp, err := s.repo.CreateIncident(ctx, req, priority, incidentSubcategoryValue(req), actorOf(ctx))
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	registerIncidentSLAClocksEvent(ctx, s.slaEngine, resp.Incident.ID, priority)
	// The enriched publish, not the title-only one. This create path arrived
	// from upstream calling the 4-argument form, which predates the escalation
	// ladder: it publishes an incident.created carrying only the subject, with
	// no team, priority or contactType. The ladder resolves its rungs from
	// those fields, so an incident created here -- which includes every one
	// raised by alert ingestion -- would reach csm-notification-service with
	// nothing to escalate on, and no ladder would run for it.
	publishIncidentCreatedEvent(ctx, s.eventPublisher, req, resp.Incident.ID,
		resp.Incident.Number, resp.Incident.CreatedOn, s.GetIncidentByID)
	return resp, nil
}

// incidentPriorityOf returns v's priority as a plain string, "" when unset --
// used by UpdateIncident's ReviseIncidentClocks gate to detect a genuine
// priority change (before vs. after), including one driven by an
// Impact/Urgency PATCH rather than an explicit Priority field.
func incidentPriorityOf(v domain.IncidentView) string {
	if v.Priority == nil {
		return ""
	}
	return *v.Priority
}

// incidentServiceIDOf/incidentCategoryOf are incidentPriorityOf's siblings
// for the other two fields the real "Cloud Incident - High Priority" sla_policy
// row's own start_condition matches on (business_service/category) -- a
// genuine review-caught gap: ReviseIncidentClocks used to run only on a
// priority change, so a ServiceID- or Category-only PATCH (both real,
// independently-settable UpdateIncidentRequest fields) could leave an
// obsolete clock running, or fail to pick up a newly-applicable one, until
// some unrelated later priority/state change happened to trigger a
// re-resolution.
func incidentServiceIDOf(v domain.IncidentView) string {
	if v.Service == nil {
		return ""
	}
	return v.Service.ID
}

func incidentCategoryOf(v domain.IncidentView) string {
	if v.Category == nil {
		return ""
	}
	return *v.Category
}

// incidentResolutionFactsChanged reports whether before vs. after differ on
// any field this engine's incident-policy resolution depends on today --
// priority, business_service (ServiceID), or category. Used to decide
// whether ReviseIncidentClocks needs to run at all.
func incidentResolutionFactsChanged(before, after domain.IncidentView) bool {
	return incidentPriorityOf(before) != incidentPriorityOf(after) ||
		incidentServiceIDOf(before) != incidentServiceIDOf(after) ||
		incidentCategoryOf(before) != incidentCategoryOf(after)
}

// incidentUpdateTouchesResolutionFacts reports whether req could change any
// of the fields incidentResolutionFactsChanged compares -- incidentUpdateTouchesPriority's
// own set (Priority/Impact/Urgency) plus ServiceID/Category.
func incidentUpdateTouchesResolutionFacts(req domain.UpdateIncidentRequest) bool {
	return incidentUpdateTouchesPriority(req) || req.ServiceID != nil || req.Category != nil
}

// incidentPriorityFor is ServiceNow's incident priority lookup (dl_u_priority)
// -- see priorityFromImpactUrgency. The create form's preview (webapp
// utils/incidentPriorityMatrix.ts) shows the same matrix.
func incidentPriorityFor(impact domain.IncidentImpact, urgency domain.IncidentUrgency) string {
	return priorityFromImpactUrgency(string(impact), string(urgency))
}

// priorityFromImpactUrgency is ServiceNow's impact x urgency -> priority
// lookup. Incidents use dl_u_priority ("Priority Lookup"), problems
// dl_problem_priority ("Priority Problem Lookup"); both run on insert and
// update, overwrite whatever priority was set, and hold the same nine rows
// (discovery scripts 64 and 65), so one table serves both. Labels are the
// shared HIGH/MEDIUM/LOW and CRITICAL..PLANNING enum spellings.
//
//	impact \ urgency   HIGH       MEDIUM     LOW
//	HIGH              CRITICAL   HIGH       MODERATE
//	MEDIUM            HIGH       MODERATE   LOW
//	LOW               MODERATE   LOW        PLANNING
func priorityFromImpactUrgency(impact, urgency string) string {
	rank := func(v string) int {
		switch v {
		case "HIGH":
			return 0
		case "MEDIUM":
			return 1
		default:
			return 2
		}
	}
	return [...]string{"CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"}[rank(impact)+rank(urgency)]
}

// createIncidentSNFirst implements CreateIncident's
// DATA_SOURCE=postgres-servicenow-dual-write path: ServiceNow-FIRST and
// SYNCHRONOUS, exactly mirroring caseService.createCaseSNFirst's reasoning
// -- see that method's own doc comment for why CREATE must be ServiceNow
// -first rather than Postgres-first-and-async: a Postgres row with no
// ServiceNow counterpart would be a PERMANENT orphan (ServiceNow is still
// the real backing store this platform proxies most writes onto), while an
// async-after-commit UPDATE has no equivalent failure mode.
//
// This call is made exactly once: no internal retry. If ServiceNow's HTTP
// response is lost after it actually created the record server-side, an
// internal retry here would create a second, duplicate ServiceNow record --
// worse than a request that surfaces the error and lets the caller decide
// whether to retry. Retry policy is the caller's responsibility.
//
// The CSM database is the record of truth, so the Postgres insert writes every
// field the request carries (CreateIncidentFromServiceNow). Its rules are
// checked first (CheckIncidentFromServiceNow: the same insert, rolled back), so
// a request Postgres would refuse -- a referenced id it does not hold, a
// subcategory of another category, an unknown watcher -- is refused before
// ServiceNow creates anything, rather than leaving an incident in ServiceNow
// with no CSM row.
//
// On success, id/number/createdBy come from ServiceNow's own response and
// are used AS-IS for the Postgres insert
// (IncidentRepository.CreateIncidentFromServiceNow) rather than generated --
// see that method's own doc comment for why there is no wso2ID parameter
// here, unlike case's equivalent. This is also what makes incident creation
// possible on Postgres at all in this mode, for the same reason case's own
// pilot did: IncidentRepository's own doc comment explains why plain
// CreateIncident can't generate work_item.number itself.
func (s *incidentService) createIncidentSNFirst(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	// ServiceNow derives priority from impact x urgency but does not return it; store the same value so reads and incident.created carry it.
	priority := incidentPriorityFor(req.Impact, req.Urgency)
	subcategoryValue := incidentSubcategoryValue(req)
	if err := s.repo.CheckIncidentFromServiceNow(ctx, req, subcategoryValue, priority, actorOf(ctx)); err != nil {
		// Nothing exists anywhere yet: ServiceNow is never called.
		return domain.CreateIncidentResponse{}, err
	}

	snResp, err := s.snMirror.CreateIncident(ctx, req)
	if err != nil {
		// ServiceNow never accepted the incident -- nothing is written to
		// Postgres at all, by construction (s.repo.CreateIncidentFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateIncidentResponse{}, err
	}

	resp, err := s.repo.CreateIncidentFromServiceNow(ctx, req, subcategoryValue, snResp.Incident.ID, snResp.Incident.Number, priority, snResp.Incident.CreatedBy)
	if err != nil {
		// ServiceNow already has the incident at this point -- this is now
		// real drift (ServiceNow has it, Postgres doesn't) needing operator
		// attention, not a safely-rejected request. Logged loudly rather
		// than only returned, same convention as
		// caseService.createCaseSNFirst's identical failure shape.
		slog.ErrorContext(ctx, "sn create incident: ServiceNow incident created but the Postgres insert failed",
			"incidentId", snResp.Incident.ID, "snNumber", snResp.Incident.Number, "error", err)
		return domain.CreateIncidentResponse{}, err
	}
	// Only now -- the Postgres row "sla".work_item_id's hard foreign key
	// depends on actually exists -- is it safe to register SLA clocks, same
	// "wait for the real insert" reasoning as caseService.createCaseSNFirst's
	// own slaEngine call (see that method's own doc comment for the
	// foreign-key-violation bug this avoids).
	registerIncidentSLAClocksEvent(ctx, s.slaEngine, resp.Incident.ID, priority)
	// Only now -- Postgres has confirmed the row this mode's reads actually
	// depend on -- is it safe to publish. See publishIncidentCreatedEvent's
	// doc comment for why this can't just be snIncidentService's own
	// automatic publish (that fires right after the ServiceNow POST, before
	// this Postgres insert was even attempted).
	publishIncidentCreatedEvent(ctx, s.eventPublisher, req, resp.Incident.ID, snResp.Incident.Number, snResp.Incident.CreatedOn, s.GetIncidentByID)
	return resp, nil
}

// UpdateIncident supports three field groups, under
// DATA_SOURCE=postgres-servicenow-dual-write (s.snWriteback != nil) and on
// plain DATA_SOURCE=postgres when this instance creates its own incidents
// (NewIncidentServiceWithPublisher; alert-born SRE incidents need their
// follow-up alerts as work notes and their engineer's claim). There the
// writes below are the whole result and there is no ServiceNow mirror. The
// read-only NewIncidentService still answers 503. The three groups:
//
//   - The state transition the portal's incident action bar sends:
//     State, plus AssignedEngineerID (the "claim" sent with In Progress on
//     an unassigned incident) and ResolutionCode/ResolutionNotes/
//     ResolvedByID (sent with Resolved/Closed). Written to incident/work_item
//     in one transaction by IncidentRepository.UpdateIncidentLifecycle; see
//     repository.IncidentLifecycleUpdate for the rules. No state requires an
//     assignee or group (In Progress included), matching ServiceNow. These
//     used to be rejected here, so no incident could leave New in this mode.
//   - The record's own fields, the rest of what the portal's edit dialog
//     sends: Subject, Category, Subcategory, ContactType, Impact, Urgency,
//     Priority, ParentID, ParentIncidentID, AssignmentGroupID, ServiceID,
//     ServiceOfferingID, ConfigurationItemID, ChangeRequestID, ProblemID,
//     CausedByID, IncidentReport, Environment and WatchList -- the set
//     snIncidentService.UpdateIncident accepts, with its checks
//     (incidentRecordUpdateFromRequest). Written in the same transaction.
//     A change to impact or urgency recomputes priority with ServiceNow's
//     lookup (priorityFromImpactUrgency); a new category that does not hold
//     the current subcategory clears it. These used to be rejected here, so
//     the edit dialog could not save them in this mode.
//   - WorkNotes and AdditionalComments (below).
//
// A field sent as null is not sent, as on the ServiceNow path; only
// Environment can be cleared.
//
// WorkNotes/AdditionalComments each become their own comment row
// (comment.work_item_id = req.ID, IncidentRepository.CreateIncidentComment)
// -- WORK_NOTE/COMMENT respectively (see that method's own doc comment for
// the enum mapping). At least one of the two must be set; both may be set
// in the same call, producing two rows. This Postgres write is synchronous
// and IS this method's real, authoritative result -- GetIncidentByID re-reads
// the incident afterward purely to build the response's full IncidentView
// (a cheap local Postgres read, not a live ServiceNow pre-fetch).
//
// A best-effort, async ServiceNow mirror write follows via s.snWriteback,
// exactly the Postgres-first/async-mirror shape
// caseService.UpdateCase/CreateCaseComment already use, for the identical
// reason: a failed mirror here just leaves ServiceNow's copy of an
// EXISTING incident stale on one field until retried by hand, not a
// permanent orphan the way a failed async CREATE would be (see
// createIncidentSNFirst's own doc comment for why CREATE, unlike UPDATE,
// must be ServiceNow-first and synchronous instead).
//
// Unlike snCaseService.UpdateCase (which does a live GetCaseByID pre-fetch
// before its PATCH whenever State/Severity is set, forcing case's own
// patchCaseFields/snFieldPatcher indirection so this mode never pays for
// that read -- see UpdateCase's own doc comment), snIncidentService.UpdateIncident
// does no live pre-read at all: it's a straightforward validate-then-PATCH.
// So the mirror dispatch below calls s.snMirror.UpdateIncident directly,
// with a request carrying only ID plus the field(s) actually being
// mirrored -- no narrow patcher interface needed, unlike case's.
func (s *incidentService) UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	if !s.updatable {
		// NewIncidentService: a plain read-only Postgres instance that does not create incidents either.
		return domain.UpdateIncidentResponse{}, &apierror.ServiceUnavailableError{
			Msg: "updating an incident is not available on this data source yet",
		}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	lifecycle, hasLifecycle, err := incidentLifecycleUpdateFromRequest(req)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	hasRecord, err := incidentRecordUpdateFromRequest(req, &lifecycle)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	if !hasLifecycle && !hasRecord && req.WorkNotes == nil && req.AdditionalComments == nil {
		// The ServiceNow path's own message for the same request.
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	// The incident before this change: the activity feed records what each field was, and a claim
	// or a move out of NEW is told apart from a re-send of what it already had
	// (publishIncidentStopSignals). Read whenever the request changes a field; notes alone need it not.
	var before domain.IncidentView
	var beforeRead bool
	if hasLifecycle || hasRecord {
		if b, err := s.repo.GetIncidentByID(ctx, req.ID); err == nil {
			before, beforeRead = b, true
		} else {
			slog.WarnContext(ctx, "update incident: could not read the incident before the change; no stop signal or activity entry will be written",
				"incidentId", req.ID, "error", err)
		}
	}

	if hasLifecycle || hasRecord {
		if actor.ID != "" {
			lifecycle.DefaultResolvedByID = &actor.ID
		}
		// The notes ride in the same transaction: a failed note leaves nothing saved.
		lifecycle.WorkNotes, lifecycle.AdditionalComments = req.WorkNotes, req.AdditionalComments
		if err := s.repo.UpdateIncidentLifecycle(ctx, req.ID, lifecycle, actor.Email); err != nil {
			return domain.UpdateIncidentResponse{}, err
		}
	} else if err := s.repo.CreateIncidentNotes(ctx, req.ID, req.WorkNotes, req.AdditionalComments, actor.Email); err != nil {
		// Both notes commit together, so a retry after a failed second insert cannot save the first twice.
		return domain.UpdateIncidentResponse{}, err
	}

	view, err := s.incidentView(ctx, req.ID)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	// Deliberately independent of s.eventPublisher, same reasoning
	// caseService.UpdateCase's own ApplyCaseStateEffects/ReviseCaseClocks
	// call sites give: a deployment without Event Hub configured must not
	// lose SLA pause/resume/completion/revision as a side effect. Run
	// before publishIncidentStopSignals below, not after, for the same
	// ordering reason caseService revises BEFORE publishing
	// case.severity_changed -- a consumer reacting to an incident event by
	// re-reading this engine's current clocks must never observe the OLD
	// ones. req.State != nil alone gates ApplyIncidentStateEffects (every
	// effect it applies is idempotent, so a caller re-PATCHing the
	// incident's own current state just harmlessly re-applies it); revision
	// requires BOTH beforeRead (a notes-only update, or a failed pre-fetch,
	// never populates `before` at all -- comparing a zero-valued before
	// against view's real values would read as "changed" on every single
	// such update, cancelling and restarting a live RESOLUTION clock for
	// nothing more than a work note -- a real bug a review caught) and
	// incidentUpdateTouchesResolutionFacts(req) (the request must be able to
	// change priority, ServiceID, or Category at all -- not priority alone:
	// the real "Cloud Incident - High Priority" policy's own start_condition
	// also matches on business_service/category, a second gap the same
	// review caught -- hasRecord already fetches `before` for a
	// ServiceID/Category-only PATCH, same as it does for Priority/Impact/
	// Urgency, so no new baseline fetch is needed here), on top of the
	// GENUINE-change comparison itself (incidentResolutionFactsChanged) --
	// the same "only on real change" gate ReviseCaseClocks' own call site
	// uses.
	if s.slaEngine != nil {
		if req.State != nil {
			s.slaEngine.ApplyIncidentStateEffects(ctx, req.ID, *req.State)
		}
		if beforeRead && incidentUpdateTouchesResolutionFacts(req) &&
			incidentResolutionFactsChanged(before, view) && view.Priority != nil {
			p := domain.IncidentPriority(*view.Priority)
			s.slaEngine.ReviseIncidentClocks(ctx, req.ID, &p)
		}
	}

	publishIncidentStopSignals(ctx, s.eventPublisher, req, before, view)
	s.recordIncidentFieldChanges(ctx, req, before, view, actor.Email)

	// Dual-write: a resolve with a workaround creates its problem in both stores.
	problemID := s.createWorkaroundProblem(ctx, req, before, view)
	if problemID != "" {
		if v, err := s.incidentView(ctx, req.ID); err == nil {
			view = v
		}
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (guaranteed by the s.snWriteback == nil return just below). Postgres has
	// already committed by this point; this fires after, asynchronously, and
	// never affects this response. It forwards the request as received: every
	// field accepted above is one snIncidentService.UpdateIncident accepts too.
	// Priority goes only as sent, never the value computed above from impact x
	// urgency -- ServiceNow runs its own lookup on the impact/urgency it gets.
	if s.snWriteback == nil {
		// DATA_SOURCE=postgres creating its own incidents: there is no ServiceNow copy to keep in step.
		return domain.UpdateIncidentResponse{Message: "Incident updated successfully", Incident: view}, nil
	}
	mirrorReq := req
	if problemID != "" {
		// ServiceNow's incident gets the same problem link as Postgres's.
		mirrorReq.ProblemID = &problemID
	}
	// UpdateIncidentRequest keeps ID out of its JSON; the failure record needs it.
	payload := struct {
		ID string `json:"id"`
		domain.UpdateIncidentRequest
	}{ID: req.ID, UpdateIncidentRequest: mirrorReq}
	s.snWriteback.Dispatch(ctx, "incident", req.ID, "update",
		payload,
		func(writeCtx context.Context) error {
			_, err := s.snMirror.UpdateIncident(writeCtx, mirrorReq)
			return err
		},
	)

	return domain.UpdateIncidentResponse{
		Message:  "Incident updated successfully",
		Incident: view,
	}, nil
}
