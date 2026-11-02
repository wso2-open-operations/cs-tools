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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// CatalogSet is one column assignment of a saved change. Column and Cast
// come from the service's fixed field table, never from the request, so they
// are safe to interpolate. Value is bound as a parameter.
type CatalogSet struct {
	Column string
	// Cast is the SQL expression applied to the bound parameter, with %s
	// standing for it, e.g. "%s::date" or "upper(%s)::product_unit_enum".
	Cast  string
	Value any
}

// CatalogApply is one change ready to apply: the service has validated the
// payload and turned it into column assignments.
type CatalogApply struct {
	EntityType string
	Operation  string
	// EntityID is empty for CREATE.
	EntityID string
	// Sets of a product version UPDATE apply to every live row of its public
	// listing (all its deployment profiles); RowSets to the one row only.
	Sets    []CatalogSet
	RowSets []CatalogSet
	// CarbonSet is true when the payload carried a carbon key; Carbon nil
	// then deletes the row.
	CarbonSet bool
	Carbon    *domain.CatalogCarbon
	// Profiles are the deployment profiles a product version CREATE adds,
	// one row each, sharing one public listing ("" is a row with no profile).
	Profiles []string
}

// ProductCatalogRepository writes the product catalogue and reads its
// reference tables (migrations 0269-0271). Products and versions
// are read through ProductRepository and ProductVersionRepository.
type ProductCatalogRepository interface {
	// ListReleaseLines and GetReleaseLine (nil when there is no such id).
	ListReleaseLines(ctx context.Context, includeRetired bool) ([]domain.ReleaseLine, error)
	GetReleaseLine(ctx context.Context, id string) (*domain.ReleaseLine, error)
	ListCarbonPlatformHistory(ctx context.Context) ([]domain.CarbonPlatformHistory, error)
	ListCarbonKernelReleases(ctx context.Context) ([]domain.CarbonKernelRelease, error)
	// StoredVersionDates returns a version's release and support EOL dates
	// ("YYYY-MM-DD" or nil); found is false when there is no such row.
	StoredVersionDates(ctx context.Context, id string) (release, eol *string, found bool, err error)

	// EntityExists reports whether the row a change targets exists.
	EntityExists(ctx context.Context, entityType, id string) (bool, error)
	// ApplyChange applies one validated change in one transaction and
	// returns the ids of the rows it wrote: the new row (or one per profile
	// for a version CREATE), the listing rows an UPDATE or RETIRE of a
	// version reached, or the one row otherwise.
	ApplyChange(ctx context.Context, actor string, apply CatalogApply) ([]string, error)
}

type productCatalogRepo struct {
	db *pgxpool.Pool
}

// NewProductCatalogRepository constructs a Postgres-backed ProductCatalogRepository.
func NewProductCatalogRepository(db *pgxpool.Pool) ProductCatalogRepository {
	return &productCatalogRepo{db: db}
}

const releaseLineSelect = `SELECT rl.id::text, rl.name, rl.start_date::timestamptz, rl.status, rl.legacy_id,
	(SELECT count(*) FROM product_version pv WHERE pv.release_line_id = rl.id AND pv.retired_on IS NULL),
	rl.retired_on, rl.created_on, rl.updated_on
	FROM release_line rl`

func scanReleaseLine(row pgx.Row) (domain.ReleaseLine, error) {
	var l domain.ReleaseLine
	err := row.Scan(&l.ID, &l.Name, &l.StartDate, &l.Status, &l.LegacyID, &l.VersionCount,
		&l.RetiredOn, &l.CreatedOn, &l.UpdatedOn)
	return l, err
}

func (r *productCatalogRepo) ListReleaseLines(ctx context.Context, includeRetired bool) ([]domain.ReleaseLine, error) {
	rows, err := r.db.Query(ctx, releaseLineSelect+`
		WHERE ($1 OR rl.retired_on IS NULL)
		ORDER BY lower(rl.name), rl.id`, includeRetired)
	if err != nil {
		return nil, fmt.Errorf("list release lines: %w", err)
	}
	defer rows.Close()
	out := []domain.ReleaseLine{}
	for rows.Next() {
		l, err := scanReleaseLine(rows)
		if err != nil {
			return nil, fmt.Errorf("list release lines: scan: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list release lines: %w", err)
	}
	return out, nil
}

func (r *productCatalogRepo) GetReleaseLine(ctx context.Context, id string) (*domain.ReleaseLine, error) {
	l, err := scanReleaseLine(r.db.QueryRow(ctx, releaseLineSelect+` WHERE rl.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get release line: %w", err)
	}
	return &l, nil
}

func (r *productCatalogRepo) StoredVersionDates(ctx context.Context, id string) (*string, *string, bool, error) {
	var release, eol *string
	err := r.db.QueryRow(ctx, `SELECT to_char(release_date, 'YYYY-MM-DD'), to_char(support_eol_date, 'YYYY-MM-DD')
		FROM product_version WHERE id = $1::uuid`, id).Scan(&release, &eol)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("stored version dates: %w", err)
	}
	return release, eol, true, nil
}

func (r *productCatalogRepo) ListCarbonPlatformHistory(ctx context.Context) ([]domain.CarbonPlatformHistory, error) {
	rows, err := r.db.Query(ctx, `SELECT id, carbon_version, to_char(release_date, 'YYYY-MM-DD'), release_date_text,
		esb_version, governance_registry_version, application_server_version, data_services_server_version,
		identity_server_version, business_process_server_version, mashup_server_version, gadget_server_version,
		business_activity_monitor_version, api_manager_version, business_rules_server_version, event_server_version,
		message_broker_version, complex_event_processor_version, elastic_load_balancer_version, storage_server_version,
		stratos_version, cloud_gateway_version, wsas_version, app_factory_version, user_engagement_server_version
		FROM carbon_platform_history ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list carbon platform history: %w", err)
	}
	defer rows.Close()
	out := []domain.CarbonPlatformHistory{}
	for rows.Next() {
		var h domain.CarbonPlatformHistory
		if err := rows.Scan(&h.ID, &h.CarbonVersion, &h.ReleaseDate, &h.ReleaseDateText,
			&h.ESBVersion, &h.GovernanceRegistryVersion, &h.ApplicationServerVersion, &h.DataServicesServerVersion,
			&h.IdentityServerVersion, &h.BusinessProcessServerVersion, &h.MashupServerVersion, &h.GadgetServerVersion,
			&h.BusinessActivityMonitorVersion, &h.APIManagerVersion, &h.BusinessRulesServerVersion, &h.EventServerVersion,
			&h.MessageBrokerVersion, &h.ComplexEventProcessorVersion, &h.ElasticLoadBalancerVersion, &h.StorageServerVersion,
			&h.StratosVersion, &h.CloudGatewayVersion, &h.WSASVersion, &h.AppFactoryVersion, &h.UserEngagementServerVersion); err != nil {
			return nil, fmt.Errorf("list carbon platform history: scan: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list carbon platform history: %w", err)
	}
	return out, nil
}

func (r *productCatalogRepo) ListCarbonKernelReleases(ctx context.Context) ([]domain.CarbonKernelRelease, error) {
	rows, err := r.db.Query(ctx, `SELECT id, kernel_release, to_char(release_date, 'YYYY-MM-DD'), carbon_platform
		FROM carbon_kernel_release ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list carbon kernel releases: %w", err)
	}
	defer rows.Close()
	out := []domain.CarbonKernelRelease{}
	for rows.Next() {
		var k domain.CarbonKernelRelease
		if err := rows.Scan(&k.ID, &k.KernelRelease, &k.ReleaseDate, &k.CarbonPlatform); err != nil {
			return nil, fmt.Errorf("list carbon kernel releases: scan: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list carbon kernel releases: %w", err)
	}
	return out, nil
}

// Catalogue write operations.
const (
	CatalogOpCreate = "CREATE"
	CatalogOpUpdate = "UPDATE"
	CatalogOpRetire = "RETIRE"
)

// catalogTables maps a change's entity type to its live table. The values are
// constants, safe to interpolate.
var catalogTables = map[string]string{
	domain.CatalogEntityProduct:        "product",
	domain.CatalogEntityProductVersion: "product_version",
	domain.CatalogEntityReleaseLine:    "release_line",
}

func (r *productCatalogRepo) EntityExists(ctx context.Context, entityType, id string) (bool, error) {
	table, ok := catalogTables[entityType]
	if !ok {
		return false, fmt.Errorf("unknown catalog entity type %q", entityType)
	}
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("catalog entity exists: %w", err)
	}
	return exists, nil
}

func (r *productCatalogRepo) ApplyChange(ctx context.Context, actor string, a CatalogApply) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("apply catalog change: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ids, publicIDs, err := applyCatalogChange(ctx, tx, actor, a)
	if err != nil {
		return nil, changeError(a, err)
	}
	for _, publicID := range publicIDs {
		if err := checkListingAgrees(ctx, tx, publicID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("apply catalog change: commit: %w", err)
	}
	return ids, nil
}

// checkListingAgrees refuses a change that leaves the live rows of one public
// listing disagreeing on a field the feeds publish once per listing.
func checkListingAgrees(ctx context.Context, tx pgx.Tx, publicID int64) error {
	var split bool
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT (current_support_status, release_date, earliest_possible_support_eol_date,
			support_eol_date, support_eol_announcement_date, public_name, public_abbreviation,
			is_public, show_in_support_matrix)) > 1
		FROM product_version WHERE public_id = $1 AND retired_on IS NULL`, publicID).Scan(&split); err != nil {
		return fmt.Errorf("apply catalog change: check listing: %w", err)
	}
	if split {
		return &apierror.ValidationError{Msg: fmt.Sprintf(
			"the deployment profiles of public listing %d would disagree on status, dates, public name or feed flags", publicID)}
	}
	return nil
}

// changeError turns a database constraint failure into a readable 400.
func changeError(a CatalogApply, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		noun := entityNoun(a.EntityType)
		switch pgErr.Code {
		case "23505":
			return &apierror.ValidationError{Msg: fmt.Sprintf("this %s would duplicate an existing one: %s", noun, pgErr.Detail)}
		case "23503":
			return &apierror.ValidationError{Msg: fmt.Sprintf("this %s refers to a row that does not exist: %s", noun, pgErr.Detail)}
		case "23502", "23514", "22P02", "22007", "22008", "22001":
			return &apierror.ValidationError{Msg: fmt.Sprintf("invalid value for this %s: %s", noun, pgErr.Message)}
		}
	}
	var nf *apierror.NotFoundError
	var ve *apierror.ValidationError
	if errors.As(err, &nf) || errors.As(err, &ve) {
		return err
	}
	return fmt.Errorf("apply catalog change: %w", err)
}

func entityNoun(entityType string) string {
	return strings.ReplaceAll(entityType, "_", " ")
}

// applyCatalogChange writes one change. It returns the ids of the rows it
// wrote and, for product versions, the public listings to check.
func applyCatalogChange(ctx context.Context, tx pgx.Tx, actor string, a CatalogApply) ([]string, []int64, error) {
	table, ok := catalogTables[a.EntityType]
	if !ok {
		return nil, nil, fmt.Errorf("unknown catalog entity type %q", a.EntityType)
	}
	isVersion := a.EntityType == domain.CatalogEntityProductVersion

	var ids []string
	switch a.Operation {
	case CatalogOpCreate:
		if !isVersion {
			id, err := insertCatalogRow(ctx, tx, table, actor, a.Sets, "")
			if err != nil {
				return nil, nil, err
			}
			return []string{id}, nil, nil
		}
		// The first profile takes the listing's public id (the one joined, or
		// the next in the sequence); the others reuse it.
		rowSets := append([]CatalogSet{}, a.RowSets...)
		publicIDExpr := "nextval('product_version_public_id_seq')"
		for _, s := range rowSets {
			if s.Column == "public_id" {
				publicIDExpr = ""
			}
		}
		for i, profile := range a.Profiles {
			var p *string
			if profile != "" {
				p = &profile
			}
			sets := append(append(append([]CatalogSet{}, a.Sets...), rowSets...),
				CatalogSet{Column: "deployment_profile", Cast: "%s", Value: p})
			id, err := insertCatalogRow(ctx, tx, table, actor, sets, publicIDExpr)
			if err != nil {
				return nil, nil, err
			}
			if i == 0 && publicIDExpr != "" {
				var publicID int64
				if err := tx.QueryRow(ctx, `SELECT public_id FROM product_version WHERE id = $1::uuid`, id).Scan(&publicID); err != nil {
					return nil, nil, err
				}
				rowSets = append(rowSets, CatalogSet{Column: "public_id", Cast: "%s::bigint", Value: &publicID})
				publicIDExpr = ""
			}
			// Serial number as the sync writes it: <abbreviation>-<version>.
			if _, err := tx.Exec(ctx, `UPDATE product_version pv
				SET serial_number = COALESCE(pv.public_abbreviation, p.code) || '-' || pv.version
				FROM product p
				WHERE pv.id = $1::uuid AND p.id = pv.product_id AND pv.serial_number IS NULL
				  AND COALESCE(pv.public_abbreviation, p.code) IS NOT NULL`, id); err != nil {
				return nil, nil, err
			}
			ids = append(ids, id)
		}
		if a.CarbonSet {
			for _, id := range ids {
				if err := setCatalogCarbon(ctx, tx, id, actor, a.Carbon); err != nil {
					return nil, nil, err
				}
			}
		}
	case CatalogOpUpdate:
		target := "id = $1::uuid"
		if isVersion {
			// A product version's listing-level fields move with every live
			// row of its public listing.
			target = versionListingTarget
		}
		var err error
		if ids, err = catalogRowIDs(ctx, tx, table, target, a.EntityID); err != nil {
			return nil, nil, err
		}
		if len(ids) == 0 {
			return nil, nil, &apierror.NotFoundError{Msg: fmt.Sprintf("this %s no longer exists", entityNoun(a.EntityType))}
		}
		if err := updateCatalogRows(ctx, tx, table, target, a.EntityID, actor, a.Sets); err != nil {
			return nil, nil, err
		}
		if isVersion {
			if err := updateCatalogRows(ctx, tx, table, "id = $1::uuid", a.EntityID, actor, a.RowSets); err != nil {
				return nil, nil, err
			}
			if a.CarbonSet {
				// Carbon details belong to the version, so they go to every
				// live row of the listing, like the other listing fields.
				for _, id := range ids {
					if err := setCatalogCarbon(ctx, tx, id, actor, a.Carbon); err != nil {
						return nil, nil, err
					}
				}
			}
		}
	case CatalogOpRetire:
		// Retiring a version retires its whole listing (every live profile).
		target := "id = $1::uuid"
		if isVersion {
			target = versionListingTarget
		}
		rows, err := tx.Query(ctx, fmt.Sprintf(`UPDATE %s SET retired_on = NOW(), retired_by = $2, updated_on = NOW(), updated_by = $2
			WHERE %s AND retired_on IS NULL RETURNING id::text`, table, target), a.EntityID, actor)
		if err != nil {
			return nil, nil, err
		}
		if ids, err = collectIDs(rows); err != nil {
			return nil, nil, err
		}
		if len(ids) == 0 {
			return nil, nil, &apierror.ValidationError{Msg: fmt.Sprintf("this %s does not exist or is already retired", entityNoun(a.EntityType))}
		}
		return ids, nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown catalog operation %q", a.Operation)
	}

	if !isVersion {
		return ids, nil, nil
	}
	var publicIDs []int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(DISTINCT public_id) FILTER (WHERE public_id IS NOT NULL), '{}')
		FROM product_version WHERE id = ANY($1::uuid[])`, ids).Scan(&publicIDs); err != nil {
		return nil, nil, err
	}
	return ids, publicIDs, nil
}

// versionListingTarget selects a version row ($1) and the other live rows of
// its public listing.
const versionListingTarget = `(id = $1::uuid OR (public_id IS NOT NULL AND retired_on IS NULL
	AND public_id = (SELECT public_id FROM product_version WHERE id = $1::uuid)))`

// insertCatalogRow inserts one row from sets. publicIDExpr, when set, is the
// constant SQL expression that fills public_id.
func insertCatalogRow(ctx context.Context, tx pgx.Tx, table, actor string, sets []CatalogSet, publicIDExpr string) (string, error) {
	cols := []string{"created_on", "created_by", "updated_on", "updated_by"}
	vals := []string{"NOW()", "$1", "NOW()", "$1"}
	args := []any{actor}
	for _, s := range sets {
		args = append(args, s.Value)
		cols = append(cols, s.Column)
		vals = append(vals, fmt.Sprintf(s.Cast, fmt.Sprintf("$%d", len(args))))
	}
	if publicIDExpr != "" {
		cols = append(cols, "public_id")
		vals = append(vals, publicIDExpr)
	}
	var id string
	q := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING id::text`, table, strings.Join(cols, ", "), strings.Join(vals, ", "))
	err := tx.QueryRow(ctx, q, args...).Scan(&id)
	return id, err
}

// catalogRowIDs returns the ids of the rows target selects ($1 is the id).
func catalogRowIDs(ctx context.Context, tx pgx.Tx, table, target, id string) ([]string, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id::text FROM %s WHERE %s ORDER BY id`, table, target), id)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

func collectIDs(rows pgx.Rows) ([]string, error) {
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// updateCatalogRows applies sets to the rows target selects ($1 is the id).
func updateCatalogRows(ctx context.Context, tx pgx.Tx, table, target, id, actor string, sets []CatalogSet) error {
	if len(sets) == 0 {
		return nil
	}
	assign := []string{"updated_on = NOW()", "updated_by = $2"}
	args := []any{id, actor}
	for _, s := range sets {
		args = append(args, s.Value)
		assign = append(assign, fmt.Sprintf("%s = %s", s.Column, fmt.Sprintf(s.Cast, fmt.Sprintf("$%d", len(args)))))
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET %s WHERE %s`, table, strings.Join(assign, ", "), target), args...)
	return err
}

// setCatalogCarbon writes (carbon non-nil) or deletes one version's Carbon row.
func setCatalogCarbon(ctx context.Context, tx pgx.Tx, id, actor string, carbon *domain.CatalogCarbon) error {
	if carbon == nil {
		_, err := tx.Exec(ctx, `DELETE FROM product_version_carbon WHERE product_version_id = $1::uuid`, id)
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO product_version_carbon
		(product_version_id, platform_release, platform_version, carbon_kernel_version, release_chunk, kernel_patch_no, created_by, updated_by)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (product_version_id) DO UPDATE SET
			platform_release = EXCLUDED.platform_release, platform_version = EXCLUDED.platform_version,
			carbon_kernel_version = EXCLUDED.carbon_kernel_version, release_chunk = EXCLUDED.release_chunk,
			kernel_patch_no = EXCLUDED.kernel_patch_no, updated_on = NOW(), updated_by = EXCLUDED.updated_by`,
		id, carbon.PlatformRelease, carbon.PlatformVersion, carbon.CarbonKernelVersion,
		carbon.ReleaseChunk, carbon.KernelPatchNo, actor)
	return err
}
