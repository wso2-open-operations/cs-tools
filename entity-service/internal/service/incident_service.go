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
	"log/slog"
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
func parseIncidentFieldFiltersPostgres(f domain.SearchIncidentsFilters, now time.Time) (priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, err error) {
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
			// Accepted, validated, but never applied -- no backing column.
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
	return priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate, nil
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
	// snWriteback is nil in every mode except
	// DATA_SOURCE=postgres-servicenow-dual-write, same convention as
	// caseService's identical field -- see NewCaseServiceWithSNWriteback's
	// own doc comment. Set only via NewIncidentServiceWithSNMirror. Backs
	// UpdateIncident's best-effort async ServiceNow mirror write.
	snWriteback *SNWritebackDispatcher
	// eventPublisher is nil in every mode except
	// DATA_SOURCE=postgres-servicenow-dual-write. createIncidentSNFirst
	// publishes incident.created itself, after CreateIncidentFromServiceNow
	// succeeds -- the mirror IncidentService above is always constructed
	// with its own publisher=nil in this mode, specifically so it never
	// publishes prematurely (before the Postgres insert this mode's reads
	// actually depend on has even been attempted). See
	// publishIncidentCreatedEvent's doc comment for the full reasoning.
	eventPublisher EventPublisherService
}

// NewIncidentService constructs an IncidentService backed by Postgres.
func NewIncidentService(repo repository.IncidentRepository) IncidentService {
	return &incidentService{repo: repo}
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
	return &incidentService{repo: repo, userRepo: userRepo, snMirror: mirror, eventPublisher: eventPublisher, snWriteback: dispatcher}
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

	searchReq := domain.SearchIncidentsRequest{Filters: req.Filters}
	return s.repo.AggregateIncidents(ctx, searchReq, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStart, createdEnd, req.GroupBy, req.MaxGroups)
}

// GetIncidentByID implements IncidentService.
func (s *incidentService) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentView{}, err
	}
	return s.repo.GetIncidentByID(ctx, id)
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
	}, nil
}

// CreateIncident implements IncidentService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createIncidentSNFirst instead of the plain Postgres path's
// ServiceUnavailableError below -- see that method's own doc comment.
func (s *incidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	if s.snMirror != nil {
		// ConfigurationItemID has no backing column on this data source at
		// all (unlike Subcategory/AssignedEngineerID/WatchList/
		// AdditionalComments/WorkNotes, which are accepted but silently
		// not persisted -- a separate, tracked follow-up per CodeRabbit's
		// finding on PR #1922). Rejecting it explicitly is strictly
		// better than the alternative: ServiceNow would already have
		// accepted and stored it by the time Postgres is ever touched, so
		// silently dropping it here would mean the caller's request
		// appears to succeed while quietly losing data they explicitly
		// asked to set.
		//
		// AssignmentGroupID, by contrast, DOES have a backing column
		// (work_item.assignment_group_id, migration 0075) and is now
		// persisted by CreateIncidentFromServiceNow -- see that
		// function's own doc comment.
		if req.ConfigurationItemID != nil {
			return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "configurationItemId is not supported for this data source"}
		}
		return s.createIncidentSNFirst(ctx, req)
	}
	// CreateIncident is not supported for the plain PostgreSQL data source:
	// like CaseRepository.CreateCase, work_item.number has no DB default and
	// no backing sequence anywhere in migrations/.
	return domain.CreateIncidentResponse{}, &apierror.ServiceUnavailableError{
		Msg: "creating an incident is not available on this data source: work_item.number has no generation strategy defined here",
	}
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
// On success, id/number/createdBy come from ServiceNow's own response and
// are used AS-IS for the Postgres insert
// (IncidentRepository.CreateIncidentFromServiceNow) rather than generated --
// see that method's own doc comment for why there is no wso2ID parameter
// here, unlike case's equivalent. This is also what makes incident creation
// possible on Postgres at all in this mode, for the same reason case's own
// pilot did: IncidentRepository's own doc comment explains why plain
// CreateIncident can't generate work_item.number itself.
func (s *incidentService) createIncidentSNFirst(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	snResp, err := s.snMirror.CreateIncident(ctx, req)
	if err != nil {
		// ServiceNow never accepted the incident -- nothing is written to
		// Postgres at all, by construction (s.repo.CreateIncidentFromServiceNow
		// is simply never called on this path). No orphan gets created.
		return domain.CreateIncidentResponse{}, err
	}

	resp, err := s.repo.CreateIncidentFromServiceNow(ctx, req, snResp.Incident.ID, snResp.Incident.Number, snResp.Incident.CreatedBy)
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
	// Only now -- Postgres has confirmed the row this mode's reads actually
	// depend on -- is it safe to publish. See publishIncidentCreatedEvent's
	// doc comment for why this can't just be snIncidentService's own
	// automatic publish (that fires right after the ServiceNow POST, before
	// this Postgres insert was even attempted).
	publishIncidentCreatedEvent(ctx, s.eventPublisher, req, resp.Incident.ID, snResp.Incident.Number, snResp.Incident.CreatedOn, s.GetIncidentByID)
	return resp, nil
}

// UpdateIncident is not supported for the plain PostgreSQL data source
// (s.snWriteback == nil): several fields have no backing column at all
// (AssignmentGroupID, ConfigurationItemID, WatchList), same blocker
// UpdateIncident always had here.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (s.snWriteback != nil),
// this supports EXACTLY WorkNotes and AdditionalComments -- a deliberate,
// narrow scope, not a stepping stone left half-built: every other field
// (Subject/Priority/State/Category/Subcategory/ContactType/ResolutionCode/
// ParentID/ParentIncidentID/AssignmentGroupID/AssignedEngineerID/ServiceID/
// ServiceOfferingID/ConfigurationItemID/ChangeRequestID/ProblemID/
// CausedByID/ResolvedByID/ResolutionNotes/IncidentReport/WatchList) is
// rejected with a ValidationError if set, mirroring caseService.UpdateCase's
// own narrow-field-set rejection style/wording (case_service.go) --
// wiring up incident State/Priority/etc against their real backing Postgres
// columns is separate, future work.
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
	if s.snWriteback == nil {
		return domain.UpdateIncidentResponse{}, &apierror.ServiceUnavailableError{
			Msg: "updating an incident is not available on this data source yet",
		}
	}
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	if req.Subject != nil || req.Priority != nil || req.State != nil || req.Category != nil ||
		req.Subcategory != nil || req.ContactType != nil || req.ResolutionCode != nil ||
		req.ParentID != nil || req.ParentIncidentID != nil || req.AssignmentGroupID != nil ||
		req.AssignedEngineerID != nil || req.ServiceID != nil || req.ServiceOfferingID != nil ||
		req.ConfigurationItemID != nil || req.ChangeRequestID != nil || req.ProblemID != nil ||
		req.CausedByID != nil || req.ResolvedByID != nil || req.ResolutionNotes != nil ||
		req.IncidentReport != nil || req.WatchList != nil {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "subject, priority, state, category, subcategory, contactType, resolutionCode, parentId, parentIncidentId, assignmentGroupId, assignedEngineerId, serviceId, serviceOfferingId, configurationItemId, changeRequestId, problemId, causedById, resolvedById, resolutionNotes, incidentReport, and watchList are only supported for the ServiceNow data source"}
	}
	if req.WorkNotes == nil && req.AdditionalComments == nil {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "at least one of workNotes or additionalComments must be provided"}
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	if req.WorkNotes != nil {
		if _, err := s.repo.CreateIncidentComment(ctx, req.ID, domain.CommentTypeWorkNote, *req.WorkNotes, actor.Email); err != nil {
			return domain.UpdateIncidentResponse{}, err
		}
	}
	if req.AdditionalComments != nil {
		if _, err := s.repo.CreateIncidentComment(ctx, req.ID, domain.CommentTypeComment, *req.AdditionalComments, actor.Email); err != nil {
			return domain.UpdateIncidentResponse{}, err
		}
	}

	view, err := s.repo.GetIncidentByID(ctx, req.ID)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (guaranteed by the s.snWriteback == nil guard above). Postgres has
	// already committed both comment rows by this point; this fires after,
	// asynchronously, and never affects this response. mirrorReq carries only
	// ID plus the field(s) this call actually set -- never forwards req
	// itself -- so this can never accidentally carry an unsupported field
	// into the mirror call.
	mirrorReq := domain.UpdateIncidentRequest{ID: req.ID, WorkNotes: req.WorkNotes, AdditionalComments: req.AdditionalComments}
	s.snWriteback.Dispatch(ctx, "incident", req.ID, "update",
		map[string]any{"id": req.ID, "workNotes": req.WorkNotes, "additionalComments": req.AdditionalComments},
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

// HandOffIncidentToSpecialist is not supported for the PostgreSQL data
// source: this is an inherently ServiceNow-workflow-specific feature (moves
// the incident to a specialist group, opens a runbook-gap task, files a
// GitHub issue) with no assignment-group or handoff-tracking concept
// anywhere in this schema to derive an equivalent from.
func (s *incidentService) HandOffIncidentToSpecialist(_ context.Context, _ domain.HandOffIncidentToSpecialistRequest) (domain.HandOffIncidentToSpecialistResponse, error) {
	return domain.HandOffIncidentToSpecialistResponse{}, &apierror.ServiceUnavailableError{
		Msg: "specialist handoff is not available on this data source: no assignment-group or handoff-tracking concept exists in this schema",
	}
}
