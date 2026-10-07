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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProjectTypeRow is one row of the project_type table (migration
// 0031_project_type_table), including the feature-entitlement columns
// migration 0130 added -- a transcription of ServiceNow's
// ProjectTypeFeatureManager.FEATURE_MATRIX. The has_* columns default FALSE
// and AcceptedSeverityValues/*ProductCategories default nil/empty for a type
// FEATURE_MATRIX itself has no entry for, so a caller needs no separate
// nil-check the way a second, joined table would have required.
type ProjectTypeRow struct {
	ID   string
	Name string

	HasServiceRequestWriteAccess   bool
	HasServiceRequestReadAccess    bool
	HasChangeRequestReadAccess     bool
	HasSraWriteAccess              bool
	HasSraReadAccess               bool
	HasEngagementsReadAccess       bool
	HasUpdatesReadAccess           bool
	HasDeploymentWriteAccess       bool
	HasDeploymentReadAccess        bool
	HasTimeLogsReadAccess          bool
	HasComponentAnalysisReadAccess bool
	HasUsageMetricsReadAccess      bool
	// AcceptedSeverityValues/*ProductCategories carry this schema's own enum
	// labels verbatim (case_severity_enum's "S0".."S4",
	// deployed_product_category_enum's "MS"/"PC"/"CL"/"PDP"/"PS") -- the
	// service layer translates them into whatever wire shape a caller
	// expects (see project_metadata_service.go's severityChoiceItems).
	AcceptedSeverityValues       []string
	DefaultCaseProductCategories []string
	SrProductCategories          []string
}

// ReferenceDataRepository backs the choice-list/reference-data reads shared
// by the project-metadata (GET /projects/{id}/metadata) and system-metadata
// (GET /metadata) endpoints: project_type rows and Postgres enum labels.
type ReferenceDataRepository interface {
	// ListProjectTypes returns every project_type row not explicitly marked
	// inactive, ordered by name.
	ListProjectTypes(ctx context.Context) ([]ProjectTypeRow, error)
	// GetProjectByID reports whether a project with this id exists, and its
	// linked project_type row (nil if the project has none set via
	// project.project_type_id).
	GetProjectByID(ctx context.Context, projectID string) (found bool, projectType *ProjectTypeRow, err error)
	// EnumLabels returns, for each of enumTypeNames, its ordered label list --
	// queried live from pg_catalog rather than hardcoded, so the result
	// always matches whatever the migrations currently define. A requested
	// type with no matching rows is simply absent from the returned map.
	EnumLabels(ctx context.Context, enumTypeNames []string) (map[string][]string, error)
	// ListTimeZones returns every row of the timezone reference table
	// (value, label), ordered by value. This table is not declared in this
	// repo's own migrations/ -- same "built outside this directory" class
	// as several other tables documented in CLAUDE.md's "Staging schema
	// drift" section -- so check the live schema before assuming its shape,
	// not this file.
	ListTimeZones(ctx context.Context) ([]TimeZoneRow, error)
	// ListSLADurationPolicy returns every row of sla_duration_policy
	// (migration 0192), ordered by severity then clock_type -- backs
	// GET /sla-duration-policy. Severity is already translated from the raw
	// case_severity_enum label ("S0") to the uppercase English word every
	// case.* event's own Priority field carries ("CATASTROPHIC"), via this
	// same package's caseSeverityFromEnum (case_repo.go) -- the one place
	// that mapping is defined, so this method stays the only repository
	// read anywhere that needs to apply it for this table.
	ListSLADurationPolicy(ctx context.Context) ([]SLADurationPolicyRow, error)
	// ListFeedbackEmojis returns the five case-feedback emoji choices (the
	// "<rating> - Reasons" rows of work_item_feedback_metric, migration
	// 0127), each with its own reason chips -- backs GET /metadata's
	// feedbackEmojies field. See case_feedback_repo.go's own doc comment
	// for the shared rating-scale design this and
	// GetCaseFeedback/CreateCaseFeedback both depend on.
	ListFeedbackEmojis(ctx context.Context) ([]FeedbackEmojiRow, error)
}

// FeedbackEmojiChipRow is one selectable reason chip under a feedback emoji
// (a work_item_feedback_metric_option row).
type FeedbackEmojiChipRow struct {
	ID    string
	Name  string
	Value string
}

// FeedbackEmojiRow is one of the five feedback-form emoji choices.
type FeedbackEmojiRow struct {
	ID              string
	Name            string
	Value           string
	UnselectedImage string
	SelectedImage   string
	Chips           []FeedbackEmojiChipRow
}

// SLADurationPolicyRow is one row of the sla_duration_policy table, already
// severity-translated -- see ListSLADurationPolicy's own doc comment.
// DurationSeconds is duration's whole-second EXTRACT(EPOCH FROM ...) -- an
// INTERVAL has no direct Go scan target in this connection's type map, same
// reasoning project_repo.go's own EXTRACT(EPOCH FROM ...) columns already
// document.
type SLADurationPolicyRow struct {
	Severity        string
	ClockType       string
	DurationSeconds int64
}

// TimeZoneRow is one row of the timezone reference table. utc_offset/dst
// exist on the table but have no slot in domain.ChoiceListItem (the
// {id, label} shape GET /metadata's own timeZones field has always used,
// matching the ServiceNow-backed response this replaces) -- left unread
// rather than widening that wire contract for data nothing consumes yet.
type TimeZoneRow struct {
	Value string
	Label string
}

type referenceDataRepo struct {
	db *pgxpool.Pool
}

// NewReferenceDataRepository constructs a ReferenceDataRepository backed by the given connection pool.
func NewReferenceDataRepository(db *pgxpool.Pool) ReferenceDataRepository {
	return &referenceDataRepo{db: db}
}

// ListProjectTypes implements ReferenceDataRepository.
func (r *referenceDataRepo) ListProjectTypes(ctx context.Context) ([]ProjectTypeRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name FROM project_type WHERE is_active IS DISTINCT FROM FALSE ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list project types: %w", err)
	}
	defer rows.Close()

	var out []ProjectTypeRow
	for rows.Next() {
		var pt ProjectTypeRow
		if err := rows.Scan(&pt.ID, &pt.Name); err != nil {
			return nil, fmt.Errorf("scan project type: %w", err)
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

// ListTimeZones implements ReferenceDataRepository.
func (r *referenceDataRepo) ListTimeZones(ctx context.Context) ([]TimeZoneRow, error) {
	rows, err := r.db.Query(ctx, `SELECT value, label FROM timezone ORDER BY value`)
	if err != nil {
		return nil, fmt.Errorf("list time zones: %w", err)
	}
	defer rows.Close()

	var out []TimeZoneRow
	for rows.Next() {
		var tz TimeZoneRow
		if err := rows.Scan(&tz.Value, &tz.Label); err != nil {
			return nil, fmt.Errorf("scan time zone: %w", err)
		}
		out = append(out, tz)
	}
	return out, rows.Err()
}

// ListSLADurationPolicy implements ReferenceDataRepository.
func (r *referenceDataRepo) ListSLADurationPolicy(ctx context.Context) ([]SLADurationPolicyRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT severity::TEXT, clock_type, EXTRACT(EPOCH FROM duration)::BIGINT
		 FROM sla_duration_policy ORDER BY severity, clock_type`)
	if err != nil {
		return nil, fmt.Errorf("list sla duration policy: %w", err)
	}
	defer rows.Close()

	var out []SLADurationPolicyRow
	for rows.Next() {
		var rawSeverity, clockType string
		var durationSeconds int64
		if err := rows.Scan(&rawSeverity, &clockType, &durationSeconds); err != nil {
			return nil, fmt.Errorf("scan sla duration policy: %w", err)
		}
		severity, ok := caseSeverityFromEnum[rawSeverity]
		if !ok {
			return nil, fmt.Errorf("list sla duration policy: unrecognized severity %q", rawSeverity)
		}
		out = append(out, SLADurationPolicyRow{
			Severity:        strings.ToUpper(string(severity)),
			ClockType:       clockType,
			DurationSeconds: durationSeconds,
		})
	}
	return out, rows.Err()
}

// ListFeedbackEmojis implements ReferenceDataRepository.
func (r *referenceDataRepo) ListFeedbackEmojis(ctx context.Context) ([]FeedbackEmojiRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::TEXT, name, selected_image, unselected_image
		FROM work_item_feedback_metric
		WHERE selected_image IS NOT NULL AND is_active
		ORDER BY display_order NULLS LAST, name`)
	if err != nil {
		return nil, fmt.Errorf("list feedback emojis: %w", err)
	}

	var emojis []FeedbackEmojiRow
	for rows.Next() {
		var id, name string
		var selectedImage, unselectedImage *string
		if err := rows.Scan(&id, &name, &selectedImage, &unselectedImage); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan feedback emoji: %w", err)
		}
		rating, label, ok := resolveCaseFeedbackRating(name)
		if !ok {
			// Not one of the five known "<rating> - Reasons" rows -- a
			// malformed or renamed row the fixed scale can't place. Skipped
			// rather than surfaced with a guessed label.
			continue
		}
		emojis = append(emojis, FeedbackEmojiRow{
			ID:              id,
			Name:            label,
			Value:           strconv.Itoa(rating),
			UnselectedImage: stringOrEmpty(unselectedImage),
			SelectedImage:   stringOrEmpty(selectedImage),
		})
	}
	closeErr := rows.Err()
	rows.Close()
	if closeErr != nil {
		return nil, fmt.Errorf("iterate feedback emojis: %w", closeErr)
	}

	for i := range emojis {
		chipRows, err := r.db.Query(ctx, `
			SELECT id::TEXT, label, value
			FROM work_item_feedback_metric_option
			WHERE metric_id = $1
			ORDER BY display_order NULLS LAST, label`,
			emojis[i].ID,
		)
		if err != nil {
			return nil, fmt.Errorf("list feedback emoji chips: %w", err)
		}
		var chips []FeedbackEmojiChipRow
		for chipRows.Next() {
			var chipID, label string
			var value int
			if err := chipRows.Scan(&chipID, &label, &value); err != nil {
				chipRows.Close()
				return nil, fmt.Errorf("scan feedback emoji chip: %w", err)
			}
			chips = append(chips, FeedbackEmojiChipRow{ID: chipID, Name: label, Value: strconv.Itoa(value)})
		}
		chipErr := chipRows.Err()
		chipRows.Close()
		if chipErr != nil {
			return nil, fmt.Errorf("iterate feedback emoji chips: %w", chipErr)
		}
		emojis[i].Chips = chips
	}

	return emojis, nil
}

// GetProjectByID implements ReferenceDataRepository.
func (r *referenceDataRepo) GetProjectByID(ctx context.Context, projectID string) (bool, *ProjectTypeRow, error) {
	var (
		ptID, ptName *string
		hasSRWrite, hasSRRead, hasCR, hasSraWrite, hasSraRead, hasEngagements, hasUpdates,
		hasDeployWrite, hasDeployRead, hasTimeLogs, hasComponentAnalysis, hasUsageMetrics *bool
		acceptedSeverities                  []string
		defaultCaseCategories, srCategories []string
	)
	// ::TEXT[] on the three enum-array columns: this connection's pgx type map
	// has no custom enum types registered, so scanning case_severity_enum[]/
	// deployed_product_category_enum[] directly into []string fails -- same
	// reason every other enum column in this repository package is selected
	// as ::TEXT (see e.g. case_repo.go's severity::TEXT) rather than its
	// native enum type.
	err := r.db.QueryRow(ctx,
		`SELECT
			pt.id, pt.name,
			pt.has_service_request_write_access, pt.has_service_request_read_access, pt.has_change_request_read_access,
			pt.has_sra_write_access, pt.has_sra_read_access, pt.has_engagements_read_access, pt.has_updates_read_access,
			pt.has_deployment_write_access, pt.has_deployment_read_access, pt.has_time_logs_read_access,
			pt.has_component_analysis_read_access, pt.has_usage_metrics_read_access,
			pt.accepted_severity_values::TEXT[], pt.default_case_product_categories::TEXT[], pt.sr_product_categories::TEXT[]
		 FROM project p
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 WHERE p.id = $1`, projectID,
	).Scan(
		&ptID, &ptName,
		&hasSRWrite, &hasSRRead, &hasCR,
		&hasSraWrite, &hasSraRead, &hasEngagements, &hasUpdates,
		&hasDeployWrite, &hasDeployRead, &hasTimeLogs,
		&hasComponentAnalysis, &hasUsageMetrics,
		&acceptedSeverities, &defaultCaseCategories, &srCategories,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("get project by id: %w", err)
	}
	if ptID == nil {
		return true, nil, nil
	}
	// The has_* columns are NOT NULL on project_type itself, but the LEFT
	// JOIN still produces NULL for all of them when a project has no
	// project_type_id at all (ptID == nil, handled above) -- deref helper
	// below covers the remaining case where LEFT JOIN found no matching
	// project_type row for some other reason.
	deref := func(b *bool) bool {
		if b == nil {
			return false
		}
		return *b
	}
	return true, &ProjectTypeRow{
		ID:   *ptID,
		Name: *ptName,

		HasServiceRequestWriteAccess:   deref(hasSRWrite),
		HasServiceRequestReadAccess:    deref(hasSRRead),
		HasChangeRequestReadAccess:     deref(hasCR),
		HasSraWriteAccess:              deref(hasSraWrite),
		HasSraReadAccess:               deref(hasSraRead),
		HasEngagementsReadAccess:       deref(hasEngagements),
		HasUpdatesReadAccess:           deref(hasUpdates),
		HasDeploymentWriteAccess:       deref(hasDeployWrite),
		HasDeploymentReadAccess:        deref(hasDeployRead),
		HasTimeLogsReadAccess:          deref(hasTimeLogs),
		HasComponentAnalysisReadAccess: deref(hasComponentAnalysis),
		HasUsageMetricsReadAccess:      deref(hasUsageMetrics),
		AcceptedSeverityValues:         acceptedSeverities,
		DefaultCaseProductCategories:   defaultCaseCategories,
		SrProductCategories:            srCategories,
	}, nil
}

// EnumLabels implements ReferenceDataRepository.
func (r *referenceDataRepo) EnumLabels(ctx context.Context, enumTypeNames []string) (map[string][]string, error) {
	// pg_type_is_visible(t.oid) scopes this to whichever single schema this
	// connection's search_path would actually resolve typname to -- the same
	// resolution an unqualified CREATE TYPE/enum reference in a migration
	// gets. This deployment's schema is neither a fixed literal (verified
	// live: it's "$user"-resolved, e.g. "csm_platform_stg_user" in staging,
	// not "public") nor discoverable from any config/compose/migration file
	// in this repo, so pg_type_is_visible is the only portable way to avoid
	// scanning every schema in the database -- without it, a same-named enum
	// type in another schema would silently merge its labels into this one's
	// map entry.
	rows, err := r.db.Query(ctx,
		`SELECT t.typname::text, e.enumlabel
		 FROM pg_type t
		 JOIN pg_enum e ON e.enumtypid = t.oid
		 WHERE pg_catalog.pg_type_is_visible(t.oid)
		   AND t.typname::text = ANY($1::text[])
		 ORDER BY t.typname, e.enumsortorder`, enumTypeNames)
	if err != nil {
		return nil, fmt.Errorf("enum labels: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]string, len(enumTypeNames))
	for rows.Next() {
		var typ, label string
		if err := rows.Scan(&typ, &label); err != nil {
			return nil, fmt.Errorf("scan enum label: %w", err)
		}
		out[typ] = append(out[typ], label)
	}
	return out, rows.Err()
}
