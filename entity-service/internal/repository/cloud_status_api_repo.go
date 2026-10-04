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
	"github.com/jackc/pgx/v5/pgxpool"
)

// The reads behind the public cloud status dashboard.
//
// Postgres equivalents of the ServiceNow Scripted REST APIs in
// wso2-enterprise/uptime-dashboard under servicenow/scripted-rest-api/. Those
// scripts were read directly; each query below notes what it reproduces and
// where it deliberately differs.

// MonitorRow is one active cloud monitor for a cloud.
type MonitorRow struct {
	Region            string
	Group             string
	Name              string
	Description       *string
	Status            string
	ServiceOfferingID string
}

// AvailabilityRow is one uptime figure for one service offering.
type AvailabilityRow struct {
	ServiceOfferingID string
	// Duration is the dashboard's label, already translated from the enum.
	Duration string
	// Availability is carried as text because the dashboard renders it
	// verbatim; see domain.CloudStatusAvailability.
	Availability string
}

// OngoingOutageRow is an outage still in progress against one service
// offering, used to attach a message to a monitor.
type OngoingOutageRow struct {
	ServiceOfferingID string
	ShortDescription  string
	Type              string
}

// IncidentRow is one outage as the incident history shows it.
type IncidentRow struct {
	ID               string
	Begin            string
	End              string
	Type             string
	ShortDescription string
}

// CloudStatusDashboardRepository reads what the public status dashboard
// renders. Reads only: this data is published to customers, and the service
// that shows it must never be able to change it.
type CloudStatusDashboardRepository interface {
	Monitors(ctx context.Context, cloud string) ([]MonitorRow, error)
	Availabilities(ctx context.Context, offeringIDs []string) ([]AvailabilityRow, error)
	OngoingOutages(ctx context.Context, offeringIDs []string) ([]OngoingOutageRow, error)
	Incidents(ctx context.Context, cloud string, since time.Time) ([]IncidentRow, error)
	// ParentAvailabilities returns the four window figures for every offering
	// under one parent service -- the slice the weighting is applied over.
	ParentAvailabilities(ctx context.Context, parentID string) ([]ParentAvailabilityRow, error)
	// DailyAvailability returns one figure per offering per local day across
	// the given window, bucketed in the named timezone.
	DailyAvailability(ctx context.Context, offeringIDs []string, tz, from, to string) ([]DailyAvailabilityRow, error)
	// MonitorsForHistory returns active monitors in the HISTORY endpoint's
	// order, which is not the monitors endpoint's -- see the query.
	MonitorsForHistory(ctx context.Context, cloud string) ([]MonitorRow, error)
	// IncidentDetail returns one outage's detail view, or nil when no outage
	// with that id belongs to that cloud.
	IncidentDetail(ctx context.Context, id, cloud string) (*IncidentDetailRow, error)
	// OutageComments returns one outage's customer-facing updates, newest
	// first.
	OutageComments(ctx context.Context, outageID string) ([]OutageCommentRow, error)
}

type cloudStatusDashboardRepository struct {
	db *pgxpool.Pool
}

// NewCloudStatusDashboardRepository constructs the dashboard reader.
func NewCloudStatusDashboardRepository(db *pgxpool.Pool) CloudStatusDashboardRepository {
	return &cloudStatusDashboardRepository{db: db}
}

// monitorsSQL reproduces monitors.js's own query.
//
//	gr.addEncodedQuery('u_cloud_offering=' + cloud + '^u_active=true');
//	gr.orderByDesc('u_group_priority');
//	gr.orderBy('u_group');
//	gr.orderBy('u_name');
//
// THE ORDER IS LOAD-BEARING, not cosmetic. The script builds its response by
// appending to whichever group it last saw, so the row order decides both the
// order of groups on the page and which monitors land together. Sorting
// differently here would silently reshuffle a customer-facing page.
//
// NULLS LAST on group_priority because Postgres sorts NULLs first on DESC and
// ServiceNow treats an empty priority as the lowest.
const monitorsSQL = `
    SELECT LOWER(COALESCE(cm.region, '')),
           COALESCE(cm."group", ''),
           COALESCE(cm.name, ''),
           cm.description,
           COALESCE(cm.status::text, ''),
           COALESCE(cm.service_offering_id::text, '')
      FROM cloud_monitor cm
     WHERE cm.cloud_offering = $1::cloud_monitor_cloud_offering_enum
       AND cm.is_active IS TRUE
     ORDER BY cm.group_priority DESC NULLS LAST, cm."group", cm.name
`

// Monitors returns every active monitor for one cloud, in the dashboard's order.
func (r *cloudStatusDashboardRepository) Monitors(ctx context.Context, cloud string) ([]MonitorRow, error) {
	rows, err := r.db.Query(ctx, monitorsSQL, cloud)
	if err != nil {
		return nil, fmt.Errorf("query cloud status monitors: %w", err)
	}
	defer rows.Close()

	out := make([]MonitorRow, 0)
	for rows.Next() {
		var m MonitorRow
		if err := rows.Scan(&m.Region, &m.Group, &m.Name, &m.Description, &m.Status, &m.ServiceOfferingID); err != nil {
			return nil, fmt.Errorf("scan cloud status monitor: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// availabilitiesSQL replaces the per-monitor lookup the script does inside its
// loop:
//
//	'service_offering.sys_id=' + serviceOffering + '^typeINlast30days,last90days'
//
// Batched over every offering in one round trip instead of one query per
// monitor. With 36 monitors on the busiest cloud that is 36 queries collapsed
// into one, and the result is identical because the script's inner query has
// no ordering or limit to preserve.
//
// DISTINCT ON keeps the most recent row per (offering, window). The table
// holds 213k rows across many windows and re-computations; the script relied
// on there being exactly one match per type, which is not guaranteed.
const availabilitiesSQL = `
    SELECT DISTINCT ON (sa.service_offering_id, sa.type)
           sa.service_offering_id::text,
           sa.type::text,
           COALESCE(sa.absolute_availability::text, '')
      FROM service_availability sa
     WHERE sa.service_offering_id = ANY($1::uuid[])
       AND sa.type IN ('LAST_30_DAYS', 'LAST_90_DAYS')
     ORDER BY sa.service_offering_id, sa.type, sa.end_on DESC NULLS LAST
`

// availabilityDuration maps the stored enum to the dashboard's own label.
// The strings are the script's, and the frontend renders them directly.
var availabilityDuration = map[string]string{
	"LAST_30_DAYS": "Last 30 days",
	"LAST_90_DAYS": "Last 90 days",
}

// Availabilities returns the 30- and 90-day uptime for each offering.
func (r *cloudStatusDashboardRepository) Availabilities(ctx context.Context, offeringIDs []string) ([]AvailabilityRow, error) {
	if len(offeringIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, availabilitiesSQL, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("query cloud status availabilities: %w", err)
	}
	defer rows.Close()

	out := make([]AvailabilityRow, 0)
	for rows.Next() {
		var a AvailabilityRow
		var enumType string
		if err := rows.Scan(&a.ServiceOfferingID, &enumType, &a.Availability); err != nil {
			return nil, fmt.Errorf("scan cloud status availability: %w", err)
		}
		a.Duration = availabilityDuration[enumType]
		out = append(out, a)
	}
	return out, rows.Err()
}

// ongoingOutagesSQL replaces the script's nested lookup for a monitor's
// message:
//
//	'ci_item.sys_id=' + serviceOffering + '^outage.endISEMPTY'
//
// then reading short_description and type off each outage. Batched the same
// way, and for the same reason.
//
// Note this joins outage_affected_ci, which a separately tracked sync-side
// change provides. Until
// that lands the query returns nothing and monitors simply carry no message --
// degrading to the dashboard's own empty-message case rather than failing.
const ongoingOutagesSQL = `
    SELECT ac.ci_id::text,
           COALESCE(o.name, ''),
           COALESCE(o.type::text, '')
      FROM outage_affected_ci ac
      JOIN outage o ON o.id = ac.outage_id
     WHERE ac.ci_id = ANY($1::uuid[])
       AND o.end_on IS NULL
`

// OngoingOutages returns outages still in progress against the given offerings.
func (r *cloudStatusDashboardRepository) OngoingOutages(ctx context.Context, offeringIDs []string) ([]OngoingOutageRow, error) {
	if len(offeringIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, ongoingOutagesSQL, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("query ongoing outages: %w", err)
	}
	defer rows.Close()

	out := make([]OngoingOutageRow, 0)
	for rows.Next() {
		var o OngoingOutageRow
		if err := rows.Scan(&o.ServiceOfferingID, &o.ShortDescription, &o.Type); err != nil {
			return nil, fmt.Errorf("scan ongoing outage: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// incidentsSQL reproduces incidents.js's query:
//
//	'cmdb_ci.ref_service_offering.parent.nameSTARTSWITH' + cloud +
//	'^task_numberISNOTEMPTY' +
//	'^beginBETWEENgs.beginningOfLast2Quarters()@gs.endOfToday()'
//	orderByDesc('sys_created_on')
//
// *** THE CLOUD FILTER HERE IS BY NAME, NOT BY THE FOURTEEN SYSIDS. *** The
// notification flow filters on a hardcoded list of service sys_ids; this
// script matches on the parent service's NAME starting with the cloud. They
// are different populations and both are reproduced faithfully, each in its
// own place. Substituting one for the other would change which incidents the
// public page lists.
//
// task_number IS NOT EMPTY becomes work_item_id IS NOT NULL: the sync maps
// that reference across. It matters -- only 188 of 636 outages carry one, so
// the filter removes two thirds of the table.
//
// The date window is passed in rather than computed here, so the caller owns
// "last two quarters" and it can be tested without freezing a clock.
const incidentsSQL = `
    SELECT o.id::text,
           to_char(o.start_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'),
           COALESCE(to_char(o.end_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), ''),
           COALESCE(o.type::text, ''),
           COALESCE(o.name, '')
      FROM outage o
      JOIN service_offering so ON so.id = o.service_offering_id
      JOIN service s ON s.id = so.parent_id
     WHERE s.name ILIKE $1 || '%'
       AND o.work_item_id IS NOT NULL
       AND o.start_on >= $2
       AND o.start_on IS NOT NULL
     ORDER BY o.created_on DESC
`

// Incidents returns the outages the dashboard lists for one cloud since the
// given instant.
func (r *cloudStatusDashboardRepository) Incidents(ctx context.Context, cloud string, since time.Time) ([]IncidentRow, error) {
	rows, err := r.db.Query(ctx, incidentsSQL, cloud, since)
	if err != nil {
		return nil, fmt.Errorf("query cloud status incidents: %w", err)
	}
	defer rows.Close()

	out := make([]IncidentRow, 0)
	for rows.Next() {
		var i IncidentRow
		if err := rows.Scan(&i.ID, &i.Begin, &i.End, &i.Type, &i.ShortDescription); err != nil {
			return nil, fmt.Errorf("scan cloud status incident: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ── /availabilities ────────────────────────────────────────────────────

// ParentAvailabilityRow is one offering's uptime for one window, under one
// parent service.
type ParentAvailabilityRow struct {
	ServiceOfferingID string
	Window            string
	Availability      float64
}

// parentAvailabilitiesSQL reproduces the availabilities script's query:
//
//	'service_offering.parent.sys_id=' + offeringParentSysId +
//	'^type=last12months^ORtype=last7days^ORtype=last30days^ORtype=last90days'
//
// *** THE DISTINCT ON IS NOT OPTIONAL, AND IT IS NOT COSMETIC. *** The mirror
// holds exactly TWO rows per (offering, window) -- the same figure computed on
// consecutive days, e.g. windows 08-27..09-26 and 08-28..09-27. The script
// sums availMap without deduplicating, so on this data it would count every
// offering's weight twice and publish ~200%. The live API returns 100.000, so
// ServiceNow itself sees one row per pair and the second is an artifact of how
// csm-sync-service mirrors the table. Checked 2026-09-29: 584 pairs, two rows
// each, and ZERO disagree on absolute_availability -- so which one wins cannot
// change a published figure, only how many times it is added.
//
// Newest window wins, matching "the figure ServiceNow currently shows".
const parentAvailabilitiesSQL = `
    SELECT DISTINCT ON (sa.service_offering_id, sa.type)
           sa.service_offering_id::text,
           sa.type::text,
           COALESCE(sa.absolute_availability, 0)::float8
      FROM service_availability sa
      JOIN service_offering so ON so.id = sa.service_offering_id
     WHERE so.parent_id = $1::uuid
       AND sa.type IN ('LAST_7_DAYS', 'LAST_30_DAYS', 'LAST_90_DAYS', 'LAST_12_MONTHS')
     ORDER BY sa.service_offering_id, sa.type, sa.end_on DESC NULLS LAST, sa.id
`

// ParentAvailabilities returns every offering's four window figures for one
// parent service -- the unit the weighting is applied over.
func (r *cloudStatusDashboardRepository) ParentAvailabilities(ctx context.Context, parentID string) ([]ParentAvailabilityRow, error) {
	rows, err := r.db.Query(ctx, parentAvailabilitiesSQL, parentID)
	if err != nil {
		return nil, fmt.Errorf("query parent availabilities: %w", err)
	}
	defer rows.Close()

	out := make([]ParentAvailabilityRow, 0)
	for rows.Next() {
		var a ParentAvailabilityRow
		if err := rows.Scan(&a.ServiceOfferingID, &a.Window, &a.Availability); err != nil {
			return nil, fmt.Errorf("scan parent availability: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── /history ───────────────────────────────────────────────────────────

// DailyAvailabilityRow is one day's uptime for one offering.
type DailyAvailabilityRow struct {
	ServiceOfferingID string
	Date              string
	Availability      float64
}

// dailyAvailabilitySQL reproduces the history script's per-monitor query:
//
//	'service_offering.sys_id=' + serviceOffering +
//	'^typeINdaily^startONLast 90 days@gs.beginningOfLast90Days()@gs.endOfLast90Days()'
//
// Batched over every offering rather than issued once per monitor, the same
// way the monitors port batches its inner lookups.
//
// *** THE DAY IS THE INSTANCE'S LOCAL DAY, NOT UTC. *** The script renders
// each row with getDate().getDisplayValue(), which is the calling user's
// timezone. The stored buckets confirm it: start_on values sit at 18:30:00Z,
// i.e. exactly midnight Asia/Colombo. Bucketing by UTC date would shift every
// point across the +05:30 boundary and silently relabel the whole chart.
//
// *** DEDUP: A DELIBERATE DIVERGENCE, BECAUSE THE SOURCE HAS NO RULE. *** The
// script's query carries no ORDER BY and its dedup keeps whichever row the
// database happened to return first -- so on the 15 (offering, day) pairs
// whose duplicate rows DISAGREE on availability, which value is published is
// genuinely arbitrary in ServiceNow. That cannot be reproduced; it can only be
// replaced with a rule. Newest computation wins, which is the same rule the
// window query above uses.
const dailyAvailabilitySQL = `
    SELECT DISTINCT ON (sa.service_offering_id, (sa.start_on AT TIME ZONE $2)::date)
           sa.service_offering_id::text,
           to_char((sa.start_on AT TIME ZONE $2)::date, 'YYYY-MM-DD'),
           COALESCE(sa.absolute_availability, 0)::float8
      FROM service_availability sa
     WHERE sa.service_offering_id = ANY($1::uuid[])
       AND sa.type = 'DAILY'
       AND (sa.start_on AT TIME ZONE $2)::date >= $3::date
       AND (sa.start_on AT TIME ZONE $2)::date <= $4::date
     ORDER BY sa.service_offering_id,
              (sa.start_on AT TIME ZONE $2)::date,
              sa.created_on DESC NULLS LAST,
              sa.id
`

// DailyAvailability returns one uptime figure per offering per local day
// across the window, ordered oldest first within each offering.
func (r *cloudStatusDashboardRepository) DailyAvailability(ctx context.Context, offeringIDs []string, tz, from, to string) ([]DailyAvailabilityRow, error) {
	if len(offeringIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, dailyAvailabilitySQL, offeringIDs, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("query daily availability: %w", err)
	}
	defer rows.Close()

	out := make([]DailyAvailabilityRow, 0)
	for rows.Next() {
		var d DailyAvailabilityRow
		if err := rows.Scan(&d.ServiceOfferingID, &d.Date, &d.Availability); err != nil {
			return nil, fmt.Errorf("scan daily availability: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// monitorsForHistorySQL is the history script's own monitor query:
//
//	gr.addEncodedQuery('u_cloud_offering=' + cloud + '^u_active=true');
//	gr.orderBy('u_group');
//	gr.orderBy('u_name');
//
// *** IT DOES NOT ORDER BY u_group_priority, AND THE MONITORS SCRIPT DOES. ***
// Two endpoints over the same table with different orderings is the sort of
// difference that looks like an oversight and is nonetheless what runs, so
// both are reproduced separately rather than sharing one query. Collapsing
// them would reorder the groups on one of the two pages.
const monitorsForHistorySQL = `
    SELECT LOWER(COALESCE(cm.region, '')),
           COALESCE(cm."group", ''),
           COALESCE(cm.name, ''),
           COALESCE(cm.service_offering_id::text, '')
      FROM cloud_monitor cm
     WHERE cm.cloud_offering = $1::cloud_monitor_cloud_offering_enum
       AND cm.is_active IS TRUE
     ORDER BY cm."group", cm.name
`

// MonitorsForHistory returns active monitors in the history endpoint's order.
func (r *cloudStatusDashboardRepository) MonitorsForHistory(ctx context.Context, cloud string) ([]MonitorRow, error) {
	rows, err := r.db.Query(ctx, monitorsForHistorySQL, cloud)
	if err != nil {
		return nil, fmt.Errorf("query monitors for history: %w", err)
	}
	defer rows.Close()

	out := make([]MonitorRow, 0)
	for rows.Next() {
		var m MonitorRow
		if err := rows.Scan(&m.Region, &m.Group, &m.Name, &m.ServiceOfferingID); err != nil {
			return nil, fmt.Errorf("scan monitor for history: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── /incident/{id} ─────────────────────────────────────────────────────

// IncidentDetailRow is one outage's detail view, with the state of the
// incident it links to.
type IncidentDetailRow struct {
	ID               string
	Begin            string
	End              string
	Type             string
	ShortDescription string
	// IncidentState is the linked incident's state, or "" when the outage
	// has no work item at all. The caller applies the qualifying gate.
	IncidentState string
}

// OutageCommentRow is one customer-facing update on an outage.
type OutageCommentRow struct {
	Comment   string
	CreatedOn string
}

// incidentDetailSQL reproduces incident.js's outage lookup:
//
//	'sys_id=' + sysId + '^cmdb_ci.ref_service_offering.parent.nameSTARTSWITH' + cloud
//
// plus the incident the payload is gated on:
//
//	incidentGr.addEncodedQuery('sys_id=' + task_number + '^stateNOT IN1,3,8')
//
// The join is LEFT because the two failure modes are different and the
// caller must tell them apart: no outage for this cloud is a 404, whereas an
// outage whose incident does not qualify is a 200 carrying attachments only.
// An inner join would collapse both into "not found" and turn 449 of 637
// outages into errors.
//
// The state gate itself is applied in the service rather than here, so the
// SQL answers "what is there" and the Go answers "what does that mean".
const incidentDetailSQL = `
    SELECT o.id::text,
           COALESCE(to_char(o.start_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), ''),
           COALESCE(to_char(o.end_on   AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), ''),
           COALESCE(o.type::text, ''),
           COALESCE(o.name, ''),
           COALESCE(i.state::text, '')
      FROM outage o
      JOIN service_offering so ON so.id = o.service_offering_id
      JOIN service s ON s.id = so.parent_id
      LEFT JOIN incident i ON i.id = o.work_item_id
     WHERE o.id = $1::uuid
       AND s.name ILIKE $2 || '%'
`

// IncidentDetail returns one outage for one cloud, or nil when there is none.
func (r *cloudStatusDashboardRepository) IncidentDetail(ctx context.Context, id, cloud string) (*IncidentDetailRow, error) {
	var d IncidentDetailRow
	err := r.db.QueryRow(ctx, incidentDetailSQL, id, cloud).Scan(
		&d.ID, &d.Begin, &d.End, &d.Type, &d.ShortDescription, &d.IncidentState)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query incident detail: %w", err)
	}
	return &d, nil
}

// outageCommentsSQL reproduces the journal read behind an incident's
// comments:
//
//	journalFieldGr.addQuery('element_id', <the OUTAGE's sys_id>);
//	journalFieldGr.addQuery('element', 'u_external_outage_communications');
//	journalFieldGr.orderByDesc('sys_created_on');
//
// *** NOT THE INCIDENT'S COMMENTS. *** The line reading element 'comments'
// is commented out in the ServiceNow source, and reinstating it here would
// publish internal incident commentary -- Postgres holds 923 such comments
// on outage-linked incidents, plus work notes and approval history -- on a
// public status page. The live API returns none of them.
const outageCommentsSQL = `
    SELECT oc.comment,
           COALESCE(to_char(oc.created_on AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'), '')
      FROM outage_communication oc
     WHERE oc.outage_id = $1::uuid
       -- *** EXTERNAL ONLY. *** The table also holds the outage API's
       -- internal and additional communications (migration 0184). This
       -- endpoint feeds the public status page, so anything else here would
       -- publish internal notes to customers.
       AND oc.channel = 'external'
     ORDER BY oc.created_on DESC, oc.id
`

// OutageComments returns one outage's customer-facing updates, newest first.
func (r *cloudStatusDashboardRepository) OutageComments(ctx context.Context, outageID string) ([]OutageCommentRow, error) {
	rows, err := r.db.Query(ctx, outageCommentsSQL, outageID)
	if err != nil {
		return nil, fmt.Errorf("query outage comments: %w", err)
	}
	defer rows.Close()

	out := make([]OutageCommentRow, 0)
	for rows.Next() {
		var c OutageCommentRow
		if err := rows.Scan(&c.Comment, &c.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan outage comment: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
