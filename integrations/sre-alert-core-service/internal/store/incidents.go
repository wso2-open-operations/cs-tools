// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"alert-core-service/internal/model"
	"alert-core-service/internal/pglock"
)

// IncidentRepo owns the incidents_processed and incident_notes tables.
type IncidentRepo struct {
	pool   *pgxpool.Pool
	locker *pglock.Locker
}

// NewIncidentRepo wraps pool for incident reads and writes; locker gives cross-replica delivery exclusivity.
func NewIncidentRepo(pool *pgxpool.Pool, locker *pglock.Locker) *IncidentRepo {
	return &IncidentRepo{pool: pool, locker: locker}
}

const incidentColumns = `id, fingerprint, incident_id, incident_number, status, severity, impact, urgency, service,
	metric_name, category, environment, source, alert_count, first_seen, last_seen, state_checked_at, fallback,
	csm_confirmed, csm_attempts, csm_permanently_failed, csm_last_attempt_at, fold_version, created_at,
	assignment_group, source_topic, source_account, unique_identifier, severity_label,
	muted, muted_at, unmuted_at, mute_rule_id, mute_rule_number`

// FoldPlan is what folding a fingerprint's alerts writes: notes and counters on the current incident, and any new incidents.
type FoldPlan struct {
	Current *CurrentFold
	New     []NewIncident
}

// CurrentFold adds alerts to an existing incident.
type CurrentFold struct {
	ID       int64
	Added    int
	LastSeen time.Time
	// Category fills the incident's category only if it is still empty.
	Category string
	// SeverityLabel, when set, replaces the incident's severity_label, and Severity, Impact and Urgency its severity,
	// impact and urgency: a later alert of the same fingerprint was more severe, so alert mute rules match the
	// incident's worst severity, not its first, and an incident not yet in CSM is created with that priority.
	SeverityLabel string
	Severity      int
	Impact        string
	Urgency       string
	Notes         []model.Note
}

// NewIncident starts a new incident with its notes, the first being its creation note.
type NewIncident struct {
	Incident model.Incident
	Notes    []model.Note
}

// Decide turns the fingerprint's current incident (nil if none) and the alert ids already recorded into a FoldPlan.
type Decide func(current *model.Incident, recorded map[string]bool) FoldPlan

const (
	selectCurrentQuery = `SELECT ` + incidentColumns + ` FROM incidents_processed WHERE fingerprint = $1 ORDER BY first_seen DESC LIMIT 1`
	updateCurrentQuery = `UPDATE incidents_processed SET alert_count = alert_count + $2, last_seen = greatest(last_seen, $3),
	category = CASE WHEN category = '' THEN $4 ELSE category END,
	severity_label = CASE WHEN $5 <> '' THEN $5 ELSE severity_label END,
	severity = CASE WHEN $5 <> '' THEN $6 ELSE severity END,
	impact = CASE WHEN $5 <> '' THEN $7 ELSE impact END,
	urgency = CASE WHEN $5 <> '' THEN $8 ELSE urgency END,
	fold_version = fold_version + 1, delivery_due_at = least(coalesce(delivery_due_at, now()), now())
	WHERE id = $1`
	insertNotesQuery = `INSERT INTO incident_notes (incident, alert_id, kind, note, chat_pending)
	SELECT $1::bigint, * FROM unnest($2::text[], $3::text[], $4::text[], $5::bool[])
	ON CONFLICT (alert_id) DO NOTHING`
	insertIncidentQuery = `WITH inc AS (
	INSERT INTO incidents_processed (fingerprint, incident_number, status, severity, impact, urgency, service, metric_name,
		category, environment, source, alert_count, first_seen, last_seen, delivery_due_at,
		assignment_group, source_topic, source_account, unique_identifier, severity_label)
	VALUES ($1, $2, 'new', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now(), $18, $19, $20, $21, $22)
	RETURNING id)
	INSERT INTO incident_notes (incident, alert_id, kind, note, chat_pending)
	SELECT inc.id, n.* FROM inc, unnest($14::text[], $15::text[], $16::text[], $17::bool[]) AS n
	ON CONFLICT (alert_id) DO NOTHING`
)

// Fold locks fp for this transaction across every replica, reads its current incident and which alertIDs are already recorded, applies decide's plan, and commits; two round trips per call.
func (r *IncidentRepo) Fold(ctx context.Context, fp string, alertIDs []string, decide Decide) (plan FoldPlan, err error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return FoldPlan{}, fmt.Errorf("fold %s: acquire: %w", fp, err)
	}
	defer conn.Release()
	defer func() {
		if err != nil {
			rollback(ctx, conn)
		}
	}()

	reads := &pgx.Batch{}
	reads.Queue(`BEGIN`)
	reads.Queue(`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "fold:"+fp)
	reads.Queue(selectCurrentQuery, fp)
	reads.Queue(`SELECT alert_id FROM incident_notes WHERE alert_id = ANY($1)`, alertIDs)
	br := conn.SendBatch(ctx, reads)
	current, recorded, err := readFoldState(br)
	if closeErr := br.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return FoldPlan{}, fmt.Errorf("fold %s: read: %w", fp, err)
	}

	plan = decide(current, recorded)
	writes := &pgx.Batch{}
	if c := plan.Current; c != nil {
		writes.Queue(updateCurrentQuery, c.ID, c.Added, c.LastSeen, c.Category, c.SeverityLabel, c.Severity, c.Impact, c.Urgency)
		if len(c.Notes) > 0 {
			writes.Queue(insertNotesQuery, append([]any{c.ID}, noteArrays(c.Notes)...)...)
		}
	}
	for _, n := range plan.New {
		inc := n.Incident
		args := []any{inc.Fingerprint, inc.IncidentNumber, inc.Severity, inc.Impact, inc.Urgency, inc.Service,
			inc.MetricName, inc.Category, inc.Environment, inc.Source, inc.AlertCount, inc.FirstSeen, inc.LastSeen}
		args = append(args, noteArrays(n.Notes)...)
		args = append(args, inc.AssignmentGroup, inc.SourceTopic, inc.SourceAccount, inc.UniqueIdentifier, inc.SeverityLabel)
		writes.Queue(insertIncidentQuery, args...)
	}
	writes.Queue(`COMMIT`)
	br = conn.SendBatch(ctx, writes)
	for range writes.Len() {
		if _, err = br.Exec(); err != nil {
			break
		}
	}
	if closeErr := br.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return FoldPlan{}, fmt.Errorf("fold %s: write: %w", fp, err)
	}
	return plan, nil
}

func readFoldState(br pgx.BatchResults) (*model.Incident, map[string]bool, error) {
	if _, err := br.Exec(); err != nil {
		return nil, nil, err
	}
	if _, err := br.Exec(); err != nil {
		return nil, nil, err
	}
	rows, err := br.Query()
	if err != nil {
		return nil, nil, err
	}
	current, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Incident])
	if errors.Is(err, pgx.ErrNoRows) {
		current, err = nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err = br.Query()
	if err != nil {
		return nil, nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	recorded := make(map[string]bool, len(ids))
	for _, id := range ids {
		recorded[id] = true
	}
	return current, recorded, nil
}

func noteArrays(notes []model.Note) []any {
	ids := make([]string, len(notes))
	kinds := make([]string, len(notes))
	texts := make([]string, len(notes))
	chat := make([]bool, len(notes))
	for i, n := range notes {
		ids[i], kinds[i], texts[i], chat[i] = n.AlertID, n.Kind, n.Text, n.ChatPending
	}
	return []any{ids, kinds, texts, chat}
}

// rollback ends a failed transaction; pgxpool also destroys a connection released mid-transaction, so a failed rollback cannot leak one.
func rollback(ctx context.Context, conn *pgxpool.Conn) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, _ = conn.Exec(rctx, `ROLLBACK`)
}

// TryLock takes the delivery lock for incident id without waiting; ok=false means another worker or replica is delivering it.
func (r *IncidentRepo) TryLock(ctx context.Context, id int64) (unlock func(), ok bool, err error) {
	return r.locker.TryLock(ctx, fmt.Sprintf("deliver:%d", id))
}

// ListDue returns up to limit incidents whose delivery is due, longest-waiting first.
func (r *IncidentRepo) ListDue(ctx context.Context, limit int) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM incidents_processed WHERE delivery_due_at <= now() ORDER BY delivery_due_at LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list due incidents: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, fmt.Errorf("list due incidents: %w", err)
	}
	return ids, nil
}

// Get reads one incident, reporting absence as false.
func (r *IncidentRepo) Get(ctx context.Context, id int64) (model.Incident, bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+incidentColumns+` FROM incidents_processed WHERE id = $1`, id)
	if err != nil {
		return model.Incident{}, false, fmt.Errorf("read incident %d: %w", id, err)
	}
	inc, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Incident])
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Incident{}, false, nil
	}
	if err != nil {
		return model.Incident{}, false, fmt.Errorf("read incident %d: %w", id, err)
	}
	return inc, true, nil
}

// PendingNotes returns up to limit notes still owed to CSM or Chat, in the order they were folded.
func (r *IncidentRepo) PendingNotes(ctx context.Context, id int64, limit int) ([]model.Note, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, alert_id, kind, note, csm_pending, chat_pending FROM incident_notes
		WHERE incident = $1 AND (csm_pending OR chat_pending) ORDER BY id LIMIT $2`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("read pending notes for incident %d: %w", id, err)
	}
	notes, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Note])
	if err != nil {
		return nil, fmt.Errorf("read pending notes for incident %d: %w", id, err)
	}
	return notes, nil
}

func (r *IncidentRepo) exec(ctx context.Context, what string, id int64, sql string, args ...any) error {
	if _, err := r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s for incident %d: %w", what, id, err)
	}
	return nil
}

// RecordCSMAttemptStarted persists the attempt before calling CSM, so CSMAttempts is a lower bound on creates that may have reached it.
func (r *IncidentRepo) RecordCSMAttemptStarted(ctx context.Context, id int64, attempts int) error {
	return r.exec(ctx, "record csm attempt", id,
		`UPDATE incidents_processed SET csm_attempts = $2, csm_last_attempt_at = now() WHERE id = $1`, id, attempts)
}

// RecordCSMIncident writes CSM's id, number and confirmation together; a just-created incident is known open, so its status counts as checked now.
func (r *IncidentRepo) RecordCSMIncident(ctx context.Context, id int64, csmID, number string) error {
	return r.exec(ctx, "record csm incident", id, `UPDATE incidents_processed SET incident_id = $2, incident_number = $3,
		csm_confirmed = true, status = 'open', state_checked_at = now() WHERE id = $1`, id, csmID, number)
}

// RecordCSMAttemptFailure stores a failed create; permanent stops further CSM attempts.
func (r *IncidentRepo) RecordCSMAttemptFailure(ctx context.Context, id int64, permanent bool) error {
	return r.exec(ctx, "record csm failure", id,
		`UPDATE incidents_processed SET csm_permanently_failed = $2 WHERE id = $1`, id, permanent)
}

// MarkFallback records that the incident reached every Chat webhook.
func (r *IncidentRepo) MarkFallback(ctx context.Context, id int64) error {
	return r.exec(ctx, "mark fallback", id, `UPDATE incidents_processed SET fallback = true WHERE id = $1`, id)
}

// SyncStatus stores CSM's status and when it was checked.
func (r *IncidentRepo) SyncStatus(ctx context.Context, id int64, status string, checkedAt time.Time) error {
	return r.exec(ctx, "sync status", id,
		`UPDATE incidents_processed SET status = $2, state_checked_at = $3 WHERE id = $1`, id, status, checkedAt)
}

// ClearNotes marks notes as no longer owed to CSM (csm) and/or Chat (chat).
func (r *IncidentRepo) ClearNotes(ctx context.Context, incident int64, noteIDs []int64, csm, chat bool) error {
	return r.exec(ctx, "clear notes", incident, `UPDATE incident_notes
		SET csm_pending = csm_pending AND NOT $2, chat_pending = chat_pending AND NOT $3 WHERE id = ANY($1)`, noteIDs, csm, chat)
}

// SettleNotes clears every note of the incident still owed to CSM (csm) and/or Chat (chat).
func (r *IncidentRepo) SettleNotes(ctx context.Context, incident int64, csm, chat bool) error {
	return r.exec(ctx, "settle notes", incident, `UPDATE incident_notes
		SET csm_pending = csm_pending AND NOT $2, chat_pending = chat_pending AND NOT $3
		WHERE incident = $1 AND ((csm_pending AND $2) OR (chat_pending AND $3))`, incident, csm, chat)
}

// FinishDelivery schedules the next delivery at next (nil means nothing owed), unless a fold added notes since foldVersion was read, in which case it is due now.
func (r *IncidentRepo) FinishDelivery(ctx context.Context, id, foldVersion int64, next *time.Time) error {
	return r.exec(ctx, "finish delivery", id, `UPDATE incidents_processed
		SET delivery_due_at = CASE WHEN fold_version = $2 THEN $3::timestamptz ELSE least(coalesce(delivery_due_at, now()), now()) END
		WHERE id = $1`, id, foldVersion, next)
}

// Mute marks the incident muted by alert mute rule ruleID/ruleNumber, keeping the time it was first muted.
func (r *IncidentRepo) Mute(ctx context.Context, id int64, ruleID, ruleNumber string) error {
	return r.exec(ctx, "mute", id, `UPDATE incidents_processed
		SET muted = true, muted_at = coalesce(muted_at, now()), mute_rule_id = $2, mute_rule_number = $3 WHERE id = $1`,
		id, ruleID, ruleNumber)
}

// Unmute releases a muted incident for normal delivery. It takes the fingerprint's fold lock, so no Fold runs between
// its checks and its write, and it does nothing in two cases:
//   - a fold changed the incident since inc.FoldVersion (an OK that just arrived must be seen before the incident is
//     delivered): it returns false, false, and that fold left the incident due, so the next pass decides again;
//   - a newer incident of the same fingerprint exists. A fold that starts a new incident does not touch the old one's
//     fold_version, so the version check alone would let both generations reach CSM: it returns false, true, and the
//     caller closes this one quietly instead.
func (r *IncidentRepo) Unmute(ctx context.Context, inc model.Incident) (unmuted, superseded bool, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, false, fmt.Errorf("unmute incident %d: begin: %w", inc.ID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "fold:"+inc.Fingerprint); err != nil {
		return false, false, fmt.Errorf("unmute incident %d: lock: %w", inc.ID, err)
	}
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM incidents_processed WHERE fingerprint = $1 AND first_seen > $2)`,
		inc.Fingerprint, inc.FirstSeen).Scan(&superseded)
	if err != nil {
		return false, false, fmt.Errorf("unmute incident %d: check newer incident: %w", inc.ID, err)
	}
	if superseded {
		return false, true, nil
	}
	tag, err := tx.Exec(ctx, `UPDATE incidents_processed SET muted = false, unmuted_at = now()
		WHERE id = $1 AND fold_version = $2`, inc.ID, inc.FoldVersion)
	if err != nil {
		return false, false, fmt.Errorf("unmute incident %d: %w", inc.ID, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return false, false, fmt.Errorf("unmute incident %d: commit: %w", inc.ID, err)
	}
	return tag.RowsAffected() == 1, false, nil
}

// MuteOutcome reports, for a muted incident no rule matches any more, whether its last alert was a resolving one
// and whether a newer incident of the same fingerprint has replaced it.
func (r *IncidentRepo) MuteOutcome(ctx context.Context, inc model.Incident) (resolved, superseded bool, err error) {
	var lastKind string
	err = r.pool.QueryRow(ctx, `SELECT
		coalesce((SELECT kind FROM incident_notes WHERE incident = $1 ORDER BY id DESC LIMIT 1), ''),
		EXISTS (SELECT 1 FROM incidents_processed WHERE fingerprint = $2 AND first_seen > $3)`,
		inc.ID, inc.Fingerprint, inc.FirstSeen).Scan(&lastKind, &superseded)
	if err != nil {
		return false, false, fmt.Errorf("read mute outcome for incident %d: %w", inc.ID, err)
	}
	return lastKind == model.NoteOK, superseded, nil
}

// CloseMuted closes a muted incident without delivering anything: it is marked closed and unmuted, every pending note
// is settled, and nothing more is due. Closing it again (an OK folded in after the quiet close) keeps the first
// unmuted_at, the time the mute ended. It does nothing, and returns false, if a fold changed it since foldVersion.
func (r *IncidentRepo) CloseMuted(ctx context.Context, id, foldVersion int64) (bool, error) {
	var closed int
	err := r.pool.QueryRow(ctx, `WITH inc AS (
		UPDATE incidents_processed SET muted = false, unmuted_at = coalesce(unmuted_at, now()), status = 'closed', delivery_due_at = NULL
		WHERE id = $1 AND fold_version = $2 RETURNING id),
	notes AS (
		UPDATE incident_notes SET csm_pending = false, chat_pending = false
		WHERE incident IN (SELECT id FROM inc) AND (csm_pending OR chat_pending))
	SELECT count(*) FROM inc`, id, foldVersion).Scan(&closed)
	if err != nil {
		return false, fmt.Errorf("close muted incident %d: %w", id, err)
	}
	return closed == 1, nil
}

// MutedByRule returns one page of the incidents alert mute rule ruleID muted, newest first, and how many there are in all.
// An incident keeps mute_rule_id after it is released, so the page includes muted incidents that later became CSM
// incidents or were closed quietly. Rows the retention job purged are gone.
func (r *IncidentRepo) MutedByRule(ctx context.Context, ruleID string, limit, offset int) ([]model.Incident, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM incidents_processed WHERE mute_rule_id = $1`, ruleID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count incidents muted by rule %s: %w", ruleID, err)
	}
	rows, err := r.pool.Query(ctx, `SELECT `+incidentColumns+` FROM incidents_processed
		WHERE mute_rule_id = $1 ORDER BY first_seen DESC, id DESC LIMIT $2 OFFSET $3`, ruleID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents muted by rule %s: %w", ruleID, err)
	}
	incs, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Incident])
	if err != nil {
		return nil, 0, fmt.Errorf("list incidents muted by rule %s: %w", ruleID, err)
	}
	return incs, total, nil
}

// MutedByRuleSummary counts the incidents ruleID muted by outcome (see model.Incident.MuteOutcome).
func (r *IncidentRepo) MutedByRuleSummary(ctx context.Context, ruleID string) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+incidentColumns+` FROM incidents_processed WHERE mute_rule_id = $1`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("summarize incidents muted by rule %s: %w", ruleID, err)
	}
	incs, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Incident])
	if err != nil {
		return nil, fmt.Errorf("summarize incidents muted by rule %s: %w", ruleID, err)
	}
	counts := map[string]int{}
	for _, inc := range incs {
		counts[inc.MuteOutcome()]++
	}
	return counts, nil
}

// MutedCounts returns how many incidents each of ruleIDs muted (rules that muted nothing are absent), in one query, so a
// list of rules does not ask once per rule.
func (r *IncidentRepo) MutedCounts(ctx context.Context, ruleIDs []string) (map[string]int, error) {
	counts := map[string]int{}
	if len(ruleIDs) == 0 {
		return counts, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT mute_rule_id, count(*) FROM incidents_processed
		WHERE mute_rule_id = ANY($1) GROUP BY mute_rule_id`, ruleIDs)
	if err != nil {
		return nil, fmt.Errorf("count incidents muted by %d rules: %w", len(ruleIDs), err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("count incidents muted by %d rules: %w", len(ruleIDs), err)
		}
		counts[id] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count incidents muted by %d rules: %w", len(ruleIDs), err)
	}
	return counts, nil
}

// fieldValueColumns maps the alert fields a mute rule condition can name to incidents_processed columns.
var fieldValueColumns = map[string]string{
	"service": "service", "environment": "environment", "source": "source", "metric_name": "metric_name",
	"category": "category", "severity": "severity_label", "unique_identifier": "unique_identifier",
}

// fieldValuesWindow bounds a field-values search to recently seen incidents.
const fieldValuesWindow = "30 days"

// minUniqueIdentifierQuery is how many characters a unique_identifier search needs before it runs.
const minUniqueIdentifierQuery = 3

// likeEscaper escapes LIKE wildcards so a search matches them literally.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// FieldValues returns up to limit distinct non-empty values of alert field field that contain query (case-insensitive),
// most recently seen first. They come from the incidents still kept, so they are the values real alerts carry, which is
// what an alert mute rule must match. ok is false for a field it doesn't know.
func (r *IncidentRepo) FieldValues(ctx context.Context, field, query string, limit int) (values []string, ok bool, err error) {
	col, ok := fieldValueColumns[field]
	if !ok {
		return nil, false, nil
	}
	// unique_identifier is close to one value per alert, so it is only searched, never listed whole.
	if field == "unique_identifier" && len([]rune(query)) < minUniqueIdentifierQuery {
		return []string{}, true, nil
	}
	// Bounded to recently seen incidents so a longer retention never makes a keystroke group the whole table. The
	// portal debounces typing (300 ms) and only asks while the field's list is open.
	rows, err := r.pool.Query(ctx, `SELECT `+col+` FROM incidents_processed
		WHERE last_seen > now() - $3::interval AND `+col+` <> '' AND `+col+` ILIKE '%' || $1 || '%'
		GROUP BY `+col+` ORDER BY max(last_seen) DESC, `+col+` LIMIT $2`, likeEscaper.Replace(query), limit, fieldValuesWindow)
	if err != nil {
		return nil, true, fmt.Errorf("list %s values: %w", field, err)
	}
	values, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, true, fmt.Errorf("list %s values: %w", field, err)
	}
	return values, true, nil
}

// Purge deletes up to limit incidents, and their notes, last seen before cutoff with no delivery still owed.
func (r *IncidentRepo) Purge(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM incidents_processed WHERE id IN (
		SELECT id FROM incidents_processed WHERE last_seen < $1 AND delivery_due_at IS NULL ORDER BY id LIMIT $2)`, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("purge incidents: %w", err)
	}
	return tag.RowsAffected(), nil
}
