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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// ConversationRepository defines the persistence operations for conversation
// (migration 0057), a work_item type extension (id IS work_item.id) --
// same shared-PK pattern as "case"/change_request. conversation itself has
// only a `state` column beyond the shared PK. InitialMessage is
// work_item.description, where csm-sync-service lands ServiceNow's
// u_initial_message (the field SN's own API returned as initialMessage),
// falling back to the earliest comment for a row with no description.
// MessageCount is the conversation's total comment count (the generic
// `comment` table, migration 0040, keyed by work_item_id).
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
	// CreateConversation inserts a conversation (work_item + conversation
	// rows) in one transaction. Returns a ForbiddenError when the caller is
	// not a member of the project, a ValidationError when the project does
	// not exist.
	CreateConversation(ctx context.Context, in CreateConversationInput) (domain.CreatedConversation, error)
}

// CreateConversationInput is CreateConversation's input. ID and Number are
// empty for a natively created conversation (both generated here), and set
// to ServiceNow's values when ServiceNow created it first
// (DATA_SOURCE=postgres-servicenow-dual-write).
type CreateConversationInput struct {
	ID             string
	Number         string
	ProjectID      string
	Subject        string
	InitialMessage string
	CreatedBy      string
	State          domain.ConversationState
}

type conversationRepo struct {
	db *Scoped
}

// NewConversationRepository constructs a ConversationRepository backed by the given connection pool.
func NewConversationRepository(db *Scoped) ConversationRepository {
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

// conversationSearchFrom is the whole FROM clause SearchConversations' WHERE
// needs: every filter reads work_item (wi) or conversation (c) and nothing
// else. The display joins (project, linked case, creator) live only in the
// page query below, and are applied to the rows of one page.
const conversationSearchFrom = `
	FROM work_item wi
	JOIN conversation c ON c.id = wi.id`

// conversationSearchQueries renders the COUNT and page queries for one
// conversation search from its WHERE clause (conversationWhereClause).
// pageArgs is the number of bind arguments the WHERE uses; LIMIT and OFFSET
// are the two placeholders after them.
//
// The page is chosen first, by an inner query that selects only wi.id, and the
// display columns are joined onto just those rows afterwards -- the same shape
// SearchCases uses. The count carries no display joins either. Before, both
// queries ran the three display joins for every matching conversation, and the
// creator join (LOWER("user".email) = LOWER(wi.created_by)) is not on a unique
// key: the planner could choose a nested loop that rescanned the whole "user"
// table once per matching row, so a search matching a few hundred
// conversations cost hundreds of milliseconds of database time per query (and
// two queries per request). That could not be fixed with an index alone: the
// plan flipped with the planner's row estimate for the free-text filter.
//
// The creator is resolved by LATERAL ... LIMIT 1, ordered by id, for the
// reason SearchWorkItemAttachments does the same: "user".email has no unique
// constraint, so a plain join fans one conversation out into one row per user
// sharing the address, and the COUNT (which cannot see that join) disagrees
// with the page. Every join here is a LEFT JOIN and the WHERE reads only
// wi/c, so neither query returns a row it did not before; row-level security
// still applies to every table involved on every statement.
func conversationSearchQueries(where, sortCol, sortDir string, pageArgs int) (countQuery, dataQuery string) {
	countQuery = "SELECT COUNT(*) " + conversationSearchFrom + " " + where
	dataQuery = fmt.Sprintf(
		`SELECT wi.id, wi.number, p.id, p.name, case_wi.id, case_wi.number, c.state::TEXT,
		        wi.created_on, u.id, wi.created_by, u.name, u.first_name, u.last_name, wi.description
		 FROM (SELECT wi.id %s %s
		       ORDER BY %s %s, wi.id
		       LIMIT $%d OFFSET $%d) page
		 JOIN work_item wi ON wi.id = page.id
		 JOIN conversation c ON c.id = wi.id
		 LEFT JOIN project p ON p.id = wi.project_id
		 LEFT JOIN work_item case_wi ON case_wi.id = wi.parent_id
		 LEFT JOIN LATERAL (
		     SELECT u2.id, u2.name, u2.first_name, u2.last_name
		     FROM "user" u2
		     WHERE LOWER(u2.email) = LOWER(wi.created_by)
		     ORDER BY u2.id
		     LIMIT 1
		 ) u ON TRUE
		 ORDER BY %s %s, wi.id`,
		conversationSearchFrom, where, sortCol, sortDir, pageArgs+1, pageArgs+2,
		sortCol, sortDir,
	)
	return countQuery, dataQuery
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
	if f.StartUpdatedDate != nil {
		add("wi.updated_on >= $%d", *f.StartUpdatedDate)
	}
	if f.EndUpdatedDate != nil {
		add("wi.updated_on <= $%d", *f.EndUpdatedDate)
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

	countQuery, dataQuery := conversationSearchQueries(where, sortCol, sortDir, len(args))
	dataArgs := append(append([]any{}, args...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var views []domain.SearchConversationView

	eg, egCtx := errgroup.WithContext(ctx)

	// SkipTotal: the caller does not show a total (global search shows a handful
	// of hits), so the COUNT is not run at all -- it is as costly as the page
	// query and holds a second pool connection while it runs.
	if req.SkipTotal {
		total = domain.TotalNotComputed
	} else {
		eg.Go(func() error {
			if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
				return fmt.Errorf("count conversations: %w", err)
			}
			return nil
		})
	}

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query conversations: %w", err)
		}
		defer rows.Close()

		type row struct {
			view        domain.SearchConversationView
			workItemID  string
			description *string
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
				description                       *string
			)
			if err := rows.Scan(&id, &number, &projID, &projName, &caseID, &caseNumber, &state,
				&createdOn, &userID, &createdBy, &userName, &userFirstName, &userLast, &description); err != nil {
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
			out = append(out, row{view: v, workItemID: id, description: description})
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
			o.view.InitialMessage = initialMessage(o.description, s.initialMessage)
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
		       wi.created_on, wi.created_by, wi.updated_on, wi.updated_by, wi.description
		` + conversationFromJoins + `
		WHERE wi.id = $1 AND wi.type = 'CONVERSATION'`

	var (
		id2, number          string
		projID, projName     *string
		caseID, caseNumber   *string
		state                *string
		createdOn, updatedOn time.Time
		createdBy, updatedBy string
		description          *string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&id2, &number, &projID, &projName, &caseID, &caseNumber, &state,
		&createdOn, &createdBy, &updatedOn, &updatedBy, &description,
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
	s := stats[id2]
	d.InitialMessage = initialMessage(description, s.initialMessage)
	d.MessageCount = s.count
	return d, nil
}

// initialMessage prefers work_item.description (ServiceNow's
// u_initial_message, or the first message of a conversation created here)
// over the earliest comment, which need not be what the customer first
// asked (the REST create path stores only the assistant's reply as a
// comment).
func initialMessage(description, earliestComment *string) *string {
	if description != nil && *description != "" {
		return description
	}
	return earliestComment
}

// UpdateConversation implements ConversationRepository.
func (r *conversationRepo) UpdateConversation(ctx context.Context, id string, state domain.ConversationState, actorEmail string) (domain.UpdatedConversation, error) {
	var number string
	var updatedOn time.Time
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE conversation SET state = $1::text::conversation_state_enum WHERE id = $2`,
			conversationStateToEnum(state), id,
		)
		if err != nil {
			return fmt.Errorf("update conversation state: %w", err)
		}
		// conversation's RLS USING clause (migration 0146) silently
		// excludes a row the caller isn't a project member of -- a plain
		// Exec with no RETURNING never surfaces that as pgx.ErrNoRows the
		// way the work_item UPDATE below does, so it must be checked
		// explicitly here or a non-member caller would see a false
		// "success" with the state left unchanged (same fix already applied
		// to PatchChangeRequest, migration 0145).
		if ct.RowsAffected() == 0 {
			return &apierror.NotFoundError{Msg: "conversation not found"}
		}

		err = tx.QueryRow(ctx,
			`UPDATE work_item SET updated_on = NOW(), updated_by = $1 WHERE id = $2 AND type = 'CONVERSATION' RETURNING number, updated_on`,
			actorEmail, id,
		).Scan(&number, &updatedOn)
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "conversation not found"}
		}
		if err != nil {
			return fmt.Errorf("update conversation work_item: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.UpdatedConversation{}, err
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

// insertConversationWorkItemQuery inserts the work_item half of a
// conversation. id/number are COALESCEd so one query serves both callers:
// NULL generates them (gen_random_uuid(), next_portal_work_item_number(),
// migration 0140 -- the same numbering every natively created work item
// uses), and a ServiceNow-first create supplies its own. COALESCE evaluates
// lazily, so a supplied number never draws from the sequence. Subject and
// description carry the first message the same way csm-sync-service lands
// u_initial_message (truncated subject, full description).
const insertConversationWorkItemQuery = `
	INSERT INTO work_item (
		id, created_on, updated_on, created_by, updated_by,
		number, subject, description, type, project_id
	)
	VALUES (
		COALESCE($1::uuid, gen_random_uuid()), NOW(), NOW(), $2, $2,
		COALESCE($3, next_portal_work_item_number()), $4, $5, 'CONVERSATION'::work_item_type_enum, $6::uuid
	)
	RETURNING id, number, created_on`

// CreateConversation implements ConversationRepository. The two inserts are
// separate statements rather than one CTE: conversation_write (migration
// 0190) looks the project up from work_item, and a sibling CTE's insert is
// not visible to that subquery, while an earlier statement in the same
// transaction is.
func (r *conversationRepo) CreateConversation(ctx context.Context, in CreateConversationInput) (domain.CreatedConversation, error) {
	var (
		id, number string
		createdOn  time.Time
	)
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, insertConversationWorkItemQuery,
			nullIfEmpty(in.ID), in.CreatedBy, nullIfEmpty(in.Number), in.Subject, in.InitialMessage, in.ProjectID,
		).Scan(&id, &number, &createdOn); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO conversation (id, state) VALUES ($1, $2::text::conversation_state_enum)`,
			id, conversationStateToEnum(in.State),
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if IsRLSPolicyViolation(err) {
			return domain.CreatedConversation{}, &apierror.ForbiddenError{Msg: "not authorized to create conversations for this project"}
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation: project_id
			return domain.CreatedConversation{}, &apierror.ValidationError{Msg: "projectId does not exist: " + in.ProjectID}
		}
		return domain.CreatedConversation{}, fmt.Errorf("create conversation: %w", err)
	}

	state := string(in.State)
	return domain.CreatedConversation{
		ID:        id,
		Number:    number,
		CreatedBy: in.CreatedBy,
		CreatedOn: createdOn.UTC().Format(time.RFC3339),
		State:     &state,
	}, nil
}
