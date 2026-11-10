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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertMuteRuleRepository reads and writes alert mute rules (migration 0278). Every method takes the caller's clock reading (now)
// so a rule's state is judged by one clock, the service's. A write that names a state guards it in its own WHERE clause, so
// it is atomic with the change; nil (or false) means no row with that id was in that state, and the service tells a missing
// id from a wrong state by reading it again.
type AlertMuteRuleRepository interface {
	// Search returns a page of rules in any of states (all when empty) matching query, newest created first, and the total.
	Search(ctx context.Context, states []domain.AlertMuteRuleState, query string, now time.Time, limit, offset int) ([]domain.AlertMuteRule, int, error)
	// Get returns the rule, or nil when there is none with that id.
	Get(ctx context.Context, id string) (*domain.AlertMuteRule, error)
	// Active returns every rule muting alerts at now, oldest start first.
	Active(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error)
	// Create stores a new rule with the next number.
	Create(ctx context.Context, actor string, w domain.AlertMuteRuleWrite) (domain.AlertMuteRule, error)
	// Update replaces every field of a rule that is still scheduled at now.
	Update(ctx context.Context, actor, id string, w domain.AlertMuteRuleWrite, now time.Time) (*domain.AlertMuteRule, error)
	// Extend moves the end of a rule that is scheduled or active at now to endOn, which must be later than its end.
	Extend(ctx context.Context, actor, id string, endOn, now time.Time) (*domain.AlertMuteRule, error)
	// End ends a rule that is active at now, at now.
	End(ctx context.Context, actor, id string, now time.Time) (*domain.AlertMuteRule, error)
	// Delete removes a rule that is still scheduled at now.
	Delete(ctx context.Context, id string, now time.Time) (bool, error)
}

type alertMuteRuleRepo struct {
	db *Scoped
}

// NewAlertMuteRuleRepository constructs a Postgres-backed AlertMuteRuleRepository. alert_mute_rule is under row-level security
// (internal callers only, migration 100028), so every query carries the caller's identity.
func NewAlertMuteRuleRepository(db *Scoped) AlertMuteRuleRepository {
	return &alertMuteRuleRepo{db: db}
}

const alertMuteRuleColumns = `id::text, number, name, description, time_zone, start_on, end_on, conditions,
	created_by, created_on, updated_by, updated_on, ended_by, ended_on`

// The state predicates, with the clock reading as $1.
var alertMuteRuleStateSQL = map[domain.AlertMuteRuleState]string{
	domain.AlertMuteRuleStateScheduled: `(ended_on IS NULL AND start_on > $1)`,
	domain.AlertMuteRuleStateActive:    `(ended_on IS NULL AND start_on <= $1 AND end_on > $1)`,
	// Expired takes the rules ended early too.
	domain.AlertMuteRuleStateExpired: `(ended_on IS NOT NULL OR end_on <= $1)`,
	domain.AlertMuteRuleStateEnded:   `(ended_on IS NOT NULL)`,
}

func scanAlertMuteRule(row pgx.Row) (domain.AlertMuteRule, error) {
	var b domain.AlertMuteRule
	var conditions []byte
	if err := row.Scan(&b.ID, &b.Number, &b.Name, &b.Description, &b.TimeZone, &b.StartOn, &b.EndOn, &conditions,
		&b.CreatedBy, &b.CreatedOn, &b.UpdatedBy, &b.UpdatedOn, &b.EndedBy, &b.EndedOn); err != nil {
		return b, err
	}
	b.Conditions = []domain.AlertMuteRuleCondition{}
	if err := json.Unmarshal(conditions, &b.Conditions); err != nil {
		return b, fmt.Errorf("decode conditions of alert mute rule %s: %w", b.ID, err)
	}
	b.StartOn, b.EndOn, b.CreatedOn, b.UpdatedOn = b.StartOn.UTC(), b.EndOn.UTC(), b.CreatedOn.UTC(), b.UpdatedOn.UTC()
	if b.EndedOn != nil {
		t := b.EndedOn.UTC()
		b.EndedOn = &t
	}
	return b, nil
}

func collectAlertMuteRules(rows pgx.Rows) ([]domain.AlertMuteRule, error) {
	defer rows.Close()
	out := []domain.AlertMuteRule{}
	for rows.Next() {
		b, err := scanAlertMuteRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// oneAlertMuteRule scans a RETURNING row; no row is nil, not an error.
func oneAlertMuteRule(row pgx.Row, op string) (*domain.AlertMuteRule, error) {
	b, err := scanAlertMuteRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &b, nil
}

func (r *alertMuteRuleRepo) Search(ctx context.Context, states []domain.AlertMuteRuleState, query string, now time.Time, limit, offset int) ([]domain.AlertMuteRule, int, error) {
	args := []any{now}
	var where []string
	if len(states) > 0 {
		var ors []string
		for _, s := range states {
			pred, ok := alertMuteRuleStateSQL[s]
			if !ok {
				return nil, 0, fmt.Errorf("search alert mute rules: unknown state %q", s)
			}
			ors = append(ors, pred)
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if pattern := likePattern(query); pattern != "" {
		args = append(args, pattern)
		where = append(where, `(name ILIKE $2 ESCAPE '\' OR number ILIKE $2 ESCAPE '\' OR description ILIKE $2 ESCAPE '\'
			OR created_by ILIKE $2 ESCAPE '\'
			OR EXISTS (SELECT 1 FROM jsonb_array_elements(conditions) c WHERE c->>'value' ILIKE $2 ESCAPE '\'))`)
	}
	// $1 (the clock) is always passed; reference it so Postgres can type it when no state filter uses it.
	cond := "$1::timestamptz IS NOT NULL"
	if len(where) > 0 {
		cond += " AND " + strings.Join(where, " AND ")
	}

	var list []domain.AlertMuteRule
	var total int
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		pageArgs := append(append([]any{}, args...), limit, offset)
		n := len(args)
		rows, err := r.db.Query(gctx, fmt.Sprintf(`SELECT %s FROM alert_mute_rule WHERE %s
			ORDER BY created_on DESC, id DESC LIMIT $%d OFFSET $%d`, alertMuteRuleColumns, cond, n+1, n+2), pageArgs...)
		if err != nil {
			return err
		}
		list, err = collectAlertMuteRules(rows)
		return err
	})
	g.Go(func() error {
		return r.db.QueryRow(gctx, `SELECT count(*) FROM alert_mute_rule WHERE `+cond, args...).Scan(&total)
	})
	if err := g.Wait(); err != nil {
		return nil, 0, fmt.Errorf("search alert mute rules: %w", err)
	}
	return list, total, nil
}

func (r *alertMuteRuleRepo) Get(ctx context.Context, id string) (*domain.AlertMuteRule, error) {
	return oneAlertMuteRule(r.db.QueryRow(ctx, `SELECT `+alertMuteRuleColumns+` FROM alert_mute_rule WHERE id = $1::uuid`, id), "get alert mute rule")
}

func (r *alertMuteRuleRepo) Active(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error) {
	rows, err := r.db.Query(ctx, `SELECT `+alertMuteRuleColumns+` FROM alert_mute_rule
		WHERE ended_on IS NULL AND start_on <= $1 AND end_on > $1 ORDER BY start_on, id`, now)
	if err != nil {
		return nil, fmt.Errorf("list active alert mute rules: %w", err)
	}
	list, err := collectAlertMuteRules(rows)
	if err != nil {
		return nil, fmt.Errorf("list active alert mute rules: %w", err)
	}
	return list, nil
}

func conditionsJSON(c []domain.AlertMuteRuleCondition) (string, error) {
	if c == nil {
		c = []domain.AlertMuteRuleCondition{}
	}
	b, err := json.Marshal(c)
	return string(b), err
}

func (r *alertMuteRuleRepo) Create(ctx context.Context, actor string, w domain.AlertMuteRuleWrite) (domain.AlertMuteRule, error) {
	conditions, err := conditionsJSON(w.Conditions)
	if err != nil {
		return domain.AlertMuteRule{}, fmt.Errorf("create alert mute rule: %w", err)
	}
	b, err := scanAlertMuteRule(r.db.QueryRow(ctx, `INSERT INTO alert_mute_rule
		(name, description, time_zone, start_on, end_on, conditions, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $7) RETURNING `+alertMuteRuleColumns,
		w.Name, w.Description, w.TimeZone, w.StartOn, w.EndOn, conditions, actor))
	if err != nil {
		return domain.AlertMuteRule{}, fmt.Errorf("create alert mute rule: %w", err)
	}
	return b, nil
}

func (r *alertMuteRuleRepo) Update(ctx context.Context, actor, id string, w domain.AlertMuteRuleWrite, now time.Time) (*domain.AlertMuteRule, error) {
	conditions, err := conditionsJSON(w.Conditions)
	if err != nil {
		return nil, fmt.Errorf("update alert mute rule: %w", err)
	}
	return oneAlertMuteRule(r.db.QueryRow(ctx, `UPDATE alert_mute_rule
		SET name = $3, description = $4, time_zone = $5, start_on = $6, end_on = $7, conditions = $8::jsonb,
		    updated_by = $9, updated_on = $1
		WHERE id = $2::uuid AND `+alertMuteRuleStateSQL[domain.AlertMuteRuleStateScheduled]+` RETURNING `+alertMuteRuleColumns,
		now, id, w.Name, w.Description, w.TimeZone, w.StartOn, w.EndOn, conditions, actor), "update alert mute rule")
}

func (r *alertMuteRuleRepo) Extend(ctx context.Context, actor, id string, endOn, now time.Time) (*domain.AlertMuteRule, error) {
	return oneAlertMuteRule(r.db.QueryRow(ctx, `UPDATE alert_mute_rule
		SET end_on = $3, updated_by = $4, updated_on = $1
		WHERE id = $2::uuid AND ended_on IS NULL AND end_on > $1 AND $3 > end_on AND $3 - start_on <= INTERVAL '72 hours'
		RETURNING `+alertMuteRuleColumns,
		now, id, endOn, actor), "extend alert mute rule")
}

func (r *alertMuteRuleRepo) End(ctx context.Context, actor, id string, now time.Time) (*domain.AlertMuteRule, error) {
	return oneAlertMuteRule(r.db.QueryRow(ctx, `UPDATE alert_mute_rule
		SET ended_on = $1, ended_by = $3, updated_by = $3, updated_on = $1
		WHERE id = $2::uuid AND `+alertMuteRuleStateSQL[domain.AlertMuteRuleStateActive]+` RETURNING `+alertMuteRuleColumns,
		now, id, actor), "end alert mute rule")
}

func (r *alertMuteRuleRepo) Delete(ctx context.Context, id string, now time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM alert_mute_rule WHERE id = $2::uuid AND `+alertMuteRuleStateSQL[domain.AlertMuteRuleStateScheduled],
		now, id)
	if err != nil {
		return false, fmt.Errorf("delete alert mute rule: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
