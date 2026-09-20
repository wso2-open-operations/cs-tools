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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ConversationRepository defines the persistence operations for conversation
// (migration 000057), a work_item type extension (id IS work_item.id) --
// same shared-PK pattern as "case"/change_request. conversation itself has
// only a `state` column beyond the shared PK; InitialMessage/MessageCount
// have no backing column at all and are derived from the generic `comment`
// table (migration 000037, keyed by work_item_id): InitialMessage is the
// earliest comment's content, MessageCount is the total comment count --
// the only tables in this schema that could plausibly answer "what was said
// in this conversation."
//
// CreateConversation has no Postgres implementation: work_item.number has
// no DB default and no backing sequence anywhere in migrations/, the same
// blocker CaseRepository.CreateCase/ChangeRequestRepository's own doc
// comment already describe.
type ConversationRepository interface {
	// SearchConversations returns a filtered, sorted, paginated slice of
	// conversations together with the total count of matching rows before
	// pagination.
	SearchConversations(ctx context.Context, req domain.SearchConversationsRequest, callerEmail string) ([]domain.SearchConversationView, int, error)
	// GetConversation returns the full detail of a single conversation by
	// its UUID, or a NotFoundError if no matching row exists.
	GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error)
	// UpdateConversation transitions the conversation's state, returning the
	// updated summary. Returns a NotFoundError if id does not exist.
	UpdateConversation(ctx context.Context, id string, state domain.ConversationState, actorEmail string) (domain.UpdatedConversation, error)
}

type conversationRepo struct {
	db *pgxpool.Pool
}

// NewConversationRepository constructs a ConversationRepository backed by the given connection pool.
func NewConversationRepository(db *pgxpool.Pool) ConversationRepository {
	return &conversationRepo{db: db}
}

// conversationStateToEnum/conversationStateFromEnum bridge one deliberate
// mismatch: conversation_state_enum's label for "closed" is 'CLOSE' (no D),
// not domain.ConversationStateClosed's "CLOSED" -- every other state
// matches by identity. Used on every read, write, and filter path so none
// of them can drift from the others.
func conversationStateToEnum(s domain.ConversationState) string {
	if s == domain.ConversationStateClosed {
		return "CLOSE"
	}
	return string(s)
}

func conversationStateFromEnum(enumValue string) domain.ConversationState {
	if enumValue == "CLOSE" {
		return domain.ConversationStateClosed
	}
	return domain.ConversationState(enumValue)
}

const conversationFromJoins = `
	FROM work_item wi
	JOIN conversation c ON c.id = wi.id
	LEFT JOIN project p ON p.id = wi.project_id
	LEFT JOIN work_item case_wi ON case_wi.id = wi.parent_id
	LEFT JOIN "user" u ON LOWER(u.email) = LOWER(wi.created_by)`

// conversationMessageStats batch-fetches each conversation's earliest
// comment content and total comment count, avoiding one query per
// conversation. Keyed by work_item_id.
type conversationMessageStats struct {
	initialMessage *string
	count          int
}

func (r *conversationRepo) fetchMessageStats(ctx context.Context, workItemIDs []string) (map[string]conversationMessageStats, error) {
	out := map[string]conversationMessageStats{}
	if len(workItemIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT work_item_id, COUNT(*),
		       (ARRAY_AGG(content ORDER BY created_on ASC))[1]
		FROM comment
		WHERE work_item_id = ANY($1::uuid[])
		GROUP BY work_item_id`, workItemIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch conversation message stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var workItemID string
		var count int
		var initial *string
		if err := rows.Scan(&workItemID, &count, &initial); err != nil {
			return nil, fmt.Errorf("scan conversation message stats: %w", err)
		}
		out[workItemID] = conversationMessageStats{initialMessage: initial, count: count}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversation message stats: %w", err)
	}
	return out, nil
}

func conversationWhereClause(f domain.SearchConversationsFilters, callerEmail string) (string, []any) {
	where := "WHERE wi.type = 'CONVERSATION'"
	args := []any{}
	argIdx := 1

	add := func(clause string, val any) {
		where += fmt.Sprintf(" AND "+clause, argIdx)
		args = append(args, val)
		argIdx++
	}

	if len(f.ProjectIDs) > 0 {
		add("wi.project_id = ANY($%d::uuid[])", f.ProjectIDs)
	}
	if len(f.States) > 0 {
		states := make([]string, len(f.States))
		for i, st := range f.States {
			states[i] = conversationStateToEnum(st)
		}
		add("c.state = ANY($%d::text[]::conversation_state_enum[])", states)
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
	if f.CreatedByMe && callerEmail != "" {
		add("LOWER(wi.created_by) = LOWER($%d)", callerEmail)
	}
	if len(f.CreatedBy) > 0 {
		add("LOWER(wi.created_by) = ANY(SELECT LOWER(x) FROM unnest($%d::text[]) x)", f.CreatedBy)
	}

	return where, args
}

// SearchConversations implements ConversationRepository.
func (r *conversationRepo) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest, callerEmail string) ([]domain.SearchConversationView, int, error) {
	where, args := conversationWhereClause(req.Filters, callerEmail)

	sortCol := "wi.created_on"
	if req.SortBy.Field == domain.ConversationSortFieldUpdatedOn {
		sortCol = "wi.updated_on"
	}
	sortDir := "DESC"
	if req.SortBy.Order == domain.ConversationSortOrderAsc {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + conversationFromJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT wi.id, wi.number, p.id, p.name, case_wi.id, case_wi.number, c.state::TEXT,
		        wi.created_on, u.id, wi.created_by, u.name, u.first_name, u.last_name
		 %s %s
		 ORDER BY %s %s, wi.id
		 LIMIT $%d OFFSET $%d`,
		conversationFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchConversationView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count conversations: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query conversations: %w", err)
		}
		defer rows.Close()

		type row struct {
			view       domain.SearchConversationView
			workItemID string
		}
		var out []row
		for rows.Next() {
			var (
				id, number                        string
				projID, projName                  *string
				caseID, caseNumber                *string
				state                             *string
				createdOn                         time.Time
				userID                            *string
				createdBy                         string
				userName, userFirstName, userLast *string
			)
			if err := rows.Scan(&id, &number, &projID, &projName, &caseID, &caseNumber, &state,
				&createdOn, &userID, &createdBy, &userName, &userFirstName, &userLast); err != nil {
				return fmt.Errorf("scan conversation: %w", err)
			}
			v := domain.SearchConversationView{ID: &id, Number: &number, CreatedOn: createdOn.UTC().Format(time.RFC3339)}
			if projID != nil {
				v.Project = &domain.EntityRef{ID: *projID, Name: stringOrEmpty(projName)}
			}
			if caseID != nil {
				v.Case = &domain.EntityRef{ID: *caseID, Name: stringOrEmpty(caseNumber)}
			}
			if state != nil {
				s := string(conversationStateFromEnum(*state))
				v.State = &s
			}
			name := stringOrEmpty(userName)
			if name == "" {
				name = strings.TrimSpace(stringOrEmpty(userFirstName) + " " + stringOrEmpty(userLast))
			}
			uid := ""
			if userID != nil {
				uid = *userID
			}
			v.CreatedBy = domain.NewUserReference(uid, createdBy, name)
			out = append(out, row{view: v, workItemID: id})
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate conversations: %w", err)
		}

		workItemIDs := make([]string, len(out))
		for i, o := range out {
			workItemIDs[i] = o.workItemID
		}
		stats, err := r.fetchMessageStats(egCtx, workItemIDs)
		if err != nil {
			return err
		}
		result := make([]domain.SearchConversationView, len(out))
		for i, o := range out {
			s := stats[o.workItemID]
			o.view.InitialMessage = s.initialMessage
			o.view.MessageCount = s.count
			result[i] = o.view
		}
		views = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// GetConversation implements ConversationRepository.
func (r *conversationRepo) GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error) {
	query := `
		SELECT wi.id, wi.number, p.id, p.name, case_wi.id, case_wi.number, c.state::TEXT,
		       wi.created_on, wi.created_by, wi.updated_on, wi.updated_by
		` + conversationFromJoins + `
		WHERE wi.id = $1 AND wi.type = 'CONVERSATION'`

	var (
		id2, number          string
		projID, projName     *string
		caseID, caseNumber   *string
		state                *string
		createdOn, updatedOn time.Time
		createdBy, updatedBy string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &projID, &projName, &caseID, &caseNumber, &state,
		&createdOn, &createdBy, &updatedOn, &updatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ConversationDetails{}, &apierror.NotFoundError{Msg: "conversation not found"}
	}
	if err != nil {
		return domain.ConversationDetails{}, fmt.Errorf("get conversation: %w", err)
	}

	d := domain.ConversationDetails{
		ID: id2, Number: &number,
		CreatedOn: createdOn.UTC().Format(time.RFC3339), CreatedBy: createdBy,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339), UpdatedBy: updatedBy,
	}
	if projID != nil {
		d.Project = &domain.EntityRef{ID: *projID, Name: stringOrEmpty(projName)}
	}
	if caseID != nil {
		d.Case = &domain.EntityRef{ID: *caseID, Name: stringOrEmpty(caseNumber)}
	}
	if state != nil {
		s := string(conversationStateFromEnum(*state))
		d.State = &s
	}

	stats, err := r.fetchMessageStats(ctx, []string{id2})
	if err != nil {
		return domain.ConversationDetails{}, err
	}
	if s, ok := stats[id2]; ok {
		d.InitialMessage = s.initialMessage
		d.MessageCount = s.count
	}
	return d, nil
}

// UpdateConversation implements ConversationRepository.
func (r *conversationRepo) UpdateConversation(ctx context.Context, id string, state domain.ConversationState, actorEmail string) (domain.UpdatedConversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.UpdatedConversation{}, fmt.Errorf("update conversation: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE conversation SET state = $1::text::conversation_state_enum WHERE id = $2`,
		conversationStateToEnum(state), id,
	); err != nil {
		return domain.UpdatedConversation{}, fmt.Errorf("update conversation state: %w", err)
	}

	var number string
	var updatedOn time.Time
	err = tx.QueryRow(ctx,
		`UPDATE work_item SET updated_on = NOW(), updated_by = $1 WHERE id = $2 AND type = 'CONVERSATION' RETURNING number, updated_on`,
		actorEmail, id,
	).Scan(&number, &updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UpdatedConversation{}, &apierror.NotFoundError{Msg: "conversation not found"}
	}
	if err != nil {
		return domain.UpdatedConversation{}, fmt.Errorf("update conversation work_item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.UpdatedConversation{}, fmt.Errorf("update conversation: commit: %w", err)
	}

	stateStr := string(state)
	return domain.UpdatedConversation{
		ID:        id,
		Number:    &number,
		UpdatedOn: updatedOn.UTC().Format(time.RFC3339),
		UpdatedBy: actorEmail,
		State:     &stateStr,
	}, nil
}
