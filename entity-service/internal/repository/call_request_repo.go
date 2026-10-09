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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// callRequestStateLabels is the display label per customer_call_state_enum
// value (migration 0073), keyed by the domain state id (the lowercased enum
// label -- all eight labels match domain.CallRequestStateType exactly once
// case-folded, e.g. CANCELED/canceled; unlike case_state_enum there is no
// spelling drift to normalize).
var callRequestStateLabels = map[domain.CallRequestStateType]string{
	domain.CallRequestStatePendingOnCustomer: "Pending on Customer",
	domain.CallRequestStatePendingOnWSO2:     "Pending on WSO2",
	domain.CallRequestStateScheduled:         "Scheduled",
	domain.CallRequestStateCustomerRejected:  "Customer Rejected",
	domain.CallRequestStateWSO2Rejected:      "WSO2 Rejected",
	domain.CallRequestStateCanceled:          "Canceled",
	domain.CallRequestStateNotesPending:      "Notes Pending",
	domain.CallRequestStateConcluded:         "Concluded",
}

// callRequestStateToEnum converts a domain state id to its
// customer_call_state_enum label.
func callRequestStateToEnum(s domain.CallRequestStateType) string {
	return strings.ToUpper(string(s))
}

// CallRequestStateFromEnum converts a customer_call_state_enum label to the
// domain state (id + display label). A NULL/unrecognized label yields an
// empty id, mirroring how the ServiceNow adapter treats an unknown state key.
// Exported so project metadata can offer call-request states in the same
// lowercase vocabulary the call-request endpoints accept.
func CallRequestStateFromEnum(enumLabel string) domain.CallRequestState {
	id := domain.CallRequestStateType(strings.ToLower(enumLabel))
	label, ok := callRequestStateLabels[id]
	if !ok {
		return domain.CallRequestState{}
	}
	return domain.CallRequestState{ID: string(id), Label: label}
}

// finalTimeLayouts are the timestamp spellings customer_call.final_times
// holds. Synced rows carry two ServiceNow-side spellings (checked against
// staging: MM/DD/YYYY 324, YYYY-MM-DD 168 of the parseable times), and rows
// written by this service carry RFC 3339. Every one is UTC -- scheduled_on
// equals the first time as a UTC instant on synced rows.
var finalTimeLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "01/02/2006 15:04:05"}

// normalizeFinalTime returns raw as RFC 3339 UTC, or false if it is not a time
// in any known layout. The column also holds ServiceNow script error text where
// a time should be (e.g. "Error: Missing parameters (localTime or timezone)."),
// which must not reach the UI as if it were a time.
func normalizeFinalTime(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range finalTimeLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t.UTC().Format(time.RFC3339), true
		}
	}
	return "", false
}

// decodeFinalTimes reads customer_call.final_times (JSONB, migration 0073)
// as the call request's preferred times, as RFC 3339 UTC strings.
//
// Two shapes exist in the column: synced rows are an array of objects
// ({"time": "...", "index": 0}, sometimes with extra keys such as "state"),
// ordered by "index" when present; rows this service writes are a plain array of
// strings. Both are read. An element that is not a recognisable time is skipped.
// NULL or any other shape yields an empty (non-nil, so it serializes as [])
// list rather than an error, so one odd row cannot fail a whole search.
func decodeFinalTimes(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return out
	}

	type entry struct {
		at    string
		index int
		pos   int
	}
	entries := make([]entry, 0, len(elems))
	for pos, e := range elems {
		var (
			text  string
			index = pos
		)
		var obj struct {
			Time  string `json:"time"`
			Index *int   `json:"index"`
		}
		switch {
		case json.Unmarshal(e, &text) == nil:
			// a plain string element
		case json.Unmarshal(e, &obj) == nil:
			text = obj.Time
			if obj.Index != nil {
				index = *obj.Index
			}
		default:
			continue
		}
		if at, ok := normalizeFinalTime(text); ok {
			entries = append(entries, entry{at: at, index: index, pos: pos})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].index < entries[j].index })
	for _, e := range entries {
		out = append(out, e.at)
	}
	return out
}

// parseActualDurationMin parses customer_call.actual_call_duration (a free
// VARCHAR, migration 0073) as a whole number of minutes -- the format this
// service itself writes. Anything unparseable (e.g. a differently formatted
// value from a data sync) is nil rather than a guess.
func parseActualDurationMin(raw *string) *int {
	if raw == nil {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(*raw))
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

// CallRequestRepository defines the persistence operations for customer_call
// (migration 0073), the Postgres backing for call requests.
//
// Known gaps, all left at their zero value rather than guessed:
//   - number has no default/sequence and no confirmed format, so a created
//     call request has a NULL number (returned as "").
//   - CancellationReason has no column ("reason" is the request's own
//     reason), so it is never stored or returned -- the service rejects a
//     request that supplies one rather than silently dropping it.
//   - closed_on/closed_by_id are never set by UpdateCallRequest: which states
//     count as "closed" isn't specified anywhere.
//   - State transitions are not validated against the current state, except that
//     concluding without notes needs a scheduled or notes-pending call (see
//     UpdateCallRequest).
type CallRequestRepository interface {
	// CreateCallRequest inserts a call request for req.CaseID in state
	// pending_on_wso2, opened by callerID/callerEmail. Returns a NotFoundError
	// if req.CaseID is not an existing case-like work item.
	CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error)
	// CreateCallRequestFromServiceNow inserts a call request using id/createdBy/
	// createdOn exactly as ServiceNow assigned them (DATA_SOURCE=postgres-servicenow-dual-write's
	// ServiceNow-first CREATE path, callRequestService.createCallRequestSNFirst)
	// rather than generating its own id -- see that method's own doc comment
	// for why. callerID still attributes customer_call.opened_by_id to the
	// resolved caller, matching CreateCallRequest's own attribution; number
	// is left NULL, matching CreateCallRequest (ServiceNow's call-request
	// create response carries no number at all). Returns a NotFoundError if
	// req.CaseID is not an existing case-like work item.
	CreateCallRequestFromServiceNow(ctx context.Context, req domain.CreateCallRequestRequest, id, createdBy string, createdOn time.Time, callerID string) (domain.CreateCallRequestResponse, error)
	// SearchCallRequests returns the call requests of one case, newest first,
	// optionally narrowed to states, with the total before pagination.
	SearchCallRequests(ctx context.Context, caseID string, states []domain.CallRequestStateType, pagination domain.Pagination) ([]domain.CallRequestView, int, error)
	// SearchAllCallRequests returns call requests across all cases matching
	// the filters, with the total before pagination.
	SearchAllCallRequests(ctx context.Context, filters domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, pagination domain.Pagination) ([]domain.CallRequestView, int, error)
	// UpdateCallRequest applies req to the call request req.ID. assigneeID is
	// the already-resolved user id for req.Assignee (nil to leave it
	// unchanged). Returns a NotFoundError if no call request matches (or, when
	// req.CaseID is set, none belongs to that case).
	UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string) (domain.UpdateCallRequestResponse, error)
	// SetCallRequestSNSysID best-effort persists ServiceNow's own sys_id for
	// the call request identified by id (migration 0135) -- called from
	// CreateCallRequest's async ServiceNow mirror success path, never from
	// the synchronous request path. A no-op (returns nil) if id does not
	// exist: the row may have raced with a concurrent delete, and this is
	// itself a best-effort follow-up to an already-succeeded mirror write,
	// not something that should surface as a dispatcher failure.
	SetCallRequestSNSysID(ctx context.Context, id, snSysID string) error
	// GetCallRequestSNSysID returns the ServiceNow sys_id previously stored
	// for id by SetCallRequestSNSysID, or nil if none is stored yet (the
	// CREATE mirror hasn't run, hasn't finished, or failed -- see
	// sn_writeback_failures). Returns a NotFoundError if id does not exist.
	GetCallRequestSNSysID(ctx context.Context, id string) (*string, error)
}

type callRequestRepo struct {
	db *Scoped
}

// NewCallRequestRepository constructs a CallRequestRepository backed by the
// given Scoped connection. customer_call's project-membership visibility
// (migration 0143) is enforced entirely by Postgres RLS now -- this
// repository applies no project filtering of its own, closing what was
// previously an acknowledged, unfixed gap (see this file's git history):
// neither SearchCallRequests nor SearchAllCallRequests ever did any
// caller-scoped authorization at all.
func NewCallRequestRepository(db *Scoped) CallRequestRepository {
	return &callRequestRepo{db: db}
}

// callRequestSelect/callRequestFrom are shared by both searches. The parent
// case ref comes from work_item (LEFT JOINed: customer_call.work_item_id is
// nullable), and the assignee's display name from "user".
//
// The "case" + case-like extension joins are needed only for the case-state
// filters of SearchAllCallRequests, but are included in the shared FROM so
// the count and page queries can never drift apart.
const callRequestSelect = `
	SELECT cc.id, cc.number, cc.state::TEXT, cc.reason, cc.final_times,
	       (EXTRACT(EPOCH FROM cc.duration) / 60)::INT, cc.scheduled_on, cc.call_link,
	       cc.created_on, cc.updated_on,
	       cc.all_notes, cc.plan, cc.attendees, cc.action_items, cc.actual_call_duration,
	       wi.id, wi.subject, wi.number,
	       COALESCE(au.name, NULLIF(TRIM(CONCAT_WS(' ', au.first_name, au.last_name)), ''), au.email)`

const callRequestFrom = `
	FROM customer_call cc
	LEFT JOIN work_item wi ON wi.id = cc.work_item_id
	LEFT JOIN "case" c ON c.id = wi.id` + caseLikeJoins + `
	LEFT JOIN "user" au ON au.id = cc.assigned_to_id`

func scanCallRequest(row pgx.Row) (domain.CallRequestView, error) {
	var (
		v                                           domain.CallRequestView
		id                                          string
		number, state, reason, callLink             *string
		finalTimes                                  []byte
		durationMin                                 *int
		scheduledOn                                 *time.Time
		createdOn, updatedOn                        time.Time
		notes, plan, attendees, actionItems, actual *string
		caseID, caseSubject, caseNumber             *string
		assignee                                    *string
	)
	if err := row.Scan(
		&id, &number, &state, &reason, &finalTimes,
		&durationMin, &scheduledOn, &callLink,
		&createdOn, &updatedOn,
		&notes, &plan, &attendees, &actionItems, &actual,
		&caseID, &caseSubject, &caseNumber,
		&assignee,
	); err != nil {
		return domain.CallRequestView{}, err
	}

	v.ID = id
	v.Number = stringOrEmpty(number)
	v.Case = domain.CallRequestCaseRef{ID: stringOrEmpty(caseID), Name: stringOrEmpty(caseSubject), Number: caseNumber}
	v.Reason = reason
	v.PreferredTimes = decodeFinalTimes(finalTimes)
	if durationMin != nil {
		v.DurationMin = *durationMin
	}
	if scheduledOn != nil {
		s := scheduledOn.UTC().Format(time.RFC3339)
		v.ScheduleTime = &s
	}
	v.MeetingLink = callLink
	v.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	v.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	if state != nil {
		v.State = CallRequestStateFromEnum(*state)
	}
	v.Assignee = assignee
	v.Notes = notes
	v.Plan = plan
	v.Attendees = attendees
	v.ActionItems = actionItems
	v.ActualDurationMin = parseActualDurationMin(actual)
	return v, nil
}

// runCallRequestSearch executes the count and page queries concurrently for
// the given WHERE/ORDER BY and their bound args, each through Scoped so the
// caller's identity (pulled from ctx) is set for both customer_call's own
// RLS policy (migration 0143) and, through callRequestFrom's caseLikeJoins,
// the RLS-protected `announcement` table's (migration 000085) -- both must
// see the SAME transaction's identity, which Scoped guarantees per call.
func (r *callRequestRepo) runCallRequestSearch(ctx context.Context, where, orderBy string, args []any, pagination domain.Pagination) ([]domain.CallRequestView, int, error) {
	// Hide a call request whose parent is an ANNOUNCEMENT the caller cannot
	// see: work_item RLS alone passes any project member, and the LEFT JOINed
	// case fields (subject/number) would otherwise come back for it. The
	// wi.id IS NULL branch keeps a call request with no parent work item.
	// Added to the shared WHERE so the count and page queries stay in step.
	// Skipped for an Unrestricted caller (announcementLeakGuardFor); a ctx with
	// no identity at all keeps the guard.
	callerScope, _ := CallerIdentityFromContext(ctx)
	where += " AND (wi.id IS NULL OR " + announcementLeakGuardFor(callerScope) + ")"
	countQuery := `SELECT COUNT(*) ` + callRequestFrom + ` ` + where
	dataQuery := fmt.Sprintf(`%s %s %s %s LIMIT $%d OFFSET $%d`,
		callRequestSelect, callRequestFrom, where, orderBy, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), pagination.Limit, pagination.Offset)

	var total int
	views := make([]domain.CallRequestView, 0, pagination.Limit)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count call requests: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query call requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			v, err := scanCallRequest(rows)
			if err != nil {
				return fmt.Errorf("scan call request: %w", err)
			}
			views = append(views, v)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func callRequestStatesToEnums(states []domain.CallRequestStateType) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = callRequestStateToEnum(s)
	}
	return out
}

func caseStatesToEnums(states []domain.CaseState) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = strings.ToUpper(string(s))
	}
	return out
}

// SearchCallRequests implements CallRequestRepository.
func (r *callRequestRepo) SearchCallRequests(ctx context.Context, caseID string, states []domain.CallRequestStateType, pagination domain.Pagination) ([]domain.CallRequestView, int, error) {
	args := []any{caseID}
	where := `WHERE cc.work_item_id = $1::text::uuid`
	if len(states) > 0 {
		args = append(args, callRequestStatesToEnums(states))
		where += fmt.Sprintf(` AND cc.state = ANY($%d::text[]::customer_call_state_enum[])`, len(args))
	}
	return r.runCallRequestSearch(ctx, where, `ORDER BY cc.created_on DESC, cc.id`, args, pagination)
}

// callRequestSortColumns maps the accepted sort fields to their columns.
// scheduleTime sorts NULLs (unscheduled requests) last in either direction.
var callRequestSortColumns = map[domain.CallRequestSortField]string{
	domain.CallRequestSortFieldCreatedOn:    "cc.created_on",
	domain.CallRequestSortFieldUpdatedOn:    "cc.updated_on",
	domain.CallRequestSortFieldScheduleTime: "cc.scheduled_on",
}

// SearchAllCallRequests implements CallRequestRepository.
func (r *callRequestRepo) SearchAllCallRequests(ctx context.Context, f domain.SearchAllCallRequestsFilters, sortBy domain.CallRequestSort, pagination domain.Pagination) ([]domain.CallRequestView, int, error) {
	var args []any
	where := `WHERE 1=1`
	add := func(clause string, val any) {
		args = append(args, val)
		where += fmt.Sprintf(" AND "+clause, len(args))
	}

	// "Assigned to" means the PARENT CASE's assignee (the documented contract:
	// csm-portal's SearchAllCallRequestsPayload.assignedUserIds), so a
	// dashboard's "My Call Requests" lists the calls on the cases I own. The
	// call's own assignee -- the engineer who agreed to attend, set only once
	// the call is scheduled and optional even then -- is OR'd in so a call
	// handed to someone other than the case owner still reaches them. Matching
	// only cc.assigned_to_id (what this did before) left every
	// pending_on_wso2 call out: nothing assigns one until it is scheduled.
	// $%[1]d is reused for both columns so the array is bound once.
	if len(f.AssignedUserIDs) > 0 {
		add(`(wi.assigned_to_id = ANY($%[1]d::text[]::uuid[]) OR cc.assigned_to_id = ANY($%[1]d::text[]::uuid[]))`, f.AssignedUserIDs)
	}
	// A case's team here is its account's CRE team (account.cre_team_id) -- the
	// same path the case search's creTeam filter takes, and the id the
	// dashboards' team selector hands over (BeTeam.creGroupId). The case's own
	// work_item.assignment_group_id is not used: it is unpopulated on synced
	// data (a case's assignedTeam comes back null). EXISTS rather than another
	// JOIN keeps the shared FROM, and so the count/page query pair, unchanged.
	if len(f.AssignmentTeamIDs) > 0 {
		add(`EXISTS (SELECT 1 FROM account ta WHERE ta.id = wi.account_id AND ta.cre_team_id = ANY($%d::text[]::uuid[]))`, f.AssignmentTeamIDs)
	}
	if len(f.States) > 0 {
		add(`cc.state = ANY($%d::text[]::customer_call_state_enum[])`, callRequestStatesToEnums(f.States))
	}
	if len(f.CaseStates) > 0 {
		add(caseLikeStateColumn+` = ANY($%d::text[])`, caseStatesToEnums(f.CaseStates))
	}
	if len(f.ExcludeCaseStates) > 0 {
		add(`(`+caseLikeStateColumn+` IS NULL OR `+caseLikeStateColumn+` <> ALL($%d::text[]))`, caseStatesToEnums(f.ExcludeCaseStates))
	}

	col, ok := callRequestSortColumns[sortBy.Field]
	if !ok {
		col = callRequestSortColumns[domain.CallRequestSortFieldUpdatedOn]
	}
	dir := "DESC"
	if sortBy.Order == domain.CallRequestSortOrderAsc {
		dir = "ASC"
	}
	orderBy := fmt.Sprintf(`ORDER BY %s %s NULLS LAST, cc.id`, col, dir)

	return r.runCallRequestSearch(ctx, where, orderBy, args, pagination)
}

// CreateCallRequest implements CallRequestRepository.
func (r *callRequestRepo) CreateCallRequest(ctx context.Context, req domain.CreateCallRequestRequest, callerID, callerEmail string) (domain.CreateCallRequestResponse, error) {
	// A new request is raised by the customer, so it starts pending on WSO2
	// (who must schedule or counter-propose). preferred times land in
	// final_times as a JSON array, the shape decodeFinalTimes reads back.
	times, err := json.Marshal(req.UTCTimes)
	if err != nil {
		return domain.CreateCallRequestResponse{}, fmt.Errorf("encode utcTimes: %w", err)
	}

	// The announcementVisibilityLeakGuard (case_repo.go) keeps a caller from
	// raising a call request against an ANNOUNCEMENT whose extension row RLS
	// hides from them: work_item RLS alone would let it through.
	//
	// INSERT ... SELECT ... FROM work_item so a nonexistent (or non-case)
	// work item yields zero rows -> NotFoundError, instead of a bare
	// foreign-key violation.
	query := `
		INSERT INTO customer_call (
			id, created_on, updated_on, created_by, updated_by,
			work_item_id, opened_by_id, opened_on, is_active, state,
			duration, reason, final_times
		)
		SELECT gen_random_uuid(), NOW(), NOW(), $1, $1,
		       wi.id, $2::text::uuid, NOW(), TRUE, 'PENDING_ON_WSO2'::customer_call_state_enum,
		       make_interval(mins => $3::int), $4::text, $5::text::jsonb
		FROM work_item wi
		WHERE wi.id = $6::text::uuid AND wi.type = ANY(` + caseLikeWorkItemTypes + `)
		  AND ` + announcementVisibilityLeakGuard + `
		RETURNING id, created_on`

	var id string
	var createdOn time.Time
	err = r.db.QueryRow(ctx, query,
		callerEmail, callerID, req.DurationMinutes, req.Reason, string(times), req.CaseID,
	).Scan(&id, &createdOn)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return domain.CreateCallRequestResponse{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CreateCallRequestResponse{}, fmt.Errorf("create call request: %w", err)
	}

	var resp domain.CreateCallRequestResponse
	resp.Message = "Call request created successfully."
	resp.CallRequest.ID = id
	resp.CallRequest.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	resp.CallRequest.CreatedBy = callerEmail
	resp.CallRequest.State = CallRequestStateFromEnum(callRequestStateToEnum(domain.CallRequestStatePendingOnWSO2))
	return resp, nil
}

// CreateCallRequestFromServiceNow implements CallRequestRepository. Same
// shape as CreateCallRequest's own INSERT ... SELECT ... FROM work_item
// (so a nonexistent, or non-case-like, req.CaseID still yields a
// NotFoundError rather than a bare foreign-key violation, and the
// announcement visibility leak guard still applies) -- the only difference
// is id/created_on/created_by/updated_by come from the already-SUCCESSFUL
// ServiceNow create (id, createdBy, createdOn) rather than being generated
// here.
func (r *callRequestRepo) CreateCallRequestFromServiceNow(ctx context.Context, req domain.CreateCallRequestRequest, id, createdBy string, createdOn time.Time, callerID string) (domain.CreateCallRequestResponse, error) {
	times, err := json.Marshal(req.UTCTimes)
	if err != nil {
		return domain.CreateCallRequestResponse{}, fmt.Errorf("encode utcTimes: %w", err)
	}

	query := `
		INSERT INTO customer_call (
			id, created_on, updated_on, created_by, updated_by,
			work_item_id, opened_by_id, opened_on, is_active, state,
			duration, reason, final_times
		)
		SELECT $1::text::uuid, $2, $2, $3, $3,
		       wi.id, $4::text::uuid, $2, TRUE, 'PENDING_ON_WSO2'::customer_call_state_enum,
		       make_interval(mins => $5::int), $6::text, $7::text::jsonb
		FROM work_item wi
		WHERE wi.id = $8::text::uuid AND wi.type = ANY(` + caseLikeWorkItemTypes + `)
		  AND ` + announcementVisibilityLeakGuard + `
		RETURNING id, created_on`

	var gotID string
	var gotCreatedOn time.Time
	err = r.db.QueryRow(ctx, query,
		id, createdOn, createdBy, callerID, req.DurationMinutes, req.Reason, string(times), req.CaseID,
	).Scan(&gotID, &gotCreatedOn)
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		return domain.CreateCallRequestResponse{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return domain.CreateCallRequestResponse{}, fmt.Errorf("create call request from servicenow: %w", err)
	}

	var resp domain.CreateCallRequestResponse
	resp.Message = "Call request created successfully."
	resp.CallRequest.ID = gotID
	resp.CallRequest.CreatedOn = gotCreatedOn.UTC().Format(time.RFC3339)
	resp.CallRequest.CreatedBy = createdBy
	resp.CallRequest.State = CallRequestStateFromEnum(callRequestStateToEnum(domain.CallRequestStatePendingOnWSO2))
	return resp, nil
}

// UpdateCallRequest implements CallRequestRepository.
//
// Every optional field is applied with COALESCE, so an absent field leaves the
// stored value untouched. The state is always written.
//
// One transition is guarded: concluding a call WITHOUT post-call notes ("Mark as
// completed", digiops-cs#3350) only applies to a call that is scheduled or notes
// pending. Nothing else here validates the current state, so without the guard a
// stale screen (or a direct caller) could turn a cancelled, rejected or
// never-scheduled call into a completed one. The check is part of the UPDATE's own
// WHERE clause, so it is atomic with the write; a call that fails it comes back as
// a ConflictError naming its current state, not as a misleading not-found.
// Concluding WITH notes ("Send call notes") is unchanged and not guarded.
//
// That same transition is also staff-only. Before notes became optional the notes
// requirement was the only thing stopping an external caller from concluding a
// call: the customer portal's backend forwards any state key it is given, cannot
// send notes, and RLS lets a project member update their own project's calls. So a
// notes-less conclude from anyone but an internal (Unrestricted) caller is a
// ForbiddenError, answered before any lookup so it says nothing about the call.
func (r *callRequestRepo) UpdateCallRequest(ctx context.Context, req domain.UpdateCallRequestRequest, assigneeID *string, callerEmail string) (domain.UpdateCallRequestResponse, error) {
	var finalTimes *string
	if req.UTCTimes != nil {
		b, err := json.Marshal(req.UTCTimes)
		if err != nil {
			return domain.UpdateCallRequestResponse{}, fmt.Errorf("encode utcTimes: %w", err)
		}
		s := string(b)
		finalTimes = &s
	}

	var scheduledOn *time.Time
	if req.MeetingDate != nil {
		t, err := time.Parse(time.RFC3339, *req.MeetingDate)
		if err != nil {
			return domain.UpdateCallRequestResponse{}, &apierror.ValidationError{Msg: "meetingDate must be a valid RFC3339 timestamp"}
		}
		scheduledOn = &t
	}

	var actual *string
	if req.ActualDurationMin != nil {
		s := strconv.Itoa(*req.ActualDurationMin)
		actual = &s
	}

	var caseID *string
	if req.CaseID != "" {
		caseID = &req.CaseID
	}

	// nil (SQL NULL) means "any current state"; set only for a notes-less conclude.
	// That path also never writes the notes column (blank notes included), so
	// completing a call cannot erase what is already recorded on it.
	notes := req.Notes
	completableFrom := []string{
		callRequestStateToEnum(domain.CallRequestStateScheduled),
		callRequestStateToEnum(domain.CallRequestStateNotesPending),
	}
	var onlyFromStates any
	if req.State == domain.CallRequestStateConcluded && (req.Notes == nil || strings.TrimSpace(*req.Notes) == "") {
		scope, ok := CallerIdentityFromContext(ctx)
		if !ok || !scope.Unrestricted {
			return domain.UpdateCallRequestResponse{}, &apierror.ForbiddenError{Msg: "only WSO2 staff can mark a call request as completed"}
		}
		notes = nil
		onlyFromStates = completableFrom
	}

	const query = `
		UPDATE customer_call SET
			state = $2::text::customer_call_state_enum,
			updated_on = NOW(),
			updated_by = $3,
			final_times = COALESCE($4::text::jsonb, final_times),
			duration = COALESCE(make_interval(mins => $5::int), duration),
			scheduled_on = COALESCE($6::timestamptz, scheduled_on),
			assigned_to_id = COALESCE($7::text::uuid, assigned_to_id),
			all_notes = COALESCE($8::text, all_notes),
			plan = COALESCE($9::text, plan),
			attendees = COALESCE($10::text, attendees),
			action_items = COALESCE($11::text, action_items),
			actual_call_duration = COALESCE($12::text, actual_call_duration)
		WHERE id = $1::text::uuid
		  AND ($13::text::uuid IS NULL OR work_item_id = $13::text::uuid)
		  AND ($14::text[] IS NULL OR state::text = ANY($14::text[]))
		RETURNING id, updated_on`

	var id string
	var updatedOn time.Time
	err := r.db.QueryRow(ctx, query,
		req.ID, callRequestStateToEnum(req.State), callerEmail,
		finalTimes, req.DurationMinutes, scheduledOn, assigneeID,
		notes, req.Plan, req.Attendees, req.ActionItems, actual, caseID, onlyFromStates,
	).Scan(&id, &updatedOn)
	if errors.Is(err, pgx.ErrNoRows) && onlyFromStates != nil {
		// No row matched. Either the call does not exist (not found), or it is in a state
		// this conclude is not allowed from (conflict): look it up to tell which, so a
		// stale "Mark as completed" gets an accurate answer instead of "not found".
		var current *string
		lookup := r.db.QueryRow(ctx,
			`SELECT state::text FROM customer_call
			 WHERE id = $1::text::uuid AND ($2::text::uuid IS NULL OR work_item_id = $2::text::uuid)`,
			req.ID, caseID).Scan(&current)
		switch {
		case lookup == nil:
			if current != nil && (*current == completableFrom[0] || *current == completableFrom[1]) {
				// It became completable between the UPDATE and this lookup (a concurrent
				// reschedule, say): saying "not allowed from <state>" would be wrong.
				return domain.UpdateCallRequestResponse{}, &apierror.ConflictError{Msg: "the call request changed while this was being applied; please try again"}
			}
			label := "in an unknown state"
			if current != nil {
				label = "currently " + CallRequestStateFromEnum(*current).Label
			}
			return domain.UpdateCallRequestResponse{}, &apierror.ConflictError{Msg: "a call request can only be marked completed while it is scheduled or notes pending (this one is " + label + ")"}
		case errors.Is(lookup, pgx.ErrNoRows):
			// Not visible or not there: fall through to not found below.
		default:
			// A failed lookup is a failure, not a missing call.
			return domain.UpdateCallRequestResponse{}, fmt.Errorf("look up call request after refused conclude: %w", lookup)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) || IsRLSPolicyViolation(err) {
		if caseID != nil {
			return domain.UpdateCallRequestResponse{}, &apierror.NotFoundError{Msg: "call request not found for this case"}
		}
		return domain.UpdateCallRequestResponse{}, &apierror.NotFoundError{Msg: "call request not found"}
	}
	if err != nil {
		return domain.UpdateCallRequestResponse{}, fmt.Errorf("update call request: %w", err)
	}

	var resp domain.UpdateCallRequestResponse
	resp.Message = "Call request updated successfully."
	resp.CallRequest.ID = id
	resp.CallRequest.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	resp.CallRequest.UpdatedBy = callerEmail
	return resp, nil
}

// SetCallRequestSNSysID implements CallRequestRepository.
func (r *callRequestRepo) SetCallRequestSNSysID(ctx context.Context, id, snSysID string) error {
	_, err := r.db.Exec(ctx, `UPDATE customer_call SET sn_sys_id = $1 WHERE id = $2::text::uuid`, snSysID, id)
	if err != nil {
		return fmt.Errorf("set call request sn sys id: %w", err)
	}
	return nil
}

// GetCallRequestSNSysID implements CallRequestRepository.
func (r *callRequestRepo) GetCallRequestSNSysID(ctx context.Context, id string) (*string, error) {
	var snSysID *string
	err := r.db.QueryRow(ctx, `SELECT sn_sys_id FROM customer_call WHERE id = $1::text::uuid`, id).Scan(&snSysID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "call request not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("get call request sn sys id: %w", err)
	}
	return snSysID, nil
}
