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

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
	"golang.org/x/sync/errgroup"
)

// IncidentRepository defines the read operations for incident (migration
// 000058, extended by 000060), a work_item type extension (id IS
// work_item.id) -- same shared-PK pattern as "case"/change_request.
//
// IncidentView/SearchIncidentView render State/Priority/Category/Subcategory/
// ContactType/ResolutionCode as plain, unvalidated strings (per those types'
// own field comments), and the real enum column text is passed through --
// except where a label differs from the value the API accepts and the
// ServiceNow data source returns: state 'CANCELED', resolution code
// 'SOLVED_WORK_AROUND'/'NOT_ACTIONABLE_ALERT' and contact type 'SITE_24_7'
// are mapped back on read (incidentStateFromEnum and its siblings), so a
// client sees the same values in both data sources. The SEARCH FILTER path
// uses the strict domain enums (SearchIncidentsFilters.Priorities, the
// generic Filters array's "state"), and three of them have real, easy-to-miss
// mismatches against their Postgres enum's actual labels:
//   - incident_state_enum's "canceled" label is spelled with one L
//     ('CANCELED'), not domain.IncidentStateCancelled's two ("CANCELLED").
//   - incident_priority_enum has no 'PLANNING' label at all (only
//     CRITICAL/HIGH/MODERATE/LOW) -- domain.IncidentPriorityPlanning is
//     rejected with a ValidationError on this data source rather than
//     silently dropped or miscast.
//
// See incidentStateToEnum/incidentPriorityToEnum for both mappings.
//
// The specialist handoff writes through ApplySpecialistHandoff, and
// GetIncidentByID derives IncidentView.SpecialistHandoff from its work notes
// and runbook task (specialistHandoffSummary).
//
// CreateIncidentFromServiceNow (below) is the exception, same as
// CaseRepository.CreateCaseFromServiceNow: it backs
// DATA_SOURCE=postgres-servicenow-dual-write's SN-first incident creation,
// where identity comes from ServiceNow rather than being generated here.
type IncidentRepository interface {
	// SupportGroupOfService returns the service's name and support group
	// (Found false when no service has the id, GroupID "" when it has no
	// support group). CreateIncident uses it to derive an incident's
	// assignment group from its service, or from the default service.
	SupportGroupOfService(ctx context.Context, serviceID string) (ServiceSupportGroup, error)
	// IsSupportGroup reports whether groupID is an active group that is the
	// support group of at least one service or is on the configured extra
	// allowlist (incidentAssignableGroupSQL): the only
	// groups CreateIncident accepts as an explicit assignmentGroupId.
	IsSupportGroup(ctx context.Context, groupID string) (bool, error)
	// SearchIncidents returns a filtered, sorted, paginated slice of
	// incidents together with the total count of matching rows before
	// pagination. priorities/states are the already-mapped Postgres enum
	// label form of req.Filters.Priorities/the generic Filters array's
	// "state" entries -- mapping happens in the service layer since it can
	// return a ValidationError (e.g. for PLANNING), never here.
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) ([]domain.SearchIncidentView, int, error)
	// AggregateIncidents returns server-side aggregated counts of incidents
	// per value of groupBy, capped to the top maxGroups buckets with the
	// remainder folded into the returned OthersCount.
	AggregateIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, groupBy string, maxGroups int) (domain.AggregateResponse, error)
	// GetIncidentByID returns the full detail of a single incident by its
	// UUID, or a NotFoundError if no matching row exists.
	GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error)
	// SearchIncidentActivities returns a paginated activity feed for an
	// incident (comments + field changes), newest first.
	SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error)
	// CreateIncidentComment inserts a new comment row for the given incident
	// -- WorkNotes/AdditionalComments side effect of UpdateIncident's
	// DATA_SOURCE=postgres-servicenow-dual-write path (see
	// incidentService.UpdateIncident's own doc comment). Mirrors
	// CaseRepository.CreateCaseComment's INSERT-with-existence-check shape,
	// but scoped to the "incident" subtype table specifically rather than
	// the generic "work_item" table -- unlike CreateCaseComment (whose own
	// doc comment explains why it deliberately checks against work_item,
	// not "case": one comment endpoint backs five different case-like
	// types), this method backs incidents alone, so scoping the existence
	// check to "incident" is strictly more specific with no coverage loss,
	// matching SearchIncidentActivities' own existence check against the
	// "incident" table rather than "work_item". commentType must be
	// CommentTypeWorkNote or CommentTypeComment -- every other
	// domain.CommentType value (including CommentTypeActivity, which is
	// never writer-authored) is rejected with a ValidationError before any
	// query runs. Returns a ValidationError, not a raw FK error, when
	// incidentID does not identify an existing incident. createdBy is the
	// resolved actor's email -- comment.created_by is a free-text VARCHAR,
	// not a UUID FK, matching CreateCaseComment's own convention (see that
	// method's doc comment), so the caller (incidentService.UpdateIncident)
	// resolves the actor and passes the email straight through.
	CreateIncidentComment(ctx context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error)
	// CreateIncidentNotes inserts a work note and/or a public comment on an incident in one
	// transaction: both are saved or neither is, so a retried request never saves one twice.
	// nil or blank texts are skipped.
	CreateIncidentNotes(ctx context.Context, incidentID string, workNotes, additionalComments *string, createdBy string) error
	// RecordIncidentFieldChanges inserts one work_item_activity row per change, all in one
	// statement, attributed to actorEmail -- the write half of SearchIncidentActivities'
	// field_change branch, which nothing wrote on the Postgres data sources before (the same
	// gap CaseRepository.RecordCaseFieldChangeActivity closed for cases). Values longer than
	// the 255-character columns are cut. No changes is a no-op. Callers treat a failure as
	// best-effort: the update it describes has already committed.
	RecordIncidentFieldChanges(ctx context.Context, incidentID string, changes []IncidentFieldChange, actorEmail string) error
	// CreateIncidentFromServiceNow inserts a new incident row (both work_item
	// and "incident"), for DATA_SOURCE=postgres-servicenow-dual-write's SN-first
	// incident creation (see incidentService.createIncidentSNFirst's own doc
	// comment). Unlike CaseRepository.CreateCaseFromServiceNow, no wso2ID
	// parameter exists here: work_item.wso2_id is only required (by the
	// work_item_wso2_id_required_by_type CHECK constraint, migration 0021)
	// for CASE/SERVICE_REQUEST/ANNOUNCEMENT/ENGAGEMENT/
	// SECURITY_REPORT_ANALYSIS -- INCIDENT is deliberately excluded from that
	// list, and ServiceNow's own incident-create response
	// (snCreateIncidentResponse) has no equivalent field to supply one from
	// anyway. id/number/createdBy are exactly what ServiceNow already
	// returned for the incident it just created. id must be a canonical UUID
	// (sysidToUUID(sn sys_id), the same identity convention every
	// DataSource=servicenow response already uses). Returns a
	// ValidationError if id is not a valid UUID or if a row already exists
	// for id/number (unique violation) -- the latter should not happen in
	// practice since ServiceNow only just generated these, but is reported
	// precisely rather than as an opaque infrastructure error if it ever
	// does.
	//
	// Every field the plain CreateIncident writes is written here too, in the
	// same transaction: the CSM database is the record of truth, so nothing
	// the request carries is left for a later sync to fill in. That is the
	// assignment group and assigned engineer, the subcategory
	// (subcategoryValue: its incident_subcategory.value, ServiceNow's choice
	// spelling, resolved under req.Category -- one of another category is a
	// ValidationError), the configuration item (incident.cmdb_ci_id, migration
	// 0185, deliberately not a foreign key), the watch list (ids or emails,
	// resolveIncidentWatchers) and the work note / additional comment.
	CreateIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, subcategoryValue *string, id, number, priority, createdBy string) (domain.CreateIncidentResponse, error)
	// CheckIncidentFromServiceNow runs CreateIncidentFromServiceNow's insert
	// under a placeholder id and number in a transaction that is always rolled
	// back, and returns the error that insert would return (a ValidationError
	// for a referenced id that does not exist, a subcategory of another
	// category, an unknown watcher), or nil. The ServiceNow-first create calls
	// it before ServiceNow creates anything: once ServiceNow has the incident,
	// a refused Postgres insert leaves it there with no CSM row. Nothing is
	// written; the rules are the insert's own, so the two cannot drift.
	CheckIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, subcategoryValue *string, priority, createdBy string) error
	// CreateIncident inserts a new incident row (both work_item and
	// "incident") for the plain-Postgres data source (no ServiceNow at all)
	// -- createIncidentPortalQuery's own doc comment has the full
	// field-by-field reasoning, which mirrors CreateIncidentFromServiceNow's
	// exactly except identity (id/number) is generated here via
	// gen_random_uuid()/next_portal_work_item_number() instead of being
	// supplied by a prior ServiceNow response, and createdBy is the calling
	// user's own resolved email rather than ServiceNow's echoed value.
	//
	// priority is the incident_priority_enum label the service derived from
	// impact x urgency; subcategoryValue is the ServiceNow choice value of
	// req.Subcategory (incident_subcategory.value), resolved to its row here.
	// req.AdditionalComments/WorkNotes become COMMENT/WORK_NOTE rows and
	// req.WatchList becomes work_item_watcher rows, all in the same
	// transaction as the record itself.
	CreateIncident(ctx context.Context, req domain.CreateIncidentRequest, priority string, subcategoryValue *string, createdBy string) (domain.CreateIncidentResponse, error)
	// UpdateIncidentLifecycle writes an incident's state transition and the
	// fields that travel with one -- the PATCH the portal sends to move an
	// incident to In Progress (with an optional assignedEngineerId claim),
	// On Hold, Resolved/Closed (with resolutionCode/resolutionNotes), or
	// Cancelled. See IncidentLifecycleUpdate for the field-by-field rules.
	// One transaction. Returns NotFoundError when id is not an incident the
	// caller can see, and ValidationError for an unknown assignee/resolver,
	// a Resolved/Closed target with no resolution code or notes, or a parent
	// incident that is the incident itself or one of its descendants.
	//
	// Resolving an incident resolves its open child incidents in the same
	// transaction (cascadeChildIncidentResolution); they are returned, so the
	// caller can do for each what it does after its own commit.
	UpdateIncidentLifecycle(ctx context.Context, id string, u IncidentLifecycleUpdate, actorEmail string) ([]ResolvedChildIncident, error)
	// ApplySpecialistHandoff hands an incident to a specialist group in one
	// transaction: it locks the incident, passes its current state to plan
	// (which applies the eligibility rules and may refuse with an error),
	// then moves the incident to plan's group, clears its assignee, opens the
	// runbook task and writes plan's work notes. Returns NotFoundError when
	// id is not an incident, and ValidationError when the group plan names
	// is not in this database.
	ApplySpecialistHandoff(ctx context.Context, id, actorEmail string, plan func(SpecialistHandoffSnapshot) (SpecialistHandoffPlan, error)) (SpecialistHandoffWritten, error)
}

// SpecialistHandoffSnapshot is the incident as ApplySpecialistHandoff found it,
// locked, before the handoff. State is the incident_state_enum label.
type SpecialistHandoffSnapshot struct {
	IncidentID          string
	Number              string
	Subject             string
	Description         *string
	State               string
	ServiceID           *string
	AssignmentGroupID   *string
	AssignmentGroupName *string
	// AssignedToID is the engineer the incident is assigned to; nil when
	// nobody is. A handoff needs one.
	AssignedToID *string
	// AlertNote is the work note sre-alert-core-service creates an
	// alert-born incident with ("<p>Incident auto-created from Alert:
	// ALT…</p>" and the alert's fields as a table), the earliest when there
	// are several; nil for an incident not created from an alert. That
	// service sets no description, so this note carries what ServiceNow's
	// alert flow put in the description.
	AlertNote *string

	// What the handoff's GitHub issue shows about the incident. Priority,
	// Impact, Urgency and Category are their enum labels; any may be nil.
	Priority    *string
	Impact      *string
	Urgency     *string
	Category    *string
	Environment *string
	ServiceName *string
	// OpenedOn is when the incident was opened (its creation time when no
	// opened_on is recorded).
	OpenedOn time.Time
	// RecentWorkNotes are the incident's latest work notes, newest first, at
	// most SpecialistHandoffRecentNotes, leaving out the alert creation note
	// and earlier handoffs' reason notes.
	RecentWorkNotes []SpecialistHandoffNote
}

// SpecialistHandoffNote is one work note as the handoff's GitHub issue lists
// it. Author is the writer's name when this database knows them, else what
// the note recorded (an email).
type SpecialistHandoffNote struct {
	CreatedOn time.Time
	Author    string
	Content   string
}

// SpecialistHandoffRecentNotes is how many work notes the snapshot carries.
const SpecialistHandoffRecentNotes = 3

// AlertCreationNotePrefix opens the work note sre-alert-core-service creates
// an incident with (model.BuildCreationNote there).
const AlertCreationNotePrefix = "<p>Incident auto-created from Alert:"

// handoffReasonNotePrefix opens the reason note a handoff writes
// (handoffReasonBlob in the service).
const handoffReasonNotePrefix = `{"reasonCode":`

// SpecialistHandoffPlan is what a handoff writes: the group the incident
// moves to, the runbook task, and the work notes, in order.
type SpecialistHandoffPlan struct {
	GroupID     string
	TaskSubject string
	// TaskGroupID is the runbook task's assignment group -- the Special Ops
	// group itself, which owns the runbooks now. Nil, or a group not in this
	// database, leaves the task unassigned rather than failing the handoff,
	// as IncidentReportTx.CreateIncidentTask does.
	TaskGroupID *string
	WorkNotes   []string
}

// SpecialistHandoffWritten is what ApplySpecialistHandoff committed.
type SpecialistHandoffWritten struct {
	Before     SpecialistHandoffSnapshot
	GroupName  string
	TaskID     string
	TaskNumber string
}

// IncidentLifecycleUpdate is UpdateIncidentLifecycle's input. Every field is
// optional; nil leaves the column unchanged. Enum fields already carry their
// Postgres label (the service maps domain values -- e.g. CANCELLED to
// 'CANCELED', SOLVED_WORKAROUND to 'SOLVED_WORK_AROUND' -- before calling).
//
// The rules mirror what ServiceNow enforces on the same transitions: no
// state requires an assignee or assignment group (In Progress included), and
// no old-state -> new-state legality is checked (the portal's own
// getLegalNextIncidentStates is a UI guardrail, not an SN rule). Resolved and
// Closed need a resolution code and resolution notes, taken from the request
// or already on the record. Entering Resolved stamps resolved_on, and
// resolved_by_id from ResolvedByID, falling back to DefaultResolvedByID.
// Entering Closed or Canceled also closes the incident's open incident tasks,
// as ServiceNow's "Cascade closure of Incident Tasks" does (see
// cascadeIncidentTaskClosure). Entering Resolved also resolves the open child
// incidents, as ServiceNow's "Update Child Incidents" does (see
// cascadeChildIncidentResolution).
type IncidentLifecycleUpdate struct {
	State               *string // incident_state_enum label
	AssignedEngineerID  *string
	ResolutionCode      *string // incident_resolution_code_enum label
	ResolutionNotes     *string // incident.close_notes
	ResolvedByID        *string
	DefaultResolvedByID *string // the acting user, used only when entering Resolved without ResolvedByID

	// WorkNotes and AdditionalComments are written as comment rows in the same transaction as the
	// state change, so a failed note leaves the state change unsaved too. Nil or blank writes nothing.
	WorkNotes          *string
	AdditionalComments *string

	// The record's own fields, the rest of what the portal's edit dialog
	// sends -- the same set ServiceNow's incident update accepts. Enum fields
	// carry their Postgres label. Nil leaves the column unchanged, as on the
	// ServiceNow path, where a field sent as null is not sent at all.
	Subject             *string // work_item.subject
	ParentID            *string // work_item.parent_id
	AssignmentGroupID   *string // work_item.assignment_group_id
	Category            *string // incident_category_enum label
	SubcategoryValue    *string // incident_subcategory.value, looked up under the new or current category
	ContactType         *string // domain.IncidentContactType; mapped to incident_contact_type_enum here, as on create
	Impact              *string // incident_impact_enum label
	Urgency             *string // incident_urgency_enum label
	Priority            *string // incident_priority_enum label; used only when neither Impact nor Urgency is set
	ServiceID           *string
	ServiceOfferingID   *string
	ConfigurationItemID *string // incident.cmdb_ci_id
	ChangeRequestID     *string
	ProblemID           *string
	CausedByID          *string
	ParentIncidentID    **string // non-nil outer, nil inner unlinks it from its parent
	IncidentReport      *string
	Environment         **string  // non-nil outer, nil inner clears it
	WatchList           *[]string // replaces the whole list, as ServiceNow does; empty clears it

	// PriorityFor is ServiceNow's priority lookup (impact x urgency). When
	// Impact or Urgency is set, priority is recomputed from the new value and
	// the one already on record, as ServiceNow's lookup does on every save.
	PriorityFor func(impact, urgency string) string
}

// ResolvedChildIncident is a child incident UpdateIncidentLifecycle resolved
// because its parent was, at any depth. PreviousState, PreviousResolutionCode
// and ResolutionCode are domain values, as GetIncidentByID reads them; the
// new state is always RESOLVED.
type ResolvedChildIncident struct {
	ID                     string
	Number                 string
	PreviousState          string
	PreviousResolutionCode *string
	ResolutionCode         *string
}

type incidentRepo struct {
	db *Scoped
	// extraGroupIDs is INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS (see
	// incidentAssignableGroupSQL).
	extraGroupIDs []string
}

// SupportGroupOfService implements IncidentRepository.
func (r *incidentRepo) SupportGroupOfService(ctx context.Context, serviceID string) (ServiceSupportGroup, error) {
	var name, group, groupName *string
	err := r.db.QueryRow(ctx, `
		SELECT s.name, s.support_group_id::text, g.name
		FROM service s
		LEFT JOIN "group" g ON g.id = s.support_group_id
		WHERE s.id = $1::uuid`, serviceID).Scan(&name, &group, &groupName)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceSupportGroup{}, nil
	}
	if err != nil {
		return ServiceSupportGroup{}, fmt.Errorf("read support group of service %s: %w", serviceID, err)
	}
	return ServiceSupportGroup{
		Found:       true,
		ServiceName: stringOrEmpty(name),
		GroupID:     stringOrEmpty(group),
		GroupName:   stringOrEmpty(groupName),
	}, nil
}

// IsSupportGroup implements IncidentRepository.
func (r *incidentRepo) IsSupportGroup(ctx context.Context, groupID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (`+incidentAssignableGroupSQL(1)+` AND g.id = $2::uuid)`,
		extraGroupIDsArg(r.extraGroupIDs), groupID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check support group %s: %w", groupID, err)
	}
	return ok, nil
}

// NewIncidentRepository constructs an IncidentRepository backed by the given
// connection pool. extraGroupIDs (optional) are group ids an incident may be
// created in although they support no service (INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS).
func NewIncidentRepository(db *Scoped, extraGroupIDs ...string) IncidentRepository {
	return &incidentRepo{db: db, extraGroupIDs: extraGroupIDs}
}

const incidentFromJoins = `
	FROM work_item wi
	JOIN incident inc ON inc.id = wi.id
	LEFT JOIN incident_subcategory sc ON sc.id = inc.subcategory_id
	LEFT JOIN service svc ON svc.id = inc.service_id
	LEFT JOIN service_offering so ON so.id = inc.service_offering_id
	LEFT JOIN "user" caller ON caller.id = inc.caller_id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN work_item parent_wi ON parent_wi.id = wi.parent_id
	LEFT JOIN incident parent_inc ON parent_inc.id = inc.parent_incident_id
	LEFT JOIN work_item parent_inc_wi ON parent_inc_wi.id = parent_inc.id
	LEFT JOIN change_request cr ON cr.id = inc.change_request_id
	LEFT JOIN work_item cr_wi ON cr_wi.id = cr.id
	LEFT JOIN change_request caused_by_cr ON caused_by_cr.id = inc.caused_by_id
	LEFT JOIN work_item caused_by_wi ON caused_by_wi.id = caused_by_cr.id
	LEFT JOIN problem prob ON prob.id = inc.problem_id
	LEFT JOIN work_item prob_wi ON prob_wi.id = prob.id
	LEFT JOIN "user" rb ON rb.id = inc.resolved_by_id`

func incidentWhereClause(f domain.SearchIncidentsFilters, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) (string, []any) {
	where := "WHERE wi.type = 'INCIDENT'"
	args := []any{}
	argIdx := 1

	add := func(clause string, val any) {
		where += fmt.Sprintf(" AND "+clause, argIdx)
		args = append(args, val)
		argIdx++
	}

	if f.Number != nil && *f.Number != "" {
		add("wi.number = $%d", *f.Number)
	}
	if f.CorrelationID != nil && *f.CorrelationID != "" {
		add("inc.correlation_id = $%d", *f.CorrelationID)
	}
	if f.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (wi.subject ILIKE $%d ESCAPE '\\' OR wi.number ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}
	if len(priorities) > 0 {
		add("inc.priority = ANY($%d::text[]::incident_priority_enum[])", priorities)
	}
	if len(f.ParentIDs) > 0 {
		add("wi.parent_id = ANY($%d::uuid[])", f.ParentIDs)
	}
	if len(f.ParentIncidentIDs) > 0 {
		add("inc.parent_incident_id = ANY($%d::uuid[])", f.ParentIncidentIDs)
	}
	if len(states) > 0 {
		add("inc.state = ANY($%d::text[]::incident_state_enum[])", states)
	}
	if len(serviceIDs) > 0 {
		add("inc.service_id = ANY($%d::uuid[])", serviceIDs)
	}
	if len(assignedUserIDs) > 0 {
		add("wi.assigned_to_id = ANY($%d::uuid[])", assignedUserIDs)
	}
	if madeSla != nil {
		add("inc.is_sla_met = $%d", *madeSla)
	}
	if slaViolated != nil {
		where += slaBreachedPredicate("wi.id", *slaViolated)
	}
	{
		var slaClause string
		slaClause, args, argIdx = slaPercentPredicate("wi.id", f.ParsedTaskSLA, args, argIdx)
		where += slaClause
	}
	if createdStartDate != nil {
		add("wi.created_on >= $%d", *createdStartDate)
	}
	if createdEndDate != nil {
		add("wi.created_on <= $%d", *createdEndDate)
	}
	if len(f.AssignmentGroupIDs) > 0 {
		add("wi.assignment_group_id = ANY($%d::uuid[])", f.AssignmentGroupIDs)
	}
	if len(f.Categories) > 0 {
		// Text, not an enum cast: a label the enum does not have matches
		// nothing instead of failing the query.
		add("inc.category::TEXT = ANY($%d::text[])", f.Categories)
	}
	if f.UpdatedStart != nil {
		add("wi.updated_on >= $%d", *f.UpdatedStart)
	}
	if f.UpdatedEnd != nil {
		add("wi.updated_on <= $%d", *f.UpdatedEnd)
	}
	// anyOf: each branch is the AND of its own conditions, the branches are
	// OR'd, and the whole is ANDed with everything above (case search's shape).
	if len(f.OrGroups) > 0 {
		branches := make([]string, 0, len(f.OrGroups))
		for _, g := range f.OrGroups {
			var preds []string
			bind := func(clause string, val any) {
				preds = append(preds, fmt.Sprintf(clause, argIdx))
				args = append(args, val)
				argIdx++
			}
			if len(g.States) > 0 {
				bind("inc.state::TEXT = ANY($%d::text[])", g.States)
			}
			if len(g.Priorities) > 0 {
				bind("inc.priority::TEXT = ANY($%d::text[])", g.Priorities)
			}
			if len(g.Categories) > 0 {
				bind("inc.category::TEXT = ANY($%d::text[])", g.Categories)
			}
			if len(g.ServiceIDs) > 0 {
				bind("inc.service_id = ANY($%d::uuid[])", g.ServiceIDs)
			}
			if len(g.AssignmentGroupIDs) > 0 {
				bind("wi.assignment_group_id = ANY($%d::uuid[])", g.AssignmentGroupIDs)
			}
			if len(g.AssignedUserIDs) > 0 {
				bind("wi.assigned_to_id = ANY($%d::uuid[])", g.AssignedUserIDs)
			}
			if len(preds) == 0 {
				branches = append(branches, "TRUE")
				continue
			}
			branches = append(branches, "("+strings.Join(preds, " AND ")+")")
		}
		where += " AND (" + strings.Join(branches, " OR ") + ")"
	}
	// productName has no confirmed product-to-incident mapping -- deliberately
	// not applied here. businessServiceId IS applied above, mapped to
	// inc.service_id -- "business service" is ServiceNow's own name for
	// what this schema calls "service" (service_offering.parent_id in
	// service_offering_repo.go already makes the same identification).

	return where, args
}

const incidentSelectColumns = `
	wi.id, wi.number, wi.subject, inc.opened_on,
	caller.id, COALESCE(caller.name, NULLIF(TRIM(CONCAT_WS(' ', caller.first_name, caller.last_name)), '')),
	inc.priority::TEXT, inc.state::TEXT, inc.category::TEXT,
	parent_wi.id, parent_wi.number,
	parent_inc.id, parent_inc_wi.number,
	ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
	wi.created_on, wi.created_by, wi.updated_on, wi.updated_by,
	sag.id, sag.name`

// incidentSearchGroupJoin resolves the assignment group the search list shows
// (its own alias: GetIncidentByID joins "group" as ag after incidentFromJoins).
const incidentSearchGroupJoin = `
	LEFT JOIN "group" sag ON sag.id = wi.assignment_group_id`

func scanSearchIncidentView(row interface{ Scan(...any) error }) (domain.SearchIncidentView, error) {
	var (
		id, number, subject          string
		openedOn                     *time.Time
		callerID, callerName         *string
		priority, state, category    *string
		parentID, parentNumber       *string
		parentIncID, parentIncNumber *string
		aeID, aeName                 *string
		createdOn, updatedOn         time.Time
		createdBy, updatedBy         string
		agID, agName                 *string
	)
	if err := row.Scan(
		&id, &number, &subject, &openedOn,
		&callerID, &callerName,
		&priority, &state, &category,
		&parentID, &parentNumber,
		&parentIncID, &parentIncNumber,
		&aeID, &aeName,
		&createdOn, &createdBy, &updatedOn, &updatedBy,
		&agID, &agName,
	); err != nil {
		return domain.SearchIncidentView{}, err
	}
	v := domain.SearchIncidentView{
		ID: &id, Number: &number, Subject: &subject,
		Priority: priority, State: incidentStateFromEnum(state), Category: category,
		CreatedOn: createdOn.UTC().Format(time.RFC3339), CreatedBy: createdBy,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339), UpdatedBy: updatedBy,
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		v.OpenedOn = &s
	}
	if callerID != nil {
		v.Caller = &domain.EntityRef{ID: *callerID, Name: stringOrEmpty(callerName)}
	}
	if parentID != nil {
		v.Parent = &domain.EntityRef{ID: *parentID, Name: stringOrEmpty(parentNumber)}
	}
	if parentIncID != nil {
		v.ParentIncident = &domain.EntityRef{ID: *parentIncID, Name: stringOrEmpty(parentIncNumber)}
	}
	if aeID != nil {
		v.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		v.AssignmentGroup = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	return v, nil
}

// SearchIncidents implements IncidentRepository.
//
// crvis: internal callers only: /incidents routes are wrapped by internalOnly (server/routes.go); the change_request join only names the linked change request
func (r *incidentRepo) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time) ([]domain.SearchIncidentView, int, error) {
	where, args := incidentWhereClause(req.Filters, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate)

	sortCol := "wi.created_on"
	switch req.SortBy.Field {
	case domain.IncidentSortFieldUpdatedOn:
		sortCol = "wi.updated_on"
	case domain.IncidentSortFieldOpenedOn:
		sortCol = "inc.opened_on"
	}
	sortDir := "DESC"
	if req.SortBy.Order == domain.IncidentSortOrderAsc {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + incidentFromJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT %s %s %s %s ORDER BY %s %s, wi.id LIMIT $%d OFFSET $%d`,
		incidentSelectColumns, incidentFromJoins, incidentSearchGroupJoin, where, sortCol, sortDir, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchIncidentView

	eg, egCtx := errgroup.WithContext(ctx)

	// SkipTotal: the caller does not show a total (global search shows a handful
	// of hits), so the COUNT is not run at all -- it is as costly as the page
	// query and holds a second pool connection while it runs.
	if req.SkipTotal {
		total = domain.TotalNotComputed
	} else {
		eg.Go(func() error {
			if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
				return fmt.Errorf("count incidents: %w", err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query incidents: %w", err)
		}
		defer rows.Close()

		result := make([]domain.SearchIncidentView, 0, req.Pagination.Limit)
		for rows.Next() {
			v, err := scanSearchIncidentView(rows)
			if err != nil {
				return fmt.Errorf("scan incident: %w", err)
			}
			result = append(result, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate incidents: %w", err)
		}
		views = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// incidentAggregateColumns maps a groupBy value to the real column/cast
// used to group by it -- the three values openapi.yaml's
// AggregateIncidentsRequest.groupBy allows are "state"/"assignmentGroup"/
// "businessService". "assignmentGroup" is deliberately absent (no backing
// column); "businessService" maps to inc.service_id, the same
// businessServiceId/service_id identification incidentWhereClause's own
// doc comment makes for the filter of the same name.
var incidentAggregateColumns = map[string]string{
	"state":           "inc.state::TEXT",
	"businessService": "inc.service_id::TEXT",
}

// AggregateIncidents implements IncidentRepository.
//
// crvis: internal callers only: /incidents routes are wrapped by internalOnly (server/routes.go); the change_request join only names the linked change request
func (r *incidentRepo) AggregateIncidents(ctx context.Context, req domain.SearchIncidentsRequest, priorities, states, serviceIDs, assignedUserIDs []string, madeSla, slaViolated *bool, createdStartDate, createdEndDate *time.Time, groupBy string, maxGroups int) (domain.AggregateResponse, error) {
	col, ok := incidentAggregateColumns[groupBy]
	if !ok {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy=" + groupBy + " is not supported on the PostgreSQL data source"}
	}

	where, args := incidentWhereClause(req.Filters, priorities, states, serviceIDs, assignedUserIDs, madeSla, slaViolated, createdStartDate, createdEndDate)

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS bucket_count
		%s %s AND %s IS NOT NULL
		GROUP BY %s
		ORDER BY bucket_count DESC, bucket`, col, incidentFromJoins, where, col, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate incidents: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	var totalRecords int
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan incident bucket: %w", err)
		}
		if groupBy == "state" {
			key = *incidentStateFromEnum(&key)
		}
		lowerKey := strings.ToLower(key)
		buckets = append(buckets, domain.AggregateBucket{Key: lowerKey, Label: lowerKey, Count: count})
		totalRecords += count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate incident buckets: %w", err)
	}

	if maxGroups <= 0 || maxGroups >= len(buckets) {
		return domain.AggregateResponse{Groups: buckets, TotalRecords: totalRecords}, nil
	}

	othersCount := 0
	for _, b := range buckets[maxGroups:] {
		othersCount += b.Count
	}
	return domain.AggregateResponse{
		Groups:       buckets[:maxGroups],
		OthersCount:  othersCount,
		TotalRecords: totalRecords,
	}, nil
}

// GetIncidentByID implements IncidentRepository.
//
// crvis: internal callers only: /incidents routes are wrapped by internalOnly (server/routes.go); the change_request join only names the linked change request
func (r *incidentRepo) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	query := `
		SELECT wi.id, wi.number, wi.subject, inc.opened_on,
		       caller.id, COALESCE(caller.name, NULLIF(TRIM(CONCAT_WS(' ', caller.first_name, caller.last_name)), '')),
		       inc.priority::TEXT, inc.state::TEXT, inc.category::TEXT, sc.label,
		       parent_wi.id, parent_wi.number,
		       parent_inc.id, parent_inc_wi.number,
		       ae.id, COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		       svc.id, svc.name, so.id, so.name,
		       inc.contact_type::TEXT,
		       inc.impact::TEXT, inc.urgency::TEXT,
		       cr.id, cr_wi.number, prob.id, prob_wi.number,
		       caused_by_cr.id, caused_by_wi.number,
		       inc.resolution_code::TEXT, inc.close_notes,
		       rb.id, COALESCE(rb.name, NULLIF(TRIM(CONCAT_WS(' ', rb.first_name, rb.last_name)), '')),
		       inc.resolved_on, inc.resolved_with_parent, inc.incident_report, wi.description,
		       wi.created_on, wi.created_by, wi.updated_on, wi.updated_by,
		       ag.id, ag.name
		` + incidentFromJoins + `
		LEFT JOIN "group" ag ON ag.id = wi.assignment_group_id
		WHERE wi.id = $1 AND wi.type = 'INCIDENT'`

	var (
		id2, number, subject               string
		openedOn                           *time.Time
		callerID, callerName               *string
		priority, state, category, subcatL *string
		parentID, parentNumber             *string
		parentIncID, parentIncNumber       *string
		aeID, aeName                       *string
		svcID, svcName, soID, soName       *string
		contactType                        *string
		impact, urgency                    *string
		crID, crNumber, probID, probNumber *string
		causedByID, causedByNumber         *string
		resolutionCode, closeNotes         *string
		rbID, rbName                       *string
		resolvedOn                         *time.Time
		resolvedWithParent                 bool
		incidentReport                     *string
		description                        *string
		createdOn, updatedOn               time.Time
		createdBy, updatedBy               string
		agID, agName                       *string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &subject, &openedOn,
		&callerID, &callerName,
		&priority, &state, &category, &subcatL,
		&parentID, &parentNumber,
		&parentIncID, &parentIncNumber,
		&aeID, &aeName,
		&svcID, &svcName, &soID, &soName,
		&contactType,
		&impact, &urgency,
		&crID, &crNumber, &probID, &probNumber,
		&causedByID, &causedByNumber,
		&resolutionCode, &closeNotes,
		&rbID, &rbName,
		&resolvedOn, &resolvedWithParent, &incidentReport, &description,
		&createdOn, &createdBy, &updatedOn, &updatedBy,
		&agID, &agName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IncidentView{}, &apierror.NotFoundError{Msg: "incident not found"}
	}
	if err != nil {
		return domain.IncidentView{}, fmt.Errorf("get incident: %w", err)
	}

	v := domain.IncidentView{
		ID: &id2, Number: &number, Subject: &subject,
		Priority: priority, State: incidentStateFromEnum(state), Category: category, Subcategory: subcatL,
		ContactType: incidentContactTypeFromEnum(contactType), Impact: impact, Urgency: urgency,
		ResolutionCode: incidentResolutionCodeFromEnum(resolutionCode), ResolutionNotes: closeNotes, IncidentReport: incidentReport,
		ResolvedWithParent:    resolvedWithParent,
		Description:           description,
		WatchList:             []domain.IncidentWatchListItem{},
		LinkedServiceRequests: []domain.LinkedServiceRequestRef{},
		CreatedOn:             createdOn.UTC().Format(time.RFC3339), CreatedBy: createdBy,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339), UpdatedBy: updatedBy,
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		v.OpenedOn = &s
	}
	if callerID != nil {
		v.Caller = &domain.EntityRef{ID: *callerID, Name: stringOrEmpty(callerName)}
	}
	if parentID != nil {
		v.Parent = &domain.EntityRef{ID: *parentID, Name: stringOrEmpty(parentNumber)}
	}
	if parentIncID != nil {
		v.ParentIncident = &domain.EntityRef{ID: *parentIncID, Name: stringOrEmpty(parentIncNumber)}
	}
	if aeID != nil {
		v.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		v.AssignmentGroup = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	if svcID != nil {
		v.Service = &domain.EntityRef{ID: *svcID, Name: stringOrEmpty(svcName)}
	}
	if soID != nil {
		v.ServiceOffering = &domain.EntityRef{ID: *soID, Name: stringOrEmpty(soName)}
	}
	if crID != nil {
		v.ChangeRequest = &domain.EntityRef{ID: *crID, Name: stringOrEmpty(crNumber)}
	}
	if probID != nil {
		v.Problem = &domain.EntityRef{ID: *probID, Name: stringOrEmpty(probNumber)}
	}
	if causedByID != nil {
		v.CausedBy = &domain.EntityRef{ID: *causedByID, Name: stringOrEmpty(causedByNumber)}
	}
	if resolvedOn != nil {
		s := resolvedOn.UTC().Format(time.RFC3339)
		v.ResolvedOn = &s
	}
	if rbID != nil {
		name := stringOrEmpty(rbName)
		v.ResolvedBy = &name
	}
	// Derived at read time from the handoff's work notes and runbook task,
	// as ServiceNow's getHandoffSummary does; nothing is stored for it.
	sum, err := r.specialistHandoffSummary(ctx, id2, v.AssignmentGroup)
	if err != nil {
		return domain.IncidentView{}, err
	}
	v.SpecialistHandoff = sum
	watchers, err := incidentWatchers(ctx, r.db, id2)
	if err != nil {
		return domain.IncidentView{}, err
	}
	v.WatchList = watchers
	return v, nil
}

// incidentWatchers reads the incident's watch list from work_item_watcher, the
// table UpdateIncidentLifecycle's watchList replaces, so a caller that adds to
// or removes from the list it reads (the list's bulk edit) starts from what is
// stored rather than from nothing. Ordered by name, then id, for a stable list.
func incidentWatchers(ctx context.Context, q rowsQuerier, incidentID string) ([]domain.IncidentWatchListItem, error) {
	rows, err := q.Query(ctx, `
		SELECT u.id::text,
		       COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.user_name, ''),
		       COALESCE(u.email, '')
		FROM work_item_watcher w
		JOIN "user" u ON u.id = w.user_id
		WHERE w.work_item_id = $1
		ORDER BY 2, 1`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("get incident watch list: %w", err)
	}
	defer rows.Close()
	watchers := []domain.IncidentWatchListItem{}
	for rows.Next() {
		var w domain.IncidentWatchListItem
		if err := rows.Scan(&w.ID, &w.Name, &w.Email); err != nil {
			return nil, fmt.Errorf("get incident watch list: %w", err)
		}
		watchers = append(watchers, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get incident watch list: %w", err)
	}
	return watchers, nil
}

// recentHandoffNotes reads the incident's latest work notes for the handoff's
// GitHub issue: newest first, without the alert creation note (the issue
// already shows it as the description) or earlier handoffs' reason notes.
// The author is the writer's name when a user with that email is known.
func recentHandoffNotes(ctx context.Context, tx pgx.Tx, incidentID string) ([]SpecialistHandoffNote, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.created_on, COALESCE(c.content, ''),
		       COALESCE(NULLIF(btrim(concat_ws(' ', u.first_name, u.last_name)), ''), c.created_by, '')
		FROM comment c
		LEFT JOIN LATERAL (
			SELECT first_name, last_name FROM "user"
			WHERE lower(email) = lower(c.created_by)
			ORDER BY is_active DESC NULLS LAST LIMIT 1
		) u ON true
		WHERE c.work_item_id = $1 AND c.type = 'WORK_NOTE'::comment_type_enum
		  AND NOT starts_with(COALESCE(c.content, ''), $2)
		  AND NOT starts_with(COALESCE(c.content, ''), $3)
		ORDER BY c.created_on DESC, c.id DESC
		LIMIT $4`, incidentID, AlertCreationNotePrefix, handoffReasonNotePrefix, SpecialistHandoffRecentNotes)
	if err != nil {
		return nil, fmt.Errorf("specialist handoff: read work notes: %w", err)
	}
	defer rows.Close()
	var notes []SpecialistHandoffNote
	for rows.Next() {
		var n SpecialistHandoffNote
		if err := rows.Scan(&n.CreatedOn, &n.Content, &n.Author); err != nil {
			return nil, fmt.Errorf("specialist handoff: read work notes: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("specialist handoff: read work notes: %w", err)
	}
	return notes, nil
}

// ApplySpecialistHandoff implements IncidentRepository.
//
// WithSystemIdentity: the route is internal-only, and the new runbook task's
// work_item insert is admitted by work_item's RLS insert policy (0147) only
// as internal -- the same stamp CreateIncidentFromServiceNow uses.
func (r *incidentRepo) ApplySpecialistHandoff(ctx context.Context, id, actorEmail string, plan func(SpecialistHandoffSnapshot) (SpecialistHandoffPlan, error)) (SpecialistHandoffWritten, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (SpecialistHandoffWritten, error) {
		var snap SpecialistHandoffSnapshot
		err := tx.QueryRow(ctx, `
			SELECT wi.id::text, wi.number, wi.subject, wi.description, inc.state::text,
			       inc.service_id::text, wi.assignment_group_id::text, g.name, wi.assigned_to_id::text,
			       (SELECT c.content FROM comment c
			         WHERE c.work_item_id = wi.id AND c.type = 'WORK_NOTE'::comment_type_enum
			           AND starts_with(c.content, $2)
			         ORDER BY c.created_on, c.id LIMIT 1),
			       inc.priority::text, inc.impact::text, inc.urgency::text, inc.category::text,
			       NULLIF(btrim(inc.environment), ''), s.name, COALESCE(inc.opened_on, wi.created_on)
			FROM incident inc
			JOIN work_item wi ON wi.id = inc.id
			LEFT JOIN "group" g ON g.id = wi.assignment_group_id
			LEFT JOIN service s ON s.id = inc.service_id
			WHERE inc.id = $1 AND wi.type = 'INCIDENT'
			FOR UPDATE OF inc, wi`, id, AlertCreationNotePrefix).Scan(
			&snap.IncidentID, &snap.Number, &snap.Subject, &snap.Description, &snap.State,
			&snap.ServiceID, &snap.AssignmentGroupID, &snap.AssignmentGroupName, &snap.AssignedToID,
			&snap.AlertNote,
			&snap.Priority, &snap.Impact, &snap.Urgency, &snap.Category,
			&snap.Environment, &snap.ServiceName, &snap.OpenedOn)
		if errors.Is(err, pgx.ErrNoRows) {
			return SpecialistHandoffWritten{}, &apierror.NotFoundError{Msg: "incident not found"}
		}
		if err != nil {
			return SpecialistHandoffWritten{}, fmt.Errorf("specialist handoff: read incident: %w", err)
		}
		if snap.RecentWorkNotes, err = recentHandoffNotes(ctx, tx, snap.IncidentID); err != nil {
			return SpecialistHandoffWritten{}, err
		}
		p, err := plan(snap)
		if err != nil {
			return SpecialistHandoffWritten{}, err
		}

		out := SpecialistHandoffWritten{Before: snap}
		err = tx.QueryRow(ctx, `
			WITH moved AS (
				UPDATE work_item
				SET assignment_group_id = $2::uuid, assigned_to_id = NULL, updated_on = NOW(), updated_by = $3
				WHERE id = $1
				RETURNING assignment_group_id
			)
			SELECT COALESCE(g.name, '') FROM moved LEFT JOIN "group" g ON g.id = moved.assignment_group_id`,
			id, p.GroupID, actorEmail).Scan(&out.GroupName)
		if err != nil {
			if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return SpecialistHandoffWritten{}, &apierror.ValidationError{Msg: "specialist group " + p.GroupID + " does not exist in this database"}
			}
			return SpecialistHandoffWritten{}, fmt.Errorf("specialist handoff: move incident: %w", err)
		}

		// The runbook task: a TASK-numbered incident_task (migration 0201), OPEN and active,
		// CRITICAL (ServiceNow's priority 1), on the incident's service.
		err = tx.QueryRow(ctx, `
			WITH inserted_work_item AS (
				INSERT INTO work_item (
					id, created_on, updated_on, created_by, updated_by,
					number, subject, type, assignment_group_id
				)
				VALUES (
					gen_random_uuid(), NOW(), NOW(), $1, $1,
					next_work_item_number('INCIDENT_TASK'), $2, 'INCIDENT_TASK'::work_item_type_enum,
					(SELECT g.id FROM "group" g WHERE g.id = $3::uuid)
				)
				RETURNING id, number
			),
			inserted_task AS (
				INSERT INTO incident_task (id, opened_on, priority, state, is_active, incident_id, service_id, type)
				SELECT id, NOW(), 'CRITICAL', 'OPEN', TRUE, $4::uuid, $5::uuid, 'DEFAULT'
				FROM inserted_work_item
				RETURNING id
			)
			SELECT iwi.id::text, iwi.number
			FROM inserted_work_item iwi
			JOIN inserted_task it ON it.id = iwi.id`,
			actorEmail, p.TaskSubject, p.TaskGroupID, id, snap.ServiceID).Scan(&out.TaskID, &out.TaskNumber)
		if err != nil {
			return SpecialistHandoffWritten{}, fmt.Errorf("specialist handoff: create runbook task: %w", err)
		}

		// clock_timestamp, not NOW(): the notes must keep their order, and
		// NOW() is the same instant for every statement in the transaction.
		for _, note := range p.WorkNotes {
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
				VALUES (gen_random_uuid(), clock_timestamp(), $1, $2::comment_type_enum, $3, $4)`,
				actorEmail, caseCommentTypeEnum[domain.CommentTypeWorkNote], id, note); err != nil {
				return SpecialistHandoffWritten{}, fmt.Errorf("specialist handoff: work note: %w", err)
			}
		}
		return out, nil
	})
}

// specialistHandoffSummary ports ServiceNow's IncidentHandoffUtils
// .getHandoffSummary: the newest work note holding a handoff reason blob
// ({"reasonCode":...}) is the handoff; the GitHub link comes from the oldest
// "Escalated to Special Ops team." note written after it; the task is the
// incident's latest "[Runbook Task]" task. Nil when the incident was never
// handed off. group is the incident's current assignment group.
func (r *incidentRepo) specialistHandoffSummary(ctx context.Context, id string, group *domain.EntityRef) (*domain.IncidentSpecialistHandoffSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT content, created_on, created_by
		FROM comment
		WHERE work_item_id = $1 AND type = $2::comment_type_enum
		  AND (content LIKE '%"reasonCode"%' OR content LIKE 'Escalated to Special Ops team.%')
		ORDER BY created_on DESC`, id, caseCommentTypeEnum[domain.CommentTypeWorkNote])
	if err != nil {
		return nil, fmt.Errorf("specialist handoff summary: %w", err)
	}
	defer rows.Close()

	var (
		blob          *specialistHandoffBlob
		at            time.Time
		by            string
		noteAfterBlob string
	)
	for rows.Next() {
		var content, createdBy string
		var createdOn time.Time
		if err := rows.Scan(&content, &createdOn, &createdBy); err != nil {
			return nil, fmt.Errorf("specialist handoff summary: scan: %w", err)
		}
		if !strings.Contains(content, `"reasonCode"`) {
			// Newest first, so these were written after the blob; keep the
			// oldest of them -- the note the handoff itself wrote.
			if strings.HasPrefix(content, "Escalated to Special Ops team.") {
				noteAfterBlob = content
			}
			continue
		}
		var b specialistHandoffBlob
		if json.Unmarshal([]byte(content), &b) == nil && b.ReasonCode != "" {
			blob, at, by = &b, createdOn, createdBy
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("specialist handoff summary: %w", err)
	}
	if blob == nil {
		return nil, nil
	}

	sum := &domain.IncidentSpecialistHandoffSummary{
		ReasonCode:        domain.IncidentSpecialistHandoffReasonCode(blob.ReasonCode),
		ReasonDescription: blob.ReasonDescription,
		HandedOffAt:       at.UTC().Format(time.RFC3339),
		HandedOffBy:       &by,
	}
	// The team as the blob names it; the service keeps it only when the
	// handoff configuration knows it, as getHandoffSummary keeps only the
	// teams IncidentHandoffUtils knows.
	if team := stringOrEmpty(blob.EscalationTeam); team != "" {
		t := domain.IncidentSpecialistHandoffEscalationTeam(team)
		sum.EscalationTeam = &t
	}
	if m := githubIssueURLPattern.FindString(noteAfterBlob); m != "" {
		u := strings.TrimRight(m, ").,")
		sum.GithubIssueURL = &u
	}
	if group != nil {
		sum.AssignmentGroup = *group
	}

	var number, subject string
	var state *string
	err = r.db.QueryRow(ctx, `
		SELECT wi.number, wi.subject, it.state::text
		FROM incident_task it
		JOIN work_item wi ON wi.id = it.id
		WHERE it.incident_id = $1 AND wi.subject LIKE '[Runbook Task]%'
		ORDER BY wi.created_on DESC
		LIMIT 1`, id).Scan(&number, &subject, &state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("specialist handoff summary: runbook task: %w", err)
	}
	if err == nil {
		sum.Task = domain.IncidentSpecialistHandoffSummaryTask{Number: number, Subject: subject}
		if state != nil {
			label := incidentTaskStateDisplay(*state)
			sum.Task.State, sum.Task.StateLabel = state, &label
		}
	}
	return sum, nil
}

// specialistHandoffBlob is the reason a handoff writes as its first work note
// -- the JSON the "Escalate to Special Ops" modal submits.
type specialistHandoffBlob struct {
	ReasonCode        string  `json:"reasonCode"`
	ReasonDescription string  `json:"reasonDescription"`
	EscalationTeam    *string `json:"escalationTeam"`
}

var githubIssueURLPattern = regexp.MustCompile(`https://github\.com/\S+`)

// SearchIncidentActivities implements IncidentRepository. Reuses
// scanCaseActivity's exact query/column shape (case_repo.go) -- an activity
// feed entry (comment or field change) is not inherently case-specific, and
// work_item_activity/comment are both keyed by the generic work_item_id.
// There are no incident attachments table equivalent to case_attachment
// (that table is case-specific by name and FK), so this feed never has an
// "attachment" kind entry, unlike SearchCaseActivities.
func (r *incidentRepo) SearchIncidentActivities(ctx context.Context, req domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error) {
	// Confirm req.IncidentID is actually an incident before reading its
	// activity feed -- comment/work_item_activity are both keyed by the
	// generic work_item_id with no type filter of their own, so without
	// this check a caller could pass any other work_item's UUID (a case,
	// change request, ...) through this endpoint and read that record's
	// comments/field changes instead.
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM incident WHERE id = $1)`, req.IncidentID).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check incident exists: %w", err)
	}
	if !exists {
		return nil, 0, &apierror.NotFoundError{Msg: "incident not found"}
	}

	includeFieldChanges := req.IncludeFieldChanges != nil && *req.IncludeFieldChanges

	countQuery := `SELECT (SELECT COUNT(*) FROM comment WHERE work_item_id = $1)`
	if includeFieldChanges {
		countQuery += ` + (SELECT COUNT(*) FROM work_item_activity WHERE work_item_id = $1)`
	}

	dataQuery := `
		WITH activity AS (
			SELECT
				c.id, 'comment' AS kind, c.content, c.created_on AS created_on,
				c.email, c.first_name, c.last_name, c.name, c.type::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				NULL::text AS field_name, NULL::text AS old_value, NULL::text AS new_value
			FROM (
				SELECT
					cm.id, cm.content, cm.created_on, cm.created_by AS email,
					u1.first_name, u1.last_name,
					COALESCE(u1.name, NULLIF(TRIM(CONCAT_WS(' ', u1.first_name, u1.last_name)), '')) AS name,
					cm.type
				FROM comment cm
				` + userByEmailJoin("u1", "cm.created_by") + `
				WHERE cm.work_item_id = $1
			) c`
	if includeFieldChanges {
		dataQuery += `

			UNION ALL

			SELECT
				fc.id, 'field_change' AS kind, '' AS content, fc.created_on AS created_on,
				fc.email, fc.first_name, fc.last_name, fc.name,
				NULL::text AS comment_type,
				NULL::text AS file_name, NULL::text AS content_type, NULL::bigint AS size_bytes,
				fc.field_name, fc.old_value, fc.new_value
			FROM (
				SELECT
					wa.id, wa.created_on, wa.user_email AS email,
					u3.first_name, u3.last_name,
					COALESCE(u3.name, NULLIF(TRIM(CONCAT_WS(' ', u3.first_name, u3.last_name)), '')) AS name,
					wa.field_name, wa.old_value, wa.new_value
				FROM work_item_activity wa
				` + userByEmailJoin("u3", "wa.user_email") + `
				WHERE wa.work_item_id = $1
			) fc`
	}
	dataQuery += `
		)
		SELECT id, kind, content, created_on, email, first_name, last_name, name, comment_type, file_name, content_type, size_bytes, field_name, old_value, new_value
		FROM activity
		ORDER BY created_on DESC, id
		LIMIT $2 OFFSET $3`

	var total int
	var activity []domain.CaseActivity

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, req.IncidentID).Scan(&total); err != nil {
			return fmt.Errorf("count incident activities: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, req.IncidentID, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query incident activities: %w", err)
		}
		defer rows.Close()

		result := make([]domain.CaseActivity, 0, req.Pagination.Limit)
		for rows.Next() {
			a, err := scanCaseActivity(rows)
			if err != nil {
				return fmt.Errorf("scan incident activity: %w", err)
			}
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate incident activities: %w", err)
		}
		activity = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return activity, total, nil
}

// incidentContactTypeToEnum maps domain.IncidentContactType to
// incident_contact_type_enum's real labels (migration 0058) -- identity
// for every value except "Site 24/7", where the enum spells it
// 'SITE_24_7' but domain.IncidentContactTypeSite247 spells it "SITE_247".
func incidentContactTypeToEnum(c domain.IncidentContactType) string {
	if c == domain.IncidentContactTypeSite247 {
		return "SITE_24_7"
	}
	return string(c)
}

// incidentContactTypeFromEnum is incidentContactTypeToEnum's inverse, for
// reads: the enum's 'SITE_24_7' goes back out as "SITE_247", so a channel
// (the UI's name for contact type) round-trips to the same value the API
// accepted, and the one the webapp's/microapp's option lists use. Every
// other label is passed through unchanged.
func incidentContactTypeFromEnum(label *string) *string {
	if label != nil && *label == "SITE_24_7" {
		v := string(domain.IncidentContactTypeSite247)
		return &v
	}
	return label
}

// incidentStateFromEnum is incidentStateToEnum's (incident_service.go)
// inverse, for reads: the enum's 'CANCELED' goes back out as "CANCELLED",
// the value the API accepts and the ServiceNow data source returns. Every
// other label is passed through unchanged.
func incidentStateFromEnum(label *string) *string {
	if label != nil && *label == "CANCELED" {
		v := string(domain.IncidentStateCancelled)
		return &v
	}
	return label
}

// incidentResolutionCodeFromEnum is incidentResolutionCodeToEnum's
// (incident_service.go) inverse, for reads: 'SOLVED_WORK_AROUND' and
// 'NOT_ACTIONABLE_ALERT' go back out as "SOLVED_WORKAROUND" and
// "NOT_ACTIONABLE". Every other label is passed through unchanged.
func incidentResolutionCodeFromEnum(label *string) *string {
	if label == nil {
		return nil
	}
	var v string
	switch *label {
	case "SOLVED_WORK_AROUND":
		v = string(domain.IncidentResolutionCodeSolvedWorkaround)
	case "NOT_ACTIONABLE_ALERT":
		v = string(domain.IncidentResolutionCodeNotActionable)
	default:
		return label
	}
	return &v
}

// createIncidentCommentQuery mirrors createCaseCommentQuery's (case_repo.go,
// inline in CreateCaseComment) INSERT ... SELECT shape: the SELECT's WHERE
// confirms the referenced row exists in the same round trip, RETURNING zero
// rows (not a hard-to-attribute FK error) when it doesn't. Joins against
// "incident" specifically, not the generic "work_item" table -- see
// CreateIncidentComment's own doc comment (in the IncidentRepository
// interface, above) for why that's safe and correct here even though
// CreateCaseComment itself deliberately checks the broader work_item table
// instead.
const createIncidentCommentQuery = `
	INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
	SELECT gen_random_uuid(), NOW(), $1, $2::comment_type_enum, i.id, $4
	FROM incident i
	WHERE i.id = $3
	RETURNING id, work_item_id, type, content, created_by, created_on`

// CreateIncidentComment implements IncidentRepository.
func (r *incidentRepo) CreateIncidentComment(ctx context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
	// CommentTypeActivity is never writer-authored (same restriction
	// CreateCaseComment enforces) -- and this method's only real caller
	// (UpdateIncident's WorkNotes/AdditionalComments branches) never passes
	// anything else, but the guard stays here rather than relying solely on
	// the service layer, matching CreateCaseComment's own defense-in-depth.
	if commentType == domain.CommentTypeActivity {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: `type "activity" is not writable through this endpoint`}
	}
	typeEnum, ok := caseCommentTypeEnum[commentType]
	if !ok {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(commentType)}
	}

	var c domain.CaseComment
	var typeRaw, createdByEmail string
	err := r.db.QueryRow(ctx, createIncidentCommentQuery,
		createdBy, typeEnum, incidentID, content,
	).Scan(&c.ID, &c.CaseID, &typeRaw, &c.Content, &createdByEmail, &c.CreatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CaseComment{}, &apierror.ValidationError{Msg: "incident not found: " + incidentID}
	}
	if err != nil {
		return domain.CaseComment{}, fmt.Errorf("create incident comment: %w", err)
	}
	c.Type = caseCommentEnumType[typeRaw]
	c.CreatedBy = domain.NewUserReference("", createdByEmail, "")
	return c, nil
}

// CreateIncidentNotes implements IncidentRepository.
func (r *incidentRepo) CreateIncidentNotes(ctx context.Context, incidentID string, workNotes, additionalComments *string, createdBy string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		return insertIncidentNotesTx(ctx, tx, incidentID, workNotes, additionalComments, createdBy)
	})
}

// IncidentFieldChange is one field of an incident that an update changed, as it is written to
// work_item_activity: Field is the field_name key (labelled by caseActivityFieldChangeLabel),
// OldValue/NewValue the display values, "" when the field had or has none.
type IncidentFieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// RecordIncidentFieldChanges implements IncidentRepository.
func (r *incidentRepo) RecordIncidentFieldChanges(ctx context.Context, incidentID string, changes []IncidentFieldChange, actorEmail string) error {
	if len(changes) == 0 {
		return nil
	}
	fields := make([]string, len(changes))
	oldValues := make([]string, len(changes))
	newValues := make([]string, len(changes))
	for i, c := range changes {
		fields[i], oldValues[i], newValues[i] = c.Field, c.OldValue, c.NewValue
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value, user_email)
		 SELECT gen_random_uuid(), NOW(), $1, $2::uuid, left(c.f, 255), left(c.o, 255), left(c.n, 255), $1
		   FROM unnest($3::text[], $4::text[], $5::text[]) AS c(f, o, n)`,
		actorEmail, incidentID, fields, oldValues, newValues,
	)
	if err != nil {
		return fmt.Errorf("record incident field changes: %w", err)
	}
	return nil
}

// insertIncidentNotesTx inserts the non-blank work note and public comment on incidentID inside tx,
// with createIncidentCommentQuery's own existence check: an incident that is not there is a
// ValidationError, and rolls the whole transaction back.
func insertIncidentNotesTx(ctx context.Context, tx pgx.Tx, incidentID string, workNotes, additionalComments *string, createdBy string) error {
	for _, note := range []struct {
		text *string
		kind domain.CommentType
	}{{workNotes, domain.CommentTypeWorkNote}, {additionalComments, domain.CommentTypeComment}} {
		if note.text == nil || strings.TrimSpace(*note.text) == "" {
			continue
		}
		var id string
		err := tx.QueryRow(ctx, `WITH c AS (`+createIncidentCommentQuery+`) SELECT id FROM c`,
			createdBy, caseCommentTypeEnum[note.kind], incidentID, *note.text,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.ValidationError{Msg: "incident not found: " + incidentID}
		}
		if err != nil {
			return fmt.Errorf("create incident %s: %w", strings.ToLower(string(note.kind)), err)
		}
	}
	return nil
}

// incidentLifecycleFKField names the request field behind each foreign key
// UpdateIncidentLifecycle can trip, and what it refers to, so a bad id reads
// as a ValidationError on that field instead of a 500.
var incidentLifecycleFKField = map[string][2]string{
	"work_item_assigned_to_id_fkey":      {"assignedEngineerId", "user"},
	"incident_resolved_by_id_fkey":       {"resolvedById", "user"},
	"work_item_assignment_group_id_fkey": {"assignmentGroupId", "group"},
	"work_item_parent_id_fkey":           {"parentId", "record"},
	"incident_service_id_fkey":           {"serviceId", "service"},
	"incident_service_offering_id_fkey":  {"serviceOfferingId", "service offering"},
	"incident_change_request_id_fkey":    {"changeRequestId", "change request"},
	"incident_caused_by_id_fkey":         {"causedById", "change request"},
	"incident_cmdb_ci_id_fkey":           {"configurationItemId", "configuration item"},
	"incident_problem_id_fkey":           {"problemId", "problem"},
	"incident_parent_incident_id_fkey":   {"parentIncidentId", "incident"},
	"incident_category_subcategory_fkey": {"subcategory", "subcategory of the category"},
	"work_item_watcher_user_id_fkey":     {"watchList", "user"},
}

// UpdateIncidentLifecycle implements IncidentRepository.
func (r *incidentRepo) UpdateIncidentLifecycle(ctx context.Context, id string, u IncidentLifecycleUpdate, actorEmail string) ([]ResolvedChildIncident, error) {
	children, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) ([]ResolvedChildIncident, error) {
		children, err := r.updateIncidentLifecycleTx(ctx, tx, id, u, actorEmail)
		if err != nil {
			return nil, err
		}
		return children, insertIncidentNotesTx(ctx, tx, id, u.WorkNotes, u.AdditionalComments, actorEmail)
	})
	if err == nil {
		return children, nil
	}
	var ve *apierror.ValidationError
	var nfe *apierror.NotFoundError
	if errors.As(err, &ve) || errors.As(err, &nfe) {
		return nil, err
	}
	if IsRLSPolicyViolation(err) {
		return nil, &apierror.NotFoundError{Msg: "incident not found"}
	}
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if f, ok := incidentLifecycleFKField[pgErr.ConstraintName]; ok {
			return nil, &apierror.ValidationError{Msg: f[0] + " does not identify an existing " + f[1]}
		}
		return nil, &apierror.ValidationError{Msg: "a referenced id does not exist"}
	}
	return nil, fmt.Errorf("update incident: %w", err)
}

// Work notes ServiceNow's OOB "Cascade closure of Incident Tasks" writes on
// each task it closes (incident, after insert/update, state changes to
// Closed or Canceled; on when com.snc.incident.incident_task.closure is
// "true", as it is on WSO2's instance). Established by reading the
// business rule on the live instance; the discovery scripts that did so
// are kept out of this repository.
const (
	incidentTaskClosedOnCloseNote  = "Incident Task is Closed Incomplete based on closure of %s."
	incidentTaskClosedOnCancelNote = "Incident Task is Closed Skipped based on cancelation of %s."
)

// cascadeIncidentTaskClosure ports "Cascade closure of Incident Tasks": when
// the incident moves to CLOSED every active, open incident task becomes
// CLOSED_INCOMPLETE; when it moves to CANCELED they become CLOSED_SKIPPED.
// Each gets ServiceNow's work note, written as the acting user. Tasks
// already in a closed state are left alone, as is a task with
// is_active = false (ServiceNow's addActiveQuery). A NULL state counts as
// open.
//
// The task side effects match ServiceNow's task rules ("mark closed" and
// "Set Closure Fields"): is_active false, and closed_on / closed_by_id set
// only when empty.
func cascadeIncidentTaskClosure(ctx context.Context, tx pgx.Tx, incidentID, incidentNumber, incidentState, actorEmail string) error {
	var taskState, note string
	switch incidentState {
	case "CLOSED":
		taskState, note = "CLOSED_INCOMPLETE", fmt.Sprintf(incidentTaskClosedOnCloseNote, incidentNumber)
	case "CANCELED":
		taskState, note = "CLOSED_SKIPPED", fmt.Sprintf(incidentTaskClosedOnCancelNote, incidentNumber)
	default:
		return nil
	}
	closed := make([]string, 0, len(domain.IncidentTaskClosedStates))
	for s := range domain.IncidentTaskClosedStates {
		closed = append(closed, s)
	}
	if _, err := tx.Exec(ctx, `
		WITH closed AS (
			UPDATE incident_task it
			SET state = $2::TEXT::incident_task_state_enum,
			    is_active = FALSE,
			    closed_on = COALESCE(it.closed_on, NOW()),
			    closed_by_id = COALESCE(it.closed_by_id, (SELECT id FROM "user" WHERE LOWER(email) = LOWER($3) LIMIT 1))
			WHERE it.incident_id = $1
			  AND it.is_active = TRUE
			  AND (it.state IS NULL OR NOT (it.state::TEXT = ANY($4::text[])))
			RETURNING it.id
		), touched AS (
			UPDATE work_item wi
			SET updated_on = NOW(), updated_by = $3
			FROM closed
			WHERE wi.id = closed.id
			RETURNING wi.id
		)
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), NOW(), $3, $5::comment_type_enum, touched.id, $6
		FROM touched`,
		incidentID, taskState, actorEmail, closed, caseCommentTypeEnum[domain.CommentTypeWorkNote], note); err != nil {
		return fmt.Errorf("update incident: close incident tasks: %w", err)
	}
	return nil
}

// childIncidentResolvedNote is what ServiceNow's OOB "Update Child Incidents"
// business rule writes on each child incident it resolves, as an additional
// comment (WSO2's production children carry it as a comment only, no work
// note).
const childIncidentResolvedNote = "Resolved based on resolution of Parent Incident."

// maxChildIncidentCascadeDepth is how many levels of child incidents below the
// resolved one cascadeChildIncidentResolution follows. The visited set already
// stops a loop in bad data; this stops a runaway chain.
const maxChildIncidentCascadeDepth = 10

// cascadeChildIncidentResolution ports ServiceNow's "Update Child Incidents"
// for Resolved: once incident parentID has entered Resolved in tx, every
// child incident (parent_incident_id = parentID) that is not Resolved, Closed
// or Canceled -- New and On Hold included, as ServiceNow resolves any active
// child -- is resolved with it:
//
//   - resolution_code and close_notes stay the child's own when it has
//     them, else the parent's (as on WSO2's production instance: a child that
//     already carried a code, such as Duplicate, kept it);
//   - resolved_by_id is the parent's as stored (NULL stays NULL), resolved_on
//     is now, and resolved_with_parent is set;
//   - work_item.updated_on/updated_by are stamped, and
//     childIncidentResolvedNote is added as a comment.
//
// ServiceNow's rule fires again on each child's own update, so a child's
// children follow from the child's values, recursively. Children are locked
// in id order; visited and maxChildIncidentCascadeDepth guard against bad
// data. Only Resolved cascades: Closed, Canceled and a re-open leave the
// children alone. Any failure fails the parent's update with it.
func cascadeChildIncidentResolution(ctx context.Context, tx pgx.Tx, parentID, actorEmail string, visited map[string]bool, depth int) ([]ResolvedChildIncident, error) {
	rows, err := tx.Query(ctx, `
		SELECT inc.id::text, wi.number, inc.state::text, inc.resolution_code::text
		FROM incident inc
		JOIN work_item wi ON wi.id = inc.id
		WHERE inc.parent_incident_id = $1
		  AND inc.state NOT IN ('RESOLVED', 'CLOSED', 'CANCELED')
		ORDER BY inc.id
		FOR UPDATE OF inc`, parentID)
	if err != nil {
		return nil, fmt.Errorf("update incident: read open child incidents: %w", err)
	}
	type child struct {
		id, number, state string
		code              *string
	}
	var children []child
	for rows.Next() {
		var c child
		if err := rows.Scan(&c.id, &c.number, &c.state, &c.code); err != nil {
			rows.Close()
			return nil, fmt.Errorf("update incident: scan open child incident: %w", err)
		}
		children = append(children, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("update incident: read open child incidents: %w", err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	if depth > maxChildIncidentCascadeDepth {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf(
			"child incident %s is nested more than %d levels below the incident being resolved; resolve the incidents further down first",
			children[0].number, maxChildIncidentCascadeDepth)}
	}

	note := childIncidentResolvedNote
	var resolved []ResolvedChildIncident
	for _, c := range children {
		if visited[c.id] {
			continue
		}
		visited[c.id] = true
		var code *string
		if err := tx.QueryRow(ctx, `
			UPDATE incident c
			SET state = 'RESOLVED',
			    resolution_code = COALESCE(c.resolution_code, p.resolution_code),
			    close_notes = CASE WHEN c.close_notes ~ '\S' THEN c.close_notes ELSE p.close_notes END,
			    resolved_by_id = p.resolved_by_id,
			    resolved_on = NOW(),
			    resolved_with_parent = TRUE
			FROM incident p
			WHERE c.id = $1 AND p.id = $2
			RETURNING c.resolution_code::text`, c.id, parentID).Scan(&code); err != nil {
			return nil, fmt.Errorf("update incident: resolve child incident %s: %w", c.number, err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1`, c.id, actorEmail); err != nil {
			return nil, fmt.Errorf("update incident: resolve child incident %s: %w", c.number, err)
		}
		if err := insertIncidentNotesTx(ctx, tx, c.id, nil, &note, actorEmail); err != nil {
			return nil, fmt.Errorf("update incident: resolve child incident %s: %w", c.number, err)
		}
		resolved = append(resolved, ResolvedChildIncident{
			ID:                     c.id,
			Number:                 c.number,
			PreviousState:          *incidentStateFromEnum(&c.state),
			PreviousResolutionCode: incidentResolutionCodeFromEnum(c.code),
			ResolutionCode:         incidentResolutionCodeFromEnum(code),
		})
		below, err := cascadeChildIncidentResolution(ctx, tx, c.id, actorEmail, visited, depth+1)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, below...)
	}
	return resolved, nil
}

// checkParentIncident refuses a parent incident link that would loop: the
// incident as its own parent, or a parent that already has the incident
// among its ancestors (parent_incident_id, followed up). A transaction-scoped
// advisory lock serialises these checks, so two updates linking A under B
// and B under A cannot both pass. A parent that does not exist is left to the
// foreign key.
func checkParentIncident(ctx context.Context, tx pgx.Tx, id, parentID string) error {
	if strings.EqualFold(id, parentID) {
		return &apierror.ValidationError{Msg: "parentIncidentId cannot be the incident itself"}
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('incident-parent-link', 0::bigint))`); err != nil {
		return fmt.Errorf("update incident: lock parent incident links: %w", err)
	}
	// UNION, not UNION ALL: an id seen before ends the walk, so a loop
	// already in the data cannot keep it going.
	var parentNumber *string
	var loops bool
	if err := tx.QueryRow(ctx, `
		WITH RECURSIVE ancestor(id) AS (
			SELECT $1::uuid
			UNION
			SELECT inc.parent_incident_id
			FROM ancestor a
			JOIN incident inc ON inc.id = a.id
			WHERE inc.parent_incident_id IS NOT NULL
		)
		SELECT (SELECT number FROM work_item WHERE id = $1::uuid),
		       EXISTS (SELECT 1 FROM ancestor WHERE id = $2::uuid)`, parentID, id).Scan(&parentNumber, &loops); err != nil {
		return fmt.Errorf("update incident: check parent incident: %w", err)
	}
	if loops {
		return &apierror.ValidationError{Msg: fmt.Sprintf(
			"parentIncidentId cannot be %s: it is a child incident of this incident, directly or further down",
			stringOrEmpty(parentNumber))}
	}
	return nil
}

// updateIncidentLifecycleTx is UpdateIncidentLifecycle's body: lock the
// incident row, check the Resolved/Closed resolution requirement against the
// request plus what is already on record, then update work_item (always,
// for updated_on/updated_by, plus subject, parent, group and assignee) and
// incident (state, resolution and the record's own columns, only when one is
// being set), and replace the watch list when one is given. A state change
// then cascades: Closed/Canceled to the incident tasks, Resolved to the open
// child incidents, which are returned.
func (r *incidentRepo) updateIncidentLifecycleTx(ctx context.Context, tx pgx.Tx, id string, u IncidentLifecycleUpdate, actorEmail string) ([]ResolvedChildIncident, error) {
	// Before the row lock: checkParentIncident's advisory lock must come first
	// in every transaction that takes both, or a re-link waiting for it while
	// holding this row can deadlock with a resolve whose cascade wants this row.
	// An unlink needs no check: it cannot make a loop.
	if u.ParentIncidentID != nil && *u.ParentIncidentID != nil {
		if err := checkParentIncident(ctx, tx, id, **u.ParentIncidentID); err != nil {
			return nil, err
		}
	}
	var currentState, number string
	var currentCode, currentNotes, currentCategory, currentImpact, currentUrgency, currentSubcategoryID *string
	err := tx.QueryRow(ctx, `
		SELECT inc.state::text, inc.resolution_code::text, inc.close_notes, wi.number,
		       inc.category::text, inc.impact::text, inc.urgency::text, inc.subcategory_id::text
		FROM incident inc
		JOIN work_item wi ON wi.id = inc.id
		WHERE inc.id = $1
		FOR UPDATE OF inc`, id).Scan(&currentState, &currentCode, &currentNotes, &number,
		&currentCategory, &currentImpact, &currentUrgency, &currentSubcategoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "incident not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("update incident: read current state: %w", err)
	}

	if u.State != nil && (*u.State == "RESOLVED" || *u.State == "CLOSED") {
		code, notes := currentCode, currentNotes
		if u.ResolutionCode != nil {
			code = u.ResolutionCode
		}
		if u.ResolutionNotes != nil {
			notes = u.ResolutionNotes
		}
		if code == nil || *code == "" || notes == nil || strings.TrimSpace(*notes) == "" {
			return nil, &apierror.ValidationError{Msg: "resolutionCode and resolutionNotes are required to move an incident to " + *u.State}
		}
	}

	wiSets := []string{"updated_on = NOW()", "updated_by = $1"}
	wiArgs := []any{actorEmail}
	addWI := func(assignment string, val *string) {
		if val == nil {
			return
		}
		wiArgs = append(wiArgs, *val)
		wiSets = append(wiSets, fmt.Sprintf(assignment, len(wiArgs)))
	}
	addWI("subject = $%d", u.Subject)
	addWI("parent_id = $%d::uuid", u.ParentID)
	addWI("assignment_group_id = $%d::uuid", u.AssignmentGroupID)
	addWI("assigned_to_id = $%d::uuid", u.AssignedEngineerID)
	wiArgs = append(wiArgs, id)
	var wiID string
	if err := tx.QueryRow(ctx,
		fmt.Sprintf(`UPDATE work_item SET %s WHERE id = $%d AND type = 'INCIDENT' RETURNING id`, strings.Join(wiSets, ", "), len(wiArgs)),
		wiArgs...,
	).Scan(&wiID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierror.NotFoundError{Msg: "incident not found"}
		}
		return nil, err
	}

	var incSets []string
	var incArgs []any
	addInc := func(assignment string, val any) {
		incArgs = append(incArgs, val)
		incSets = append(incSets, fmt.Sprintf(assignment, len(incArgs)))
	}
	if u.State != nil {
		addInc("state = $%d::incident_state_enum", *u.State)
	}
	if u.ResolutionCode != nil {
		addInc("resolution_code = $%d::incident_resolution_code_enum", *u.ResolutionCode)
	}
	if u.ResolutionNotes != nil {
		addInc("close_notes = $%d", *u.ResolutionNotes)
	}
	enteringResolved := u.State != nil && *u.State == "RESOLVED" && currentState != "RESOLVED"
	resolvedBy := u.ResolvedByID
	if resolvedBy == nil && enteringResolved {
		resolvedBy = u.DefaultResolvedByID
	}
	if resolvedBy != nil {
		addInc("resolved_by_id = $%d::uuid", *resolvedBy)
	}
	if enteringResolved {
		// Resolved by this update itself, not by its parent's cascade.
		incSets = append(incSets, "resolved_on = NOW()", "resolved_with_parent = FALSE")
	}
	if err := r.addIncidentRecordFields(ctx, tx, u, addInc, &incSets,
		currentCategory, currentImpact, currentUrgency, currentSubcategoryID); err != nil {
		return nil, err
	}
	if len(incSets) > 0 {
		incArgs = append(incArgs, id)
		if _, err := tx.Exec(ctx,
			fmt.Sprintf(`UPDATE incident SET %s WHERE id = $%d`, strings.Join(incSets, ", "), len(incArgs)),
			incArgs...,
		); err != nil {
			return nil, err
		}
	}
	if u.WatchList != nil {
		if err := replaceIncidentWatchers(ctx, tx, id, *u.WatchList); err != nil {
			return nil, err
		}
	}
	// After the incident's own write, as ServiceNow's cascades are after rules.
	if u.State == nil || *u.State == currentState {
		return nil, nil
	}
	if err := cascadeIncidentTaskClosure(ctx, tx, id, number, *u.State, actorEmail); err != nil {
		return nil, err
	}
	if !enteringResolved {
		return nil, nil
	}
	return cascadeChildIncidentResolution(ctx, tx, id, actorEmail, map[string]bool{id: true}, 1)
}

// addIncidentRecordFields adds the incident columns of the record's own
// fields to the UPDATE updateIncidentLifecycleTx builds.
//
// Two of them follow ServiceNow rather than the column alone:
//
//   - priority is ServiceNow's lookup on impact x urgency (PriorityFor), run
//     whenever either one changes, against the other's value on record. An
//     explicit priority is written only when neither changes.
//   - subcategory depends on category. A new subcategory is looked up under
//     the new category, or the current one. A new category whose list does
//     not hold the current subcategory clears it, as ServiceNow's dependent
//     field does; the (category, subcategory_id) foreign key would otherwise
//     reject the save.
func (r *incidentRepo) addIncidentRecordFields(ctx context.Context, tx pgx.Tx, u IncidentLifecycleUpdate,
	addInc func(assignment string, val any), incSets *[]string,
	currentCategory, currentImpact, currentUrgency, currentSubcategoryID *string,
) error {
	set := func(assignment string, val *string) {
		if val != nil {
			addInc(assignment, *val)
		}
	}
	set("category = $%d::incident_category_enum", u.Category)

	category := currentCategory
	if u.Category != nil {
		category = u.Category
	}
	switch {
	case u.SubcategoryValue != nil:
		if category == nil {
			return &apierror.ValidationError{Msg: "subcategory needs a category"}
		}
		var subcategoryID string
		err := tx.QueryRow(ctx,
			`SELECT id::text FROM incident_subcategory WHERE category = $1::incident_category_enum AND value = $2`,
			*category, *u.SubcategoryValue,
		).Scan(&subcategoryID)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.ValidationError{Msg: fmt.Sprintf("subcategory %s does not belong to category %s", *u.SubcategoryValue, *category)}
		}
		if err != nil {
			return fmt.Errorf("update incident: resolve subcategory: %w", err)
		}
		addInc("subcategory_id = $%d::uuid", subcategoryID)
	case u.Category != nil && currentSubcategoryID != nil:
		var belongs bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM incident_subcategory WHERE id = $1::uuid AND category = $2::incident_category_enum)`,
			*currentSubcategoryID, *u.Category,
		).Scan(&belongs); err != nil {
			return fmt.Errorf("update incident: check subcategory: %w", err)
		}
		if !belongs {
			*incSets = append(*incSets, "subcategory_id = NULL")
		}
	}

	if u.ContactType != nil {
		addInc("contact_type = $%d::incident_contact_type_enum", incidentContactTypeToEnum(domain.IncidentContactType(*u.ContactType)))
	}
	set("impact = $%d::incident_impact_enum", u.Impact)
	set("urgency = $%d::incident_urgency_enum", u.Urgency)
	if (u.Impact != nil || u.Urgency != nil) && u.PriorityFor != nil {
		impact, urgency := currentImpact, currentUrgency
		if u.Impact != nil {
			impact = u.Impact
		}
		if u.Urgency != nil {
			urgency = u.Urgency
		}
		// Both columns are NOT NULL DEFAULT 'LOW'; an unset value reads as LOW, as the lookup's default does.
		label := func(v *string) string {
			if v == nil {
				return "LOW"
			}
			return *v
		}
		addInc("priority = $%d::incident_priority_enum", u.PriorityFor(label(impact), label(urgency)))
	} else {
		set("priority = $%d::incident_priority_enum", u.Priority)
	}

	set("service_id = $%d::uuid", u.ServiceID)
	set("service_offering_id = $%d::uuid", u.ServiceOfferingID)
	set("cmdb_ci_id = $%d::uuid", u.ConfigurationItemID)
	set("change_request_id = $%d::uuid", u.ChangeRequestID)
	set("problem_id = $%d::uuid", u.ProblemID)
	set("caused_by_id = $%d::uuid", u.CausedByID)
	if u.ParentIncidentID != nil {
		if *u.ParentIncidentID == nil {
			*incSets = append(*incSets, "parent_incident_id = NULL")
		} else {
			addInc("parent_incident_id = $%d::uuid", **u.ParentIncidentID)
		}
	}
	set("incident_report = $%d", u.IncidentReport)
	if u.Environment != nil {
		if *u.Environment == nil {
			*incSets = append(*incSets, "environment = NULL")
		} else {
			addInc("environment = $%d", **u.Environment)
		}
	}
	return nil
}

// replaceIncidentWatchers replaces the incident's watch list with entries
// (user ids or emails), as ServiceNow's update does. An empty list clears it.
func replaceIncidentWatchers(ctx context.Context, tx pgx.Tx, incidentID string, entries []string) error {
	userIDs, err := resolveIncidentWatchers(ctx, tx, entries)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM work_item_watcher WHERE work_item_id = $1`, incidentID); err != nil {
		return fmt.Errorf("update incident: clear watch list: %w", err)
	}
	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO work_item_watcher (id, work_item_id, user_id) VALUES (gen_random_uuid(), $1, $2)`,
			incidentID, userID,
		); err != nil {
			return fmt.Errorf("update incident: add watcher: %w", err)
		}
	}
	return nil
}

// createIncidentPortalQuery is CreateIncident's (the plain-Postgres,
// caller-initiated path) insert of both halves of the row -- structurally
// identical to createIncidentFromServiceNowQuery except id/number are
// generated here (gen_random_uuid()/next_portal_work_item_number(), migration
// 0140) instead of supplied by a prior ServiceNow response. incident.state is
// left to its own column default ('NEW'), which is the state ServiceNow's
// IncidentUtils.createIncident hard-sets.
//
// Column/output order matches the trailing SELECT exactly.
const createIncidentPortalQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, type, parent_id, assignment_group_id, assigned_to_id
		)
		VALUES (
			gen_random_uuid(), NOW(), NOW(), $1, $1,
			next_portal_work_item_number(), $2, 'INCIDENT'::work_item_type_enum, $3::uuid, $4::uuid, $18::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_incident AS (
		INSERT INTO incident (
			id, caller_id, category, impact, urgency,
			service_id, service_offering_id, contact_type,
			change_request_id, caused_by_id, parent_incident_id, problem_id,
			opened_on, correlation_id, environment,
			priority, subcategory_id, cmdb_ci_id
		)
		SELECT id, $5::uuid, $6::incident_category_enum, $7::incident_impact_enum, $8::incident_urgency_enum,
		       $9::uuid, $10::uuid, $11::incident_contact_type_enum,
		       $12::uuid, $13::uuid, $14::uuid, $15::uuid,
		       NOW(), $16, $17,
		       $19::incident_priority_enum, $20::uuid, $21::uuid
		FROM inserted_work_item
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_incident ii ON ii.id = iwi.id`

// CreateIncident implements IncidentRepository.
//
// One transaction, in the order ServiceNow's IncidentUtils.createIncident
// does it: the record (with its watch list) first, then the customer-visible
// comment and the work note as journal entries against the new record.
func (r *incidentRepo) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest, priority string, subcategoryValue *string, createdBy string) (domain.CreateIncidentResponse, error) {
	var contactType *string
	if req.ContactType != nil {
		v := incidentContactTypeToEnum(*req.ContactType)
		contactType = &v
	}

	resp, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.CreateIncidentResponse, error) {
		subcategoryID, err := incidentSubcategoryIDTx(ctx, tx, req, subcategoryValue)
		if err != nil {
			return domain.CreateIncidentResponse{}, err
		}

		watcherIDs, err := resolveIncidentWatchers(ctx, tx, req.WatchList)
		if err != nil {
			return domain.CreateIncidentResponse{}, err
		}

		var (
			outID, outNumber, outSubject, outCreatedBy string
			outCreatedOn, outUpdatedOn                 time.Time
		)
		if err := tx.QueryRow(ctx, createIncidentPortalQuery,
			createdBy, req.Subject, req.ParentID, req.AssignmentGroupID,
			req.CallerID, string(req.Category), string(req.Impact), string(req.Urgency),
			req.ServiceID, req.ServiceOfferingID, contactType,
			req.ChangeRequestID, req.CausedByID, req.ParentIncidentID, req.ProblemID,
			req.CorrelationID, req.Environment,
			req.AssignedEngineerID, priority, subcategoryID, req.ConfigurationItemID,
		).Scan(&outID, &outNumber, &outSubject, &outCreatedOn, &outUpdatedOn, &outCreatedBy); err != nil {
			return domain.CreateIncidentResponse{}, err
		}

		if err := insertIncidentWatcherRowsTx(ctx, tx, outID, watcherIDs); err != nil {
			return domain.CreateIncidentResponse{}, err
		}

		journal := []struct {
			commentType domain.CommentType
			content     *string
		}{
			{domain.CommentTypeComment, req.AdditionalComments},
			{domain.CommentTypeWorkNote, req.WorkNotes},
		}
		for _, j := range journal {
			if j.content == nil || strings.TrimSpace(*j.content) == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
				 VALUES (gen_random_uuid(), NOW(), $1, $2::comment_type_enum, $3, $4)`,
				createdBy, caseCommentTypeEnum[j.commentType], outID, *j.content,
			); err != nil {
				return domain.CreateIncidentResponse{}, fmt.Errorf("insert incident %s: %w", j.commentType, err)
			}
		}

		resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
		resp.Incident.ID = outID
		resp.Incident.Number = outNumber
		resp.Incident.CreatedOn = outCreatedOn.UTC().Format(time.RFC3339)
		resp.Incident.CreatedBy = outCreatedBy
		resp.Incident.Priority = priority
		return resp, nil
	})
	if err != nil {
		var ve *apierror.ValidationError
		if errors.As(err, &ve) {
			return domain.CreateIncidentResponse{}, err
		}
		// incident_deny_all_insert (migration 0148) permits only an internal
		// caller -- incident has no project concept at all, so there is no
		// project-member OR-branch the way case/change_request have.
		// POST /incidents is already gated internalOnly at the route, so this
		// should not be reachable in practice, but map it defensively rather
		// than leaving a theoretical 42501 to surface as a raw 500.
		if IsRLSPolicyViolation(err) {
			return domain.CreateIncidentResponse{}, &apierror.NotFoundError{Msg: "incident not found"}
		}
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
				return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from integrity triggers
				return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.CreateIncidentResponse{}, fmt.Errorf("create incident: %w", err)
	}
	return resp, nil
}

// incidentSubcategoryIDTx resolves a create request's subcategory: its
// incident_subcategory.value (ServiceNow's choice spelling) under the request's
// category, or nil when none was sent. One of another category is refused.
func incidentSubcategoryIDTx(ctx context.Context, tx pgx.Tx, req domain.CreateIncidentRequest, subcategoryValue *string) (*string, error) {
	if subcategoryValue == nil {
		return nil, nil
	}
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM incident_subcategory WHERE category = $1::incident_category_enum AND value = $2`,
		string(req.Category), *subcategoryValue,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		named := *subcategoryValue
		if req.Subcategory != nil {
			named = string(*req.Subcategory)
		}
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("subcategory %s does not belong to category %s", named, req.Category)}
	}
	if err != nil {
		return nil, fmt.Errorf("resolve incident subcategory: %w", err)
	}
	return &id, nil
}

// insertIncidentWatcherRowsTx writes an incident's watchers (user ids from resolveIncidentWatchers).
func insertIncidentWatcherRowsTx(ctx context.Context, tx pgx.Tx, incidentID string, userIDs []string) error {
	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO work_item_watcher (id, work_item_id, user_id) VALUES (gen_random_uuid(), $1, $2)`,
			incidentID, userID,
		); err != nil {
			return fmt.Errorf("insert incident watcher: %w", err)
		}
	}
	return nil
}

// resolveIncidentWatchers turns a create request's watch list into user ids.
// Entries may be user ids or email addresses, the two forms ServiceNow's own
// create path accepts (watchListEmails). Every entry must name an existing
// user: ServiceNow resolves each one before inserting, and an unresolvable
// entry fails the request there too. Duplicates collapse to one watcher.
//
// An entry is never echoed in an error, as in watchListEmails: it can be a
// third party's email address, and writeServiceError logs every
// ValidationError's Msg verbatim. A refused entry is named by its position.
func resolveIncidentWatchers(ctx context.Context, tx pgx.Tx, entries []string) ([]string, error) {
	seen := make(map[string]bool, len(entries))
	ids := make([]string, 0, len(entries))
	for i, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		var id string
		var err error
		if strings.Contains(entry, "@") {
			err = tx.QueryRow(ctx, `SELECT id::text FROM "user" WHERE lower(email) = lower($1) ORDER BY id LIMIT 1`, entry).Scan(&id)
		} else if validate.IsUUID(entry) {
			err = tx.QueryRow(ctx, `SELECT id::text FROM "user" WHERE id = $1::uuid`, entry).Scan(&id)
		} else {
			// Checked here, not left to the ::uuid cast: its error names no field.
			return nil, &apierror.ValidationError{Msg: "watchList contains an entry that is neither a valid email address nor a valid user identifier"}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("watchList entry %d is not a known user", i+1)}
		}
		if err != nil {
			return nil, fmt.Errorf("resolve incident watcher: %w", err)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// createIncidentFromServiceNowQuery inserts both halves of an incident row
// (work_item + incident, the same shared-primary-key pattern
// createCaseFromServiceNowQuery documents) in one round trip via a CTE,
// using caller-supplied identity (id/number/createdBy) rather than
// generating any of it -- see CreateIncidentFromServiceNow's own doc comment
// for why. It writes the same columns as createIncidentPortalQuery.
// type is hardcoded to 'INCIDENT'::work_item_type_enum. incident.state is
// left to its own column default ('NEW') -- reliably parsing ServiceNow's
// raw create-response state label back into incident_state_enum would need
// a label lookup this service doesn't have for incident (unlike case's
// hardcoded 'OPEN', a freshly created ServiceNow incident's state is not
// knowable as a single constant the way case's is), and the schema default
// already matches the correct freshly-created value.
//
// Column/output order matches the trailing SELECT exactly.
const createIncidentFromServiceNowQuery = `
	WITH inserted_work_item AS (
		INSERT INTO work_item (
			id, created_on, updated_on, created_by, updated_by,
			number, subject, type, parent_id, assignment_group_id, assigned_to_id
		)
		VALUES (
			$1, NOW(), NOW(), $2, $2,
			$3, $4, 'INCIDENT'::work_item_type_enum, $5::uuid, $6::uuid, $21::uuid
		)
		RETURNING id, number, subject, created_on, updated_on, created_by
	),
	inserted_incident AS (
		INSERT INTO incident (
			id, caller_id, category, impact, urgency,
			service_id, service_offering_id, contact_type,
			change_request_id, caused_by_id, parent_incident_id, problem_id,
			opened_on, correlation_id, environment, priority,
			subcategory_id, cmdb_ci_id
		)
		VALUES (
			$1, $7::uuid, $8::incident_category_enum, $9::incident_impact_enum, $10::incident_urgency_enum,
			$11::uuid, $12::uuid, $13::incident_contact_type_enum,
			$14::uuid, $15::uuid, $16::uuid, $17::uuid,
			NOW(), $18, $19, $20::incident_priority_enum,
			$22::uuid, $23::uuid
		)
		RETURNING id
	)
	SELECT iwi.id, iwi.number, iwi.subject, iwi.created_on, iwi.updated_on, iwi.created_by
	FROM inserted_work_item iwi
	JOIN inserted_incident ii ON ii.id = iwi.id`

// CreateIncidentFromServiceNow implements IncidentRepository.
func (r *incidentRepo) CreateIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, subcategoryValue *string, id, number, priority, createdBy string) (domain.CreateIncidentResponse, error) {
	// WithSystemIdentity: this insert never sets a project_id on the new
	// work_item row at all (incidents have no project concept -- see this
	// file's own package doc comment), so work_item's INSERT policy
	// (migration 0147) can only be satisfied by is_internal, not
	// is_project_member(NULL). Same reasoning as CreateChangeRequestFromServiceNow/
	// CreateCaseFromServiceNow's own identical stamps: this insert only ever
	// runs after ServiceNow's own workflow already accepted the create.
	ctx = WithSystemIdentity(ctx)
	resp, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (domain.CreateIncidentResponse, error) {
		return insertIncidentFromServiceNowTx(ctx, tx, req, subcategoryValue, id, number, priority, createdBy)
	})
	if err != nil {
		return domain.CreateIncidentResponse{}, createIncidentFromServiceNowError(err, id)
	}
	return resp, nil
}

// errIncidentCheckRollback ends CheckIncidentFromServiceNow's transaction once
// its insert has gone through, so that nothing it wrote is ever committed.
var errIncidentCheckRollback = errors.New("incident check: roll back")

// CheckIncidentFromServiceNow implements IncidentRepository.
func (r *incidentRepo) CheckIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, subcategoryValue *string, priority, createdBy string) error {
	ctx = WithSystemIdentity(ctx)
	var placeholderID string
	_, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (struct{}, error) {
		// A fresh id and a number no incident can hold: the insert's own
		// rules are checked, never a clash with a real row.
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&placeholderID); err != nil {
			return struct{}{}, fmt.Errorf("incident check: placeholder id: %w", err)
		}
		if _, err := insertIncidentFromServiceNowTx(ctx, tx, req, subcategoryValue, placeholderID, "CHECK-"+placeholderID, priority, createdBy); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, errIncidentCheckRollback
	})
	if errors.Is(err, errIncidentCheckRollback) {
		return nil
	}
	return createIncidentFromServiceNowError(err, placeholderID)
}

// insertIncidentFromServiceNowTx is CreateIncidentFromServiceNow's insert: the
// row (createIncidentFromServiceNowQuery), its watchers and its journal.
func insertIncidentFromServiceNowTx(ctx context.Context, tx pgx.Tx, req domain.CreateIncidentRequest, subcategoryValue *string, id, number, priority, createdBy string) (domain.CreateIncidentResponse, error) {
	var contactType *string
	if req.ContactType != nil {
		v := incidentContactTypeToEnum(*req.ContactType)
		contactType = &v
	}
	subcategoryID, err := incidentSubcategoryIDTx(ctx, tx, req, subcategoryValue)
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	watcherIDs, err := resolveIncidentWatchers(ctx, tx, req.WatchList)
	if err != nil {
		return domain.CreateIncidentResponse{}, err
	}

	var (
		outID, outNumber, outSubject, outCreatedBy string
		outCreatedOn, outUpdatedOn                 time.Time
	)
	if err := tx.QueryRow(ctx, createIncidentFromServiceNowQuery,
		id, createdBy,
		number, req.Subject, req.ParentID, req.AssignmentGroupID,
		req.CallerID, string(req.Category), string(req.Impact), string(req.Urgency),
		req.ServiceID, req.ServiceOfferingID, contactType,
		req.ChangeRequestID, req.CausedByID, req.ParentIncidentID, req.ProblemID,
		req.CorrelationID, req.Environment, priority,
		req.AssignedEngineerID, subcategoryID, req.ConfigurationItemID,
	).Scan(&outID, &outNumber, &outSubject, &outCreatedOn, &outUpdatedOn, &outCreatedBy); err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	if err := insertIncidentWatcherRowsTx(ctx, tx, outID, watcherIDs); err != nil {
		return domain.CreateIncidentResponse{}, err
	}
	// The work note and additional comment sent with the create are stored with the record, as CreateIncident does.
	if err := insertIncidentNotesTx(ctx, tx, outID, req.WorkNotes, req.AdditionalComments, createdBy); err != nil {
		return domain.CreateIncidentResponse{}, err
	}

	resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
	resp.Incident.ID = outID
	resp.Incident.Number = outNumber
	resp.Incident.CreatedOn = outCreatedOn.UTC().Format(time.RFC3339)
	resp.Incident.CreatedBy = outCreatedBy
	resp.Incident.Priority = priority
	return resp, nil
}

// createIncidentFromServiceNowError maps a failed CreateIncidentFromServiceNow
// insert (or its check) to the error the caller sees.
func createIncidentFromServiceNowError(err error, id string) error {
	var ve *apierror.ValidationError
	if errors.As(err, &ve) {
		return err
	}
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation on id/number -- see this method's own doc comment for why this "shouldn't" happen
			return &apierror.ConflictError{Msg: "an incident already exists for this ServiceNow id/number: " + pgErr.Detail}
		case "22P02": // invalid_text_representation -- id (or another uuid-typed field) was not a valid UUID
			return &apierror.ValidationError{Msg: "id is not a valid UUID: " + id}
		case "23503": // foreign_key_violation -- one of the referenced IDs does not exist
			return &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
		case "P0001": // raise_exception from integrity triggers
			return &apierror.ValidationError{Msg: pgErr.Message}
		}
	}
	return fmt.Errorf("create incident from servicenow: %w", err)
}
