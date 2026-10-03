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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OutageRepository is the Postgres backing for the outage entity API.
//
// *** THIS MIRRORS THE SERVICENOW CONTRACT, IT DOES NOT REDESIGN IT. ***
// Every shape it returns is domain.Outage as the ServiceNow-backed service
// already returns it, because the portal is already built against that shape
// and the point of this implementation is that the same UI keeps working once
// DATA_SOURCE stops being "servicenow".
//
// Three values the ServiceNow side computes upstream are computed here
// instead, and they are the only places this can silently diverge:
//
//   - status: derived from end_on, never stored (domain.OutageStatus says so)
//   - duration: derived from begin/end rather than read, since rows written
//     before Create/Update kept the column in step may still hold NULL. The
//     column itself is now written on create and on every begin/end change,
//     as ServiceNow's "Outage Calculations" rule does, for readers that use it
//   - publishesToStatusPage / statusPageCloud: resolved from whether the
//     outage's service offering has a cloud_monitor row, which is the same
//     join the cloud status sweep uses to decide which dashboard to tell
type OutageRepository interface {
	Create(ctx context.Context, in OutageWrite) (domain.Outage, error)
	Search(ctx context.Context, req domain.SearchOutagesRequest, beginFrom time.Time) ([]domain.Outage, int, error)
	GetByID(ctx context.Context, id string) (domain.OutageDetail, error)
	Update(ctx context.Context, patch OutagePatch) (domain.Outage, error)
	// PublicationFor reports whether an offering resolves to a monitored
	// cloud, so the service can apply the acknowledgement gate before writing.
	PublicationFor(ctx context.Context, serviceOfferingID *string) (bool, *string, error)
	AddCommunication(ctx context.Context, outageID string, channel domain.OutageCommunicationChannel, body, actor string) (domain.OutageCommunication, error)
	SearchCommunications(ctx context.Context, req domain.SearchOutageCommunicationsRequest) ([]domain.OutageCommunication, int, error)
	// MonitoredClouds lists the cloud slugs any outage can publish to, for
	// the create form's warning.
	MonitoredClouds(ctx context.Context) ([]string, error)
}

// OutageWrite carries the resolved inputs for a create.
type OutageWrite struct {
	Type                  string
	Begin                 time.Time
	End                   *time.Time
	ShortDescription      string
	ServiceOfferingID     *string
	IncidentID            *string
	ExternalCommunication *string
	InternalCommunication *string
	Actor                 string
}

// OutagePatch carries an update whose timestamps are ALREADY PARSED.
//
// *** THE REPOSITORY DOES NOT PARSE TIME, AND THAT IS DELIBERATE. *** It used
// to, with time.Parse(RFC3339) only, while the service accepted both RFC3339
// and the space-separated "YYYY-MM-DD HH:mm:ss" the portal actually sends.
// The two disagreed, so closing an outage from the portal failed with
// "invalid end" while creating one worked. Parsing in exactly one place is
// the fix; a second parser is the bug.
//
// End is a pointer-to-pointer for the same three states PatchOutageRequest
// documents: nil leaves end alone, non-nil-outer with nil-inner REOPENS, and
// a value closes.
type OutagePatch struct {
	ID                string
	Type              *string
	Begin             *time.Time
	End               **time.Time
	ShortDescription  *string
	ServiceOfferingID *string
	IncidentID        *string
	Actor             string
}

// outageRepo runs every statement under the caller's own identity, never a
// system one: outageSelect LEFT JOINs work_item (RLS-protected) for the linked
// incident's number and subject, so an internal caller gets them and a
// customer, who cannot see that work_item, gets them blank.
type outageRepo struct {
	db *Scoped
}

// NewOutageRepository constructs the repository over the scoped pool.
func NewOutageRepository(db *Scoped) OutageRepository {
	return &outageRepo{db: db}
}

// outageSelect is the shared projection. The joins are all LEFT: an outage
// with no offering, no monitor and no incident is ordinary, not an error.
//
// cloud_monitor decides publication. One monitor per offering is the
// invariant in the data (146 offerings, 146 monitors), and DISTINCT guards
// the case where that stops being true rather than multiplying rows.
const outageSelect = `
SELECT o.id::text,
       COALESCE(o.number, ''),
       o.type::text,
       o.start_on,
       o.end_on,
       COALESCE(o.name, ''),
       so.id::text,
       COALESCE(so.name, ''),
       cm.cloud_offering::text,
       wi.id::text,
       COALESCE(wi.number, ''),
       COALESCE(wi.subject, ''),
       inc.state::text,
       o.created_on,
       COALESCE(o.created_by, ''),
       o.updated_on,
       COALESCE(o.updated_by, '')
  FROM outage o
  LEFT JOIN service_offering so ON so.id = o.service_offering_id
  LEFT JOIN LATERAL (
       SELECT DISTINCT m.cloud_offering
         FROM cloud_monitor m
        WHERE m.service_offering_id = o.service_offering_id
        LIMIT 1
  ) cm ON TRUE
  -- work_item carries number and subject; incident extends it and carries
  -- state. outage.work_item_id addresses both, and neither join alone is
  -- enough: incident has no number column at all.
  LEFT JOIN work_item wi ON wi.id = o.work_item_id
  LEFT JOIN incident inc ON inc.id = o.work_item_id
`

// scanOutage reads one projected row and derives status and duration.
func scanOutage(row pgx.Row) (domain.Outage, error) {
	var (
		out                                  domain.Outage
		typ, offeringID, offeringName, cloud *string
		incID, incNumber, incShort, incState *string
		begin                                *time.Time
		end                                  *time.Time
		createdOn, updatedOn                 time.Time
	)
	if err := row.Scan(&out.ID, &out.Number, &typ, &begin, &end, &out.ShortDescription,
		&offeringID, &offeringName, &cloud,
		&incID, &incNumber, &incShort, &incState,
		&createdOn, &out.CreatedBy, &updatedOn, &out.UpdatedBy); err != nil {
		return domain.Outage{}, err
	}

	if typ != nil {
		lowered := strings.ToLower(*typ)
		out.Type = &lowered
	}
	if begin != nil {
		out.Begin = begin.UTC().Format(time.RFC3339)
	}
	if end != nil {
		s := end.UTC().Format(time.RFC3339)
		out.End = &s
	}

	// Status is derived, never stored -- see domain.OutageStatus.
	status := string(domain.OutageStatusInProgress)
	if end != nil {
		status = string(domain.OutageStatusResolved)
	}
	out.Status = &status

	if begin != nil && end != nil {
		d := formatOutageInterval(end.Sub(*begin))
		out.Duration = &d
	}

	if offeringID != nil {
		name := ""
		if offeringName != nil {
			name = *offeringName
		}
		out.ConfigurationItem = &domain.OutageConfigurationItemRef{
			ID: *offeringID, Name: name, ClassName: "service_offering",
		}
	}
	if incID != nil {
		ref := domain.OutageIncidentRef{ID: *incID, State: incState}
		if incNumber != nil {
			ref.Number = *incNumber
		}
		if incShort != nil {
			ref.ShortDescription = *incShort
		}
		out.Incident = &ref
	}

	// A monitor for the offering IS the publish switch, the same join the
	// cloud status sweep uses to pick a dashboard.
	if cloud != nil && *cloud != "" {
		slug := domain.CloudOfferingSlug(*cloud)
		if slug != "" {
			out.PublishesToStatusPage = true
			out.StatusPageCloud = &slug
		}
	}

	out.AffectedConfigurationItems = []domain.OutageConfigurationItemRef{}
	out.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	out.UpdatedOn = updatedOn.UTC().Format(time.RFC3339)
	return out, nil
}

// formatOutageInterval renders a duration the way a person reads one.
//
// NOT the raw interval. Casting a Postgres interval to text produced
// "00:31:25.634362" in a real outage email once, microseconds and all, and
// nothing caught it until the message arrived.
func formatOutageInterval(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	days := int64(d / (24 * time.Hour))
	hours := int64(d/time.Hour) % 24
	mins := int64(d/time.Minute) % 60
	secs := int64(d/time.Second) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if secs > 0 && days == 0 && hours == 0 {
		parts = append(parts, fmt.Sprintf("%ds", secs))
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// Create inserts a natively-created outage and returns it in the same shape a
// read does.
//
// The number comes from outage_number_seq, seeded at 10000 so OUT0010000 sits
// above ServiceNow's current OUT0001887 and cannot collide with numbers
// ServiceNow keeps allocating while dual-write is on.
func (r *outageRepo) Create(ctx context.Context, in OutageWrite) (domain.Outage, error) {
	// *** ONE TRANSACTION, BECAUSE A PARTIAL CREATE IS WHAT ACTUALLY
	// HAPPENED. *** On the first live run the insert succeeded and the
	// read-back failed on a bad column, leaving an outage row behind with no
	// caller aware of it -- and an orphan outage on an in-scope offering is
	// not inert: the next cloud status sweep posts a real outage_begin for it
	// to a shared dashboard. The seeded journal entries belong in the same
	// transaction for the same reason.
	id, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		return insertOutage(ctx, tx, in)
	})
	if err != nil {
		return domain.Outage{}, err
	}

	detail, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.Outage{}, err
	}
	return detail.Outage, nil
}

// insertOutage is Create's transactional body: the outage row and its seeded
// journal entries, run inside tx. It returns the new outage's id.
func insertOutage(ctx context.Context, tx pgx.Tx, in OutageWrite) (string, error) {
	const insertSQL = `
INSERT INTO outage (id, number, type, start_on, end_on, name,
                    service_offering_id, work_item_id,
                    external_outage_communications, internal_outage_communications,
                    notify_internal_stakeholders, duration,
                    created_on, created_by, updated_on, updated_by)
VALUES (gen_random_uuid(),
        'OUT' || LPAD(nextval('outage_number_seq')::text, 7, '0'),
        $1::outage_type_enum, $2, $3, $4,
        $5::uuid, $6::uuid, $7, $8, FALSE,
        -- ServiceNow's "Outage Calculations" business rule: duration is
        -- end - begin, and NULL while either is missing. Readers such as the
        -- outage-communication email take it from this column, so an outage
        -- created here must carry it like a synced one does.
        $3::timestamptz - $2::timestamptz,
        NOW(), $9, NOW(), $9)
RETURNING id::text`

	var id string
	if err := tx.QueryRow(ctx, insertSQL,
		strings.ToUpper(in.Type), in.Begin, in.End, in.ShortDescription,
		in.ServiceOfferingID, in.IncidentID,
		in.ExternalCommunication, in.InternalCommunication, in.Actor,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("create outage: %w", err)
	}

	// A seeded communication is a journal entry too, not only a column on the
	// outage -- the detail page reads the journal, so writing only the column
	// would make the text invisible where people look for it.
	if in.ExternalCommunication != nil && strings.TrimSpace(*in.ExternalCommunication) != "" {
		if _, err := insertCommunication(ctx, tx, id, domain.OutageCommunicationChannelExternal, *in.ExternalCommunication, in.Actor); err != nil {
			return "", err
		}
	}
	if in.InternalCommunication != nil && strings.TrimSpace(*in.InternalCommunication) != "" {
		if _, err := insertCommunication(ctx, tx, id, domain.OutageCommunicationChannelInternal, *in.InternalCommunication, in.Actor); err != nil {
			return "", err
		}
	}
	return id, nil
}

// GetByID returns one outage plus its per-channel journal counts.
func (r *outageRepo) GetByID(ctx context.Context, id string) (domain.OutageDetail, error) {
	out, err := scanOutage(r.db.QueryRow(ctx, outageSelect+" WHERE o.id = $1::uuid", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OutageDetail{}, &apierror.NotFoundError{Msg: "outage not found"}
		}
		return domain.OutageDetail{}, fmt.Errorf("get outage: %w", err)
	}

	detail := domain.OutageDetail{Outage: out}
	const countSQL = `
SELECT channel, COUNT(*) FROM outage_communication
 WHERE outage_id = $1::uuid GROUP BY channel`
	rows, err := r.db.Query(ctx, countSQL, id)
	if err != nil {
		return domain.OutageDetail{}, fmt.Errorf("outage communication counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var channel string
		var n int
		if err := rows.Scan(&channel, &n); err != nil {
			return domain.OutageDetail{}, fmt.Errorf("scan communication count: %w", err)
		}
		switch domain.OutageCommunicationChannel(channel) {
		case domain.OutageCommunicationChannelExternal:
			detail.CommunicationCounts.External = n
		case domain.OutageCommunicationChannelInternal:
			detail.CommunicationCounts.Internal = n
		case domain.OutageCommunicationChannelAdditional:
			detail.CommunicationCounts.Additional = n
		}
	}
	return detail, rows.Err()
}

// outageSortColumns maps the API sort fields to real columns. A map rather
// than string interpolation: the field arrives from the request body, and
// building ORDER BY from it directly is how an injection gets in.
var outageSortColumns = map[domain.OutageSortField]string{
	domain.OutageSortFieldBegin:     "o.start_on",
	domain.OutageSortFieldEnd:       "o.end_on",
	domain.OutageSortFieldNumber:    "o.number",
	domain.OutageSortFieldCreatedOn: "o.created_on",
	domain.OutageSortFieldUpdatedOn: "o.updated_on",
}

// Search applies the filters and returns one page plus the unpaged total.
func (r *outageRepo) Search(ctx context.Context, req domain.SearchOutagesRequest, beginFrom time.Time) ([]domain.Outage, int, error) {
	var where []string
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	add("o.start_on >= $%d", beginFrom)
	f := req.Filters
	if f.BeginTo != nil && *f.BeginTo != "" {
		t, err := time.Parse(time.RFC3339, *f.BeginTo)
		if err != nil {
			return nil, 0, &apierror.ValidationError{Msg: "invalid beginTo"}
		}
		add("o.start_on <= $%d", t)
	}
	if len(f.Types) > 0 {
		upper := make([]string, 0, len(f.Types))
		for _, t := range f.Types {
			upper = append(upper, strings.ToUpper(string(t)))
		}
		add("o.type::text = ANY($%d)", upper)
	}
	if len(f.ConfigurationItemIDs) > 0 {
		add("o.service_offering_id::text = ANY($%d)", f.ConfigurationItemIDs)
	}
	if len(f.IncidentIDs) > 0 {
		add("o.work_item_id::text = ANY($%d)", f.IncidentIDs)
	}
	// Status is derived, so it filters on end_on rather than on a column.
	if len(f.Statuses) == 1 {
		switch f.Statuses[0] {
		case domain.OutageStatusInProgress:
			where = append(where, "o.end_on IS NULL")
		case domain.OutageStatusResolved:
			where = append(where, "o.end_on IS NOT NULL")
		}
	}
	if f.PublishedOnly != nil && *f.PublishedOnly {
		where = append(where, "EXISTS (SELECT 1 FROM cloud_monitor m WHERE m.service_offering_id = o.service_offering_id)")
	}
	if strings.TrimSpace(f.SearchTerm) != "" {
		add("(o.number ILIKE $%d OR o.name ILIKE $%d)", "%"+strings.TrimSpace(f.SearchTerm)+"%")
		// The helper only substitutes one placeholder index; rewrite the last
		// clause so both columns use the same argument.
		where[len(where)-1] = fmt.Sprintf("(o.number ILIKE $%d OR o.name ILIKE $%d)", len(args), len(args))
	}

	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM outage o"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count outages: %w", err)
	}

	col, ok := outageSortColumns[req.SortBy.Field]
	if !ok {
		col = "o.start_on"
	}
	dir := "DESC"
	if req.SortBy.Order == domain.OutageSortOrderAsc {
		dir = "ASC"
	}

	limit, offset := req.Pagination.Limit, req.Pagination.Offset
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit, offset)
	q := fmt.Sprintf("%s%s ORDER BY %s %s NULLS LAST, o.id LIMIT $%d OFFSET $%d",
		outageSelect, clause, col, dir, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search outages: %w", err)
	}
	defer rows.Close()

	outages := []domain.Outage{}
	for rows.Next() {
		o, err := scanOutage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan outage: %w", err)
		}
		outages = append(outages, o)
	}
	return outages, total, rows.Err()
}

// Update applies a partial change. Closing an outage is setting End; there is
// no separate state column, exactly as the ServiceNow contract documents.
func (r *outageRepo) Update(ctx context.Context, patch OutagePatch) (domain.Outage, error) {
	var sets []string
	var args []any
	set := func(clause string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf(clause, len(args)))
	}

	if patch.Type != nil {
		set("type = $%d::outage_type_enum", strings.ToUpper(*patch.Type))
	}
	// beginExpr/endExpr are the values start_on/end_on will hold AFTER this
	// update. In an UPDATE's SET list a column name reads the OLD value, so
	// recomputing duration from the columns would use the pre-patch times.
	beginExpr, endExpr := "start_on", "end_on"
	if patch.Begin != nil {
		set("start_on = $%d", *patch.Begin)
		beginExpr = fmt.Sprintf("$%d::timestamptz", len(args))
	}
	// End is pointer-to-pointer on purpose: omitted leaves it alone, explicit
	// null REOPENS the outage, a value closes it. Collapsing those two is how
	// a reopen silently becomes a no-op.
	if patch.End != nil {
		if *patch.End == nil {
			sets = append(sets, "end_on = NULL")
			endExpr = "NULL::timestamptz"
		} else {
			set("end_on = $%d", **patch.End)
			endExpr = fmt.Sprintf("$%d::timestamptz", len(args))
		}
	}
	// ServiceNow's "Outage Calculations" rule (before insert/update): duration
	// = end - begin, NULL if either is missing. Without it an outage closed
	// here keeps a NULL duration and the resolution email prints a blank
	// "Outage Duration:".
	if patch.Begin != nil || patch.End != nil {
		sets = append(sets, fmt.Sprintf("duration = %s - %s", endExpr, beginExpr))
	}
	if patch.ShortDescription != nil {
		set("name = $%d", *patch.ShortDescription)
	}
	if patch.ServiceOfferingID != nil {
		set("service_offering_id = $%d::uuid", *patch.ServiceOfferingID)
	}
	if patch.IncidentID != nil {
		set("work_item_id = $%d::uuid", *patch.IncidentID)
	}

	if len(sets) == 0 {
		return domain.Outage{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	args = append(args, patch.Actor)
	sets = append(sets, fmt.Sprintf("updated_by = $%d", len(args)))
	sets = append(sets, "updated_on = NOW()")

	args = append(args, patch.ID)
	q := fmt.Sprintf("UPDATE outage SET %s WHERE id = $%d::uuid RETURNING id::text",
		strings.Join(sets, ", "), len(args))

	var id string
	if err := r.db.QueryRow(ctx, q, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Outage{}, &apierror.NotFoundError{Msg: "outage not found"}
		}
		return domain.Outage{}, fmt.Errorf("update outage: %w", err)
	}

	detail, err := r.GetByID(ctx, id)
	if err != nil {
		return domain.Outage{}, err
	}
	return detail.Outage, nil
}

// PublicationFor reports whether an offering publishes to a status page, so
// the service can demand acknowledgement BEFORE writing rather than after.
func (r *outageRepo) PublicationFor(ctx context.Context, serviceOfferingID *string) (bool, *string, error) {
	if serviceOfferingID == nil || *serviceOfferingID == "" {
		return false, nil, nil
	}
	const q = `
SELECT m.cloud_offering::text FROM cloud_monitor m
 WHERE m.service_offering_id = $1::uuid LIMIT 1`
	var cloud string
	if err := r.db.QueryRow(ctx, q, *serviceOfferingID).Scan(&cloud); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("resolve publication: %w", err)
	}
	slug := domain.CloudOfferingSlug(cloud)
	if slug == "" {
		return false, nil, nil
	}
	return true, &slug, nil
}

// rowQuerier is the sliver of Scoped and pgx.Tx this file shares, so a
// journal insert reads the same whether it is standalone or inside a create.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// insertCommunication appends one journal entry on whichever handle it is given.
func insertCommunication(ctx context.Context, q rowQuerier, outageID string,
	channel domain.OutageCommunicationChannel, body, actor string) (domain.OutageCommunication, error) {
	const stmt = `
INSERT INTO outage_communication (id, outage_id, channel, comment, created_on, created_by)
VALUES (gen_random_uuid(), $1::uuid, $2, $3, NOW(), $4)
RETURNING id::text, created_on`

	var out domain.OutageCommunication
	var createdOn time.Time
	if err := q.QueryRow(ctx, stmt, outageID, string(channel), body, actor).
		Scan(&out.ID, &createdOn); err != nil {
		return domain.OutageCommunication{}, fmt.Errorf("add outage communication: %w", err)
	}
	out.Channel = channel
	out.Body = body
	out.IsPublic = channel == domain.OutageCommunicationChannelExternal
	out.CreatedOn = createdOn.UTC().Format(time.RFC3339)
	out.CreatedBy = actor
	return out, nil
}

// AddCommunication appends one journal entry.
func (r *outageRepo) AddCommunication(ctx context.Context, outageID string,
	channel domain.OutageCommunicationChannel, body, actor string) (domain.OutageCommunication, error) {
	return insertCommunication(ctx, r.db, outageID, channel, body, actor)
}

// SearchCommunications returns one page of an outage's journal, newest first.
func (r *outageRepo) SearchCommunications(ctx context.Context,
	req domain.SearchOutageCommunicationsRequest) ([]domain.OutageCommunication, int, error) {
	args := []any{req.OutageID}
	clause := "WHERE outage_id = $1::uuid"
	if len(req.Channels) > 0 {
		channels := make([]string, 0, len(req.Channels))
		for _, c := range req.Channels {
			channels = append(channels, string(c))
		}
		args = append(args, channels)
		clause += fmt.Sprintf(" AND channel = ANY($%d)", len(args))
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM outage_communication "+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count outage communications: %w", err)
	}

	limit, offset := req.Pagination.Limit, req.Pagination.Offset
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit, offset)
	q := fmt.Sprintf(`
SELECT id::text, channel, comment, created_on, COALESCE(created_by, '')
  FROM outage_communication %s
 ORDER BY created_on DESC, id
 LIMIT $%d OFFSET $%d`, clause, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search outage communications: %w", err)
	}
	defer rows.Close()

	items := []domain.OutageCommunication{}
	for rows.Next() {
		var c domain.OutageCommunication
		var channel string
		var createdOn time.Time
		if err := rows.Scan(&c.ID, &channel, &c.Body, &createdOn, &c.CreatedBy); err != nil {
			return nil, 0, fmt.Errorf("scan outage communication: %w", err)
		}
		c.Channel = domain.OutageCommunicationChannel(channel)
		c.IsPublic = c.Channel == domain.OutageCommunicationChannelExternal
		c.CreatedOn = createdOn.UTC().Format(time.RFC3339)
		items = append(items, c)
	}
	return items, total, rows.Err()
}

// MonitoredClouds lists every cloud an outage could become visible on.
func (r *outageRepo) MonitoredClouds(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT cloud_offering::text FROM cloud_monitor ORDER BY 1`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("monitored clouds: %w", err)
	}
	defer rows.Close()

	clouds := []string{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan cloud offering: %w", err)
		}
		if slug := domain.CloudOfferingSlug(raw); slug != "" {
			clouds = append(clouds, slug)
		}
	}
	return clouds, rows.Err()
}
