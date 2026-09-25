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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// EscalationRepository defines the read operations for case_escalation and
// case_escalation_notification_list (migration 000053). Only reads:
// CreateEscalation has no defined level-transition rule (does ESCALATE
// always mean "current level + 1", capped at EL5? is there a per-case-type
// override?) or notification-recipient rule (watchers? the assigned
// engineer? an account's own escalation contacts?) to derive from the
// schema alone -- see EscalationService's own doc comment for why writes
// stay ServiceNow-only.
type EscalationRepository interface {
	// SearchEscalations returns a filtered, sorted, paginated slice of
	// escalations together with the total count of matching rows before
	// pagination. projectIDs restricts to escalations on cases in these
	// projects -- nil means unrestricted (internal caller); the caller is
	// responsible for resolving this from its own AccessScope, never from
	// caller-supplied input, since SearchEscalationsFilters carries no
	// projectIds field of its own for a customer to narrow with.
	SearchEscalations(ctx context.Context, caseIDs []string, currentLevels []int, projectIDs []string, sortField, sortOrder string, limit, offset int) ([]domain.Escalation, int, error)
}

type escalationRepo struct {
	db *pgxpool.Pool
}

// NewEscalationRepository constructs an EscalationRepository backed by the given connection pool.
func NewEscalationRepository(db *pgxpool.Pool) EscalationRepository {
	return &escalationRepo{db: db}
}

// escalationLevelToEnum/escalationLevelFromEnum convert between
// SearchEscalationsFilters.CurrentLevels' plain ints (0..5, the same
// convention CaseView.EscalationLevel's own doc comment uses) and
// case_escalation_level_enum's 'EL0'..'EL5' labels.
func escalationLevelToEnum(level int) string {
	return "EL" + strconv.Itoa(level)
}

// escalationChoiceItem builds a domain.ChoiceListItem for a case_escalation_level_enum
// value. Unlike the ServiceNow data source, Postgres has no human-readable label for an
// escalation level anywhere in this schema -- ID and Label are both the plain "0".."5"
// id (case_escalation_level_enum's 'EL' prefix stripped) rather than inventing display
// text this data source has no source for.
func escalationChoiceItem(enumValue string) domain.ChoiceListItem {
	id := strings.TrimPrefix(enumValue, "EL")
	return domain.ChoiceListItem{ID: id, Label: id}
}

const escalationSelectColumns = `
	ce.id, ce.work_item_id, wi.number, wi.subject, wi.wso2_id,
	ce.current_level::TEXT, ce.previous_level::TEXT,
	ce.created_by, ce.created_on, ce.updated_on, ce.reason`

const escalationFromJoins = `
	FROM case_escalation ce
	JOIN work_item wi ON wi.id = ce.work_item_id`

func scanEscalation(row interface{ Scan(...any) error }) (domain.Escalation, error) {
	var (
		e                               domain.Escalation
		caseID, caseNumber, caseSubject string
		wso2ID                          *string
		currentLevel, previousLevel     *string
		createdOn, updatedOn            time.Time
	)
	err := row.Scan(
		&e.ID, &caseID, &caseNumber, &caseSubject, &wso2ID,
		&currentLevel, &previousLevel,
		&e.CreatedBy, &createdOn, &updatedOn, &e.Reason,
	)
	if err != nil {
		return domain.Escalation{}, err
	}
	e.Case = domain.ReferenceTableItem{ID: caseID, Name: caseSubject, Number: &caseNumber, InternalID: stringPtrOrNil(wso2ID)}
	if currentLevel != nil {
		e.CurrentLevel = escalationChoiceItem(*currentLevel)
	}
	if previousLevel != nil {
		e.PreviousLevel = escalationChoiceItem(*previousLevel)
	}
	e.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	e.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	return e, nil
}

// stringPtrOrNil returns nil for a nil or blank *string, otherwise itself --
// wso2_id can be NULL or ” (see case_repo.go's own note on this), and
// ReferenceTableItem.InternalID must stay nil rather than render "".
func stringPtrOrNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// getEscalationNotifiedUsers batch-fetches the notification list for every
// id in escalationIDs, avoiding one query per escalation.
func (r *escalationRepo) getEscalationNotifiedUsers(ctx context.Context, escalationIDs []string) (map[string][]domain.EscalationNotifiedUser, error) {
	out := map[string][]domain.EscalationNotifiedUser{}
	if len(escalationIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT cenl.case_escalation_id, u.id, u.user_name, COALESCE(u.name, NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '')), u.email
		FROM case_escalation_notification_list cenl
		JOIN "user" u ON u.id = cenl.user_id
		WHERE cenl.case_escalation_id = ANY($1::uuid[])
		ORDER BY cenl.id`, escalationIDs)
	if err != nil {
		return nil, fmt.Errorf("list escalation notified users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var escalationID, userID, userName string
		var name, email *string
		if err := rows.Scan(&escalationID, &userID, &userName, &name, &email); err != nil {
			return nil, fmt.Errorf("scan escalation notified user: %w", err)
		}
		out[escalationID] = append(out[escalationID], domain.EscalationNotifiedUser{ID: userID, UserName: userName, Name: name, Email: email})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate escalation notified users: %w", err)
	}
	return out, nil
}

// SearchEscalations implements EscalationRepository.
func (r *escalationRepo) SearchEscalations(ctx context.Context, caseIDs []string, currentLevels []int, projectIDs []string, sortField, sortOrder string, limit, offset int) ([]domain.Escalation, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(caseIDs) > 0 {
		args = append(args, caseIDs)
		where += fmt.Sprintf(" AND ce.work_item_id = ANY($%d::uuid[])", len(args))
	}
	if len(projectIDs) > 0 {
		args = append(args, projectIDs)
		where += fmt.Sprintf(" AND wi.project_id = ANY($%d::uuid[])", len(args))
	}
	if len(currentLevels) > 0 {
		levels := make([]string, len(currentLevels))
		for i, l := range currentLevels {
			levels[i] = escalationLevelToEnum(l)
		}
		args = append(args, levels)
		// ::text[] before ::case_escalation_level_enum[]: this repository
		// never registers case_escalation_level_enum/_case_escalation_level_enum
		// with pgx, so binding a []string directly to the enum array type has
		// no encode plan -- same fix as time_card_repo.go's state filter.
		where += fmt.Sprintf(" AND ce.current_level = ANY($%d::text[]::case_escalation_level_enum[])", len(args))
	}

	sortCol := "ce.created_on"
	if sortField == "updatedOn" {
		sortCol = "ce.updated_on"
	}
	sortDir := "DESC"
	if sortOrder == "asc" {
		sortDir = "ASC"
	}

	countQuery := "SELECT COUNT(*) " + escalationFromJoins + " " + where
	dataQuery := fmt.Sprintf("SELECT %s %s %s ORDER BY %s %s, ce.id LIMIT $%d OFFSET $%d",
		escalationSelectColumns, escalationFromJoins, where, sortCol, sortDir, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var escalations []domain.Escalation

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count escalations: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query escalations: %w", err)
		}
		defer rows.Close()

		out := make([]domain.Escalation, 0, limit)
		for rows.Next() {
			e, err := scanEscalation(rows)
			if err != nil {
				return fmt.Errorf("scan escalation: %w", err)
			}
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate escalations: %w", err)
		}
		escalations = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	ids := make([]string, len(escalations))
	for i, e := range escalations {
		ids[i] = e.ID
	}
	notifiedByEscalation, err := r.getEscalationNotifiedUsers(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range escalations {
		// openapi.yaml declares notificationSentTo as a required, non-nullable
		// array -- a map miss (no notification list for this escalation)
		// returns a nil slice, which json.Encode would render as null.
		notified := notifiedByEscalation[escalations[i].ID]
		if notified == nil {
			notified = []domain.EscalationNotifiedUser{}
		}
		escalations[i].NotificationSentTo = notified
	}

	return escalations, total, nil
}
