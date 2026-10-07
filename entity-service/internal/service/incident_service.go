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
	// updatable is set by the constructors whose instance may write an incident update
	// (NewIncidentServiceWithPublisher, NewIncidentServiceWithSNMirror). NewIncidentService stays
	// read-only for updates whatever it is given.
	updatable bool
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
	}, nil
}

// CreateIncident implements IncidentService.
//
// Under DATA_SOURCE=postgres-servicenow-dual-write (snMirror != nil), this
// delegates to createIncidentSNFirst -- see that method's own doc comment.
// Otherwise it is createIncidentPortal, the native Postgres create.
func (s *incidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	var err error
	if req, err = s.withAssignmentGroupFromService(ctx, req); err != nil {
		return domain.CreateIncidentResponse{}, err
	}
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
	return s.createIncidentPortal(ctx, req)
}

// withAssignmentGroupFromService sets an incident's assignment group to its
// service's support group.
//
// *** THE ONLY PLACE THE GROUP IS CHOSEN. *** The create request has no
// assignmentGroupId (see domain.CreateIncidentRequest.AssignmentGroupID), so
// the portal, the microapp, alert-born incidents from sre-alert-core-service
// and any M2M client all get the same group from one call, read live from the
// service as ServiceNow's alert business rule does. Done before either create
// path, so in dual-write mode ServiceNow and Postgres get the same group.
//
// A service with no support group leaves the incident unassigned. An invalid
// service id is left for request validation to reject.
func (s *incidentService) withAssignmentGroupFromService(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentRequest, error) {
	req.AssignmentGroupID = nil
	serviceID := strings.TrimSpace(req.ServiceID)
	if serviceID == "" || validateUUIDs("serviceId", []string{serviceID}) != nil || s.repo == nil {
		return req, nil
	}
	group, err := s.repo.SupportGroupOfService(ctx, serviceID)
	if err != nil {
		return req, err
	}
	if group != "" {
		req.AssignmentGroupID = &group
	}
	return req, nil
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

	var subcategoryValue *string
	if req.Subcategory != nil {
		v := snIncidentSubcategoryKeyMap[*req.Subcategory]
		subcategoryValue = &v
	}

	resp, err := s.repo.CreateIncident(ctx, req, incidentPriorityFor(req.Impact, req.Urgency), subcategoryValue, actorOf(ctx))
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}
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

// UpdateIncident supports two field groups, under
// DATA_SOURCE=postgres-servicenow-dual-write (s.snWriteback != nil) and on
// plain DATA_SOURCE=postgres when this instance creates its own incidents
// (NewIncidentServiceWithPublisher; alert-born SRE incidents need their
// follow-up alerts as work notes and their engineer's claim). There the
// writes below are the whole result and there is no ServiceNow mirror. The
// read-only NewIncidentService still answers 503. The two groups:
//
//   - The state transition the portal's incident action bar sends:
//     State, plus AssignedEngineerID (the "claim" sent with In Progress on
//     an unassigned incident) and ResolutionCode/ResolutionNotes/
//     ResolvedByID (sent with Resolved/Closed). Written to incident/work_item
//     in one transaction by IncidentRepository.UpdateIncidentLifecycle; see
//     repository.IncidentLifecycleUpdate for the rules. No state requires an
//     assignee or group (In Progress included), matching ServiceNow. These
//     used to be rejected here, so no incident could leave New in this mode.
//   - WorkNotes and AdditionalComments (below).
//
// Every other field (Subject/Priority/Category/Subcategory/ContactType/
// ParentID/ParentIncidentID/AssignmentGroupID/ServiceID/ServiceOfferingID/
// ConfigurationItemID/ChangeRequestID/ProblemID/CausedByID/IncidentReport/
// WatchList) is still rejected with a ValidationError if set, mirroring
// caseService.UpdateCase's narrow-field-set rejection style -- wiring those
// up is separate, future work.
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
	if req.Subject != nil || req.Priority != nil || req.Category != nil ||
		req.Subcategory != nil || req.ContactType != nil ||
		req.ParentID != nil || req.ParentIncidentID != nil || req.AssignmentGroupID != nil ||
		req.ServiceID != nil || req.ServiceOfferingID != nil ||
		req.ConfigurationItemID != nil || req.ChangeRequestID != nil || req.ProblemID != nil ||
		req.CausedByID != nil || req.IncidentReport != nil || req.WatchList != nil {
		// Kept under the BFF's 256-byte upstream error excerpt, so the reason
		// still reaches the portal intact if that cap is ever lowered again.
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "subject, priority, category, subcategory, contactType, parentId, parentIncidentId, assignmentGroupId, serviceId, serviceOfferingId, configurationItemId, changeRequestId, problemId, causedById, incidentReport and watchList cannot be updated on this data source yet"}
	}
	lifecycle, hasLifecycle, err := incidentLifecycleUpdateFromRequest(req)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}
	if !hasLifecycle && req.WorkNotes == nil && req.AdditionalComments == nil {
		return domain.UpdateIncidentResponse{}, &apierror.ValidationError{Msg: "at least one of state, assignedEngineerId, resolutionCode, resolutionNotes, resolvedById, workNotes or additionalComments must be provided"}
	}

	actor, err := s.resolveActor(ctx)
	if err != nil {
		return domain.UpdateIncidentResponse{}, err
	}

	// The incident before this change, so a claim or a move out of NEW can be told apart from a
	// re-send of what it already had (publishIncidentStopSignals). Read only when one could follow.
	var before domain.IncidentView
	if (s.eventPublisher != nil && (req.State != nil || req.AssignedEngineerID != nil)) ||
		(s.workaroundProblems != nil && req.State != nil) {
		if b, err := s.repo.GetIncidentByID(ctx, req.ID); err == nil {
			before = b
		} else {
			slog.WarnContext(ctx, "update incident: could not read the incident before the change; no stop signal will be sent",
				"incidentId", req.ID, "error", err)
		}
	}

	if hasLifecycle {
		if actor.ID != "" {
			lifecycle.DefaultResolvedByID = &actor.ID
		}
		// The notes ride in the state change's transaction: a failed note leaves nothing saved.
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
	publishIncidentStopSignals(ctx, s.eventPublisher, req, before, view)

	// Dual-write: a resolve with a workaround creates its problem in both stores.
	problemID := s.createWorkaroundProblem(ctx, req, before, view)
	if problemID != "" {
		if v, err := s.incidentView(ctx, req.ID); err == nil {
			view = v
		}
	}

	// Best-effort ServiceNow mirror write, DATA_SOURCE=postgres-servicenow-dual-write
	// only (guaranteed by the s.snWriteback == nil return just below). Postgres has
	// already committed both comment rows by this point; this fires after,
	// asynchronously, and never affects this response. mirrorReq carries only
	// ID plus the field(s) this call actually set -- never forwards req
	// itself -- so this can never accidentally carry an unsupported field
	// into the mirror call.
	if s.snWriteback == nil {
		// DATA_SOURCE=postgres creating its own incidents: there is no ServiceNow copy to keep in step.
		return domain.UpdateIncidentResponse{Message: "Incident updated successfully", Incident: view}, nil
	}
	mirrorReq := domain.UpdateIncidentRequest{
		ID:                 req.ID,
		State:              req.State,
		AssignedEngineerID: req.AssignedEngineerID,
		ResolutionCode:     req.ResolutionCode,
		ResolutionNotes:    req.ResolutionNotes,
		ResolvedByID:       req.ResolvedByID,
		WorkNotes:          req.WorkNotes,
		AdditionalComments: req.AdditionalComments,
	}
	payload := map[string]any{
		"id": req.ID, "state": req.State, "assignedEngineerId": req.AssignedEngineerID,
		"resolutionCode": req.ResolutionCode, "resolutionNotes": req.ResolutionNotes, "resolvedById": req.ResolvedByID,
		"workNotes": req.WorkNotes, "additionalComments": req.AdditionalComments,
	}
	if problemID != "" {
		// ServiceNow's incident gets the same problem link as Postgres's.
		mirrorReq.ProblemID = &problemID
		payload["problemId"] = problemID
	}
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
