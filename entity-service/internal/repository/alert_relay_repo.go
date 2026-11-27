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
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AlertRelayRepository is the alert relay's view of the alert table as the outbox of alert.created.
type AlertRelayRepository interface {
	// MarkAlertsToPublish takes up to limit waiting, unpublished, due alerts, priority tier first and oldest first,
	// and stamps published_on, committing at once, so no lock is held while they are published. It takes fewer, or
	// none, so that at most maxInFlight alerts are published and still undecided (the backpressure).
	MarkAlertsToPublish(ctx context.Context, maxInFlight, limit int) ([]domain.AlertToPublish, error)
	// UnmarkPublished clears published_on on alerts the relay took but could not publish, so they are taken again.
	UnmarkPublished(ctx context.Context, ids []string) error
	// RepublishStale clears published_on on alerts published longer than olderThan ago that are still undecided and
	// not claimed (a lost event, or a processor that died), so the relay publishes them again; it returns how many.
	RepublishStale(ctx context.Context, olderThan time.Duration) (int64, error)
}

// markAlertsToPublishQuery counts the alerts in flight (alert_in_flight_idx) and takes at most the room left, from
// alert_unpublished_idx (migration 0311). SKIP LOCKED lets every replica's relay run at once; the count is not locked,
// so two relays together can go a batch over the limit, which only weakens the backpressure for a moment.
const markAlertsToPublishQuery = `WITH in_flight AS (
    SELECT count(*)::int AS n FROM alert
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NOT NULL
), picked AS (
    SELECT id FROM alert
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NULL
      AND (next_attempt_on IS NULL OR next_attempt_on <= now())
    ORDER BY tier DESC, id
    LIMIT LEAST($2::int, GREATEST($1::int - (SELECT n FROM in_flight), 0))
    FOR UPDATE SKIP LOCKED
)
UPDATE alert a SET published_on = now()
FROM picked WHERE a.id = picked.id
RETURNING a.id, a.fingerprint, a.tier, a.received_on`

const unmarkPublishedQuery = `UPDATE alert SET published_on = NULL
WHERE id = ANY($1::text[]) AND processed_on IS NULL`

const republishStaleQuery = `UPDATE alert SET published_on = NULL
WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NOT NULL
  AND published_on < now() - make_interval(secs => $1::double precision)
  AND (claim_expires_on IS NULL OR claim_expires_on < now())`

type alertRelayRepository struct {
	db *Scoped
}

// NewAlertRelayRepository returns an AlertRelayRepository; callers run it under the system identity because alert is
// internal only.
func NewAlertRelayRepository(db *Scoped) AlertRelayRepository {
	return &alertRelayRepository{db: db}
}

func (r *alertRelayRepository) MarkAlertsToPublish(ctx context.Context, maxInFlight, limit int) ([]domain.AlertToPublish, error) {
	if limit <= 0 || maxInFlight <= 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, markAlertsToPublishQuery, maxInFlight, limit)
	if err != nil {
		return nil, fmt.Errorf("mark alerts to publish: %w", err)
	}
	marked, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.AlertToPublish, error) {
		var a domain.AlertToPublish
		var fingerprint *string
		var tier string
		if err := row.Scan(&a.ID, &fingerprint, &tier, &a.ReceivedOn); err != nil {
			return a, err
		}
		if fingerprint != nil {
			a.Fingerprint = *fingerprint
		}
		a.Tier = domain.AlertTier(tier)
		return a, nil
	})
	if err != nil {
		return nil, fmt.Errorf("mark alerts to publish: %w", err)
	}
	// RETURNING has no order; publish the priority tier first, then oldest first, as the query chose them.
	slices.SortFunc(marked, func(a, b domain.AlertToPublish) int {
		if a.Tier != b.Tier {
			return strings.Compare(string(b.Tier), string(a.Tier))
		}
		return strings.Compare(a.ID, b.ID)
	})
	return marked, nil
}

func (r *alertRelayRepository) UnmarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, unmarkPublishedQuery, ids); err != nil {
		return fmt.Errorf("unmark published alerts: %w", err)
	}
	return nil
}

func (r *alertRelayRepository) RepublishStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx, republishStaleQuery, olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("republish stale alerts: %w", err)
	}
	return tag.RowsAffected(), nil
}
