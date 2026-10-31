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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SkillRepository reads and writes the skill catalogue and people's skill
// profiles. Postgres only.
//
// The writes report what they found as a domain.SkillOutcome rather than
// deciding the caller's refusal: the service owns the wording, as it does for
// the rota writes.
type SkillRepository interface {
	// ResolveUserID is the platform user id of an active internal user with
	// this email, if there is one. The caller's identity is resolved here and
	// nowhere else: an identity-provider id is never compared with a platform id.
	ResolveUserID(ctx context.Context, email string) (string, bool, error)
	// IsCRESRELead is whether the email belongs to a lead (lead or
	// americas_team_lead) of a CRE or SRE team.
	IsCRESRELead(ctx context.Context, email string) (bool, error)

	ListLevelTypes(ctx context.Context) ([]domain.SkillLevelType, error)
	ListCategories(ctx context.Context) ([]domain.SkillCategory, error)
	SearchSkills(ctx context.Context, req domain.SearchSkillsRequest) ([]domain.Skill, int, error)
	// GetSkill is nil, nil when there is no such skill.
	GetSkill(ctx context.Context, id string) (*domain.Skill, error)
	// LevelTypeOfLevel is the scale a level belongs to; false when there is no such level.
	LevelTypeOfLevel(ctx context.Context, levelID string) (string, bool, error)
	LevelTypeExists(ctx context.Context, id string) (bool, error)
	// CategoriesExist is true when every id is a category.
	CategoriesExist(ctx context.Context, ids []string) (bool, error)

	ListUserSkills(ctx context.Context, userID string) ([]domain.UserSkill, error)
	AddUserSkill(ctx context.Context, userID, skillID, levelID, actor string) (domain.SkillWriteResult, error)
	SetUserSkillLevel(ctx context.Context, userID, skillID, levelID, actor string) (domain.SkillWriteResult, error)
	RemoveUserSkill(ctx context.Context, userID, skillID, actor string) (domain.SkillWriteResult, error)
	ListUserSkillHistory(ctx context.Context, userID, skillID string, limit, offset int) ([]domain.SkillHistoryEntry, int, error)

	SearchUserSkills(ctx context.Context, req domain.SearchUserSkillsRequest) ([]domain.UserSkillRow, int, error)

	CreateSkill(ctx context.Context, req domain.CreateSkillRequest, actor string) (domain.SkillWriteResult, error)
	UpdateSkill(ctx context.Context, id string, req domain.UpdateSkillRequest, actor string) (domain.SkillWriteResult, error)
}

type skillRepository struct{ db *pgxpool.Pool }

// NewSkillRepository constructs a SkillRepository over the given pool.
func NewSkillRepository(db *pgxpool.Pool) SkillRepository { return &skillRepository{db: db} }

// skillQuerier is what a read needs, so the same query serves the pool and a
// transaction (a write reads its result back inside its own transaction).
type skillQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// likePattern turns a search term into an ILIKE ... ESCAPE '\' pattern, or ""
// for no filter.
func likePattern(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}

// ResolveUserID implements SkillRepository.
func (r *skillRepository) ResolveUserID(ctx context.Context, email string) (string, bool, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id::text FROM "user"
		 WHERE lower(email) = lower($1)
		   AND user_type = 'INTERNAL'::user_type_enum
		   AND COALESCE(is_active, TRUE)
		 ORDER BY id
		 LIMIT 1`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve skill caller: %w", err)
	}
	return id, true, nil
}

// IsCRESRELead implements SkillRepository. The same lead roles
// ScheduleRepository.LeadTeamsFor reads, narrowed to the CRE and SRE families
// by the team-family expression the rota queries share.
func (r *skillRepository) IsCRESRELead(ctx context.Context, email string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM team_member tm
		    JOIN "user" u ON u.id = tm.user_id
		    JOIN team t   ON t.id = tm.team_id
		   WHERE lower(u.email) = lower($1)
		     AND u.user_type = 'INTERNAL'::user_type_enum
		     AND COALESCE(u.is_active, TRUE)
		     AND tm.role IN ('lead', 'americas_team_lead')
		     AND `+rosteredTeamWhere+`
		     AND `+teamFamilyExpr+` IN ('CRE', 'SRE'))`, email).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check skill manager: %w", err)
	}
	return ok, nil
}

// ListLevelTypes implements SkillRepository.
func (r *skillRepository) ListLevelTypes(ctx context.Context) ([]domain.SkillLevelType, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text, name, description FROM skill_level_type ORDER BY lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("query skill level types: %w", err)
	}
	defer rows.Close()
	out := []domain.SkillLevelType{}
	index := map[string]int{}
	for rows.Next() {
		var t domain.SkillLevelType
		if err := rows.Scan(&t.ID, &t.Name, &t.Description); err != nil {
			return nil, fmt.Errorf("scan skill level type: %w", err)
		}
		t.Levels = []domain.SkillLevel{}
		index[t.ID] = len(out)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill level types: %w", err)
	}
	lrows, err := r.db.Query(ctx, `
		SELECT level_type_id::text, id::text, name, value, description, color
		  FROM skill_level
		 ORDER BY level_type_id, value NULLS LAST, lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("query skill levels: %w", err)
	}
	defer lrows.Close()
	for lrows.Next() {
		var typeID string
		var l domain.SkillLevel
		if err := lrows.Scan(&typeID, &l.ID, &l.Name, &l.Value, &l.Description, &l.Color); err != nil {
			return nil, fmt.Errorf("scan skill level: %w", err)
		}
		if i, ok := index[typeID]; ok {
			out[i].Levels = append(out[i].Levels, l)
		}
	}
	return out, lrows.Err()
}

// ListCategories implements SkillRepository.
func (r *skillRepository) ListCategories(ctx context.Context) ([]domain.SkillCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name, code, category_path, is_lowest_category, parent_id::text
		  FROM skill_category
		 ORDER BY lower(COALESCE(category_path, name)), id`)
	if err != nil {
		return nil, fmt.Errorf("query skill categories: %w", err)
	}
	defer rows.Close()
	out := []domain.SkillCategory{}
	for rows.Next() {
		var c domain.SkillCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.CategoryPath, &c.IsLowestCategory, &c.ParentID); err != nil {
			return nil, fmt.Errorf("scan skill category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const skillColumns = `s.id::text, s.name, s.description, s.keywords, s.active, s.level_type_id::text, s.created_on, s.updated_on`

func scanSkill(row pgx.Row) (domain.Skill, error) {
	var s domain.Skill
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Keywords, &s.Active, &s.LevelTypeID, &s.CreatedOn, &s.UpdatedOn)
	s.Categories = []domain.SkillCategoryRef{}
	return s, err
}

// attachSkillCategories fills in each skill's categories with one query.
func attachSkillCategories(ctx context.Context, q skillQuerier, skills []domain.Skill) error {
	if len(skills) == 0 {
		return nil
	}
	ids := make([]string, len(skills))
	index := make(map[string]int, len(skills))
	for i, s := range skills {
		ids[i] = s.ID
		index[s.ID] = i
	}
	cats, err := categoriesBySkill(ctx, q, ids)
	if err != nil {
		return err
	}
	for id, refs := range cats {
		skills[index[id]].Categories = refs
	}
	return nil
}

func categoriesBySkill(ctx context.Context, q skillQuerier, skillIDs []string) (map[string][]domain.SkillCategoryRef, error) {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT l.skill_id::text, c.id::text, c.name, c.category_path
		  FROM skill_category_link l
		  JOIN skill_category c ON c.id = l.category_id
		 WHERE l.skill_id = ANY($1::uuid[])
		 ORDER BY 1, 3, 2`, skillIDs)
	if err != nil {
		return nil, fmt.Errorf("query skill categories: %w", err)
	}
	defer rows.Close()
	out := map[string][]domain.SkillCategoryRef{}
	for rows.Next() {
		var skillID string
		var ref domain.SkillCategoryRef
		if err := rows.Scan(&skillID, &ref.ID, &ref.Name, &ref.CategoryPath); err != nil {
			return nil, fmt.Errorf("scan skill category: %w", err)
		}
		out[skillID] = append(out[skillID], ref)
	}
	return out, rows.Err()
}

// A category filter includes the category's descendants: skills are filed
// under leaf categories, so a parent matches through its children.
const searchSkillsFrom = `
	WITH RECURSIVE wanted AS (
	    SELECT id FROM skill_category WHERE id = ANY($2::uuid[])
	    UNION
	    SELECT c.id FROM skill_category c JOIN wanted w ON c.parent_id = w.id
	)
	SELECT %s
	  FROM skill s
	 WHERE ($1 = '' OR s.name ILIKE $1 ESCAPE '\' OR s.keywords ILIKE $1 ESCAPE '\')
	   AND (cardinality($2::uuid[]) = 0 OR EXISTS (
	         SELECT 1 FROM skill_category_link l
	          WHERE l.skill_id = s.id AND l.category_id IN (SELECT id FROM wanted)))
	   AND ($3 = '' OR s.level_type_id = NULLIF($3, '')::uuid)
	   AND ($4 OR s.active)`

// SearchSkills implements SkillRepository.
func (r *skillRepository) SearchSkills(ctx context.Context, req domain.SearchSkillsRequest) ([]domain.Skill, int, error) {
	pattern := likePattern(req.SearchQuery)
	cats := req.CategoryIDs
	if cats == nil {
		cats = []string{}
	}
	var (
		total  int
		skills []domain.Skill
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := r.db.QueryRow(egCtx, fmt.Sprintf(searchSkillsFrom, "count(*)"),
			pattern, cats, req.LevelTypeID, req.IncludeInactive).Scan(&total)
		if err != nil {
			return fmt.Errorf("count skills: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, fmt.Sprintf(searchSkillsFrom, skillColumns)+`
		 ORDER BY lower(s.name), s.id
		 LIMIT $5 OFFSET $6`, pattern, cats, req.LevelTypeID, req.IncludeInactive, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query skills: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			s, err := scanSkill(rows)
			if err != nil {
				return fmt.Errorf("scan skill: %w", err)
			}
			skills = append(skills, s)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	if skills == nil {
		skills = []domain.Skill{}
	}
	if err := attachSkillCategories(ctx, r.db, skills); err != nil {
		return nil, 0, err
	}
	return skills, total, nil
}

// GetSkill implements SkillRepository.
func (r *skillRepository) GetSkill(ctx context.Context, id string) (*domain.Skill, error) {
	return getSkill(ctx, r.db, id)
}

func getSkill(ctx context.Context, q skillQuerier, id string) (*domain.Skill, error) {
	s, err := scanSkill(q.QueryRow(ctx, `SELECT `+skillColumns+` FROM skill s WHERE s.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get skill: %w", err)
	}
	one := []domain.Skill{s}
	if err := attachSkillCategories(ctx, q, one); err != nil {
		return nil, err
	}
	return &one[0], nil
}

// LevelTypeOfLevel implements SkillRepository.
func (r *skillRepository) LevelTypeOfLevel(ctx context.Context, levelID string) (string, bool, error) {
	var typeID string
	err := r.db.QueryRow(ctx, `SELECT level_type_id::text FROM skill_level WHERE id = $1::uuid`, levelID).Scan(&typeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("look up skill level: %w", err)
	}
	return typeID, true, nil
}

// LevelTypeExists implements SkillRepository.
func (r *skillRepository) LevelTypeExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill_level_type WHERE id = $1::uuid)`, id).Scan(&ok); err != nil {
		return false, fmt.Errorf("check skill level type: %w", err)
	}
	return ok, nil
}

// CategoriesExist implements SkillRepository.
func (r *skillRepository) CategoriesExist(ctx context.Context, ids []string) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	var n int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM skill_category WHERE id = ANY($1::uuid[])`, ids).Scan(&n); err != nil {
		return false, fmt.Errorf("check skill categories: %w", err)
	}
	return n == len(ids), nil
}

// userSkillCanonical keeps one row per person and skill. user_skill has no
// unique key on (user_id, skill_id) and the migrated rows are not known to be
// free of repeats, so every read and the writes agree on which row stands for
// the pair: an active one, a person's own before an inherited one, then the
// most recently updated.
const userSkillCanonical = `us.id = (
	SELECT x.id FROM user_skill x
	 WHERE x.user_id = us.user_id AND x.skill_id = us.skill_id AND x.active
	 ORDER BY x.inherited, x.updated_on DESC, x.id
	 LIMIT 1)`

const levelColumns = `sl.id::text, sl.name, sl.value, sl.description, sl.color`

// scanLevel builds an optional level from a LEFT JOINed set of columns.
func scanLevel(id *string, name *string, value *int, description, color *string) *domain.SkillLevel {
	if id == nil {
		return nil
	}
	l := &domain.SkillLevel{ID: *id, Value: value, Description: description, Color: color}
	if name != nil {
		l.Name = *name
	}
	return l
}

// listUserSkills reads a person's active skills, or just one of them.
func listUserSkills(ctx context.Context, q skillQuerier, userID, skillID string) ([]domain.UserSkill, error) {
	rows, err := q.Query(ctx, `
		SELECT us.skill_id::text, s.name, s.active, s.level_type_id::text, `+levelColumns+`,
		       us.inherited, us.inherited_from_group_id::text, us.updated_on
		  FROM user_skill us
		  JOIN skill s ON s.id = us.skill_id
		  LEFT JOIN skill_level sl ON sl.id = us.skill_level_id
		 WHERE us.user_id = $1::uuid
		   AND us.active
		   AND ($2 = '' OR us.skill_id = NULLIF($2, '')::uuid)
		   AND `+userSkillCanonical+`
		 ORDER BY lower(s.name), us.skill_id`, userID, skillID)
	if err != nil {
		return nil, fmt.Errorf("query user skills: %w", err)
	}
	defer rows.Close()
	out := []domain.UserSkill{}
	for rows.Next() {
		var u domain.UserSkill
		var lID, lName, lDesc, lColor *string
		var lValue *int
		if err := rows.Scan(&u.SkillID, &u.SkillName, &u.SkillActive, &u.LevelTypeID, &lID, &lName, &lValue, &lDesc, &lColor,
			&u.Inherited, &u.InheritedFromGroupID, &u.UpdatedOn); err != nil {
			return nil, fmt.Errorf("scan user skill: %w", err)
		}
		u.Level = scanLevel(lID, lName, lValue, lDesc, lColor)
		u.Categories = []domain.SkillCategoryRef{}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user skills: %w", err)
	}
	if len(out) == 0 {
		return out, nil
	}
	ids := make([]string, len(out))
	for i, u := range out {
		ids[i] = u.SkillID
	}
	cats, err := categoriesBySkill(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if refs, ok := cats[out[i].SkillID]; ok {
			out[i].Categories = refs
		}
	}
	return out, nil
}

// ListUserSkills implements SkillRepository.
func (r *skillRepository) ListUserSkills(ctx context.Context, userID string) ([]domain.UserSkill, error) {
	return listUserSkills(ctx, r.db, userID, "")
}

// userSkillRow is one stored user_skill row, as the writes see it.
type userSkillRow struct {
	id        string
	active    bool
	inherited bool
	levelID   *string
}

// inUserSkillTx runs fn in a transaction that holds the lock for one person's
// one skill, then hands it that pair's rows (own rows first). The lock is what
// stops two adds from both finding nothing and both inserting: with no unique
// key to refuse the second, the transaction itself has to.
func (r *skillRepository) inUserSkillTx(ctx context.Context, userID, skillID string, fn func(tx pgx.Tx, rows []userSkillRow) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('user_skill:' || $1 || ':' || $2, 0))`, userID, skillID); err != nil {
		return fmt.Errorf("lock user skill: %w", err)
	}
	rs, err := tx.Query(ctx, `
		SELECT id::text, active, inherited, skill_level_id::text
		  FROM user_skill
		 WHERE user_id = $1::uuid AND skill_id = $2::uuid
		 ORDER BY active DESC, inherited, updated_on DESC, id
		   FOR UPDATE`, userID, skillID)
	if err != nil {
		return fmt.Errorf("read user skill: %w", err)
	}
	var rows []userSkillRow
	for rs.Next() {
		var u userSkillRow
		if err := rs.Scan(&u.id, &u.active, &u.inherited, &u.levelID); err != nil {
			rs.Close()
			return fmt.Errorf("scan user skill: %w", err)
		}
		rows = append(rows, u)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return fmt.Errorf("iterate user skill: %w", err)
	}
	if err := fn(tx, rows); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// writeSkillHistory records one level change. from is nil the first time a
// skill is set, to is nil when it is removed.
func writeSkillHistory(ctx context.Context, tx pgx.Tx, userID, skillID string, from, to *string, actor string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_skill_history (created_on, updated_on, created_by, updated_by, user_id, skill_id, level_from_id, level_to_id)
		VALUES (NOW(), NOW(), $1, $1, $2::uuid, $3::uuid, $4::uuid, $5::uuid)`, actor, userID, skillID, from, to)
	if err != nil {
		return fmt.Errorf("write skill history: %w", err)
	}
	return nil
}

func readBack(ctx context.Context, tx pgx.Tx, userID, skillID string) (*domain.UserSkill, error) {
	rows, err := listUserSkills(ctx, tx, userID, skillID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// levelOnSkillScale reads the skill's scale under a share lock on the skill
// row, then checks the level is a rung of it. The lock is what UpdateSkill's
// FOR UPDATE queues behind, so the scale cannot change between this check and
// the commit of the write that follows. The same locked read gives the skill's
// active flag: an add (newOnProfile) is refused with SkillOutcomeSkillInactive
// when the skill was deactivated after the service looked, while a level change
// on an already-held skill is allowed whether or not it is active. It returns
// the outcome that refuses the write, or SkillOutcomeDone when the level may be
// written.
func levelOnSkillScale(ctx context.Context, tx pgx.Tx, skillID, levelID string, newOnProfile bool) (domain.SkillOutcome, error) {
	var scale *string
	var active bool
	err := tx.QueryRow(ctx, `SELECT level_type_id::text, active FROM skill WHERE id = $1::uuid FOR SHARE`, skillID).Scan(&scale, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SkillOutcomeNotFound, nil
	}
	if err != nil {
		return domain.SkillOutcomeDone, fmt.Errorf("lock skill scale: %w", err)
	}
	if newOnProfile && !active {
		return domain.SkillOutcomeSkillInactive, nil
	}
	if scale == nil {
		return domain.SkillOutcomeLevelNotInScale, nil
	}
	var onScale bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill_level WHERE id = $1::uuid AND level_type_id = $2::uuid)`, levelID, *scale).Scan(&onScale); err != nil {
		return domain.SkillOutcomeDone, fmt.Errorf("check level on scale: %w", err)
	}
	if !onScale {
		return domain.SkillOutcomeLevelNotInScale, nil
	}
	return domain.SkillOutcomeDone, nil
}

// AddUserSkill implements SkillRepository. An inactive row of the person's own
// (a migrated one) is brought back rather than leaving a second row beside it.
func (r *skillRepository) AddUserSkill(ctx context.Context, userID, skillID, levelID, actor string) (domain.SkillWriteResult, error) {
	var res domain.SkillWriteResult
	err := r.inUserSkillTx(ctx, userID, skillID, func(tx pgx.Tx, rows []userSkillRow) error {
		var revive *userSkillRow
		for i := range rows {
			switch {
			case rows[i].active && rows[i].inherited:
				res.Outcome = domain.SkillOutcomeInherited
				return nil
			case rows[i].active:
				res.Outcome = domain.SkillOutcomeAlreadyHeld
				return nil
			case !rows[i].inherited && revive == nil:
				revive = &rows[i]
			}
		}
		if outcome, err := levelOnSkillScale(ctx, tx, skillID, levelID, true); err != nil || outcome != domain.SkillOutcomeDone {
			res.Outcome = outcome
			return err
		}
		if revive != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE user_skill SET skill_level_id = $2::uuid, active = TRUE, updated_on = NOW(), updated_by = $3
				 WHERE id = $1::uuid`, revive.id, levelID, actor); err != nil {
				return fmt.Errorf("revive user skill: %w", err)
			}
		} else if _, err := tx.Exec(ctx, `
			INSERT INTO user_skill (created_on, updated_on, created_by, updated_by, user_id, skill_id, skill_level_id, active, inherited)
			VALUES (NOW(), NOW(), $1, $1, $2::uuid, $3::uuid, $4::uuid, TRUE, FALSE)`, actor, userID, skillID, levelID); err != nil {
			return fmt.Errorf("add user skill: %w", err)
		}
		if err := writeSkillHistory(ctx, tx, userID, skillID, nil, &levelID, actor); err != nil {
			return err
		}
		row, err := readBack(ctx, tx, userID, skillID)
		res.Outcome, res.Row = domain.SkillOutcomeDone, row
		return err
	})
	return res, err
}

// SetUserSkillLevel implements SkillRepository.
func (r *skillRepository) SetUserSkillLevel(ctx context.Context, userID, skillID, levelID, actor string) (domain.SkillWriteResult, error) {
	var res domain.SkillWriteResult
	err := r.inUserSkillTx(ctx, userID, skillID, func(tx pgx.Tx, rows []userSkillRow) error {
		cur, outcome := ownActiveRow(rows)
		if outcome != domain.SkillOutcomeDone {
			res.Outcome = outcome
			return nil
		}
		if cur.levelID != nil && *cur.levelID == levelID {
			row, err := readBack(ctx, tx, userID, skillID)
			res.Outcome, res.Row = domain.SkillOutcomeUnchanged, row
			return err
		}
		if outcome, err := levelOnSkillScale(ctx, tx, skillID, levelID, false); err != nil || outcome != domain.SkillOutcomeDone {
			res.Outcome = outcome
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE user_skill SET skill_level_id = $3::uuid, updated_on = NOW(), updated_by = $4
			 WHERE user_id = $1::uuid AND skill_id = $2::uuid AND active AND NOT inherited`,
			userID, skillID, levelID, actor); err != nil {
			return fmt.Errorf("change user skill level: %w", err)
		}
		if err := writeSkillHistory(ctx, tx, userID, skillID, cur.levelID, &levelID, actor); err != nil {
			return err
		}
		row, err := readBack(ctx, tx, userID, skillID)
		res.Outcome, res.Row = domain.SkillOutcomeDone, row
		return err
	})
	return res, err
}

// RemoveUserSkill implements SkillRepository. The row is deleted, not
// deactivated: the history keeps what the person held and when.
func (r *skillRepository) RemoveUserSkill(ctx context.Context, userID, skillID, actor string) (domain.SkillWriteResult, error) {
	var res domain.SkillWriteResult
	err := r.inUserSkillTx(ctx, userID, skillID, func(tx pgx.Tx, rows []userSkillRow) error {
		cur, outcome := ownActiveRow(rows)
		if outcome != domain.SkillOutcomeDone {
			res.Outcome = outcome
			return nil
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM user_skill WHERE user_id = $1::uuid AND skill_id = $2::uuid AND NOT inherited`,
			userID, skillID); err != nil {
			return fmt.Errorf("remove user skill: %w", err)
		}
		if err := writeSkillHistory(ctx, tx, userID, skillID, cur.levelID, nil, actor); err != nil {
			return err
		}
		res.Outcome = domain.SkillOutcomeDone
		return nil
	})
	return res, err
}

// ownActiveRow is the row an edit or a removal acts on, or why there is none:
// the person does not hold the skill, or what they hold is inherited.
func ownActiveRow(rows []userSkillRow) (userSkillRow, domain.SkillOutcome) {
	if len(rows) == 0 || !rows[0].active {
		return userSkillRow{}, domain.SkillOutcomeNotHeld
	}
	if rows[0].inherited {
		return userSkillRow{}, domain.SkillOutcomeInherited
	}
	return rows[0], domain.SkillOutcomeDone
}

// ListUserSkillHistory implements SkillRepository.
func (r *skillRepository) ListUserSkillHistory(ctx context.Context, userID, skillID string, limit, offset int) ([]domain.SkillHistoryEntry, int, error) {
	const where = `
		 WHERE h.user_id = $1::uuid
		   AND ($2 = '' OR h.skill_id = NULLIF($2, '')::uuid)`
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM user_skill_history h`+where, userID, skillID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count skill history: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT h.id::text, h.skill_id::text, s.name,
		       lf.id::text, lf.name, lf.value, lf.description, lf.color,
		       lt.id::text, lt.name, lt.value, lt.description, lt.color,
		       h.created_by, h.created_on
		  FROM user_skill_history h
		  JOIN skill s ON s.id = h.skill_id
		  LEFT JOIN skill_level lf ON lf.id = h.level_from_id
		  LEFT JOIN skill_level lt ON lt.id = h.level_to_id`+where+`
		 ORDER BY h.created_on DESC, h.id DESC
		 LIMIT $3 OFFSET $4`, userID, skillID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query skill history: %w", err)
	}
	defer rows.Close()
	out := []domain.SkillHistoryEntry{}
	for rows.Next() {
		var e domain.SkillHistoryEntry
		var fID, fName, fDesc, fColor, tID, tName, tDesc, tColor *string
		var fValue, tValue *int
		if err := rows.Scan(&e.ID, &e.SkillID, &e.SkillName,
			&fID, &fName, &fValue, &fDesc, &fColor,
			&tID, &tName, &tValue, &tDesc, &tColor,
			&e.ChangedBy, &e.ChangedOn); err != nil {
			return nil, 0, fmt.Errorf("scan skill history: %w", err)
		}
		e.LevelFrom = scanLevel(fID, fName, fValue, fDesc, fColor)
		e.LevelTo = scanLevel(tID, tName, tValue, tDesc, tColor)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

const searchUserSkillsFrom = `
	  FROM user_skill us
	  JOIN "user" u ON u.id = us.user_id
	               AND u.user_type = 'INTERNAL'::user_type_enum
	               AND COALESCE(u.is_active, TRUE)
	  JOIN skill s ON s.id = us.skill_id AND s.active
	  LEFT JOIN skill_level sl ON sl.id = us.skill_level_id
	 WHERE us.active
	   AND ` + userSkillCanonical + `
	   AND ($1 = '' OR u.name ILIKE $1 ESCAPE '\' OR u.email ILIKE $1 ESCAPE '\')
	   AND (cardinality($2::uuid[]) = 0 OR us.user_id = ANY($2))
	   AND (cardinality($3::uuid[]) = 0 OR us.skill_id = ANY($3))
	   AND (cardinality($4::uuid[]) = 0 OR us.skill_level_id = ANY($4))
	   AND ($5::int IS NULL OR sl.value >= $5)
	   AND (cardinality($6::text[]) = 0 OR EXISTS (
	         SELECT 1 FROM team_member tm JOIN team t ON t.id = tm.team_id
	          WHERE tm.user_id = u.id AND t.key = ANY($6)))`

// SearchUserSkills implements SkillRepository.
func (r *skillRepository) SearchUserSkills(ctx context.Context, req domain.SearchUserSkillsRequest) ([]domain.UserSkillRow, int, error) {
	pattern := likePattern(req.SearchQuery)
	nonNil := func(in []string) []string {
		if in == nil {
			return []string{}
		}
		return in
	}
	userIDs, skillIDs, levelIDs, teamKeys := nonNil(req.UserIDs), nonNil(req.SkillIDs), nonNil(req.LevelIDs), nonNil(req.TeamKeys)
	var (
		total int
		out   []domain.UserSkillRow
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, `SELECT count(*)`+searchUserSkillsFrom,
			pattern, userIDs, skillIDs, levelIDs, req.MinLevelValue, teamKeys).Scan(&total); err != nil {
			return fmt.Errorf("count user skills: %w", err)
		}
		return nil
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, `
		SELECT u.id::text, u.name, u.email,
		       COALESCE((SELECT array_agg(DISTINCT t.key ORDER BY t.key)
		                   FROM team_member tm JOIN team t ON t.id = tm.team_id
		                  WHERE tm.user_id = u.id AND t.key IS NOT NULL), '{}'::text[]),
		       us.skill_id::text, s.name, `+levelColumns+`, us.inherited, us.updated_on`+searchUserSkillsFrom+`
		 ORDER BY lower(COALESCE(u.name, u.email, '')), u.id, lower(s.name), us.skill_id
		 LIMIT $7 OFFSET $8`,
			pattern, userIDs, skillIDs, levelIDs, req.MinLevelValue, teamKeys, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return fmt.Errorf("query user skills: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var row domain.UserSkillRow
			var lID, lName, lDesc, lColor *string
			var lValue *int
			if err := rows.Scan(&row.UserID, &row.UserName, &row.UserEmail, &row.TeamKeys,
				&row.SkillID, &row.SkillName, &lID, &lName, &lValue, &lDesc, &lColor, &row.Inherited, &row.UpdatedOn); err != nil {
				return fmt.Errorf("scan user skill row: %w", err)
			}
			row.Level = scanLevel(lID, lName, lValue, lDesc, lColor)
			out = append(out, row)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.UserSkillRow{}
	}
	return out, total, nil
}

// CreateSkill implements SkillRepository.
func (r *skillRepository) CreateSkill(ctx context.Context, req domain.CreateSkillRequest, actor string) (domain.SkillWriteResult, error) {
	var res domain.SkillWriteResult
	err := r.inSkillTx(ctx, req.Name, func(tx pgx.Tx) error {
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill WHERE lower(name) = lower($1))`, req.Name).Scan(&taken); err != nil {
			return fmt.Errorf("check skill name: %w", err)
		}
		if taken {
			res.Outcome = domain.SkillOutcomeNameTaken
			return nil
		}
		active := req.Active == nil || *req.Active
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO skill (created_on, updated_on, created_by, updated_by, name, description, keywords, active, level_type_id)
			VALUES (NOW(), NOW(), $1, $1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6::uuid)
			RETURNING id::text`,
			actor, req.Name, derefString(req.Description), derefString(req.Keywords), active, req.LevelTypeID).Scan(&id); err != nil {
			return fmt.Errorf("create skill: %w", err)
		}
		if err := replaceSkillCategories(ctx, tx, id, req.CategoryIDs, actor); err != nil {
			return err
		}
		skill, err := getSkill(ctx, tx, id)
		res.Outcome, res.Skill = domain.SkillOutcomeDone, skill
		return err
	})
	return res, err
}

// UpdateSkill implements SkillRepository.
func (r *skillRepository) UpdateSkill(ctx context.Context, id string, req domain.UpdateSkillRequest, actor string) (domain.SkillWriteResult, error) {
	var res domain.SkillWriteResult
	nameKey := id
	if req.Name != nil {
		nameKey = *req.Name
	}
	err := r.inSkillTx(ctx, nameKey, func(tx pgx.Tx) error {
		var currentType *string
		err := tx.QueryRow(ctx, `SELECT level_type_id::text FROM skill WHERE id = $1::uuid FOR UPDATE`, id).Scan(&currentType)
		if errors.Is(err, pgx.ErrNoRows) {
			res.Outcome = domain.SkillOutcomeNotFound
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock skill: %w", err)
		}
		if req.Name != nil {
			var taken bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skill WHERE lower(name) = lower($1) AND id <> $2::uuid)`, *req.Name, id).Scan(&taken); err != nil {
				return fmt.Errorf("check skill name: %w", err)
			}
			if taken {
				res.Outcome = domain.SkillOutcomeNameTaken
				return nil
			}
		}
		// A person's level is a rung of the skill's scale, and the scale is
		// what makes it meaningful: it cannot be swapped under them.
		if req.LevelTypeID != nil && (currentType == nil || *currentType != *req.LevelTypeID) {
			var held bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_skill WHERE skill_id = $1::uuid AND skill_level_id IS NOT NULL)`, id).Scan(&held); err != nil {
				return fmt.Errorf("check skill levels in use: %w", err)
			}
			if held {
				res.Outcome = domain.SkillOutcomeLevelsInUse
				return nil
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE skill SET
			       name          = COALESCE($2, name),
			       description   = CASE WHEN $3::text IS NULL THEN description ELSE NULLIF($3, '') END,
			       keywords      = CASE WHEN $4::text IS NULL THEN keywords ELSE NULLIF($4, '') END,
			       active        = COALESCE($5, active),
			       level_type_id = COALESCE($6::uuid, level_type_id),
			       updated_on    = NOW(),
			       updated_by    = $7
			 WHERE id = $1::uuid`,
			id, req.Name, req.Description, req.Keywords, req.Active, req.LevelTypeID, actor); err != nil {
			return fmt.Errorf("update skill: %w", err)
		}
		if req.CategoryIDs != nil {
			if err := replaceSkillCategories(ctx, tx, id, *req.CategoryIDs, actor); err != nil {
				return err
			}
		}
		skill, err := getSkill(ctx, tx, id)
		res.Outcome, res.Skill = domain.SkillOutcomeDone, skill
		return err
	})
	return res, err
}

// inSkillTx runs fn in a transaction holding the lock for one skill name, so
// two writers cannot both find a name free and both take it (skill.name has no
// unique key).
func (r *skillRepository) inSkillTx(ctx context.Context, name string, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('skill_name:' || lower($1), 0))`, name); err != nil {
		return fmt.Errorf("lock skill name: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// replaceSkillCategories makes the skill's category links exactly ids.
func replaceSkillCategories(ctx context.Context, tx pgx.Tx, skillID string, ids []string, actor string) error {
	if ids == nil {
		ids = []string{}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM skill_category_link WHERE skill_id = $1::uuid AND NOT (category_id = ANY($2::uuid[]))`, skillID, ids); err != nil {
		return fmt.Errorf("unlink skill categories: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_category_link (created_on, updated_on, created_by, updated_by, skill_id, category_id)
		SELECT NOW(), NOW(), $1, $1, $2::uuid, c
		  FROM unnest($3::uuid[]) AS c
		 WHERE NOT EXISTS (SELECT 1 FROM skill_category_link l WHERE l.skill_id = $2::uuid AND l.category_id = c)`,
		actor, skillID, ids); err != nil {
		return fmt.Errorf("link skill categories: %w", err)
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
