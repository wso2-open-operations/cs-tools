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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// Reads and writes for the availability calculation.
//
// *** THIS WRITES A TABLE csm-sync-service OWNS, AND THAT IS THE POINT. ***
// service_availability is mirrored from ServiceNow today (digiops-cs
// migration 0084, 212,904 rows). After cutover nothing writes it, and the
// Cloud Status Dashboard reads it on every page load. This repository is
// what takes over. Until SN is switched off BOTH write it, so the sweep must
// be idempotent on the natural key rather than append — see UpsertFixed.

// AvailabilitySubject is one thing availability is computed for: a service
// offering (or CI) together with the commitment it answers to.
type AvailabilitySubject struct {
	ServiceOfferingID   *string
	CmdbCiID            *string
	ServiceCommitmentID string
	// TargetPercent is service_commitment.percentage_avail — ServiceNow's
	// `availability` column. 100 on every commitment on the instance, which
	// makes allowed downtime zero.
	TargetPercent float64
	// Timezone is the commitment's own zone. ServiceNow resolves every
	// period boundary in it.
	Timezone string
	// ScheduleID is nullable: no schedule means unrestricted (24x7).
	ScheduleID *string
	// ScheduleName is the referenced schedule's name, nil when ScheduleID is
	// nil or the schedule row is missing. Only "24 x 7" is modelled today --
	// see availabilityService.scheduleFor.
	ScheduleName *string
}

// SubjectID is whichever of the two subject columns is set. ServiceNow's
// getCiFromCommitment does the same thing: offering first, else the CI.
func (s AvailabilitySubject) SubjectID() string {
	if s.ServiceOfferingID != nil {
		return *s.ServiceOfferingID
	}
	if s.CmdbCiID != nil {
		return *s.CmdbCiID
	}
	return ""
}

// AvailabilityOutageRow is one outage interval the calculator will consume.
type AvailabilityOutageRow struct {
	Begin time.Time
	End   time.Time
	Type  string
}

// ComputedAvailabilityRow is one computed service_availability row.
//
// Named for the write side. AvailabilityRow was already taken by
// cloud_status_api_repo for the READ side the dashboard serves, and the two
// are different shapes: that one carries a rendered duration label, this one
// carries every stored column.
type ComputedAvailabilityRow struct {
	ServiceOfferingID   *string
	CmdbCiID            *string
	ServiceCommitmentID string
	Type                string
	Begin               time.Time
	End                 time.Time
	TimeZone            string

	AbsoluteDowntime  time.Duration
	ScheduledDowntime time.Duration
	ScheduledTotal    time.Duration
	AbsoluteAvail     float64
	ScheduledAvail    float64
	AbsoluteCount     int
	ScheduledCount    int
	MTBF              time.Duration
	MTRS              time.Duration
	AllowedDowntime   time.Duration
	CommitmentMet     bool
}

// AvailabilityRepository is the data access the sweep needs.
type AvailabilityRepository interface {
	Subjects(ctx context.Context) ([]AvailabilitySubject, error)
	OutagesFor(ctx context.Context, subjectID string, begin, end time.Time) ([]AvailabilityOutageRow, error)
	UpsertFixed(ctx context.Context, rows []ComputedAvailabilityRow) error
	ReplaceRolling(ctx context.Context, subjectID string, commitmentID string, types []string, rows []ComputedAvailabilityRow) error
}

// ScheduleSpanRow is one cmn_schedule_span, mirrored.
type ScheduleSpanRow struct {
	StartOn    *time.Time
	EndOn      *time.Time
	SpanType   string
	RepeatType string
	ShowAs     string
}

type availabilityRepository struct{ db db.Pool }

// NewAvailabilityRepository constructs the repository.
func NewAvailabilityRepository(db db.Pool) AvailabilityRepository {
	return &availabilityRepository{db: db}
}

// *** ONLY type = AVAILABILITY COMMITMENTS ARE SUBJECTS. ***
// A MAINTENANCE_WINDOW commitment on the same offering is schedule input,
// not a thing to compute — including it would write a second, meaningless
// row per offering. ServiceNow filters the same way.
//
// Unpublished offerings are skipped, but one with an EMPTY state is kept:
// V2's encoded query is `service_offering.state=published^ORservice_offering
// .state=`, so null and "published" behave alike and everything else is out.
// An empty ServiceNow state arrives as NULL: service_state_enum has no empty
// value, and comparing the enum to an empty string is an error, not false.
const availabilitySubjectsSQL = `
    SELECT soc.service_offering_id,
           soc.cmdb_ci_id,
           soc.service_commitment_id,
           COALESCE(sc.percentage_avail, 100),
           COALESCE(sc.timezone, ''),
           sc.schedule_id,
           sch.name
      FROM service_offering_commitment soc
      JOIN service_commitment sc ON sc.id = soc.service_commitment_id
      LEFT JOIN schedule sch ON sch.id = sc.schedule_id
      LEFT JOIN service_offering so ON so.id = soc.service_offering_id
     WHERE sc.type = 'AVAILABILITY'
       AND (soc.cmdb_ci_id IS NOT NULL
            OR so.state IS NULL OR so.state = 'PUBLISHED')
     ORDER BY soc.service_offering_id, soc.service_commitment_id`

func (r *availabilityRepository) Subjects(ctx context.Context) ([]AvailabilitySubject, error) {
	rows, err := r.db.Query(ctx, availabilitySubjectsSQL)
	if err != nil {
		return nil, fmt.Errorf("query availability subjects: %w", err)
	}
	defer rows.Close()

	var out []AvailabilitySubject
	for rows.Next() {
		var s AvailabilitySubject
		if err := rows.Scan(&s.ServiceOfferingID, &s.CmdbCiID, &s.ServiceCommitmentID,
			&s.TargetPercent, &s.Timezone, &s.ScheduleID, &s.ScheduleName); err != nil {
			return nil, fmt.Errorf("scan availability subject: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// *** THREE WAYS AN OUTAGE REACHES A SUBJECT, NOT ONE. ***
// V2 ORs a direct cmdb_ci match with a join through cmdb_outage_ci_mtom.
// The Postgres mirror splits ServiceNow's polymorphic cmdb_ci into two typed
// columns — service_id and service_offering_id — so the direct match is two
// predicates here where ServiceNow has one. Dropping either loses outages
// silently: 427 of the instance's outages point at a service and only 34 at
// an offering.
//
// type IN (OUTAGE, PLANNED) and both ends non-null are V2's own filters.
// DEGRADATION never arrives, and an outage still open is excluded outright
// rather than counted up to now — 70 outages on the instance are in that
// state, several open since 2021.
const availabilityOutagesSQL = `
    SELECT o.start_on, o.end_on, o.type::text
      FROM outage o
     WHERE (o.service_offering_id = $1::uuid
            OR o.service_id = $1::uuid
            OR EXISTS (SELECT 1 FROM outage_affected_ci a
                        WHERE a.outage_id = o.id AND a.ci_id = $1::uuid))
       AND o.type IN ('OUTAGE', 'PLANNED')
       AND o.start_on IS NOT NULL
       AND o.end_on IS NOT NULL
       AND o.start_on < $3
       AND o.end_on   > $2
     ORDER BY o.start_on`

func (r *availabilityRepository) OutagesFor(ctx context.Context, subjectID string, begin, end time.Time) ([]AvailabilityOutageRow, error) {
	rows, err := r.db.Query(ctx, availabilityOutagesSQL, subjectID, begin, end)
	if err != nil {
		return nil, fmt.Errorf("query outages for availability: %w", err)
	}
	defer rows.Close()

	var out []AvailabilityOutageRow
	for rows.Next() {
		var o AvailabilityOutageRow
		if err := rows.Scan(&o.Begin, &o.End, &o.Type); err != nil {
			return nil, fmt.Errorf("scan outage for availability: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// *** THE NATURAL KEY IS (subject, commitment, type, start) AND THERE IS NO
// UNIQUE INDEX ON IT. *** csm-sync-service keys service_availability on the
// mirrored sys_id, so two writers -- the sync and this sweep -- can produce
// two rows for the same period. Until SN is switched off that is the real
// state of the world, so this deletes by the natural key inside the
// transaction and then inserts, rather than relying on ON CONFLICT against
// a constraint that does not exist.
// Plain equality on service_offering_id, not IS NOT DISTINCT FROM: every
// subject the sweep writes is an offering (CI-only subjects fail before they
// reach here), and equality lets Postgres use the (service_offering_id,
// type, start_on) index instead of reading every row of the type.
const deleteByNaturalKeySQL = `
    DELETE FROM service_availability
     WHERE service_commitment_id = $1::uuid
       AND type = $2
       AND start_on = $3
       AND service_offering_id = $4::uuid`

const insertAvailabilitySQL = `
    INSERT INTO service_availability (
        id, created_on, updated_on, created_by, updated_by,
        service_offering_id, service_commitment_id, type, start_on, end_on, time_zone,
        absolute_downtime_duration, scheduled_downtime_duration, committed_uptime_duration,
        absolute_availability, scheduled_availability,
        absolute_count, scheduled_count,
        mtbf_duration, mtrs_duration, allowed_downtime_duration, is_commitment_met
    ) VALUES (
        gen_random_uuid(), now(), now(), 'csm-availability', 'csm-availability',
        $1::uuid, $2::uuid, $3, $4, $5, $6,
        $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
    )`

func (r *availabilityRepository) UpsertFixed(ctx context.Context, rows []ComputedAvailabilityRow) error {
	return r.writeRows(ctx, rows, nil, "", "")
}

// ReplaceRolling deletes every rolling row for this subject and commitment
// before inserting the new ones.
//
// *** ROLLING ROWS MUST BE DELETED BY TYPE, NOT BY PERIOD. *** Their start
// moves every day, so deleting by the natural key would leave yesterday's
// window behind and the table would grow by one row per subject per type per
// day. ServiceNow does the same thing with a GlideMultipleDelete keyed on
// type alone.
func (r *availabilityRepository) ReplaceRolling(
	ctx context.Context, subjectID, commitmentID string, types []string, rows []ComputedAvailabilityRow,
) error {
	return r.writeRows(ctx, rows, types, subjectID, commitmentID)
}

func (r *availabilityRepository) writeRows(
	ctx context.Context, rows []ComputedAvailabilityRow, rollingTypes []string, subjectID, commitmentID string,
) error {
	if len(rows) == 0 && len(rollingTypes) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin availability write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(rollingTypes) > 0 {
		if _, err := tx.Exec(ctx,
			`DELETE FROM service_availability
              WHERE service_commitment_id = $1::uuid
                AND type = ANY($2)
                AND service_offering_id = $3::uuid`,
			commitmentID, rollingTypes, subjectID); err != nil {
			return fmt.Errorf("clear rolling availability: %w", err)
		}
	}

	for _, row := range rows {
		if len(rollingTypes) == 0 {
			if _, err := tx.Exec(ctx, deleteByNaturalKeySQL,
				row.ServiceCommitmentID, row.Type, row.Begin, row.ServiceOfferingID); err != nil {
				return fmt.Errorf("clear availability period: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, insertAvailabilitySQL,
			row.ServiceOfferingID, row.ServiceCommitmentID,
			row.Type, row.Begin, row.End, row.TimeZone,
			row.AbsoluteDowntime, row.ScheduledDowntime, row.ScheduledTotal,
			row.AbsoluteAvail, row.ScheduledAvail,
			row.AbsoluteCount, row.ScheduledCount,
			row.MTBF, row.MTRS, row.AllowedDowntime, row.CommitmentMet,
		); err != nil {
			return fmt.Errorf("insert availability row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit availability write: %w", err)
	}
	return nil
}
