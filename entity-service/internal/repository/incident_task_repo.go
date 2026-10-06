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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// IncidentTaskRepository defines the operations for incident_task
// (migration 0066), a work_item type extension (id IS work_item.id) --
// same shared-PK pattern as "case"/change_request. Tasks are created by the
// incident flows (IncidentReportTx.CreateIncidentTask); UpdateIncidentTask
// is the portal's state/close-notes write.
//
// A task's assignment group is work_item.assignment_group_id, which the
// incident flows set on create (the incident's group for the report task,
// WSO2 SRE Team for the alert tasks). It is read back as AssignmentGroup and
// filtered by "assignmentGroupId"; grouping by it is not supported here, as
// for problems and incidents.
//
// "state" is deliberately NOT translated through the SN-specific
// parsedIncidentTaskFilters.StateKeys (raw ServiceNow integers -- see that
// type's own doc comment for why incident_task's SN state choice list has
// no confirmed-complete, unambiguous enum to translate through in the
// first place). Postgres's own incident_task_state_enum has no such
// ambiguity, so this data source instead accepts the enum's own label
// strings (case-insensitive) directly in the "state" filter -- a
// deliberate, documented divergence from the SN data source's raw-integer
// convention for the same filter field.
type IncidentTaskRepository interface {
	// SearchIncidentTasks returns a filtered, paginated slice of incident
	// tasks together with the total count of matching rows before
	// pagination.
	SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest, states, incidentIDs, assignmentGroupIDs []string) ([]domain.IncidentTask, int, error)
	// AggregateIncidentTasks returns server-side aggregated counts of
	// incident tasks per value of groupBy, capped to the top maxGroups
	// buckets with the remainder folded into the returned OthersCount.
	AggregateIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest, states, incidentIDs, assignmentGroupIDs []string, groupBy string, maxGroups int) (domain.AggregateResponse, error)
	// GetIncidentTask returns the full detail of a single incident task by
	// its UUID, or a NotFoundError if no matching row exists.
	GetIncidentTask(ctx context.Context, id string) (domain.IncidentTaskDetail, error)
	// UpdateIncidentTask writes req.State and/or req.CloseNotes in one
	// transaction, with ServiceNow's task-table side effects ("mark closed",
	// "Set Closure Fields", "task reopener"): entering a closed state
	// (domain.IncidentTaskClosedStates) from an open one sets is_active
	// false, and closed_on / closed_by_id (the "user" matching actorEmail)
	// only where they are still empty; moving back to an open state sets
	// is_active true and keeps closed_on / closed_by_id, as ServiceNow does.
	// Returns a NotFoundError if id is not an incident task.
	UpdateIncidentTask(ctx context.Context, req domain.UpdateIncidentTaskRequest, actorEmail string) error
}

type incidentTaskRepo struct {
	db *Scoped
}

// NewIncidentTaskRepository constructs an IncidentTaskRepository backed by the given connection pool.
func NewIncidentTaskRepository(db *Scoped) IncidentTaskRepository {
	return &incidentTaskRepo{db: db}
}

// incidentTaskStateDisplay renders incident_task_state_enum's SNAKE_UPPER_CASE
// labels (e.g. "WORK_IN_PROGRESS") as space-separated title case ("Work In
// Progress") for StateLabel -- same convention as taskSlaStageDisplay
// (task_sla_repo.go).
func incidentTaskStateDisplay(raw string) string {
	words := strings.Split(strings.ToLower(raw), "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

const incidentTaskFromJoins = `
	FROM work_item wi
	JOIN incident_task it ON it.id = wi.id
	LEFT JOIN incident inc ON inc.id = it.incident_id
	LEFT JOIN work_item inc_wi ON inc_wi.id = inc.id
	LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
	LEFT JOIN "group" ag ON ag.id = wi.assignment_group_id`

func incidentTaskWhereClause(f domain.SearchIncidentTasksFilters, states, incidentIDs, assignmentGroupIDs []string) (string, []any) {
	where := "WHERE 1=1"
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
	if f.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (wi.subject ILIKE $%d ESCAPE '\\' OR wi.number ILIKE $%d ESCAPE '\\')", argIdx, argIdx)
		args = append(args, pattern)
		argIdx++
	}
	if len(states) > 0 {
		add("it.state = ANY($%d::text[]::incident_task_state_enum[])", states)
	}
	if len(incidentIDs) > 0 {
		add("it.incident_id = ANY($%d::uuid[])", incidentIDs)
	}
	if len(assignmentGroupIDs) > 0 {
		add("wi.assignment_group_id = ANY($%d::uuid[])", assignmentGroupIDs)
	}

	return where, args
}

func scanIncidentTask(row interface{ Scan(...any) error }) (domain.IncidentTask, error) {
	var (
		id, number, subject string
		state               *string
		incID, incNumber    *string
		aeID, aeName        *string
		agID, agName        *string
	)
	if err := row.Scan(&id, &number, &subject, &state, &incID, &incNumber, &aeID, &aeName, &agID, &agName); err != nil {
		return domain.IncidentTask{}, err
	}
	t := domain.IncidentTask{ID: &id, Number: &number, Subject: &subject}
	if state != nil {
		lower := strings.ToUpper(*state)
		t.State = &lower
		label := incidentTaskStateDisplay(*state)
		t.StateLabel = &label
	}
	if incID != nil {
		t.Incident = &domain.CaseNumberRef{ID: *incID, Number: stringOrEmpty(incNumber)}
	}
	if aeID != nil {
		t.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		t.AssignmentGroup = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	return t, nil
}

// SearchIncidentTasks implements IncidentTaskRepository.
func (r *incidentTaskRepo) SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest, states, incidentIDs, assignmentGroupIDs []string) ([]domain.IncidentTask, int, error) {
	where, args := incidentTaskWhereClause(req.Filters, states, incidentIDs, assignmentGroupIDs)

	countQuery := "SELECT COUNT(*) " + incidentTaskFromJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT wi.id, wi.number, wi.subject, it.state::TEXT, inc.id, inc_wi.number, ae.id,
		        COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		        ag.id, ag.name
		 %s %s
		 ORDER BY wi.created_on DESC, wi.id
		 LIMIT $%d OFFSET $%d`,
		incidentTaskFromJoins, where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var tasks []domain.IncidentTask

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count incident tasks: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query incident tasks: %w", err)
		}
		defer rows.Close()

		result := make([]domain.IncidentTask, 0, req.Pagination.Limit)
		for rows.Next() {
			t, err := scanIncidentTask(rows)
			if err != nil {
				return fmt.Errorf("scan incident task: %w", err)
			}
			result = append(result, t)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate incident tasks: %w", err)
		}
		tasks = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// incidentTaskAggregateColumns maps a groupBy value to the real column/cast
// used to group by it. "assignmentGroup" is not supported -- see this file's
// own package doc comment.
var incidentTaskAggregateColumns = map[string]string{
	"state": "it.state::TEXT",
}

// AggregateIncidentTasks implements IncidentTaskRepository.
func (r *incidentTaskRepo) AggregateIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest, states, incidentIDs, assignmentGroupIDs []string, groupBy string, maxGroups int) (domain.AggregateResponse, error) {
	col, ok := incidentTaskAggregateColumns[groupBy]
	if !ok {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy=" + groupBy + " is not supported on the PostgreSQL data source"}
	}

	where, args := incidentTaskWhereClause(req.Filters, states, incidentIDs, assignmentGroupIDs)

	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*) AS bucket_count
		%s %s AND %s IS NOT NULL
		GROUP BY %s
		ORDER BY bucket_count DESC, bucket`, col, incidentTaskFromJoins, where, col, col)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate incident tasks: %w", err)
	}
	defer rows.Close()

	var buckets []domain.AggregateBucket
	var totalRecords int
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan incident task bucket: %w", err)
		}
		lowerKey := strings.ToLower(key)
		buckets = append(buckets, domain.AggregateBucket{Key: lowerKey, Label: incidentTaskStateDisplay(key), Count: count})
		totalRecords += count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate incident task buckets: %w", err)
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

// GetIncidentTask implements IncidentTaskRepository.
func (r *incidentTaskRepo) GetIncidentTask(ctx context.Context, id string) (domain.IncidentTaskDetail, error) {
	query := `
		SELECT wi.id, wi.number, wi.subject, it.state::TEXT, inc.id, inc_wi.number, ae.id,
		       COALESCE(ae.name, NULLIF(TRIM(CONCAT_WS(' ', ae.first_name, ae.last_name)), '')),
		       wi.description, it.priority::TEXT, it.opened_on, it.closed_on, it.close_notes,
		       ag.id, ag.name
		` + incidentTaskFromJoins + `
		WHERE wi.id = $1 AND wi.type = 'INCIDENT_TASK'`

	var (
		id2, number, subject string
		state                *string
		incID, incNumber     *string
		aeID, aeName         *string
		description          *string
		priority             *string
		openedOn, closedOn   *time.Time
		closeNotes           *string
		agID, agName         *string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &subject, &state, &incID, &incNumber, &aeID, &aeName,
		&description, &priority, &openedOn, &closedOn, &closeNotes, &agID, &agName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IncidentTaskDetail{}, &apierror.NotFoundError{Msg: "incident task not found"}
	}
	if err != nil {
		return domain.IncidentTaskDetail{}, fmt.Errorf("get incident task: %w", err)
	}

	d := domain.IncidentTaskDetail{
		ID: &id2, Number: &number, Subject: &subject,
		Description: description, Priority: priority, CloseNotes: closeNotes,
	}
	if openedOn != nil {
		s := openedOn.UTC().Format(time.RFC3339)
		d.OpenedOn = &s
	}
	if closedOn != nil {
		s := closedOn.UTC().Format(time.RFC3339)
		d.ClosedOn = &s
	}
	if state != nil {
		upper := strings.ToUpper(*state)
		d.State = &upper
		label := incidentTaskStateDisplay(*state)
		d.StateLabel = &label
	}
	if incID != nil {
		d.Incident = &domain.CaseNumberRef{ID: *incID, Number: stringOrEmpty(incNumber)}
	}
	if aeID != nil {
		d.AssignedTo = &domain.EntityRef{ID: *aeID, Name: stringOrEmpty(aeName)}
	}
	if agID != nil {
		d.AssignmentGroup = &domain.EntityRef{ID: *agID, Name: stringOrEmpty(agName)}
	}
	return d, nil
}

// UpdateIncidentTask implements IncidentTaskRepository.
func (r *incidentTaskRepo) UpdateIncidentTask(ctx context.Context, req domain.UpdateIncidentTaskRequest, actorEmail string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		var current *string
		err := tx.QueryRow(ctx, `
			SELECT it.state::TEXT
			FROM incident_task it
			JOIN work_item wi ON wi.id = it.id
			WHERE it.id = $1 AND wi.type = 'INCIDENT_TASK'
			FOR UPDATE OF it, wi`, req.ID).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "incident task not found"}
		}
		if err != nil {
			return fmt.Errorf("update incident task: read state: %w", err)
		}

		var sets []string
		var args []any
		add := func(assignment string, val any) {
			args = append(args, val)
			sets = append(sets, fmt.Sprintf(assignment, len(args)))
		}
		if req.State != nil {
			add("state = $%d::TEXT::incident_task_state_enum", *req.State)
			wasClosed := current != nil && domain.IncidentTaskClosedStates[*current]
			closing := domain.IncidentTaskClosedStates[*req.State]
			switch {
			case closing && !wasClosed:
				sets = append(sets, "is_active = FALSE", "closed_on = COALESCE(closed_on, NOW())")
				add(`closed_by_id = COALESCE(closed_by_id, (SELECT id FROM "user" WHERE LOWER(email) = LOWER($%d) LIMIT 1))`, actorEmail)
			case !closing && wasClosed:
				sets = append(sets, "is_active = TRUE")
			}
		}
		if req.CloseNotes != nil {
			add("close_notes = $%d", *req.CloseNotes)
		}
		args = append(args, req.ID)
		if _, err := tx.Exec(ctx,
			fmt.Sprintf(`UPDATE incident_task SET %s WHERE id = $%d`, strings.Join(sets, ", "), len(args)),
			args...); err != nil {
			return fmt.Errorf("update incident task: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE work_item SET updated_on = NOW(), updated_by = $2 WHERE id = $1`,
			req.ID, actorEmail); err != nil {
			return fmt.Errorf("update incident task: work_item: %w", err)
		}
		return nil
	})
}
