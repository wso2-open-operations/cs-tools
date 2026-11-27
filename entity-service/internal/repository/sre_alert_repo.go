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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SreAlertRepository reads the alert table for the portal; alert is internal only, so a caller that is not internal sees nothing.
type SreAlertRepository interface {
	Search(ctx context.Context, filters domain.SreAlertSearchFilters, sort domain.SreAlertSort, offset, limit int) ([]domain.SreAlertListItem, int, error)
	// Get finds an alert by its UUID or its ALT number; nil when there is none.
	Get(ctx context.Context, idOrNumber string) (*domain.SreAlertDetail, error)
	// Facets lists the distinct values received since, at most limit of each.
	Facets(ctx context.Context, since time.Time, limit int) (domain.SreAlertFacets, error)
}

// sreAlertSortColumns maps domain.SreAlertSortFields to their columns.
var sreAlertSortColumns = map[string]string{
	"receivedOn":  "a.received_on",
	"number":      "a.number",
	"severity":    "a.severity",
	"environment": "a.environment",
	"service":     "a.service",
}

type sreAlertRepository struct {
	db *Scoped
}

// NewSreAlertRepository returns the portal's read-only view of the alert table.
func NewSreAlertRepository(db *Scoped) SreAlertRepository {
	return &sreAlertRepository{db: db}
}

// sreAlertListColumns are the columns of a list item, then its incident's (NULL when the alert has none); scanSreAlert reads them.
const sreAlertListColumns = `a.id::text, a.number, coalesce(a.source, ''), coalesce(a.service, ''), coalesce(a.environment, ''),
coalesce(a.metric_name, ''), a.severity, a.category, a.tier, a.received_on, a.created_on,
i.id::text, w.number, w.subject, i.state::text, i.priority::text, i.impact::text, i.urgency::text, i.opened_on`

// sreAlertFrom joins each alert to its incident (alert.incident_id) and the incident's work item, which holds its number and subject.
const sreAlertFrom = ` FROM alert a LEFT JOIN incident i ON i.id = a.incident_id LEFT JOIN work_item w ON w.id = a.incident_id`

// sreAlertIncidentCols receives the incident columns, every one nullable.
type sreAlertIncidentCols struct {
	id, number, subject, state, priority, impact, urgency *string
	openedOn                                              *time.Time
}

func (c *sreAlertIncidentCols) dests() []any {
	return []any{&c.id, &c.number, &c.subject, &c.state, &c.priority, &c.impact, &c.urgency, &c.openedOn}
}

// ref is the incident, or nil when the alert belongs to none.
func (c *sreAlertIncidentCols) ref() *domain.SreAlertIncidentRef {
	if c.id == nil {
		return nil
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return &domain.SreAlertIncidentRef{ID: *c.id, Number: deref(c.number), Subject: deref(c.subject), State: deref(c.state),
		Priority: c.priority, Impact: deref(c.impact), Urgency: deref(c.urgency), OpenedOn: c.openedOn}
}

func sreAlertListDests(item *domain.SreAlertListItem, inc *sreAlertIncidentCols) []any {
	return append([]any{&item.ID, &item.Number, &item.Source, &item.Service, &item.Environment,
		&item.MetricName, &item.Severity, &item.Category, &item.Tier, &item.ReceivedOn, &item.CreatedOn}, inc.dests()...)
}

func (r *sreAlertRepository) Search(ctx context.Context, f domain.SreAlertSearchFilters, sort domain.SreAlertSort, offset, limit int) ([]domain.SreAlertListItem, int, error) {
	where, args := sreAlertWhere(f)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+sreAlertFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}
	column, ok := sreAlertSortColumns[sort.Field]
	if !ok {
		column = sreAlertSortColumns["receivedOn"]
	}
	order := "DESC"
	if sort.Order == "asc" {
		order = "ASC"
	}
	// number breaks ties, so a page boundary never repeats or skips an alert.
	query := `SELECT ` + sreAlertListColumns + sreAlertFrom + where +
		` ORDER BY ` + column + ` ` + order + ` NULLS LAST, a.number ` + order +
		` OFFSET $` + strconv.Itoa(len(args)+1) + ` LIMIT $` + strconv.Itoa(len(args)+2)
	rows, err := r.db.Query(ctx, query, append(args, offset, limit)...)
	if err != nil {
		return nil, 0, fmt.Errorf("search alerts: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SreAlertListItem, error) {
		var item domain.SreAlertListItem
		var inc sreAlertIncidentCols
		if err := row.Scan(sreAlertListDests(&item, &inc)...); err != nil {
			return item, err
		}
		item.Incident = inc.ref()
		return item, nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("search alerts: %w", err)
	}
	return items, total, nil
}

// sreAlertWhere builds the WHERE clause and its arguments; every value is a parameter.
func sreAlertWhere(f domain.SreAlertSearchFilters) (string, []any) {
	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	if q := strings.TrimSpace(f.SearchQuery); q != "" {
		p := arg("%" + escapeLike(q) + "%")
		conds = append(conds, "(a.number ILIKE "+p+" OR a.service ILIKE "+p+" OR a.metric_name ILIKE "+p+
			" OR a.description ILIKE "+p+" OR a.unique_identifier ILIKE "+p+")")
	}
	anyOf := func(column string, values []string) {
		if len(values) > 0 {
			conds = append(conds, column+" = ANY("+arg(values)+")")
		}
	}
	anyOf("a.environment", f.Environments)
	anyOf("a.service", f.Services)
	anyOf("a.severity", f.Severities)
	anyOf("a.source", f.Sources)
	if len(f.Tiers) > 0 {
		tiers := make([]string, len(f.Tiers))
		for i, t := range f.Tiers {
			tiers[i] = string(t)
		}
		anyOf("a.tier", tiers)
	}
	if id := strings.TrimSpace(f.IncidentID); id != "" {
		conds = append(conds, "a.incident_id = "+arg(id)+"::uuid")
	}
	switch f.Incident {
	case domain.SreAlertIncidentLinked:
		conds = append(conds, "a.incident_id IS NOT NULL")
	case domain.SreAlertIncidentUnlinked:
		conds = append(conds, "a.incident_id IS NULL")
	}
	if f.ReceivedFrom != nil {
		conds = append(conds, "a.received_on >= "+arg(*f.ReceivedFrom))
	}
	if f.ReceivedTo != nil {
		conds = append(conds, "a.received_on < "+arg(*f.ReceivedTo))
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// escapeLike makes % and _ in a search match themselves.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *sreAlertRepository) Get(ctx context.Context, idOrNumber string) (*domain.SreAlertDetail, error) {
	// A UUID is matched on id and anything else on number, so a malformed UUID never reaches a uuid cast.
	match := "a.number = $1"
	if isUUID(idOrNumber) {
		match = "a.id = $1::uuid"
	}
	var d domain.SreAlertDetail
	var inc sreAlertIncidentCols
	var raw []byte
	dests := append(sreAlertListDests(&d.SreAlertListItem, &inc), &d.UniqueIdentifier, &d.Description, &d.AssignmentGroup,
		&raw)
	err := r.db.QueryRow(ctx, `SELECT `+sreAlertListColumns+`, coalesce(a.unique_identifier, ''), a.description,
a.assignment_group, a.raw_body`+
		sreAlertFrom+` WHERE `+match, idOrNumber).Scan(dests...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get alert %s: %w", idOrNumber, err)
	}
	d.Incident = inc.ref()
	if raw != nil {
		// raw_body is bytea; invalid UTF-8 is replaced so the body always travels as JSON text.
		text := strings.ToValidUTF8(string(raw), "�")
		d.RawBody = &text
	}
	return &d, nil
}

// isUUID reports whether s has the 8-4-4-4-12 hex shape of a UUID.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

func (r *sreAlertRepository) Facets(ctx context.Context, since time.Time, limit int) (domain.SreAlertFacets, error) {
	facets := domain.SreAlertFacets{}
	for _, f := range []struct {
		column string
		into   *[]string
	}{
		{"environment", &facets.Environments},
		{"service", &facets.Services},
		{"severity", &facets.Severities},
		{"source", &facets.Sources},
	} {
		// The column names are fixed above, never from the request.
		rows, err := r.db.Query(ctx, `SELECT DISTINCT `+f.column+` FROM alert
WHERE received_on >= $1 AND coalesce(`+f.column+`, '') <> '' ORDER BY 1 LIMIT $2`, since, limit)
		if err != nil {
			return domain.SreAlertFacets{}, fmt.Errorf("alert %s values: %w", f.column, err)
		}
		values, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return domain.SreAlertFacets{}, fmt.Errorf("alert %s values: %w", f.column, err)
		}
		*f.into = append([]string{}, values...)
	}
	return facets, nil
}
