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
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns what a planned start / end ("plannedStartOn" / "plannedEndOn",
// "plannedStartDate" / "plannedEndDate" on create) may look like before it is
// bound into SQL.
//
// The planned window used to reach Postgres as `$n::text::timestamptz`, which
// accepts far more than a date-time: 'infinity', '-infinity', 'now', 'today',
// 'tomorrow', 'epoch', a date alone, a time-zone name, a year past 9999 -- and
// reads a value with no zone in the DATABASE SESSION's TimeZone. A customer can
// now send the window (a proposed implementation time), so each of those was
// reachable from outside: 'infinity' stored fine, then could not be scanned into
// a time.Time, which made the change request unreadable for everyone and 500ed
// the project's whole change request list.
//
// So the window is parsed here, in Go, against exactly two layouts, and only the
// parsed instant -- re-written as RFC 3339 in UTC, which means the same in every
// session -- is handed to the database:
//
//   - RFC 3339 with a zone designator ("2030-03-01T09:00:00Z", "...+05:30"): an
//     instant as written;
//   - "YYYY-MM-DD HH:MM:SS" with NO zone ("2030-03-01 09:00:00"): UTC. That is the
//     layout of the ServiceNow contract this API grew out of and the one both
//     portals send; it is UTC by definition, not by the database's setting.
//
// Anything else is a 400 that names the field.

const (
	// plannedTimestampZoneless is the layout of a window with no zone: UTC.
	plannedTimestampZoneless = "2006-01-02 15:04:05"

	// plannedYearMin / plannedYearMax bound a plausible implementation window. A
	// change planned outside them is a typing mistake (or an attack on the
	// formatting of dates well past Go's four-digit years and Postgres' 294276).
	plannedYearMin = 2000
	plannedYearMax = 2100
)

// parsePlannedTimestamp parses one planned start / end (field names it in the
// message) into an instant, or refuses it with a ValidationError.
func parsePlannedTimestamp(field, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t, err = time.Parse(plannedTimestampZoneless, value)
	}
	if err != nil {
		return time.Time{}, plannedTimestampError(field)
	}
	if y := t.UTC().Year(); y < plannedYearMin || y > plannedYearMax {
		return time.Time{}, plannedTimestampError(field)
	}
	return t.UTC(), nil
}

func plannedTimestampError(field string) error {
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"%s must be a valid date-time, either RFC 3339 (2030-03-01T09:00:00Z) or YYYY-MM-DD HH:MM:SS in UTC, in the years %d to %d",
		field, plannedYearMin, plannedYearMax)}
}

// normalizePlannedTimestamp validates *value (nil stays nil) and returns it as
// the RFC 3339 UTC string the database is given: whole microseconds, because
// that is all a timestamptz holds, so what is stored reads back as it was sent.
func normalizePlannedTimestamp(field string, value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	t, err := parsePlannedTimestamp(field, *value)
	if err != nil {
		return nil, err
	}
	out := t.Truncate(time.Microsecond).Format(time.RFC3339Nano)
	return &out, nil
}

// normalizePatchPlannedWindow returns req with its plannedStartOn / plannedEndOn
// validated and normalised (see this file's doc comment).
func normalizePatchPlannedWindow(req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestRequest, error) {
	var err error
	if req.PlannedStartOn, err = normalizePlannedTimestamp("plannedStartOn", req.PlannedStartOn); err != nil {
		return req, err
	}
	if req.PlannedEndOn, err = normalizePlannedTimestamp("plannedEndOn", req.PlannedEndOn); err != nil {
		return req, err
	}
	return req, nil
}

// NormalizeCreatePlannedWindow is the exported entry point of
// normalizeCreatePlannedWindow, for the service layer: the ServiceNow-first
// (dual-write) create validates the window with it BEFORE ServiceNow is called,
// so a refused date can never leave a ServiceNow record behind, and the
// repository applies it again to what it writes.
func NormalizeCreatePlannedWindow(req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestRequest, error) {
	return normalizeCreatePlannedWindow(req)
}

// PlannedTimestampForServiceNow re-writes a planned start / end the repository
// accepted (see parsePlannedTimestamp) in the one layout ServiceNow's change
// request API takes, "YYYY-MM-DD HH:MM:SS" in UTC, for the dual-write mirror:
// the PostgreSQL data source accepts RFC 3339 with a zone as well, which the
// ServiceNow service refuses ("must follow the format"), so an RFC 3339 PATCH
// used to commit in PostgreSQL and then fail every mirror write. A value that
// does not parse is returned unchanged (the repository has already judged it).
func PlannedTimestampForServiceNow(value string) string {
	t, err := parsePlannedTimestamp("plannedStartOn", value)
	if err != nil {
		return value
	}
	return t.Format(plannedTimestampZoneless)
}

// normalizeCreatePlannedWindow is normalizePatchPlannedWindow for the create
// request's plannedStartDate / plannedEndDate.
func normalizeCreatePlannedWindow(req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestRequest, error) {
	var err error
	if req.PlannedStartDate, err = normalizePlannedTimestamp("plannedStartDate", req.PlannedStartDate); err != nil {
		return req, err
	}
	if req.PlannedEndDate, err = normalizePlannedTimestamp("plannedEndDate", req.PlannedEndDate); err != nil {
		return req, err
	}
	return req, nil
}

// requireFutureWindow refuses a window that starts, or ends, in the past. Only a
// customer's proposed time is held to it (a proposal is for a time still to
// come); a WSO2 user re-planning a change may have a reason to record one that
// has gone. The values are the normalised RFC 3339 strings.
func requireFutureWindow(now time.Time, start, end *string) error {
	for _, p := range []struct {
		field string
		value *string
	}{{"plannedStartOn", start}, {"plannedEndOn", end}} {
		if p.value == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, *p.value)
		if err != nil {
			return plannedTimestampError(p.field)
		}
		if !t.After(now) {
			return &apierror.ValidationError{Msg: fmt.Sprintf("%s is in the past: a proposed implementation time must be one still to come", p.field)}
		}
	}
	return nil
}

// requireFutureEffectiveStart is requireFutureWindow's complement for a proposal
// that does not carry a start: requireFutureWindow only judges the bounds that
// were SENT, so an end-only proposal left the stored start in place even when it
// had already passed. The start the change will have is the proposed one, else
// the stored one, and it must be still to come. A change with no stored start (or
// one the database holds as infinity) has nothing to judge.
func requireFutureEffectiveStart(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, id string, now time.Time, proposedStart *string) error {
	if proposedStart != nil {
		return nil
	}
	var stored *time.Time
	err := q.QueryRow(ctx, `SELECT CASE WHEN isfinite(start_on) THEN start_on END FROM change_request WHERE id = $1`, id).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("propose implementation time: read the planned start: %w", err)
	}
	if stored == nil || stored.After(now) {
		return nil
	}
	return &apierror.ValidationError{Msg: fmt.Sprintf(
		"plannedStartOn is in the past: the current planned start (%s) has passed; propose a new start as well",
		stored.UTC().Format(time.RFC3339))}
}
