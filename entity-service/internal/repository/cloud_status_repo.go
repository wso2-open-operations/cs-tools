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
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CloudStatusCandidate is one in-scope outage as the sweep sees it, together
// with the transition it currently implies.
type CloudStatusCandidate struct {
	OutageID string
	Number   string
	// Cloud is empty when the outage's configuration item has no cloud
	// monitor. The sweep counts and skips those rather than guessing a route.
	Cloud string
	Event domain.CloudStatusEvent
	// Timestamp is the begin or the end instant, matching Event.
	Timestamp string
	// Type is the outage's own type, used to decide what its affected
	// monitors should show while it is ongoing. Empty is possible and is
	// handled, not assumed away.
	Type string
}

// CloudStatusRepository reads which outages owe the status dashboard a
// webhook, and records what was sent.
type CloudStatusRepository interface {
	Candidates(ctx context.Context, parentServiceIDs []string) ([]CloudStatusCandidate, error)
	CandidatesByOutage(ctx context.Context, parentServiceIDs, outageIDs []string) ([]CloudStatusCandidate, error)
	ClaimChanges(ctx context.Context, entityTypes []string, limit int) ([]OutboxChange, error)
	Record(ctx context.Context, c CloudStatusCandidate) (bool, error)
	AffectedMonitors(ctx context.Context, outageID string) ([]string, error)
	AffectedClouds(ctx context.Context, outageID string, parentServiceIDs []string) ([]string, error)
	SetMonitorStatus(ctx context.Context, monitorIDs []string, status domain.CloudMonitorStatus) (int64, error)
	Pending(ctx context.Context, limit, maxAttempts int) ([]domain.PendingCloudStatusWebhook, error)
	RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error
}

type cloudStatusRepository struct {
	db *pgxpool.Pool
}

// NewCloudStatusRepository constructs the cloud status webhook store.
func NewCloudStatusRepository(db *pgxpool.Pool) CloudStatusRepository {
	return &cloudStatusRepository{db: db}
}

// candidatesSQL finds every outage in the flow's scope and states the single
// transition it currently implies.
//
// THE SCOPE IS THE UNION OF TWO TRIGGERS, because two flows share this work.
//
//	Cloud Status Event Notification Flow               on cmdb_ci_outage
//	Cloud Status Event Notification Flow - Affected CI  on cmdb_outage_ci_mtom
//
// They apply the SAME 14-service condition to DIFFERENT records: the first to
// the outage's own configuration item, the second to each affected CI. So an
// outage whose own CI is out of scope still qualifies if any affected CI is in
// scope -- the second flow fires for it and the first never does.
//
// Filtering only on the outage's own offering, as this did at first, silently
// dropped exactly those outages. The LEFT JOIN matters for the same reason: an
// outage reachable only through its affected CIs may have no usable offering
// of its own, and an inner join would discard it before the EXISTS ran.
//
// THE FILTER ITSELF reproduces the ServiceNow trigger exactly, and the
// exactness matters. The trigger reads
//
//	Configuration Item -> Parent [Service Offering] -> Sys ID  is one of 14
//
// i.e. it dot-walks the outage's CI *as a service offering* and compares that
// offering's PARENT. An outage whose CI is a business service directly can
// never match, because the dot-walk yields nothing. So this joins through
// service_offering and filters on parent_id -- deliberately NOT on
// outage.service_id, which would widen the scope past what ServiceNow fires
// for and start posting webhooks nobody has ever seen.
//
// THE TRANSITION RULE mirrors the flow's conditional check, including its one
// non-obvious consequence:
//
//	begin set, end null  ->  OUTAGE_BEGIN
//	begin set, end set   ->  OUTAGE_END
//	begin null           ->  neither arm; the flow does nothing
//
// An outage that is created already-ended therefore yields ONLY the end event.
// That is not an oversight to tidy up: ServiceNow evaluates the record as it
// stands, so it too would send only the end, and a port that helpfully
// back-filled a begin would flip the public dashboard to degraded for an
// outage that was over before anyone heard of it.
//
// The ordinary case still produces both, in order, because the row is seen
// twice: once while ongoing and again once ended.
// THE TIMESTAMP FORMAT IS NOT A STYLE CHOICE. ServiceNow's body script passes
// `new Date()` through JSON.stringify, which emits ISO-8601 UTC with exactly
// three decimal places -- "2026-09-28T07:49:34.123Z". The dashboard has been
// parsing that shape for as long as the flow has existed, so the port emits
// it to the millisecond rather than to the second. A receiver with a strict
// parser would reject the shorter form, and a webhook rejected for its format
// fails exactly as silently as one with a wrong event name.
// candidatesSQLSelect is the projection both the sweep and the
// record-triggered path use. Shared as a constant so the two cannot drift.
const candidatesSQLSelect = `
    SELECT o.id::text,
           COALESCE(o.number, ''),
           COALESCE(cm.cloud_offering::text, ''),
           CASE WHEN o.end_on IS NULL THEN 'OUTAGE_BEGIN' ELSE 'OUTAGE_END' END,
           to_char(
               CASE WHEN o.end_on IS NULL THEN o.start_on ELSE o.end_on END
               AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'
           ),
           COALESCE(o.type::text, '')
`

const candidatesSQL = candidatesSQLSelect + `
      FROM outage o
      LEFT JOIN service_offering so ON so.id = o.service_offering_id
      -- *** ONE MONITOR PER OUTAGE, DETERMINISTICALLY. *** A plain join on
      -- service_offering_id returns a row per monitor, so an offering with
      -- two monitors made the sweep count the same outage twice: Scanned,
      -- SkippedNoCloud and UnknownOutageType all inflated, and
      -- AffectedMonitors ran more than once for it. The unique key on
      -- cloud_status_events protected the recorded events, so only the
      -- metrics lied -- which is the kind of wrong that goes unnoticed.
      --
      -- LIMIT 1 also matches the source: ServiceNow's monitor lookup is
      -- configured "Return only the first record".
      LEFT JOIN LATERAL (
          SELECT cmx.cloud_offering
            FROM cloud_monitor cmx
           WHERE cmx.service_offering_id = o.service_offering_id
           ORDER BY cmx.id
           LIMIT 1
      ) cm ON TRUE
     WHERE o.start_on IS NOT NULL
       AND (
             so.parent_id = ANY($1::uuid[])
          OR EXISTS (
                 SELECT 1
                   FROM outage_affected_ci ac
                   JOIN service_offering aso ON aso.id = ac.ci_id
                  WHERE ac.outage_id = o.id
                    AND aso.parent_id = ANY($1::uuid[])
             )
           )
     ORDER BY COALESCE(o.end_on, o.start_on)
`

// Candidates returns the transition currently implied by every in-scope
// outage. Callers pass the 14 configured parent service ids.
func (r *cloudStatusRepository) Candidates(ctx context.Context, parentServiceIDs []string) ([]CloudStatusCandidate, error) {
	rows, err := r.db.Query(ctx, candidatesSQL, parentServiceIDs)
	if err != nil {
		return nil, fmt.Errorf("query cloud status candidates: %w", err)
	}
	defer rows.Close()

	var out []CloudStatusCandidate
	for rows.Next() {
		var c CloudStatusCandidate
		if err := rows.Scan(&c.OutageID, &c.Number, &c.Cloud, &c.Event, &c.Timestamp, &c.Type); err != nil {
			return nil, fmt.Errorf("scan cloud status candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud status candidates: %w", err)
	}
	return out, nil
}

// Record claims one transition, returning whether this call is the one that
// claimed it.
//
// DO NOTHING rather than DO UPDATE: the unique key is the whole idempotency
// guarantee. ServiceNow re-ran this flow on every qualifying update to the
// outage and re-posted the same event each time -- an outage edited five times
// while ongoing produced five identical begin webhooks. Doing nothing on
// conflict is a deliberate divergence, and the better behaviour: the dashboard
// is being told a fact, and repeating it carries no information.
const recordSQL = `
    INSERT INTO cloud_status_events (outage_id, event, cloud)
    VALUES ($1::uuid, $2::cloud_status_event_enum, $3)
    ON CONFLICT (outage_id, event, cloud) DO NOTHING
    RETURNING id
`

// Record inserts the transition if it is new. The bool reports whether a row
// was created.
func (r *cloudStatusRepository) Record(ctx context.Context, c CloudStatusCandidate) (bool, error) {
	var id string
	err := r.db.QueryRow(ctx, recordSQL, c.OutageID, string(c.Event), c.Cloud).Scan(&id)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("record cloud status event: %w", err)
	}
	return true, nil
}

// pendingSQL reads transitions still owed a successful delivery.
//
// The outage is re-joined for its number and its instants rather than copying
// them onto the event row. The event row records the DECISION; the outage
// remains the source of truth for the facts, and a corrected begin time should
// be reported correctly by a webhook that has not gone out yet.
const pendingSQL = `
    SELECT e.id::text,
           e.outage_id::text,
           COALESCE(o.number, ''),
           e.event::text,
           e.cloud,
           to_char(
               CASE WHEN e.event = 'OUTAGE_BEGIN' THEN o.start_on ELSE o.end_on END
               AT TIME ZONE 'UTC',
               'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'
           ),
           e.attempt_count,
           COALESCE(e.last_error, '')
      FROM cloud_status_events e
      JOIN outage o ON o.id = e.outage_id
     WHERE e.delivered IS FALSE
       AND e.attempt_count < $2
     ORDER BY e.created_on
     LIMIT $1
`

// Pending returns up to limit undelivered webhooks that have not yet exhausted
// maxAttempts.
func (r *cloudStatusRepository) Pending(ctx context.Context, limit, maxAttempts int) ([]domain.PendingCloudStatusWebhook, error) {
	rows, err := r.db.Query(ctx, pendingSQL, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("query pending cloud status webhooks: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PendingCloudStatusWebhook, 0)
	for rows.Next() {
		var w domain.PendingCloudStatusWebhook
		if err := rows.Scan(&w.ID, &w.OutageID, &w.Number, &w.Event, &w.Cloud,
			&w.Timestamp, &w.AttemptCount, &w.LastError); err != nil {
			return nil, fmt.Errorf("scan pending cloud status webhook: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending cloud status webhooks: %w", err)
	}
	return out, nil
}

// recordDeliverySQL stamps the outcome of one attempt.
//
// attempt_count increments on both outcomes, success included. It counts what
// was tried, not what failed; a webhook that succeeded on its third attempt
// should still read as having taken three.
const recordDeliverySQL = `
    UPDATE cloud_status_events
       SET delivered       = $2,
           attempt_count   = attempt_count + 1,
           last_error      = CASE WHEN $2 THEN NULL ELSE NULLIF($3, '') END,
           last_attempt_on = NOW(),
           delivered_on    = CASE WHEN $2 THEN NOW() ELSE delivered_on END,
           updated_on      = NOW()
     WHERE id = $1::uuid
`

// RecordDelivery stamps the outcome of one webhook attempt.
func (r *cloudStatusRepository) RecordDelivery(ctx context.Context, id string, delivered bool, errMsg string) error {
	tag, err := r.db.Exec(ctx, recordDeliverySQL, id, delivered, errMsg)
	if err != nil {
		return fmt.Errorf("record cloud status delivery: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// affectedMonitorsSQL resolves one outage's affected configuration items to
// the cloud monitors that represent them.
//
// This is steps 3+5 (and 10+12) of the flow, collapsed into one query. The
// flow looked up the join rows, then looked up a monitor per row; a join does
// the same work without the round trips, and without the flow's "first record
// only" behaviour on the monitor lookup.
//
// THE JOIN TO service_offering IS WHY ci_id IS NOT A FOREIGN KEY upstream. The
// source column references cmdb_ci, the base class, so a row may point at a
// service, an application, or anything else that is not a service offering.
// Those rows simply do not join here and are skipped -- which is correct: a
// cloud monitor hangs off a service offering, so a CI that is not one has no
// monitor to update.
//
// The trigger outage's OWN configuration item is deliberately not unioned in.
// The flow kept them separate -- the affected-CI list drives the status
// writes, the outage's own CI drives the webhook's routing -- and merging them
// would silently widen which monitors get rewritten.
const affectedMonitorsSQL = `
    SELECT DISTINCT cm.id::text
      FROM outage_affected_ci ac
      JOIN service_offering so ON so.id = ac.ci_id
      JOIN cloud_monitor cm ON cm.service_offering_id = so.id
     WHERE ac.outage_id = $1::uuid
       AND ac.ci_id IS NOT NULL
`

// AffectedMonitors returns the cloud monitors for every affected CI of one
// outage that resolves to a service offering.
func (r *cloudStatusRepository) AffectedMonitors(ctx context.Context, outageID string) ([]string, error) {
	rows, err := r.db.Query(ctx, affectedMonitorsSQL, outageID)
	if err != nil {
		return nil, fmt.Errorf("query affected cloud monitors: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan affected cloud monitor: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate affected cloud monitors: %w", err)
	}
	return out, nil
}

// setMonitorStatusSQL writes the status for a set of monitors at once.
//
// *** THIS WRITES A SYNC-MIRRORED COLUMN. *** cloud_monitor is populated by
// csm-sync-service from the backing system's cloud monitor records, so in any
// environment where that sync runs against a live ServiceNow, the next run
// overwrites whatever this writes. That is understood and accepted for dev,
// where the sync is not competing. It is NOT settled for production: either
// `status` comes out of the sync mapping so Go owns the column outright, or
// the port keeps its own table and the column is switched at cutover. Do not
// promote this beyond dev until that is decided.
//
// The WHERE clause skips monitors already showing the target status so a
// repeated sweep does not churn updated_on on every tick -- which matters
// because updated_on is a sync-visible column and pointless writes to it make
// the sync's own change detection noisier.
const setMonitorStatusSQL = `
    UPDATE cloud_monitor
       SET status = $2::cloud_monitor_status_enum,
           updated_on = NOW()
     WHERE id = ANY($1::uuid[])
       AND (status IS DISTINCT FROM $2::cloud_monitor_status_enum)
`

// SetMonitorStatus writes status to every listed monitor that is not already
// showing it, returning how many rows actually changed.
func (r *cloudStatusRepository) SetMonitorStatus(ctx context.Context, monitorIDs []string, status domain.CloudMonitorStatus) (int64, error) {
	if len(monitorIDs) == 0 {
		return 0, nil
	}
	tag, err := r.db.Exec(ctx, setMonitorStatusSQL, monitorIDs, string(status))
	if err != nil {
		return 0, fmt.Errorf("set cloud monitor status: %w", err)
	}
	return tag.RowsAffected(), nil
}

// affectedCloudsSQL lists the distinct clouds an outage's affected CIs sit on.
//
// This is what the sibling flow `Cloud Status Event Notification Flow -
// Affected CI` exists to serve. It triggers on the affected-CI join table and
// routes its webhook by the affected CI's OWN cloud, not the outage's -- so an
// outage spanning two clouds produces a refresh on both dashboards, because
// they are separate deployments showing separate components.
//
// *** THE PARENT FILTER IS THE SIBLING FLOW'S OWN TRIGGER CONDITION, AND IT
// DOES NOT MATCH AffectedMonitors. *** That flow triggers on
//
//	Configuration Item -> Parent [Service Offering] -> Sys ID  is one of 14
//
// applied to the AFFECTED CI, so an affected CI hanging off some other service
// produces no webhook at all. The status writes have no such filter -- the
// outage-triggered flow updates every affected CI's monitor regardless of its
// parent, because by then the trigger has already qualified on the outage.
//
// So the two reads deliberately disagree about which affected CIs count, and
// that asymmetry is ServiceNow's, not an oversight here: without the filter
// this would post webhooks ServiceNow never sends.
//
// Returned separately from AffectedMonitors rather than derived from it: the
// monitors drive the status writes and are needed individually, the clouds
// drive the webhooks and are only needed as a set.
const affectedCloudsSQL = `
    SELECT DISTINCT cm.cloud_offering::text
      FROM outage_affected_ci ac
      JOIN service_offering so ON so.id = ac.ci_id
      JOIN cloud_monitor cm ON cm.service_offering_id = so.id
     WHERE ac.outage_id = $1::uuid
       AND ac.ci_id IS NOT NULL
       AND cm.cloud_offering IS NOT NULL
       AND so.parent_id = ANY($2::uuid[])
`

// AffectedClouds returns the distinct cloud offerings behind one outage's
// affected configuration items.
func (r *cloudStatusRepository) AffectedClouds(ctx context.Context, outageID string, parentServiceIDs []string) ([]string, error) {
	rows, err := r.db.Query(ctx, affectedCloudsSQL, outageID, parentServiceIDs)
	if err != nil {
		return nil, fmt.Errorf("query affected clouds: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan affected cloud: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate affected clouds: %w", err)
	}
	return out, nil
}

// candidatesByOutageSQL is candidatesSQL narrowed to named outages.
//
// The record-triggered path needs the same decision as the sweep for one
// outage rather than all of them, and it must be the SAME decision -- so the
// scope filter, the event derivation and the timestamp format are shared
// verbatim with candidatesSQL above rather than restated. If the two ever
// disagree, a change noticed by the trigger is handled differently from the
// same change noticed by reconciliation, which is the worst kind of bug to
// look for.
const candidatesByOutageSQL = candidatesSQLSelect + `
      FROM outage o
      LEFT JOIN service_offering so ON so.id = o.service_offering_id
      -- *** ONE MONITOR PER OUTAGE, DETERMINISTICALLY. *** A plain join on
      -- service_offering_id returns a row per monitor, so an offering with
      -- two monitors made the sweep count the same outage twice: Scanned,
      -- SkippedNoCloud and UnknownOutageType all inflated, and
      -- AffectedMonitors ran more than once for it. The unique key on
      -- cloud_status_events protected the recorded events, so only the
      -- metrics lied -- which is the kind of wrong that goes unnoticed.
      --
      -- LIMIT 1 also matches the source: ServiceNow's monitor lookup is
      -- configured "Return only the first record".
      LEFT JOIN LATERAL (
          SELECT cmx.cloud_offering
            FROM cloud_monitor cmx
           WHERE cmx.service_offering_id = o.service_offering_id
           ORDER BY cmx.id
           LIMIT 1
      ) cm ON TRUE
     WHERE o.id = ANY($2::uuid[])
       AND o.start_on IS NOT NULL
       AND (
             so.parent_id = ANY($1::uuid[])
          OR EXISTS (
                 SELECT 1
                   FROM outage_affected_ci ac
                   JOIN service_offering aso ON aso.id = ac.ci_id
                  WHERE ac.outage_id = o.id
                    AND aso.parent_id = ANY($1::uuid[])
             )
           )
`

// CandidatesByOutage returns the current transition for each named outage
// that is still in scope. An outage that has fallen out of scope simply does
// not come back, which is correct: there is nothing to tell anyone about it.
func (r *cloudStatusRepository) CandidatesByOutage(ctx context.Context, parentServiceIDs, outageIDs []string) ([]CloudStatusCandidate, error) {
	if len(outageIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, candidatesByOutageSQL, parentServiceIDs, outageIDs)
	if err != nil {
		return nil, fmt.Errorf("query cloud status candidates by outage: %w", err)
	}
	defer rows.Close()

	var out []CloudStatusCandidate
	for rows.Next() {
		var c CloudStatusCandidate
		if err := rows.Scan(&c.OutageID, &c.Number, &c.Cloud, &c.Event, &c.Timestamp, &c.Type); err != nil {
			return nil, fmt.Errorf("scan cloud status candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// claimChangesSQL takes a batch of unpublished outbox rows and marks them
// published in the same statement. Identical in shape to the change-request
// drainer's, which is deliberate: event_outbox was built to be shared, and
// "the drainer filters by type" is the contract.
//
// FOR UPDATE SKIP LOCKED is what makes more than one replica safe -- two
// drainers racing for the same batch get disjoint sets rather than blocking
// or double-delivering.
//
// Claiming at read time rather than after the work means a crash mid-batch
// loses those notifications. For cloud status that is recoverable in a way it
// is not for the CR notices: the reconciliation sweep re-derives the same
// transitions from current state and records anything missed. The trigger is
// the fast path; the sweep is the guarantee.
const claimChangesSQL = `
    WITH claimed AS (
        SELECT id FROM event_outbox
        WHERE published_on IS NULL
          AND entity_type = ANY($1::text[])
        ORDER BY id
        LIMIT $2
        FOR UPDATE SKIP LOCKED
    )
    UPDATE event_outbox o
       SET published_on = NOW()
      FROM claimed c
     WHERE o.id = c.id
    RETURNING o.id, o.entity_type, o.entity_id, o.changes, o.snapshot
`

// ClaimChanges takes up to limit unpublished outbox rows for the given entity
// types, oldest first, and marks them published.
func (r *cloudStatusRepository) ClaimChanges(ctx context.Context, entityTypes []string, limit int) ([]OutboxChange, error) {
	rows, err := r.db.Query(ctx, claimChangesSQL, entityTypes, limit)
	if err != nil {
		return nil, fmt.Errorf("claim cloud status outbox rows: %w", err)
	}
	defer rows.Close()

	var out []OutboxChange
	for rows.Next() {
		var c OutboxChange
		var changes, snapshot []byte
		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &changes, &snapshot); err != nil {
			return nil, fmt.Errorf("scan cloud status outbox row: %w", err)
		}
		if err := json.Unmarshal(changes, &c.Changes); err != nil {
			return nil, fmt.Errorf("decode outbox changes: %w", err)
		}
		if err := json.Unmarshal(snapshot, &c.Snapshot); err != nil {
			return nil, fmt.Errorf("decode outbox snapshot: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
