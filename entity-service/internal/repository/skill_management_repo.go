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
	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SkillManagementRepository is the skill management revamp's part of the
// skills repository: the category admin, skill requests and their approval,
// and the people directory.
type SkillManagementRepository interface {
	// DefaultLevelTypeID is the scale marked as the default (migration 0283),
	// or the only scale when none is marked.
	DefaultLevelTypeID(ctx context.Context) (string, bool, error)
	// CategoriesAreActiveLeaves is true when every id is an active category
	// with no child category: the only place a skill is filed.
	CategoriesAreActiveLeaves(ctx context.Context, ids []string) (bool, error)
	CreateCategory(ctx context.Context, req domain.CreateSkillCategoryRequest, actor string) (*domain.SkillCategory, domain.SkillOutcome, error)
	UpdateCategory(ctx context.Context, id string, req domain.UpdateSkillCategoryRequest, actor string) (*domain.SkillCategory, domain.SkillOutcome, error)

	CheckSkillRequest(ctx context.Context, normalizedName string) (domain.CheckSkillRequestResponse, error)
	CreateSkillRequest(ctx context.Context, userID, normalizedName string, req domain.CreateSkillRequestRequest, actor string) (*domain.SkillRequest, domain.SkillOutcome, error)
	GetSkillRequest(ctx context.Context, id string) (*domain.SkillRequest, error)
	// ListSkillRequests is newest first; userID narrows it to one requester.
	ListSkillRequests(ctx context.Context, userID string, statuses []domain.SkillRequestStatus, limit, offset int) ([]domain.SkillRequest, int, error)
	CountPendingSkillRequests(ctx context.Context) (int, error)
	WithdrawSkillRequest(ctx context.Context, id, userID, actor string) (*domain.SkillRequest, domain.SkillOutcome, error)
	RejectSkillRequest(ctx context.Context, id, reviewerID, reason, actor string) (*domain.SkillRequest, domain.SkillOutcome, error)
	ApproveSkillRequest(ctx context.Context, a domain.SkillApproval) (*domain.SkillRequest, domain.SkillOutcome, error)

	SearchSkillDirectory(ctx context.Context, req domain.SearchSkillDirectoryRequest) ([]domain.SkillDirectoryPerson, int, error)
	// GetUserSkillProfile is nil, nil when there is no such internal user.
	GetUserSkillProfile(ctx context.Context, userID string) (*domain.UserSkillProfileResponse, error)
}

// isSkillUniqueViolation is a unique-index refusal, optionally of one index.
func isSkillUniqueViolation(err error, index string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return index == "" || pgErr.ConstraintName == index
}

// ListCategories implements SkillRepository. SkillCount is the active skills
// filed directly under each category. Without includeInactive, a deactivated
// category and everything under it are left out.
func (r *skillRepository) ListCategories(ctx context.Context, includeInactive bool) ([]domain.SkillCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.name, c.code, c.category_path, c.is_lowest_category, c.parent_id::text,
		       c.active, c.description,
		       (SELECT count(*) FROM skill_category_link l JOIN skill s ON s.id = l.skill_id AND s.active
		         WHERE l.category_id = c.id)::int
		  FROM skill_category c
		 ORDER BY lower(COALESCE(c.category_path, c.name)), c.id`)
	if err != nil {
		return nil, fmt.Errorf("query skill categories: %w", err)
	}
	defer rows.Close()
	all := []domain.SkillCategory{}
	for rows.Next() {
		var c domain.SkillCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.CategoryPath, &c.IsLowestCategory, &c.ParentID,
			&c.Active, &c.Description, &c.SkillCount); err != nil {
			return nil, fmt.Errorf("scan skill category: %w", err)
		}
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill categories: %w", err)
	}
	if includeInactive {
		return all, nil
	}
	byID := make(map[string]domain.SkillCategory, len(all))
	for _, c := range all {
		byID[c.ID] = c
	}
	visible := func(c domain.SkillCategory) bool {
		for seen := 0; seen < len(all); seen++ { // bounded: a cycle cannot loop forever
			if !c.Active {
				return false
			}
			if c.ParentID == nil {
				return true
			}
			parent, ok := byID[*c.ParentID]
			if !ok {
				return true
			}
			c = parent
		}
		return true
	}
	out := []domain.SkillCategory{}
	for _, c := range all {
		if visible(c) {
			out = append(out, c)
		}
	}
	return out, nil
}

// attachHolderCounts fills in how many people hold each skill.
func attachHolderCounts(ctx context.Context, q skillQuerier, skills []domain.Skill) error {
	if len(skills) == 0 {
		return nil
	}
	ids := make([]string, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
	}
	rows, err := q.Query(ctx, `
		SELECT us.skill_id::text, count(DISTINCT us.user_id)::int
		  FROM user_skill us
		 WHERE us.active AND us.skill_id = ANY($1::uuid[])
		 GROUP BY us.skill_id`, ids)
	if err != nil {
		return fmt.Errorf("count skill holders: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return fmt.Errorf("scan skill holders: %w", err)
		}
		counts[id] = n
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate skill holders: %w", err)
	}
	for i := range skills {
		n := counts[skills[i].ID]
		skills[i].HolderCount = &n
	}
	return nil
}

// DefaultLevelTypeID implements SkillManagementRepository.
func (r *skillRepository) DefaultLevelTypeID(ctx context.Context) (string, bool, error) {
	return defaultLevelTypeID(ctx, r.db)
}

func defaultLevelTypeID(ctx context.Context, q skillQuerier) (string, bool, error) {
	var id string
	// The marked default; failing that, the only scale there is. A database
	// whose scales were added after migration 0283 ran (a fresh one, seeded
	// afterwards) has none marked, and a single scale is unambiguous.
	err := q.QueryRow(ctx, `
		SELECT id::text FROM skill_level_type
		 WHERE is_default OR (SELECT count(*) FROM skill_level_type) = 1
		 ORDER BY is_default DESC
		 LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read default skill scale: %w", err)
	}
	return id, true, nil
}

// CategoriesAreActiveLeaves implements SkillManagementRepository.
func (r *skillRepository) CategoriesAreActiveLeaves(ctx context.Context, ids []string) (bool, error) {
	return categoriesAreActiveLeaves(ctx, r.db, ids)
}

func categoriesAreActiveLeaves(ctx context.Context, q skillQuerier, ids []string) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	var n int
	err := q.QueryRow(ctx, `
		SELECT count(*) FROM skill_category c
		 WHERE c.id = ANY($1::uuid[]) AND c.active
		   AND NOT EXISTS (SELECT 1 FROM skill_category ch WHERE ch.parent_id = c.id)`, ids).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check skill categories: %w", err)
	}
	return n == len(ids), nil
}

// categoryLock serialises category writes: a path rewrite, a new child and a
// deactivation each read the tree before changing it.
const categoryLock = `SELECT pg_advisory_xact_lock(hashtextextended('skill_category_tree', 0))`

func (r *skillRepository) inCategoryTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, categoryLock); err != nil {
		return fmt.Errorf("lock skill categories: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func getCategory(ctx context.Context, q skillQuerier, id string) (*domain.SkillCategory, error) {
	var c domain.SkillCategory
	err := q.QueryRow(ctx, `
		SELECT c.id::text, c.name, c.code, c.category_path, c.is_lowest_category, c.parent_id::text,
		       c.active, c.description,
		       (SELECT count(*) FROM skill_category_link l JOIN skill s ON s.id = l.skill_id AND s.active
		         WHERE l.category_id = c.id)::int
		  FROM skill_category c WHERE c.id = $1::uuid`, id).
		Scan(&c.ID, &c.Name, &c.Code, &c.CategoryPath, &c.IsLowestCategory, &c.ParentID, &c.Active, &c.Description, &c.SkillCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get skill category: %w", err)
	}
	return &c, nil
}

// categoryPathSep is how the stored paths join names ("Engineering / Frontend").
const categoryPathSep = " / "

// CreateCategory implements SkillManagementRepository. A child goes under an
// active parent with no skills filed directly under it (skills live on
// leaves), and the parent stops being a leaf.
func (r *skillRepository) CreateCategory(ctx context.Context, req domain.CreateSkillCategoryRequest, actor string) (*domain.SkillCategory, domain.SkillOutcome, error) {
	var out *domain.SkillCategory
	outcome := domain.SkillOutcomeDone
	err := r.inCategoryTx(ctx, func(tx pgx.Tx) error {
		path := req.Name
		if req.ParentID != nil {
			parent, err := getCategory(ctx, tx, *req.ParentID)
			if err != nil {
				return err
			}
			if parent == nil {
				outcome = domain.SkillOutcomeCategoryNotFound
				return nil
			}
			if !parent.Active {
				outcome = domain.SkillOutcomeCategoryInvalid
				return nil
			}
			var filed bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill_category_link WHERE category_id = $1::uuid)`, parent.ID).Scan(&filed); err != nil {
				return fmt.Errorf("check category skills: %w", err)
			}
			if filed {
				outcome = domain.SkillOutcomeCategoryInvalid
				return nil
			}
			base := parent.Name
			if parent.CategoryPath != nil && *parent.CategoryPath != "" {
				base = *parent.CategoryPath
			}
			path = base + categoryPathSep + req.Name
			if _, err := tx.Exec(ctx, `UPDATE skill_category SET is_lowest_category = FALSE, updated_on = NOW(), updated_by = $2
				WHERE id = $1::uuid AND is_lowest_category`, parent.ID, actor); err != nil {
				return fmt.Errorf("update parent category: %w", err)
			}
		}
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO skill_category (created_on, updated_on, created_by, updated_by, name, category_path,
			                            is_lowest_category, parent_id, active, description)
			VALUES (NOW(), NOW(), $1, $1, $2, $3, TRUE, $4::uuid, TRUE, NULLIF($5, ''))
			RETURNING id::text`, actor, req.Name, path, req.ParentID, derefString(req.Description)).Scan(&id)
		if isSkillUniqueViolation(err, "ux_skill_category_parent_name") {
			outcome = domain.SkillOutcomeNameTaken
			return nil
		}
		if err != nil {
			return fmt.Errorf("create skill category: %w", err)
		}
		out, err = getCategory(ctx, tx, id)
		return err
	})
	if outcome != domain.SkillOutcomeDone {
		return nil, outcome, err
	}
	return out, outcome, err
}

// UpdateCategory implements SkillManagementRepository. A rename rewrites the
// stored path of the category and everything under it. Deactivating is refused
// while an active child category or an active skill is filed under it;
// activating is refused under an inactive parent.
func (r *skillRepository) UpdateCategory(ctx context.Context, id string, req domain.UpdateSkillCategoryRequest, actor string) (*domain.SkillCategory, domain.SkillOutcome, error) {
	var out *domain.SkillCategory
	outcome := domain.SkillOutcomeDone
	err := r.inCategoryTx(ctx, func(tx pgx.Tx) error {
		cur, err := getCategory(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur == nil {
			outcome = domain.SkillOutcomeCategoryNotFound
			return nil
		}
		if req.Active != nil && !*req.Active && cur.Active {
			var inUse bool
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM skill_category WHERE parent_id = $1::uuid AND active)
				    OR EXISTS (SELECT 1 FROM skill_category_link l JOIN skill s ON s.id = l.skill_id AND s.active
				                WHERE l.category_id = $1::uuid)`, id).Scan(&inUse); err != nil {
				return fmt.Errorf("check category in use: %w", err)
			}
			if inUse {
				outcome = domain.SkillOutcomeCategoryInUse
				return nil
			}
		}
		if req.Active != nil && *req.Active && !cur.Active && cur.ParentID != nil {
			parent, err := getCategory(ctx, tx, *cur.ParentID)
			if err != nil {
				return err
			}
			if parent != nil && !parent.Active {
				outcome = domain.SkillOutcomeCategoryInvalid
				return nil
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE skill_category SET
			       name        = COALESCE($2, name),
			       description = CASE WHEN $3::text IS NULL THEN description ELSE NULLIF($3, '') END,
			       active      = COALESCE($4, active),
			       updated_on  = NOW(),
			       updated_by  = $5
			 WHERE id = $1::uuid`, id, req.Name, req.Description, req.Active, actor)
		if isSkillUniqueViolation(err, "ux_skill_category_parent_name") {
			outcome = domain.SkillOutcomeNameTaken
			return nil
		}
		if err != nil {
			return fmt.Errorf("update skill category: %w", err)
		}
		if req.Name != nil && *req.Name != cur.Name {
			oldPath := cur.Name
			if cur.CategoryPath != nil && *cur.CategoryPath != "" {
				oldPath = *cur.CategoryPath
			}
			newPath := *req.Name
			if i := strings.LastIndex(oldPath, categoryPathSep); i >= 0 {
				newPath = oldPath[:i+len(categoryPathSep)] + *req.Name
			}
			if _, err := tx.Exec(ctx, `
				WITH RECURSIVE sub AS (
				    SELECT id FROM skill_category WHERE id = $1::uuid
				    UNION
				    SELECT c.id FROM skill_category c JOIN sub ON c.parent_id = sub.id
				)
				UPDATE skill_category c
				   SET category_path = $3 || substr(COALESCE(c.category_path, c.name), length($2) + 1),
				       updated_on = NOW(), updated_by = $4
				 WHERE c.id IN (SELECT id FROM sub)
				   AND left(COALESCE(c.category_path, c.name), length($2)) = $2`, id, oldPath, newPath, actor); err != nil {
				return fmt.Errorf("rewrite category paths: %w", err)
			}
		}
		out, err = getCategory(ctx, tx, id)
		return err
	})
	if outcome != domain.SkillOutcomeDone {
		return nil, outcome, err
	}
	return out, outcome, err
}

// --- skill requests ---

const skillRequestSelect = `
	SELECT r.id::text, r.requested_name, r.description, r.suggested_category, r.status::text, r.created_on,
	       c.id::text, c.name, c.category_path,
	       rl.id::text, rl.name, rl.value, rl.description, rl.color,
	       ru.id::text, ru.name, ru.email,
	       vu.id::text, vu.name, vu.email, r.reviewed_on, r.review_comment, r.resolution,
	       rs.id::text, rs.name, rs.active,
	       al.id::text, al.name, al.value, al.description, al.color,
	       r.auto_add_note
	  FROM skill_request r
	  JOIN "user" ru ON ru.id = r.requested_by_user_id
	  LEFT JOIN skill_category c ON c.id = r.category_id
	  LEFT JOIN skill_level rl ON rl.id = r.requested_level_id
	  LEFT JOIN "user" vu ON vu.id = r.reviewed_by_user_id
	  LEFT JOIN skill rs ON rs.id = r.resolved_skill_id
	  LEFT JOIN skill_level al ON al.id = r.auto_added_level_id`

func scanSkillRequest(row pgx.Row) (domain.SkillRequest, error) {
	var (
		q                             domain.SkillRequest
		status                        string
		cID, cName, cPath             *string
		rlID, rlName, rlDesc, rlColor *string
		rlValue, alValue              *int
		vID, vName, vEmail            *string
		rsID, rsName                  *string
		rsActive                      *bool
		alID, alName, alDesc, alColor *string
	)
	err := row.Scan(&q.ID, &q.RequestedName, &q.Description, &q.SuggestedCategory, &status, &q.CreatedOn,
		&cID, &cName, &cPath,
		&rlID, &rlName, &rlValue, &rlDesc, &rlColor,
		&q.RequestedBy.ID, &q.RequestedBy.Name, &q.RequestedBy.Email,
		&vID, &vName, &vEmail, &q.ReviewedOn, &q.ReviewComment, &q.Resolution,
		&rsID, &rsName, &rsActive,
		&alID, &alName, &alValue, &alDesc, &alColor,
		&q.AutoAddNote)
	if err != nil {
		return q, err
	}
	q.Status = domain.SkillRequestStatus(status)
	if cID != nil {
		q.Category = &domain.SkillCategoryRef{ID: *cID, Name: derefString(cName), CategoryPath: cPath}
	}
	q.RequestedLevel = scanLevel(rlID, rlName, rlValue, rlDesc, rlColor)
	if vID != nil {
		q.ReviewedBy = &domain.SkillPersonRef{ID: *vID, Name: vName, Email: vEmail}
	}
	if rsID != nil {
		q.ResolvedSkill = &domain.SkillRef{ID: *rsID, Name: derefString(rsName), Active: rsActive != nil && *rsActive}
	}
	q.AutoAddedLevel = scanLevel(alID, alName, alValue, alDesc, alColor)
	return q, nil
}

func getSkillRequest(ctx context.Context, q skillQuerier, id string) (*domain.SkillRequest, error) {
	req, err := scanSkillRequest(q.QueryRow(ctx, skillRequestSelect+` WHERE r.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get skill request: %w", err)
	}
	return &req, nil
}

// GetSkillRequest implements SkillManagementRepository.
func (r *skillRepository) GetSkillRequest(ctx context.Context, id string) (*domain.SkillRequest, error) {
	return getSkillRequest(ctx, r.db, id)
}

// CheckSkillRequest implements SkillManagementRepository. Similar names are
// those that contain the asked-for name or are contained in it.
func (r *skillRepository) CheckSkillRequest(ctx context.Context, normalizedName string) (domain.CheckSkillRequestResponse, error) {
	resp := domain.CheckSkillRequestResponse{Similar: []domain.SkillRef{}}
	if normalizedName == "" {
		return resp, nil
	}
	var exact domain.SkillRef
	err := r.db.QueryRow(ctx, `
		SELECT id::text, name, active FROM skill
		 WHERE lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) = $1
		 ORDER BY active DESC LIMIT 1`, normalizedName).Scan(&exact.ID, &exact.Name, &exact.Active)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return resp, fmt.Errorf("check skill name: %w", err)
	default:
		resp.ExactMatch = &exact
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, active FROM skill
		 WHERE lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) <> $1
		   AND length(btrim(name)) >= 2
		   AND (strpos(lower(name), $1) > 0 OR strpos($1, lower(btrim(name))) > 0)
		 ORDER BY active DESC, length(name), lower(name)
		 LIMIT 8`, normalizedName)
	if err != nil {
		return resp, fmt.Errorf("find similar skills: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s domain.SkillRef
		if err := rows.Scan(&s.ID, &s.Name, &s.Active); err != nil {
			return resp, fmt.Errorf("scan similar skill: %w", err)
		}
		resp.Similar = append(resp.Similar, s)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("iterate similar skills: %w", err)
	}
	pending, err := scanSkillRequest(r.db.QueryRow(ctx, skillRequestSelect+`
		 WHERE r.normalized_name = $1 AND r.status = 'PENDING'`, normalizedName))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return resp, fmt.Errorf("check pending skill request: %w", err)
	default:
		resp.PendingRequest = &pending
	}
	return resp, nil
}

// CreateSkillRequest implements SkillManagementRepository. Refused when an
// active skill already has the name (SkillOutcomeSkillExists) or a pending
// request asks for it (SkillOutcomePendingExists, with that request).
func (r *skillRepository) CreateSkillRequest(ctx context.Context, userID, normalizedName string, req domain.CreateSkillRequestRequest, actor string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('skill_name:' || $1, 0))`, normalizedName); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("lock skill name: %w", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill WHERE active AND lower(btrim(regexp_replace(name, '\s+', ' ', 'g'))) = $1)`,
		normalizedName).Scan(&exists); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("check skill name: %w", err)
	}
	if exists {
		return nil, domain.SkillOutcomeSkillExists, nil
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO skill_request (created_by, updated_by, requested_by_user_id, requested_name, normalized_name,
		                           category_id, suggested_category, description, requested_level_id)
		VALUES ($1, $1, $2::uuid, $3, $4, $5::uuid, NULLIF($6, ''), NULLIF($7, ''), $8::uuid)
		RETURNING id::text`,
		actor, userID, req.Name, normalizedName, req.CategoryID, derefString(req.SuggestedCategory),
		derefString(req.Description), req.LevelID).Scan(&id)
	if isSkillUniqueViolation(err, "ux_skill_request_pending_name") {
		_ = tx.Rollback(ctx)
		pending, perr := scanSkillRequest(r.db.QueryRow(ctx, skillRequestSelect+`
			 WHERE r.normalized_name = $1 AND r.status = 'PENDING'`, normalizedName))
		if perr != nil && !errors.Is(perr, pgx.ErrNoRows) {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("read pending skill request: %w", perr)
		}
		if perr != nil {
			return nil, domain.SkillOutcomePendingExists, nil
		}
		return &pending, domain.SkillOutcomePendingExists, nil
	}
	if err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("create skill request: %w", err)
	}
	out, err := getSkillRequest(ctx, tx, id)
	if err != nil {
		return nil, domain.SkillOutcomeDone, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("commit: %w", err)
	}
	return out, domain.SkillOutcomeDone, nil
}

// ListSkillRequests implements SkillManagementRepository. Pending requests
// come first (oldest first, so the queue is worked in order), then the rest,
// newest first.
func (r *skillRepository) ListSkillRequests(ctx context.Context, userID string, statuses []domain.SkillRequestStatus, limit, offset int) ([]domain.SkillRequest, int, error) {
	st := make([]string, 0, len(statuses))
	for _, s := range statuses {
		st = append(st, string(s))
	}
	const where = `
		 WHERE ($1 = '' OR r.requested_by_user_id = NULLIF($1, '')::uuid)
		   AND (cardinality($2::text[]) = 0 OR r.status::text = ANY($2))`
	var (
		total int
		out   []domain.SkillRequest
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, `SELECT count(*) FROM skill_request r`+where, userID, st).Scan(&total); err != nil {
			return fmt.Errorf("count skill requests: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, skillRequestSelect+where+`
			 ORDER BY (r.status = 'PENDING') DESC,
			          CASE WHEN r.status = 'PENDING' THEN r.created_on END ASC,
			          COALESCE(r.reviewed_on, r.updated_on) DESC, r.id DESC
			 LIMIT $3 OFFSET $4`, userID, st, limit, offset)
		if err != nil {
			return fmt.Errorf("query skill requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			q, err := scanSkillRequest(rows)
			if err != nil {
				return fmt.Errorf("scan skill request: %w", err)
			}
			out = append(out, q)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.SkillRequest{}
	}
	return out, total, nil
}

// CountPendingSkillRequests implements SkillManagementRepository.
func (r *skillRepository) CountPendingSkillRequests(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM skill_request WHERE status = 'PENDING'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending skill requests: %w", err)
	}
	return n, nil
}

// decideSkillRequest runs one status change that applies only to a pending
// request: the UPDATE carries the "still pending" condition, so two reviewers
// cannot both decide it. No row changed means it was decided (or does not
// exist), and the request as it now stands is returned with the outcome.
func (r *skillRepository) decideSkillRequest(ctx context.Context, id, sql string, args ...any) (*domain.SkillRequest, domain.SkillOutcome, error) {
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("update skill request: %w", err)
	}
	req, err := getSkillRequest(ctx, r.db, id)
	if err != nil {
		return nil, domain.SkillOutcomeDone, err
	}
	if req == nil {
		return nil, domain.SkillOutcomeNotFound, nil
	}
	if tag.RowsAffected() == 0 {
		return req, domain.SkillOutcomeRequestNotPending, nil
	}
	return req, domain.SkillOutcomeDone, nil
}

// WithdrawSkillRequest implements SkillManagementRepository. Only the
// requester's own pending request; another person's is "not found".
func (r *skillRepository) WithdrawSkillRequest(ctx context.Context, id, userID, actor string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	req, outcome, err := r.decideSkillRequest(ctx, id, `
		UPDATE skill_request SET status = 'WITHDRAWN', updated_on = NOW(), updated_by = $3
		 WHERE id = $1::uuid AND requested_by_user_id = $2::uuid AND status = 'PENDING'`, id, userID, actor)
	if err == nil && req != nil && req.RequestedBy.ID != userID {
		return nil, domain.SkillOutcomeNotFound, nil
	}
	return req, outcome, err
}

// RejectSkillRequest implements SkillManagementRepository.
func (r *skillRepository) RejectSkillRequest(ctx context.Context, id, reviewerID, reason, actor string) (*domain.SkillRequest, domain.SkillOutcome, error) {
	return r.decideSkillRequest(ctx, id, `
		UPDATE skill_request SET status = 'REJECTED', reviewed_by_user_id = $2::uuid, reviewed_on = NOW(),
		       review_comment = $3, updated_on = NOW(), updated_by = $4
		 WHERE id = $1::uuid AND status = 'PENDING'`, id, reviewerID, reason, actor)
}

// ApproveSkillRequest implements SkillManagementRepository: in one transaction
// that locks the request row, it creates, reactivates or points at the skill,
// marks the request approved, and adds the skill to the requester's profile.
// Any refusal rolls the whole of it back.
func (r *skillRepository) ApproveSkillRequest(ctx context.Context, a domain.SkillApproval) (*domain.SkillRequest, domain.SkillOutcome, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		status, requesterID, requestedLevelID string
	)
	err = tx.QueryRow(ctx, `
		SELECT status::text, requested_by_user_id::text, requested_level_id::text
		  FROM skill_request WHERE id = $1::uuid FOR UPDATE`, a.RequestID).Scan(&status, &requesterID, &requestedLevelID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.SkillOutcomeNotFound, nil
	}
	if err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("lock skill request: %w", err)
	}
	if status != string(domain.SkillRequestPending) {
		req, err := getSkillRequest(ctx, tx, a.RequestID)
		return req, domain.SkillOutcomeRequestNotPending, err
	}

	skillID, resolution := a.SkillID, ""
	switch a.Mode {
	case domain.SkillApproveCreate:
		resolution = domain.SkillResolutionCreated
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('skill_name:' || lower($1), 0))`, strings.TrimSpace(a.Name)); err != nil {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("lock skill name: %w", err)
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill WHERE lower(btrim(name)) = lower(btrim($1)))`, a.Name).Scan(&taken); err != nil {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("check skill name: %w", err)
		}
		if taken {
			return nil, domain.SkillOutcomeNameTaken, nil
		}
		scale, ok, err := defaultLevelTypeID(ctx, tx)
		if err != nil {
			return nil, domain.SkillOutcomeDone, err
		}
		if !ok {
			return nil, domain.SkillOutcomeNoDefaultScale, nil
		}
		if a.CategoryID != nil {
			ok, err := categoriesAreActiveLeaves(ctx, tx, []string{*a.CategoryID})
			if err != nil {
				return nil, domain.SkillOutcomeDone, err
			}
			if !ok {
				return nil, domain.SkillOutcomeCategoryInvalid, nil
			}
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO skill (created_on, updated_on, created_by, updated_by, name, description, keywords, active, level_type_id)
			VALUES (NOW(), NOW(), $1, $1, $2, NULLIF($3, ''), NULLIF($4, ''), TRUE, $5::uuid)
			RETURNING id::text`, a.Actor, a.Name, derefString(a.Description), derefString(a.Keywords), scale).Scan(&skillID)
		if isSkillUniqueViolation(err, "ux_skill_name_lower") {
			return nil, domain.SkillOutcomeNameTaken, nil
		}
		if err != nil {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("create skill: %w", err)
		}
		if a.CategoryID != nil {
			if err := replaceSkillCategories(ctx, tx, skillID, []string{*a.CategoryID}, a.Actor); err != nil {
				return nil, domain.SkillOutcomeDone, err
			}
		}
	case domain.SkillApproveReactivate, domain.SkillApproveExisting:
		var active bool
		err := tx.QueryRow(ctx, `SELECT active FROM skill WHERE id = $1::uuid FOR UPDATE`, skillID).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.SkillOutcomeNotFound, nil
		}
		if err != nil {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("lock skill: %w", err)
		}
		if a.Mode == domain.SkillApproveExisting {
			resolution = domain.SkillResolutionExisting
			if !active {
				return nil, domain.SkillOutcomeSkillNotActive, nil
			}
			break
		}
		resolution = domain.SkillResolutionReactivated
		if active {
			return nil, domain.SkillOutcomeSkillNotInactive, nil
		}
		if a.CategoryID != nil {
			ok, err := categoriesAreActiveLeaves(ctx, tx, []string{*a.CategoryID})
			if err != nil {
				return nil, domain.SkillOutcomeDone, err
			}
			if !ok {
				return nil, domain.SkillOutcomeCategoryInvalid, nil
			}
			if err := replaceSkillCategories(ctx, tx, skillID, []string{*a.CategoryID}, a.Actor); err != nil {
				return nil, domain.SkillOutcomeDone, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE skill SET active = TRUE, updated_on = NOW(), updated_by = $2 WHERE id = $1::uuid`,
			skillID, a.Actor); err != nil {
			return nil, domain.SkillOutcomeDone, fmt.Errorf("reactivate skill: %w", err)
		}
	default:
		return nil, domain.SkillOutcomeDone, fmt.Errorf("unknown approval mode %q", a.Mode)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE skill_request SET status = 'APPROVED', reviewed_by_user_id = $2::uuid, reviewed_on = NOW(),
		       review_comment = NULLIF($3, ''), resolution = $4, resolved_skill_id = $5::uuid,
		       updated_on = NOW(), updated_by = $6
		 WHERE id = $1::uuid`, a.RequestID, a.ReviewerID, derefString(a.Comment), resolution, skillID, a.Actor); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("approve skill request: %w", err)
	}

	levelID, note, outcome, err := autoAddRequestedSkill(ctx, tx, a, requesterID, skillID, requestedLevelID)
	if err != nil || outcome != domain.SkillOutcomeDone {
		return nil, outcome, err
	}
	if _, err := tx.Exec(ctx, `UPDATE skill_request SET auto_added_level_id = $2::uuid, auto_add_note = $3 WHERE id = $1::uuid`,
		a.RequestID, levelID, note); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("record auto add: %w", err)
	}
	out, err := getSkillRequest(ctx, tx, a.RequestID)
	if err != nil {
		return nil, domain.SkillOutcomeDone, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domain.SkillOutcomeDone, fmt.Errorf("commit: %w", err)
	}
	return out, domain.SkillOutcomeDone, nil
}

// autoAddRequestedSkill adds the approved skill to the requester's profile
// (decision C.3). The level is the reviewer's override when given (it must be
// on the skill's scale), else the requested level when it is on the skill's
// scale, else the level of that scale with the same value or the closest lower
// one, else its lowest. It returns the level given (nil when nothing was added)
// and what happened.
func autoAddRequestedSkill(ctx context.Context, tx pgx.Tx, a domain.SkillApproval, requesterID, skillID, requestedLevelID string) (*string, string, domain.SkillOutcome, error) {
	var activeUser bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM "user" WHERE id = $1::uuid AND user_type = 'INTERNAL'::user_type_enum AND COALESCE(is_active, TRUE))`,
		requesterID).Scan(&activeUser); err != nil {
		return nil, "", domain.SkillOutcomeDone, fmt.Errorf("check requester: %w", err)
	}
	if !activeUser {
		return nil, domain.SkillAutoAddInactiveUser, domain.SkillOutcomeDone, nil
	}
	var scale *string
	if err := tx.QueryRow(ctx, `SELECT level_type_id::text FROM skill WHERE id = $1::uuid`, skillID).Scan(&scale); err != nil {
		return nil, "", domain.SkillOutcomeDone, fmt.Errorf("read skill scale: %w", err)
	}
	if scale == nil {
		return nil, domain.SkillAutoAddNoScale, domain.SkillOutcomeDone, nil
	}
	var levelID string
	note := domain.SkillAutoAddAdded
	if a.LevelID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill_level WHERE id = $1::uuid AND level_type_id = $2::uuid)`,
			*a.LevelID, *scale).Scan(&ok); err != nil {
			return nil, "", domain.SkillOutcomeDone, fmt.Errorf("check level: %w", err)
		}
		if !ok {
			return nil, "", domain.SkillOutcomeLevelNotInScale, nil
		}
		levelID = *a.LevelID
		if levelID != requestedLevelID {
			note = domain.SkillAutoAddLevelAdjusted
		}
	} else {
		err := tx.QueryRow(ctx, `
			WITH asked AS (SELECT level_type_id, COALESCE(value, 0) AS value FROM skill_level WHERE id = $1::uuid)
			SELECT sl.id::text FROM skill_level sl, asked
			 WHERE sl.level_type_id = $2::uuid
			 ORDER BY (sl.id = $1::uuid) DESC,
			          (COALESCE(sl.value, 0) <= asked.value) DESC,
			          CASE WHEN COALESCE(sl.value, 0) <= asked.value THEN -COALESCE(sl.value, 0) ELSE COALESCE(sl.value, 0) END,
			          sl.id
			 LIMIT 1`, requestedLevelID, *scale).Scan(&levelID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", domain.SkillOutcomeLevelNotInScale, nil
		}
		if err != nil {
			return nil, "", domain.SkillOutcomeDone, fmt.Errorf("choose level: %w", err)
		}
		if levelID != requestedLevelID {
			note = domain.SkillAutoAddLevelAdjusted
		}
	}
	rows, err := lockUserSkill(ctx, tx, requesterID, skillID)
	if err != nil {
		return nil, "", domain.SkillOutcomeDone, err
	}
	res, err := addUserSkillLocked(ctx, tx, rows, requesterID, skillID, levelID, a.Actor, &a.RequestID)
	if err != nil {
		return nil, "", domain.SkillOutcomeDone, err
	}
	switch res.Outcome {
	case domain.SkillOutcomeDone:
		if res.Action == domain.SkillHistoryReadded && note == domain.SkillAutoAddAdded {
			note = domain.SkillAutoAddReadded
		}
		return &levelID, note, domain.SkillOutcomeDone, nil
	case domain.SkillOutcomeAlreadyHeld, domain.SkillOutcomeInherited:
		return nil, domain.SkillAutoAddAlreadyHeld, domain.SkillOutcomeDone, nil
	default:
		return nil, "", res.Outcome, nil
	}
}

// --- directory ---

// One row per person: an internal, active user holding at least one active
// skill, who matches the name search, the teams and the categories, and meets
// ALL (or, with $6 false, ANY) of the criteria. A criterion is met by holding
// the skill at a level whose value is at least its minimum (0 = any level).
const directoryPeopleQuery = `
	WITH RECURSIVE wanted AS (
	    SELECT id FROM skill_category WHERE id = ANY($3::uuid[])
	    UNION
	    SELECT c.id FROM skill_category c JOIN wanted w ON c.parent_id = w.id
	),
	crit AS (SELECT * FROM unnest($4::uuid[], $5::int[]) AS c(skill_id, min_value)),
	held AS (
	    SELECT us.user_id, us.skill_id, COALESCE(sl.value, 0) AS value
	      FROM user_skill us
	      JOIN skill s ON s.id = us.skill_id AND s.active
	      LEFT JOIN skill_level sl ON sl.id = us.skill_level_id
	     WHERE us.active AND ` + userSkillCanonical + `
	),
	people AS (
	    SELECT u.id, u.name, u.email,
	           (SELECT count(*) FROM crit c JOIN held h ON h.skill_id = c.skill_id AND h.value >= c.min_value
	             WHERE h.user_id = u.id)::int AS matched
	      FROM "user" u
	     WHERE u.user_type = 'INTERNAL'::user_type_enum
	       AND COALESCE(u.is_active, TRUE)
	       AND EXISTS (SELECT 1 FROM held h WHERE h.user_id = u.id)
	       AND ($1 = '' OR u.name ILIKE $1 ESCAPE '\' OR u.email ILIKE $1 ESCAPE '\')
	       AND (cardinality($2::text[]) = 0 OR EXISTS (
	             SELECT 1 FROM team_member tm JOIN team t ON t.id = tm.team_id
	              WHERE tm.user_id = u.id AND t.key = ANY($2)))
	       AND (cardinality($3::uuid[]) = 0 OR EXISTS (
	             SELECT 1 FROM held h JOIN skill_category_link l ON l.skill_id = h.skill_id
	              WHERE h.user_id = u.id AND l.category_id IN (SELECT id FROM wanted)))
	),
	matching AS (
	    SELECT * FROM people p
	     WHERE cardinality($4::uuid[]) = 0
	        OR p.matched >= CASE WHEN $6 THEN cardinality($4::uuid[]) ELSE 1 END
	)`

// SearchSkillDirectory implements SkillManagementRepository. Best matches
// first (for ANY), then by name.
func (r *skillRepository) SearchSkillDirectory(ctx context.Context, req domain.SearchSkillDirectoryRequest) ([]domain.SkillDirectoryPerson, int, error) {
	pattern := likePattern(req.SearchQuery)
	nonNil := func(in []string) []string {
		if in == nil {
			return []string{}
		}
		return in
	}
	teamKeys, cats := nonNil(req.TeamKeys), nonNil(req.CategoryIDs)
	critSkills := make([]string, len(req.Criteria))
	critMins := make([]int32, len(req.Criteria))
	for i, c := range req.Criteria {
		critSkills[i] = c.SkillID
		critMins[i] = int32(c.MinLevelValue)
	}
	all := req.Match != "ANY"
	args := []any{pattern, teamKeys, cats, critSkills, critMins, all}

	var total int
	if err := r.db.QueryRow(ctx, directoryPeopleQuery+` SELECT count(*) FROM matching`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count skill directory: %w", err)
	}
	rows, err := r.db.Query(ctx, directoryPeopleQuery+`
		SELECT m.id::text, m.name, m.email, m.matched,
		       COALESCE((SELECT array_agg(DISTINCT t.key ORDER BY t.key)
		                   FROM team_member tm JOIN team t ON t.id = tm.team_id
		                  WHERE tm.user_id = m.id AND t.key IS NOT NULL), '{}'::text[])
		  FROM matching m
		 ORDER BY m.matched DESC, lower(COALESCE(m.name, m.email, '')), m.id
		 LIMIT $7 OFFSET $8`, append(args, req.Pagination.Limit, req.Pagination.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("query skill directory: %w", err)
	}
	out := []domain.SkillDirectoryPerson{}
	for rows.Next() {
		var p domain.SkillDirectoryPerson
		if err := rows.Scan(&p.UserID, &p.UserName, &p.UserEmail, &p.MatchedCount, &p.TeamKeys); err != nil {
			rows.Close()
			return nil, 0, fmt.Errorf("scan skill directory person: %w", err)
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate skill directory: %w", err)
	}
	for i := range out {
		skills, err := listUserSkills(ctx, r.db, out[i].UserID, "")
		if err != nil {
			return nil, 0, err
		}
		out[i].Skills = skills
	}
	return out, total, nil
}

// GetUserSkillProfile implements SkillManagementRepository.
func (r *skillRepository) GetUserSkillProfile(ctx context.Context, userID string) (*domain.UserSkillProfileResponse, error) {
	var p domain.UserSkillProfileResponse
	err := r.db.QueryRow(ctx, `
		SELECT u.id::text, u.name, u.email,
		       COALESCE((SELECT array_agg(DISTINCT t.key ORDER BY t.key)
		                   FROM team_member tm JOIN team t ON t.id = tm.team_id
		                  WHERE tm.user_id = u.id AND t.key IS NOT NULL), '{}'::text[])
		  FROM "user" u
		 WHERE u.id = $1::uuid AND u.user_type = 'INTERNAL'::user_type_enum`, userID).
		Scan(&p.UserID, &p.UserName, &p.UserEmail, &p.TeamKeys)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get skill profile user: %w", err)
	}
	skills, err := listUserSkills(ctx, r.db, userID, "")
	if err != nil {
		return nil, err
	}
	p.Skills = skills
	return &p, nil
}
