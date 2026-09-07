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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/taxonomy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IssuesHandler serves GET /issues and GET /issues/{id} (SPEC §6.3/§6.4).
// Privacy: the row shape it returns carries no title, labels, assignees,
// opener, or event actors — see SPEC's non-negotiable #1.
type IssuesHandler struct {
	pool *pgxpool.Pool
	cfg  *config.AppConfig
}

// NewIssuesHandler creates an IssuesHandler.
func NewIssuesHandler(pool *pgxpool.Pool, cfg *config.AppConfig) *IssuesHandler {
	return &IssuesHandler{pool: pool, cfg: cfg}
}

type slaWire struct {
	BudgetHours    *float64 `json:"budgetHours"`
	ConsumedHours  float64  `json:"consumedHours"`
	RemainingHours *float64 `json:"remainingHours"`
	PctConsumed    *float64 `json:"pctConsumed"`
	SlaState       string   `json:"slaState"`
	SlaRunning     bool     `json:"slaRunning"`
}

type issueWire struct {
	ID              int32      `json:"id"`
	Number          int        `json:"number"`
	State           string     `json:"state"`
	URL             *string    `json:"url"`
	Repo            *string    `json:"repo"`
	Priority        *string    `json:"priority"`
	CurrentStatus   *string    `json:"currentStatus"`
	GithubCreatedAt *time.Time `json:"githubCreatedAt"`
	GithubUpdatedAt *time.Time `json:"githubUpdatedAt"`
	Sla             *slaWire   `json:"sla"`
}

type issueRow struct {
	ID              int32
	GithubNumber    int
	State           string
	HTMLURL         *string
	Owner           string
	Name            string
	Priority        *string
	CurrentStatus   *string
	GithubCreatedAt *time.Time
	GithubUpdatedAt *time.Time
	BudgetHours     *float64
	ConsumedHours   *float64
	RemainingHours  *float64
	PctConsumed     *float64
	SlaState        *string
	SlaRunning      *bool
}

const issueListSelect = `
	i.id, i.github_number, i.state, i.html_url, r.owner, r.name, i.priority, i.current_status,
	i.github_created_at, i.github_updated_at,
	s.budget_hours, s.consumed_hours, s.remaining_hours, s.pct_consumed, s.sla_state, s.sla_running
`

const issueListFrom = `
	FROM issues i
	JOIN repositories r ON r.id = i.repository_id
	LEFT JOIN issue_sla s ON s.issue_id = i.id
`

func scanIssueRow(row pgx.Row) (issueRow, error) {
	var r issueRow
	err := row.Scan(
		&r.ID, &r.GithubNumber, &r.State, &r.HTMLURL, &r.Owner, &r.Name, &r.Priority, &r.CurrentStatus,
		&r.GithubCreatedAt, &r.GithubUpdatedAt,
		&r.BudgetHours, &r.ConsumedHours, &r.RemainingHours, &r.PctConsumed, &r.SlaState, &r.SlaRunning,
	)
	return r, err
}

func toIssueWire(r issueRow) issueWire {
	repo := r.Owner + "/" + r.Name
	w := issueWire{
		ID:              r.ID,
		Number:          r.GithubNumber,
		State:           r.State,
		URL:             r.HTMLURL,
		Repo:            &repo,
		Priority:        r.Priority,
		CurrentStatus:   r.CurrentStatus,
		GithubCreatedAt: r.GithubCreatedAt,
		GithubUpdatedAt: r.GithubUpdatedAt,
	}
	if r.SlaState != nil {
		consumed := 0.0
		if r.ConsumedHours != nil {
			consumed = *r.ConsumedHours
		}
		running := false
		if r.SlaRunning != nil {
			running = *r.SlaRunning
		}
		w.Sla = &slaWire{
			BudgetHours:    r.BudgetHours,
			ConsumedHours:  consumed,
			RemainingHours: r.RemainingHours,
			PctConsumed:    r.PctConsumed,
			SlaState:       *r.SlaState,
			SlaRunning:     running,
		}
	}
	return w
}

// ListIssues handles GET /issues.
func (h *IssuesHandler) ListIssues(w http.ResponseWriter, r *http.Request) {
	q, errMsg := parseIssuesQuery(r.URL.Query())
	if errMsg != "" {
		apierror.ValidationFailed(w, errMsg)
		return
	}

	whereSQL, args := buildIssuesWhere(taxonomy.CsStatuses(h.cfg), q)

	orderSQL := "i.github_updated_at DESC"
	if q.Order == "budget_desc" {
		// Uses the issue_sla.pct_consumed index (SPEC §4).
		orderSQL = "s.pct_consumed DESC NULLS LAST"
	}

	limitArgs := &sqlArgs{values: args}
	limitPlaceholder := limitArgs.add(q.Limit)

	query := fmt.Sprintf("SELECT %s %s WHERE %s ORDER BY %s LIMIT %s", issueListSelect, issueListFrom, whereSQL, orderSQL, limitPlaceholder)
	rows, err := h.pool.Query(r.Context(), query, limitArgs.values...)
	if err != nil {
		apierror.Internal(w, r, "list issues query failed", err)
		return
	}
	defer rows.Close()

	result := make([]issueWire, 0)
	for rows.Next() {
		row, err := scanIssueRow(rows)
		if err != nil {
			apierror.Internal(w, r, "scan issue row failed", err)
			return
		}
		result = append(result, toIssueWire(row))
	}
	if err := rows.Err(); err != nil {
		apierror.Internal(w, r, "iterate issue rows failed", err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type eventWire struct {
	ID             int32     `json:"id"`
	PreviousStatus *string   `json:"previousStatus"`
	Status         *string   `json:"status"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type issueDetailWire struct {
	issueWire
	Events []eventWire `json:"events"`
}

// GetIssue handles GET /issues/{id}: the issue row plus its full event
// timeline, ascending, no actors. 404 not_found when absent or its
// repository is disabled.
func (h *IssuesHandler) GetIssue(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		apierror.ValidationFailed(w, "id must be an integer")
		return
	}

	ctx := r.Context()
	query := fmt.Sprintf("SELECT %s %s WHERE i.id = $1 AND r.enabled = true", issueListSelect, issueListFrom)
	row, err := scanIssueRow(h.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			apierror.NotFound(w, "issue not found")
			return
		}
		apierror.Internal(w, r, "get issue query failed", err)
		return
	}

	events, err := fetchIssueEvents(ctx, h.pool, row.ID)
	if err != nil {
		apierror.Internal(w, r, "fetch issue events failed", err)
		return
	}

	writeJSON(w, http.StatusOK, issueDetailWire{issueWire: toIssueWire(row), Events: events})
}

func fetchIssueEvents(ctx context.Context, pool *pgxpool.Pool, issueID int32) ([]eventWire, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, previous_status, status, occurred_at
		FROM issue_status_events
		WHERE issue_id = $1
		ORDER BY occurred_at ASC
	`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]eventWire, 0)
	for rows.Next() {
		var e eventWire
		if err := rows.Scan(&e.ID, &e.PreviousStatus, &e.Status, &e.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
