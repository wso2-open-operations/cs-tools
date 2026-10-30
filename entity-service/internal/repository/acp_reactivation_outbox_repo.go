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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ACPReactivationOutbox is the queue of "Back to Open state" notices: the
// ACP reopen writes one into event_outbox in the same transaction as the
// reopen (ACPReopenRepository.Reopen), and ProjectReactivationDrainer
// publishes it, retrying until Event Hub takes it. The rows are the
// event_outbox rows of entity_type ACPReactivationOutboxEntityType, with
// the notice as the snapshot; attempts / last_error / last_attempt_on
// (migration 0181) carry the retries, as for the incident report flows.
type ACPReactivationOutbox interface {
	// Pending returns up to limit unpublished notices whose backoff has
	// passed, oldest first.
	Pending(ctx context.Context, limit int) ([]int64, error)
	// Process locks one notice and hands it to publish; the notice is
	// marked published, in the same transaction, only when publish
	// succeeds. processed is false when the row was taken or finished
	// meanwhile.
	Process(ctx context.Context, outboxID int64, publish func(ctx context.Context, projectID string, payload json.RawMessage) error) (processed bool, err error)
	// RecordFailure counts a failed attempt, and parks the notice (marks it
	// published, with the error kept) once maxAttempts is reached.
	RecordFailure(ctx context.Context, outboxID int64, cause string, maxAttempts int) (parked bool, err error)
}

type acpReactivationOutbox struct {
	db *Scoped
}

// NewACPReactivationOutbox constructs the notice queue over the scoped pool.
func NewACPReactivationOutbox(db *Scoped) ACPReactivationOutbox {
	return &acpReactivationOutbox{db: db}
}

// Pending implements ACPReactivationOutbox. The backoff is the incident
// report flows': 30s, doubling, at most an hour.
func (r *acpReactivationOutbox) Pending(ctx context.Context, limit int) ([]int64, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT id FROM event_outbox
		WHERE published_on IS NULL AND entity_type = $1
		  AND (last_attempt_on IS NULL
		       OR last_attempt_on + LEAST(INTERVAL '1 hour',
		                                  INTERVAL '30 seconds' * POWER(2, GREATEST(attempts - 1, 0)))
		          <= NOW())
		ORDER BY id
		LIMIT $2`, ACPReactivationOutboxEntityType, limit)
	if err != nil {
		return nil, fmt.Errorf("acp reactivation outbox: list pending: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("acp reactivation outbox: scan pending: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Process implements ACPReactivationOutbox.
func (r *acpReactivationOutbox) Process(ctx context.Context, outboxID int64, publish func(ctx context.Context, projectID string, payload json.RawMessage) error) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	processed := false
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var projectID string
		var payload []byte
		// SKIP LOCKED: a row another replica is publishing is that replica's.
		err := tx.QueryRow(ctx, `
			SELECT entity_id::text, snapshot FROM event_outbox
			WHERE id = $1 AND entity_type = $2 AND published_on IS NULL
			FOR UPDATE SKIP LOCKED`, outboxID, ACPReactivationOutboxEntityType).Scan(&projectID, &payload)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("acp reactivation outbox: lock %d: %w", outboxID, err)
		}
		if err := publish(ctx, projectID, json.RawMessage(payload)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE event_outbox SET published_on = NOW() WHERE id = $1`, outboxID); err != nil {
			return fmt.Errorf("acp reactivation outbox: mark %d published: %w", outboxID, err)
		}
		processed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return processed, nil
}

// RecordFailure implements ACPReactivationOutbox.
func (r *acpReactivationOutbox) RecordFailure(ctx context.Context, outboxID int64, cause string, maxAttempts int) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var parked bool
	err := r.db.QueryRow(ctx, `
		UPDATE event_outbox
		SET attempts = attempts + 1,
		    last_error = $2,
		    last_attempt_on = NOW(),
		    published_on = CASE WHEN attempts + 1 >= $3 THEN NOW() ELSE published_on END
		WHERE id = $1 AND published_on IS NULL
		RETURNING published_on IS NOT NULL`, outboxID, cause, maxAttempts).Scan(&parked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("acp reactivation outbox: record failure for %d: %w", outboxID, err)
	}
	return parked, nil
}
