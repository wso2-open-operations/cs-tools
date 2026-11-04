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
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RotaReminderRequestRepository queues rota reminders asked for by hand
// (team_schedule_reminder_request, migration 0242).
type RotaReminderRequestRepository interface {
	// Create queues a request, unless one of the same family that sends any of
	// the same emails (overlappingReminderKinds: weekday is Morning and
	// Evening together) is still waiting (ConflictError) or was asked for within cooldown
	// (TooManyRequestsError). A FAILED request sent nobody anything, so it
	// does not start the cooldown: it can be asked for again at once. Checked and written under one lock, so two
	// presses at once queue one request.
	// A test is its own queue per person: a test never holds up the real
	// reminders or somebody else's test.
	Create(ctx context.Context, actor, family, kind string, test bool, cooldown time.Duration) (domain.RotaReminderRequest, error)
	// List is the family's latest requests, newest first.
	List(ctx context.Context, family string, limit int) ([]domain.RotaReminderRequest, error)
	// Claim marks the family's oldest waiting request SENDING and returns it:
	// one QUEUED, or one SENDING but not reported within staleAfter. Nil when
	// there is none.
	Claim(ctx context.Context, actor, family string, staleAfter time.Duration) (*domain.RotaReminderRequest, error)
	// Report records how a SENDING request went.
	Report(ctx context.Context, actor, family, id, status, result string) (domain.RotaReminderRequest, error)
}

type rotaReminderRequestRepository struct{ db *pgxpool.Pool }

// NewRotaReminderRequestRepository constructs a RotaReminderRequestRepository.
func NewRotaReminderRequestRepository(db *pgxpool.Pool) RotaReminderRequestRepository {
	return &rotaReminderRequestRepository{db: db}
}

const rotaReminderRequestColumns = `id::text, family, kind, status, test, requested_by, requested_on, claimed_on, handled_on, COALESCE(result, '')`

func scanRotaReminderRequest(row pgx.Row) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := row.Scan(&out.ID, &out.Family, &out.Kind, &out.Status, &out.Test, &out.RequestedBy, &out.RequestedOn, &out.ClaimedOn, &out.HandledOn, &out.Result)
	return out, err
}

func (r *rotaReminderRequestRepository) inTx(ctx context.Context, actor string, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actor); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// overlappingReminderKinds is every kind whose request sends one of the
// emails a request of kind sends: weekday is the Morning and the Evening
// together, so it overlaps both, and they overlap it but not each other.
func overlappingReminderKinds(kind string) []string {
	switch kind {
	case "weekday":
		return []string{"weekday", "morning", "evening"}
	case "morning", "evening":
		return []string{kind, "weekday"}
	}
	return []string{kind}
}

// reminderRequestLockKind is the lock a kind's requests are checked under:
// the weekday kinds share one, so two that overlap never pass the check at
// once.
func reminderRequestLockKind(kind string) string {
	switch kind {
	case "weekday", "morning", "evening":
		return "weekday"
	}
	return kind
}

func (r *rotaReminderRequestRepository) Create(ctx context.Context, actor, family, kind string, test bool, cooldown time.Duration) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		// A real request is one queue per family and the kinds it overlaps; a
		// test, the same per person.
		owner := ""
		if test {
			owner = actor
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('rota-reminder-request:' || $1 || ':' || $2 || ':' || $3 || ':' || $4, 0))`,
			family, reminderRequestLockKind(kind), strconv.FormatBool(test), owner); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		kinds := overlappingReminderKinds(kind)
		// Any overlapping request still waiting refuses this one, not only the
		// latest: with several kinds in play an older one (a Morning still
		// being sent) can sit behind a newer one already sent.
		waiting, err := scanRotaReminderRequest(tx.QueryRow(ctx, `SELECT `+rotaReminderRequestColumns+`
			FROM team_schedule_reminder_request
			WHERE family = $1 AND kind = ANY($2::text[]) AND test = $3 AND ($4 = '' OR LOWER(requested_by) = LOWER($4))
			  AND status IN ('QUEUED', 'SENDING')
			ORDER BY requested_on LIMIT 1`, family, kinds, test, owner))
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("read the waiting requests: %w", err)
		default:
			what := "a " + waiting.Kind + " reminder"
			if test {
				what = "your " + waiting.Kind + " test"
			}
			return &apierror.ConflictError{Msg: fmt.Sprintf("%s asked for by %s is already waiting to be sent", what, waiting.RequestedBy)}
		}
		last, err := scanRotaReminderRequest(tx.QueryRow(ctx, `SELECT `+rotaReminderRequestColumns+`
			FROM team_schedule_reminder_request
			WHERE family = $1 AND kind = ANY($2::text[]) AND test = $3 AND ($4 = '' OR LOWER(requested_by) = LOWER($4))
			  AND status <> 'FAILED'
			ORDER BY requested_on DESC LIMIT 1`, family, kinds, test, owner))
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("read the last request: %w", err)
		case time.Since(last.RequestedOn) < cooldown:
			what := "a " + last.Kind + " reminder was"
			if test {
				what = "your " + last.Kind + " test was"
			}
			return &apierror.TooManyRequestsError{Msg: fmt.Sprintf("%s asked for less than %d minutes ago; try again later", what, int(cooldown.Minutes()))}
		}
		out, err = scanRotaReminderRequest(tx.QueryRow(ctx, `
			INSERT INTO team_schedule_reminder_request (family, kind, test, requested_by, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $4, $4)
			RETURNING `+rotaReminderRequestColumns, family, kind, test, actor))
		return err
	})
	var conflict *apierror.ConflictError
	var tooMany *apierror.TooManyRequestsError
	if err != nil && !errors.As(err, &conflict) && !errors.As(err, &tooMany) {
		return domain.RotaReminderRequest{}, fmt.Errorf("queue a rota reminder: %w", err)
	}
	return out, err
}

func (r *rotaReminderRequestRepository) List(ctx context.Context, family string, limit int) ([]domain.RotaReminderRequest, error) {
	rows, err := r.db.Query(ctx, `SELECT `+rotaReminderRequestColumns+`
		FROM team_schedule_reminder_request WHERE family = $1 ORDER BY requested_on DESC LIMIT $2`, family, limit)
	if err != nil {
		return nil, fmt.Errorf("list rota reminder requests: %w", err)
	}
	defer rows.Close()
	out := []domain.RotaReminderRequest{}
	for rows.Next() {
		req, err := scanRotaReminderRequest(rows)
		if err != nil {
			return nil, fmt.Errorf("list rota reminder requests: %w", err)
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (r *rotaReminderRequestRepository) Claim(ctx context.Context, actor, family string, staleAfter time.Duration) (*domain.RotaReminderRequest, error) {
	var out *domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		req, err := scanRotaReminderRequest(tx.QueryRow(ctx, `
			UPDATE team_schedule_reminder_request
			   SET status = 'SENDING', claimed_on = now(), updated_on = now(), updated_by = $3
			 WHERE id = (SELECT id FROM team_schedule_reminder_request
			              WHERE family = $1
			                AND (status = 'QUEUED' OR (status = 'SENDING' AND claimed_on < now() - make_interval(secs => $2)))
			              ORDER BY requested_on
			              LIMIT 1
			              FOR UPDATE SKIP LOCKED)
			RETURNING `+rotaReminderRequestColumns, family, staleAfter.Seconds(), actor))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = &req
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim a rota reminder request: %w", err)
	}
	return out, nil
}

func (r *rotaReminderRequestRepository) Report(ctx context.Context, actor, family, id, status, result string) (domain.RotaReminderRequest, error) {
	var out domain.RotaReminderRequest
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		var err error
		out, err = scanRotaReminderRequest(tx.QueryRow(ctx, `
			UPDATE team_schedule_reminder_request
			   SET status = $3, result = NULLIF($4, ''), handled_on = now(), updated_on = now(), updated_by = $5
			 WHERE id = $1::uuid AND family = $2 AND status = 'SENDING'
			RETURNING `+rotaReminderRequestColumns, id, family, status, result, actor))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RotaReminderRequest{}, &apierror.NotFoundError{Msg: "no rota reminder request is being sent with that id"}
	}
	if err != nil {
		return domain.RotaReminderRequest{}, fmt.Errorf("report a rota reminder request: %w", err)
	}
	return out, nil
}
