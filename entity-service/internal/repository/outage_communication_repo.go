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
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OutageCommunicationRepository reads the outages this flow may owe an email
// and records what was sent.
type OutageCommunicationRepository interface {
	PendingOutages(ctx context.Context, limit int) ([]domain.OutageForCommunication, error)
	Record(ctx context.Context, req domain.RecordOutageCommunicationRequest) error
	LogForOutage(ctx context.Context, number string) ([]domain.OutageCommunicationLogEntry, error)
}

type outageCommunicationRepo struct {
	db *pgxpool.Pool
}

// NewOutageCommunicationRepository wires the repository to a pool.
func NewOutageCommunicationRepository(db *pgxpool.Pool) OutageCommunicationRepository {
	return &outageCommunicationRepo{db: db}
}

// pendingOutageCommunicationsSQL selects the outages a sweep must consider,
// and answers "what have we already said?" in the same pass.
//
// *** THE TWO EXISTS CLAUSES ARE THE IDEMPOTENCY GUARD. *** ServiceNow's
// flow is "Run Trigger: Once": the platform starts one instance per outage
// and that instance sends each email exactly once, so the flow writes no
// state to the outage at all — both of its Update Outage steps are
// configured with no fields. A sweep re-reads every qualifying row forever
// and can inherit none of that, so the guard is reconstructed from the
// communication log, which the flow was already writing.
//
// *** MATCHED ON email_type_norm, NEVER ON email_type. *** The live data
// holds both "Declared" (88 rows) and "Declare" (7). An equality check on
// the long spelling misses the short one and re-announces those outages —
// real email to a real list. The generated column collapses both.
//
// *** THE LOG IS SHARED WITH OTHER FLOWS. *** Its largest bucket is
// "Update" (155 rows) written by `Outage update (Outage table)`, and it
// carries 13 "RCA" rows from a flow that is inactive today. Filtering by
// outage alone would read another flow's traffic as ours; the type filter
// is not optional.
//
// *** TWO FIELDS THE EMAIL RENDERS ARE NOT MIRRORED AT ALL. *** ServiceNow's
// body prints "Impact:" and "Current Status:", which come from cmdb_ci_outage's
// impact and state. NEITHER EXISTS on the Postgres `outage` table, under that
// name or any other -- checked against the live schema, not assumed.
//
// They are therefore left empty rather than filled from a nearby column. An
// earlier revision of this query mapped Impact to `o.message`, which is the
// outage's own message and a different field entirely: it would have rendered
// a plausible-looking wrong value in every email, which is worse than a blank.
// Add them to the sync service's outage mapping if the blanks matter.
//
// `opted_in` is outage.outage_communication, which csm-sync-service does not
// mirror yet. Until it does, this query fails with undefined_column and the
// caller degrades to "nothing to send" — see PendingOutages.
const pendingOutageCommunicationsSQL = `
SELECT o.id::text,
       COALESCE(o.number, ''),
       COALESCE(o.type::text, ''),
       -- ServiceNow's Title/Description is short_description, which the
       -- mirror lands in the name column.
       COALESCE(o.name, ''),
       o.start_on,
       o.end_on,
       -- Seconds, not interval text. Casting the interval to text renders
       -- as 00:31:25.634362 -- six decimal places of microseconds, which is
       -- what a raw Postgres interval prints and is not something anyone
       -- wants in an email. Formatting belongs in the service layer where
       -- it is testable, so the repository hands over a number.
       COALESCE(EXTRACT(EPOCH FROM o.duration)::bigint, 0),
       COALESCE(o.outage_communication, FALSE),
       EXISTS (SELECT 1 FROM outage_communication_log l
                WHERE l.outage_number = o.number
                  AND l.email_type_norm = 'DECLARED'),
       EXISTS (SELECT 1 FROM outage_communication_log l
                WHERE l.outage_number = o.number
                  AND l.email_type_norm = 'RESOLVED'),
       COALESCE((SELECT l.subject FROM outage_communication_log l
                  WHERE l.outage_number = o.number
                    AND l.email_type_norm = 'DECLARED'
                  ORDER BY l.sent_on DESC LIMIT 1), '')
  FROM outage o
 WHERE o.outage_communication IS TRUE
   AND o.start_on IS NOT NULL
   AND NOT (
        EXISTS (SELECT 1 FROM outage_communication_log l
                 WHERE l.outage_number = o.number
                   AND l.email_type_norm = 'RESOLVED')
   )
 ORDER BY o.start_on, o.id
 LIMIT $1`

func (r *outageCommunicationRepo) PendingOutages(ctx context.Context, limit int) ([]domain.OutageForCommunication, error) {
	rows, err := r.db.Query(ctx, pendingOutageCommunicationsSQL, limit)
	if err != nil {
		// outage.outage_communication arrives with a sync-side mapping that
		// does not exist yet. A database without it should report nothing to
		// send rather than fail the sweep. Narrow on purpose: only
		// undefined_table/undefined_column degrade, because every other error
		// looks identical to "no outages" once swallowed.
		pgErr := (*pgconn.PgError)(nil)
		if !errors.As(err, &pgErr) || (pgErr.Code != "42P01" && pgErr.Code != "42703") {
			return nil, fmt.Errorf("querying outages pending communication: %w", err)
		}
		slog.WarnContext(ctx, "outage communication: outage.outage_communication absent (csm-sync-service mapping not applied); nothing to send",
			"sqlstate", pgErr.Code)
		return nil, nil
	}
	defer rows.Close()

	var out []domain.OutageForCommunication
	for rows.Next() {
		var o domain.OutageForCommunication
		if err := rows.Scan(
			&o.OutageID, &o.Number, &o.Type, &o.ShortDescription,
			&o.StartOn, &o.EndOn, &o.DurationSeconds,
			&o.OptedIn, &o.AlreadyDeclared, &o.AlreadyResolved, &o.DeclaredSubject,
		); err != nil {
			return nil, fmt.Errorf("scanning outage pending communication: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading outages pending communication: %w", err)
	}
	return out, nil
}

// recordOutageCommunicationSQL appends one send to the log.
//
// Deliberately an append, with no uniqueness constraint. ServiceNow's table
// has none either, and a duplicate row is a far better failure than a
// swallowed one: it is visible, and the guard above reads EXISTS rather
// than counting.
const recordOutageCommunicationSQL = `
INSERT INTO outage_communication_log
       (outage_number, email_type, outage_status, subject,
        recipients, email_content, main_content)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

func (r *outageCommunicationRepo) Record(ctx context.Context, req domain.RecordOutageCommunicationRequest) error {
	_, err := r.db.Exec(ctx, recordOutageCommunicationSQL,
		req.OutageNumber, req.EmailType, req.OutageStatus, req.Subject,
		req.Recipients, req.EmailContent, req.MainContent)
	if err != nil {
		return fmt.Errorf("recording outage communication for %s: %w", req.OutageNumber, err)
	}
	return nil
}

// logForOutageSQL is operational visibility: what has been said about one
// outage, by this service and by ServiceNow before it, newest first.
//
// email_content is deliberately NOT selected. It is a full HTML email body
// and this endpoint exists to answer "did we send, and when" — returning
// every body would make the response enormous for no gain.
const logForOutageSQL = `
SELECT id::text,
       outage_number,
       COALESCE(email_type, ''),
       COALESCE(email_type_norm, ''),
       COALESCE(outage_status, ''),
       COALESCE(subject, ''),
       COALESCE(recipients, ''),
       COALESCE(main_content, ''),
       sent_on
  FROM outage_communication_log
 WHERE outage_number = $1
 ORDER BY sent_on DESC, id`

func (r *outageCommunicationRepo) LogForOutage(ctx context.Context, number string) ([]domain.OutageCommunicationLogEntry, error) {
	rows, err := r.db.Query(ctx, logForOutageSQL, number)
	if err != nil {
		return nil, fmt.Errorf("querying communication log for %s: %w", number, err)
	}
	defer rows.Close()

	entries := make([]domain.OutageCommunicationLogEntry, 0)
	for rows.Next() {
		var e domain.OutageCommunicationLogEntry
		if err := rows.Scan(&e.ID, &e.OutageNumber, &e.EmailType, &e.EmailTypeNorm,
			&e.OutageStatus, &e.Subject, &e.Recipients, &e.MainContent, &e.SentOn); err != nil {
			return nil, fmt.Errorf("scanning communication log row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading communication log for %s: %w", number, err)
	}
	return entries, nil
}
