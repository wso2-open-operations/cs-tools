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
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// SalesforceStoredRow is one stored row the admin resync compares with
// Salesforce: the Salesforce id it is keyed by and the stored value of each
// column Salesforce owns, as text (dates as YYYY-MM-DD, numbers and booleans
// in Postgres' text form), nil for NULL. Columns the CSM owns are never
// selected, so they cannot appear in a comparison.
type SalesforceStoredRow struct {
	// Key is the Salesforce id the row is matched by (sf_id, link_sf_id or
	// line_item_sf_id).
	Key  string
	Cols map[string]*string
}

// SalesforceResyncTarget is the account or project a resync runs for.
type SalesforceResyncTarget struct {
	SfID string
	Name *string
}

// SalesforceResyncReadRepository reads the stored Salesforce-owned columns of
// the rows the Salesforce ingest writes, for the admin resync's comparison.
// It only ever runs SELECTs, as the system (the ingest's own convention: the
// duplicate-sf_id ranking reads RLS-protected tables).
//
// sf_id is not unique (migration 0095), so each query returns every copy,
// ranked as the ingest ranks them (the copy with the most kinds of child
// first); callers keep the first row per Key.
type SalesforceResyncReadRepository interface {
	// TargetAccount and TargetProject resolve a CSM id to the Salesforce id
	// and name of the account or project. A NotFoundError when no row has the
	// id; a ValidationError when the row has no Salesforce id.
	TargetAccount(ctx context.Context, id string) (SalesforceResyncTarget, error)
	TargetProject(ctx context.Context, id string) (SalesforceResyncTarget, error)

	// Accounts returns the accounts carrying one of sfIDs, plus the accounts
	// with no sf_id whose number is one of them (the row the ingest links).
	Accounts(ctx context.Context, sfIDs []string) ([]SalesforceStoredRow, error)
	// Partners returns the partner accounts stored for the account (Key is
	// the partner's sf_id; the only column is name).
	Partners(ctx context.Context, accountSfID string) ([]SalesforceStoredRow, error)
	// Projects returns the projects carrying one of sfIDs, the projects with
	// one of keys (the ingest also matches by key), and, when accountSfID is
	// set, every project of that account.
	Projects(ctx context.Context, sfIDs, keys []string, accountSfID string) ([]SalesforceStoredRow, error)
	// Opportunities returns the opportunities carrying one of sfIDs, those of
	// accountSfID, and those linked to projectSfID (either may be empty).
	Opportunities(ctx context.Context, sfIDs []string, accountSfID, projectSfID string) ([]SalesforceStoredRow, error)
	// LineItems returns the line items stored under the opportunities
	// carrying one of oppSfIDs; each row's opportunity_sf_id column names its
	// opportunity.
	LineItems(ctx context.Context, oppSfIDs []string) ([]SalesforceStoredRow, error)
	// Links returns the project-opportunity links carrying one of linkSfIDs
	// or belonging to one of projectSfIDs.
	Links(ctx context.Context, linkSfIDs, projectSfIDs []string) ([]SalesforceStoredRow, error)
	// Invoices returns the invoices carrying one of sfIDs or belonging to one
	// of oppSfIDs.
	Invoices(ctx context.Context, sfIDs, oppSfIDs []string) ([]SalesforceStoredRow, error)
	// ContactUsers returns the "user" rows carrying one of sfIDs or one of
	// emails (lower case): the ingest resolves a contact's user by sf_id,
	// then by email. Key is the user's sf_id (empty when none).
	ContactUsers(ctx context.Context, sfIDs, emails []string) ([]SalesforceStoredRow, error)
	// AccountContacts returns the account_contact rows carrying one of sfIDs
	// and, when accountSfID is set, the active ones of that account.
	AccountContacts(ctx context.Context, sfIDs []string, accountSfID string) ([]SalesforceStoredRow, error)
	// Memberships returns the project_contact rows carrying one of sfIDs or
	// belonging to one of projectSfIDs.
	Memberships(ctx context.Context, sfIDs, projectSfIDs []string) ([]SalesforceStoredRow, error)
	// MembershipGroups returns the project group names stored for each
	// membership, keyed by the membership's sf_id.
	MembershipGroups(ctx context.Context, membershipSfIDs []string) (map[string][]string, error)
}

type salesforceResyncReadRepo struct {
	db *Scoped
}

// NewSalesforceResyncReadRepository constructs a SalesforceResyncReadRepository backed by the scoped pool.
func NewSalesforceResyncReadRepository(db *Scoped) SalesforceResyncReadRepository {
	return &salesforceResyncReadRepo{db: db}
}

// resyncDate renders a DATE or TIMESTAMPTZ column as YYYY-MM-DD text.
func resyncDate(expr string) string { return "to_char(" + expr + ", 'YYYY-MM-DD')" }

// resyncColumns is one SELECT list: the Key expression first, then
// "expr AS name" for every column. Names are the column names the service
// builds its field comparison from.
type resyncColumns struct {
	key  string
	cols [][2]string // {name, SQL expression yielding text}
}

func (c resyncColumns) selectList() string {
	parts := []string{c.key + " AS resync_key"}
	for _, col := range c.cols {
		parts = append(parts, col[1]+" AS "+col[0])
	}
	return strings.Join(parts, ", ")
}

func resyncCols(key string, cols ...[2]string) resyncColumns {
	return resyncColumns{key: key, cols: cols}
}

func rc(name, expr string) [2]string { return [2]string{name, expr} }

var (
	resyncAccountCols = resyncCols("COALESCE(a.sf_id, a.number)",
		rc("name", "a.name"), rc("industry", "a.industry"), rc("region", "a.region"), rc("global_pod", "a.global_pod"),
		rc("phone", "a.phone"), rc("sales_region", "a.sales_region"), rc("sub_region", "a.sub_region"),
		rc("life_cycle", "a.life_cycle"), rc("naics_industry", "a.naics_industry"), rc("sub_industry", "a.sub_industry"),
		rc("classification", "a.classification"), rc("street", "a.street"), rc("city", "a.city"),
		rc("state_province", "a.state_province"), rc("postal_code", "a.postal_code"), rc("country", "a.country"),
		rc("lost_reason", "a.lost_reason"), rc("account_vertical", "a.account_vertical"),
		rc("lost_reason_category", "a.lost_reason_category"),
		rc("activation_date", resyncDate("a.activation_date")), rc("lost_date", resyncDate("a.lost_date")),
		rc("deactivation_date", resyncDate("a.deactivation_date")), rc("deleted_on", resyncDate("a.deleted_on")))

	resyncProjectCols = resyncCols("p.sf_id",
		rc("key", "p.key"), rc("name", "p.name"), rc("description", "p.description"),
		rc("start_date", resyncDate("p.start_date")), rc("end_date", resyncDate("p.end_date")),
		rc("compliance_violation_date", resyncDate("p.compliance_violation_date")),
		rc("onboarding_go_live_date", resyncDate("p.onboarding_go_live_date")),
		rc("account_sf_id", "pa.sf_id"), rc("project_type", "pt.name"))

	resyncOpportunityCols = resyncCols("o.sf_id",
		rc("name", "o.name"), rc("stage", "o.stage"), rc("is_won", "o.is_won::text"),
		rc("close_date", resyncDate("o.close_date")), rc("eula_version", "o.eula_version"),
		rc("eula_version_decimal", "o.eula_version_decimal::text"), rc("account_sf_id", "oa.sf_id"))

	resyncLineItemCols = resyncCols("li.line_item_sf_id",
		rc("opportunity_sf_id", "o.sf_id"), rc("name", "li.name"), rc("product_name", "li.product_name"),
		rc("quantity", "li.quantity::text"), rc("service_start_date", resyncDate("li.service_start_date")),
		rc("service_end_date", resyncDate("li.service_end_date")), rc("product_code", "li.product_code"),
		rc("product_description", "li.product_description"), rc("product_family", "li.product_family"),
		rc("product_unit", "li.product_unit"), rc("eng_product_code", "li.eng_product_code"),
		rc("product_sf_id", "li.product_sf_id"), rc("classification", "li.classification"),
		rc("environment", "li.environment"), rc("total_price", "li.total_price::text"))

	resyncLinkCols = resyncCols("l.link_sf_id",
		rc("number", "l.number"), rc("opportunity_sf_id", "o.sf_id"), rc("project_sf_id", "p.sf_id"))

	resyncInvoiceCols = resyncCols("i.sf_id",
		rc("name", "i.name"), rc("description", "i.description"), rc("classification", "i.classification"),
		rc("opportunity_sf_id", "o.sf_id"), rc("invoiced_amount", "i.invoiced_amount::text"),
		rc("invoice_date", resyncDate("i.invoice_date")), rc("invoiced_due_date", resyncDate("i.invoiced_due_date")),
		rc("original_invoice_due_date", resyncDate("i.original_invoice_due_date")),
		rc("invoiced_paid_date", resyncDate("i.invoiced_paid_date")),
		rc("service_start_date", resyncDate("i.service_start_date")), rc("service_end_date", resyncDate("i.service_end_date")))

	resyncUserCols = resyncCols("u.sf_id",
		rc("email", "lower(u.email)"), rc("name", "u.name"), rc("first_name", "u.first_name"),
		rc("last_name", "u.last_name"), rc("is_system_user", "u.is_system_user::text"))

	resyncAccountContactCols = resyncCols("ac.sf_id",
		rc("account_sf_id", "a.sf_id"), rc("is_primary_contact", "ac.is_primary_contact::text"),
		rc("is_active", "ac.is_active::text"), rc("user_name", "ac.user_name"))

	resyncMembershipCols = resyncCols("pc.sf_id",
		rc("email", "lower(pc.email)"), rc("state", "pc.state::text"),
		rc("project_sf_id", "p.sf_id"), rc("contact_sf_id", "ac.sf_id"))
)

// The queries. Each SELECT lists the Salesforce-owned columns of resyncCols
// above and is read-only (TestSalesforceResyncSQL_ReadOnly).
var (
	resyncAccountsQuery = `SELECT ` + resyncAccountCols.selectList() + `
	FROM account a
	WHERE a.sf_id = ANY($1::text[]) OR (a.sf_id IS NULL AND a.number = ANY($1::text[]))
	ORDER BY ` + accountReferencedOrder

	resyncProjectsQuery = `SELECT ` + resyncProjectCols.selectList() + `
	FROM project p
	LEFT JOIN account pa ON pa.id = p.account_id
	LEFT JOIN project_type pt ON pt.id = p.project_type_id
	WHERE p.sf_id = ANY($1::text[]) OR p.key = ANY($2::text[]) OR ($3::text <> '' AND pa.sf_id = $3::text)
	ORDER BY ` + projectReferencedOrder

	resyncOpportunitiesQuery = `SELECT ` + resyncOpportunityCols.selectList() + `
	FROM sf_opportunity o
	LEFT JOIN account oa ON oa.id = o.account_id
	WHERE o.sf_id = ANY($1::text[])
	   OR ($2::text <> '' AND oa.sf_id = $2::text)
	   OR ($3::text <> '' AND o.id IN (
			SELECT l.opportunity_id FROM sf_opportunity_link l JOIN project lp ON lp.id = l.project_id WHERE lp.sf_id = $3::text))
	ORDER BY ` + opportunityReferencedOrder

	resyncLineItemsQuery = `SELECT ` + resyncLineItemCols.selectList() + `
	FROM sf_opportunity_product li
	JOIN sf_opportunity o ON o.id = li.opportunity_id
	WHERE o.sf_id = ANY($1::text[]) AND li.line_item_sf_id IS NOT NULL
	ORDER BY li.created_on, li.id`

	resyncLinksQuery = `SELECT ` + resyncLinkCols.selectList() + `
	FROM sf_opportunity_link l
	LEFT JOIN sf_opportunity o ON o.id = l.opportunity_id
	LEFT JOIN project p ON p.id = l.project_id
	WHERE l.link_sf_id = ANY($1::text[]) OR p.sf_id = ANY($2::text[])
	ORDER BY l.created_on, l.id`

	resyncInvoicesQuery = `SELECT ` + resyncInvoiceCols.selectList() + `
	FROM sf_invoice i
	LEFT JOIN sf_opportunity o ON o.id = i.opportunity_id
	WHERE i.sf_id = ANY($1::text[]) OR o.sf_id = ANY($2::text[])
	ORDER BY i.created_on, i.id`

	resyncUsersQuery = `SELECT ` + resyncUserCols.selectList() + `
	FROM "user" u
	WHERE u.sf_id = ANY($1::text[]) OR lower(u.email) = ANY($2::text[])
	ORDER BY u.is_active IS TRUE DESC, u.created_on, u.id`

	resyncAccountContactsQuery = `SELECT ` + resyncAccountContactCols.selectList() + `
	FROM account_contact ac
	LEFT JOIN account a ON a.id = ac.account_id
	WHERE ac.sf_id = ANY($1::text[]) OR ($2::text <> '' AND a.sf_id = $2::text AND ac.is_active IS TRUE AND ac.sf_id IS NOT NULL)
	ORDER BY ac.is_active IS TRUE DESC, ac.created_on, ac.id`

	resyncMembershipsQuery = `SELECT ` + resyncMembershipCols.selectList() + `
	FROM project_contact pc
	JOIN project p ON p.id = pc.project_id
	LEFT JOIN account_contact ac ON ac.id = pc.account_contact_id
	WHERE pc.sf_id = ANY($1::text[]) OR (pc.sf_id IS NOT NULL AND p.sf_id = ANY($2::text[]))
	ORDER BY EXISTS (SELECT 1 FROM project_contact_group g WHERE g.project_contact_id = pc.id) DESC, pc.created_on, pc.id`

	resyncMembershipGroupsQuery = `SELECT pc.sf_id, pg."group"
	FROM project_contact_group g
	JOIN project_contact pc ON pc.id = g.project_contact_id
	JOIN project_group pg ON pg.id = g.project_group_id
	WHERE pc.sf_id = ANY($1::text[])`

	resyncPartnersQuery = `SELECT pa.sf_id AS resync_key, pa.name AS name
	FROM account_relationship ar
	JOIN account pa ON pa.id = ar.from_account_id
	JOIN account ca ON ca.id = ar.to_account_id
	WHERE ca.sf_id = $1::text AND ar.relationship_label = $2::text AND ar.is_reverse_relationship = false AND pa.sf_id IS NOT NULL
	ORDER BY pa.sf_id`

	resyncTargetAccountQuery = `SELECT sf_id, name FROM account WHERE id = $1::uuid`
	resyncTargetProjectQuery = `SELECT sf_id, COALESCE(name, key) FROM project WHERE id = $1::uuid`
)

// storedRows runs a resync SELECT and scans its columns (resync_key first)
// into SalesforceStoredRow values, in the order the query returned them.
func (r *salesforceResyncReadRepo) storedRows(ctx context.Context, what, sql string, args ...any) ([]SalesforceStoredRow, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("resync read %s: %w", what, err)
	}
	defer rows.Close()
	names := rows.FieldDescriptions()
	var out []SalesforceStoredRow
	for rows.Next() {
		vals := make([]*string, len(names))
		dest := make([]any, len(names))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("resync read %s: scan: %w", what, err)
		}
		row := SalesforceStoredRow{Cols: make(map[string]*string, len(names)-1)}
		for i, fd := range names {
			if i == 0 {
				if vals[0] != nil {
					row.Key = *vals[0]
				}
				continue
			}
			row.Cols[fd.Name] = vals[i]
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resync read %s: %w", what, err)
	}
	return out, nil
}

func (r *salesforceResyncReadRepo) target(ctx context.Context, sql, id, what string) (SalesforceResyncTarget, error) {
	var sfID, name *string
	err := r.db.QueryRow(WithSystemIdentity(ctx), sql, id).Scan(&sfID, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return SalesforceResyncTarget{}, &apierror.NotFoundError{Msg: what + " not found"}
	}
	if err != nil {
		return SalesforceResyncTarget{}, fmt.Errorf("resync read %s target: %w", what, err)
	}
	if sfID == nil || strings.TrimSpace(*sfID) == "" {
		return SalesforceResyncTarget{}, &apierror.ValidationError{Msg: what + " has no Salesforce id, so there is nothing to compare"}
	}
	return SalesforceResyncTarget{SfID: strings.TrimSpace(*sfID), Name: name}, nil
}

func (r *salesforceResyncReadRepo) TargetAccount(ctx context.Context, id string) (SalesforceResyncTarget, error) {
	return r.target(ctx, resyncTargetAccountQuery, id, "account")
}

func (r *salesforceResyncReadRepo) TargetProject(ctx context.Context, id string) (SalesforceResyncTarget, error) {
	return r.target(ctx, resyncTargetProjectQuery, id, "project")
}

func (r *salesforceResyncReadRepo) Accounts(ctx context.Context, sfIDs []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "accounts", resyncAccountsQuery, nonNil(sfIDs))
}

func (r *salesforceResyncReadRepo) Partners(ctx context.Context, accountSfID string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "partners", resyncPartnersQuery, accountSfID, relationshipLabelPartnerOf)
}

func (r *salesforceResyncReadRepo) Projects(ctx context.Context, sfIDs, keys []string, accountSfID string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "projects", resyncProjectsQuery, nonNil(sfIDs), nonNil(keys), accountSfID)
}

func (r *salesforceResyncReadRepo) Opportunities(ctx context.Context, sfIDs []string, accountSfID, projectSfID string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "opportunities", resyncOpportunitiesQuery, nonNil(sfIDs), accountSfID, projectSfID)
}

func (r *salesforceResyncReadRepo) LineItems(ctx context.Context, oppSfIDs []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "line items", resyncLineItemsQuery, nonNil(oppSfIDs))
}

func (r *salesforceResyncReadRepo) Links(ctx context.Context, linkSfIDs, projectSfIDs []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "links", resyncLinksQuery, nonNil(linkSfIDs), nonNil(projectSfIDs))
}

func (r *salesforceResyncReadRepo) Invoices(ctx context.Context, sfIDs, oppSfIDs []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "invoices", resyncInvoicesQuery, nonNil(sfIDs), nonNil(oppSfIDs))
}

func (r *salesforceResyncReadRepo) ContactUsers(ctx context.Context, sfIDs, emails []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "contact users", resyncUsersQuery, nonNil(sfIDs), nonNil(emails))
}

func (r *salesforceResyncReadRepo) AccountContacts(ctx context.Context, sfIDs []string, accountSfID string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "account contacts", resyncAccountContactsQuery, nonNil(sfIDs), accountSfID)
}

func (r *salesforceResyncReadRepo) Memberships(ctx context.Context, sfIDs, projectSfIDs []string) ([]SalesforceStoredRow, error) {
	return r.storedRows(ctx, "memberships", resyncMembershipsQuery, nonNil(sfIDs), nonNil(projectSfIDs))
}

func (r *salesforceResyncReadRepo) MembershipGroups(ctx context.Context, membershipSfIDs []string) (map[string][]string, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, resyncMembershipGroupsQuery, nonNil(membershipSfIDs))
	if err != nil {
		return nil, fmt.Errorf("resync read membership groups: %w", err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var sfID, group string
		if err := rows.Scan(&sfID, &group); err != nil {
			return nil, fmt.Errorf("resync read membership groups: scan: %w", err)
		}
		out[sfID] = append(out[sfID], group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resync read membership groups: %w", err)
	}
	return out, nil
}
