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

// Port of v3's src/server/lib/overview.ts, section by section, keeping its
// section comments — they document which filters each section honors,
// which is the part SPEC §6.6 requires replicated exactly:
//
//	hero + spark honor repo + priority; priorities + matrix honor repo only;
//	projects and volume ignore both filters (volume: last 12 UTC weeks).
package metrics

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/taxonomy"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pCodeRe = regexp.MustCompile(`\((P[1-4])\)`)
	// priorityRank orders the 4 canonical tiers; anything else sorts last.
	priorityRank = map[string]int{"P1": 1, "P2": 2, "P3": 3, "P4": 4}
)

func pCode(priority string) string {
	if m := pCodeRe.FindStringSubmatch(priority); m != nil {
		return m[1]
	}
	return priority
}

var pLabelSuffixRe = regexp.MustCompile(`\s*\(P[1-4]\)\s*$`)

func pLabel(priority string) string {
	return strings.TrimSpace(pLabelSuffixRe.ReplaceAllString(priority, ""))
}

// --- Wire types (SPEC §6.6) ---

type Filters struct {
	Repo     *string `json:"repo"`
	Priority *string `json:"priority"`
}

type HeroMetric struct {
	N     int   `json:"n"`
	Delta int   `json:"delta"`
	Spark []int `json:"spark"`
}

type HeroCsByStatus struct {
	Status string `json:"status"`
	N      int    `json:"n"`
}

type HeroCs struct {
	N        int              `json:"n"`
	ByStatus []HeroCsByStatus `json:"byStatus"`
}

type Hero struct {
	Violated    HeroMetric `json:"violated"`
	AtRisk      HeroMetric `json:"atRisk"`
	Cs          HeroCs     `json:"cs"`
	ProductSide HeroMetric `json:"productSide"`
}

type Project struct {
	RepoID      int32  `json:"repoId"`
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Violated    int    `json:"violated"`
	AtRisk      int    `json:"atRisk"`
	Cs          int    `json:"cs"`
	OnTrack     int    `json:"onTrack"`
	OpenTracked int    `json:"openTracked"`
	Untracked   int    `json:"untracked"`
	Worst       bool   `json:"worst"`
	AllClear    bool   `json:"allClear"`
}

type Priority struct {
	Key         string  `json:"key"`
	Code        string  `json:"code"`
	Label       string  `json:"label"`
	BudgetHours float64 `json:"budgetHours"`
	Violated    int     `json:"violated"`
	AtRisk      int     `json:"atRisk"`
	Cs          int     `json:"cs"`
	OnTrack     int     `json:"onTrack"`
	Total       int     `json:"total"`
}

type MatrixCells struct {
	Violated int `json:"violated"`
	AtRisk   int `json:"atRisk"`
	OnTrack  int `json:"onTrack"`
	Cs       int `json:"cs"`
}

type MatrixRow struct {
	Key   string      `json:"key"`
	Code  string      `json:"code"`
	Cells MatrixCells `json:"cells"`
	Total int         `json:"total"`
}

type Matrix struct {
	Rows       []MatrixRow `json:"rows"`
	Totals     MatrixCells `json:"totals"`
	GrandTotal int         `json:"grandTotal"`
}

type WeekBuckets struct {
	P1 int `json:"P1"`
	P2 int `json:"P2"`
	P3 int `json:"P3"`
	P4 int `json:"P4"`
}

type Week struct {
	WeekStart  string      `json:"weekStart"`
	ByPriority WeekBuckets `json:"byPriority"`
	Total      int         `json:"total"`
}

type Volume struct {
	RepoID int32  `json:"repoId"`
	Name   string `json:"name"`
	Total  int    `json:"total"`
	Weeks  []Week `json:"weeks"`
}

type Overview struct {
	RefreshedAt string     `json:"refreshedAt"`
	Filters     Filters    `json:"filters"`
	Hero        Hero       `json:"hero"`
	Projects    []Project  `json:"projects"`
	Priorities  []Priority `json:"priorities"`
	Matrix      Matrix     `json:"matrix"`
	Volume      []Volume   `json:"volume"`
}

// overviewIssue is one row of the base "open, non-terminal, enabled-repo"
// issue set section 1 loads once and every other section filters/aggregates
// in memory, mirroring the reference's single findMany + in-memory
// aggregation approach.
type overviewIssue struct {
	Priority      *string
	CurrentStatus *string
	RepositoryID  int32
	RepoOwner     string
	RepoName      string
	ProjectTitle  *string
	SlaState      *string
}

func statusOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func slaStateOf(s *string) string {
	if s == nil {
		return "NO_SLA"
	}
	return *s
}

// BuildOverview builds the /metrics/overview response for the given
// optional repo ("owner/name") and priority filters.
func BuildOverview(ctx context.Context, pool *pgxpool.Pool, cfg *config.AppConfig, repo, priority *string) (Overview, error) {
	csStatuses := taxonomy.CsStatuses(cfg)
	isCsStatus := func(s string) bool { return slices.Contains(csStatuses, s) }
	productSideStatuses := taxonomy.ProductSideStatuses(cfg)
	isProductSideStatus := func(s string) bool { return slices.Contains(productSideStatuses, s) }

	var repoOwner, repoName string
	if repo != nil {
		repoOwner, repoName, _ = strings.Cut(*repo, "/")
	}

	// ── 1. All open non-terminal issues from enabled repos (narrow select) ──
	allIssues, err := fetchOverviewIssues(ctx, pool)
	if err != nil {
		return Overview{}, fmt.Errorf("metrics: fetch overview issues: %w", err)
	}
	budgets := cfg.Budgets // priority -> budgetHours, in config file order

	// Filter scopes: hero + spark honor repo + priority; priorities + matrix honor repo only.
	matchesRepo := func(i overviewIssue) bool {
		return repo == nil || (i.RepoOwner == repoOwner && i.RepoName == repoName)
	}
	matchesPriority := func(i overviewIssue) bool {
		return priority == nil || (i.Priority != nil && *i.Priority == *priority)
	}
	var heroIssues, repoFilteredIssues []overviewIssue
	for _, i := range allIssues {
		if matchesRepo(i) {
			repoFilteredIssues = append(repoFilteredIssues, i)
			if matchesPriority(i) {
				heroIssues = append(heroIssues, i)
			}
		}
	}

	// ── 2. Spark + delta (last 16 days; delta = today − yesterday) ──────────
	violatedSparkRows, err := sparkQuery(ctx, pool, "VIOLATED", repo, priority)
	if err != nil {
		return Overview{}, fmt.Errorf("metrics: violated spark query: %w", err)
	}
	atRiskSparkRows, err := sparkQuery(ctx, pool, "AT_RISK", repo, priority)
	if err != nil {
		return Overview{}, fmt.Errorf("metrics: at_risk spark query: %w", err)
	}
	productSideSparkRows, err := productSideSparkQuery(ctx, pool, productSideStatuses, repo, priority)
	if err != nil {
		return Overview{}, fmt.Errorf("metrics: product-side spark query: %w", err)
	}

	violatedSpark := fillSpark(violatedSparkRows, 16)
	atRiskSpark := fillSpark(atRiskSparkRows, 16)
	productSideSpark := fillSpark(productSideSparkRows, 16)
	violatedDelta := violatedSpark[15] - violatedSpark[14]
	atRiskDelta := atRiskSpark[15] - atRiskSpark[14]
	productSideDelta := productSideSpark[15] - productSideSpark[14]

	// ── 3. Hero aggregation (repo + priority filtered) ───────────────────────
	var heroViolated, heroAtRisk, heroCs, heroProductSide int
	heroCsByStatus := make(map[string]int, len(csStatuses))
	for _, s := range csStatuses {
		heroCsByStatus[s] = 0
	}
	for _, issue := range heroIssues {
		state := slaStateOf(issue.SlaState)
		status := statusOf(issue.CurrentStatus)
		if state == "VIOLATED" {
			heroViolated++
		}
		if state == "AT_RISK" {
			heroAtRisk++
		}
		if isCsStatus(status) {
			heroCs++
			heroCsByStatus[status]++
		}
		if isProductSideStatus(status) {
			heroProductSide++
		}
	}

	// ── 4. Projects (always all enabled repos; per-card counts honor priority) ─
	repoOrder := make([]int32, 0)
	repoMap := make(map[int32]*Project)
	for _, issue := range allIssues {
		if _, ok := repoMap[issue.RepositoryID]; !ok {
			name := issue.RepoName
			if issue.ProjectTitle != nil {
				name = *issue.ProjectTitle
			}
			repoMap[issue.RepositoryID] = &Project{
				RepoID: issue.RepositoryID,
				Name:   name,
				Repo:   issue.RepoOwner + "/" + issue.RepoName,
			}
			repoOrder = append(repoOrder, issue.RepositoryID)
		}
	}

	var projectScope []overviewIssue
	if priority != nil {
		for _, i := range allIssues {
			if i.Priority != nil && *i.Priority == *priority {
				projectScope = append(projectScope, i)
			}
		}
	} else {
		projectScope = allIssues
	}
	for _, issue := range projectScope {
		state := slaStateOf(issue.SlaState)
		status := statusOf(issue.CurrentStatus)
		isCs := isCsStatus(status)
		p := repoMap[issue.RepositoryID]
		if state == "VIOLATED" {
			p.Violated++
		}
		if state == "AT_RISK" {
			p.AtRisk++
		}
		if isCs {
			p.Cs++
		}
		if state == "OK" && !isCs {
			p.OnTrack++
		}
		if issue.Priority != nil {
			p.OpenTracked++
		} else {
			p.Untracked++
		}
	}

	worstScore := func(p Project) int { return p.Violated*3 + p.AtRisk }
	maxScore := 0
	for _, id := range repoOrder {
		if s := worstScore(*repoMap[id]); s > maxScore {
			maxScore = s
		}
	}
	projects := make([]Project, 0, len(repoOrder))
	for _, id := range repoOrder {
		p := *repoMap[id]
		p.Worst = maxScore > 0 && worstScore(p) == maxScore
		p.AllClear = p.Violated+p.AtRisk+p.Cs == 0
		projects = append(projects, p)
	}

	// ── 5. Priorities (honors repo; always the 4 canonical tiers) ───────────
	budgetMap := make(map[string]float64, len(budgets))
	for _, b := range budgets {
		budgetMap[b.Priority] = b.BudgetHours
	}
	type priAgg struct {
		Violated, AtRisk, Cs, OnTrack, Total int
	}
	priorityOrder := make([]string, 0, len(budgets))
	priorityAgg := make(map[string]*priAgg, len(budgets))
	for _, b := range budgets {
		priorityAgg[b.Priority] = &priAgg{}
		priorityOrder = append(priorityOrder, b.Priority)
	}
	for _, issue := range repoFilteredIssues {
		if issue.Priority == nil {
			continue
		}
		p, ok := priorityAgg[*issue.Priority]
		if !ok {
			continue // non-canonical priority label — not one of the 4 tiers
		}
		state := slaStateOf(issue.SlaState)
		isCs := isCsStatus(statusOf(issue.CurrentStatus))
		p.Total++
		if state == "VIOLATED" {
			p.Violated++
		}
		if state == "AT_RISK" {
			p.AtRisk++
		}
		if isCs {
			p.Cs++
		}
		if state == "OK" && !isCs {
			p.OnTrack++
		}
	}

	priorities := make([]Priority, 0, len(priorityOrder))
	for _, key := range priorityOrder {
		agg := priorityAgg[key]
		priorities = append(priorities, Priority{
			Key: key, Code: pCode(key), Label: pLabel(key), BudgetHours: budgetMap[key],
			Violated: agg.Violated, AtRisk: agg.AtRisk, Cs: agg.Cs, OnTrack: agg.OnTrack, Total: agg.Total,
		})
	}
	rankOf := func(code string) int {
		if r, ok := priorityRank[code]; ok {
			return r
		}
		return 99
	}
	sort.SliceStable(priorities, func(i, j int) bool { return rankOf(priorities[i].Code) < rankOf(priorities[j].Code) })

	// ── 6. Matrix (honors repo; all 4 tiers, independent cells) ─────────────
	matrixRows := make([]MatrixRow, 0, len(priorities))
	for _, p := range priorities {
		var violated, atRisk, onTrack, cs, total int
		for _, i := range repoFilteredIssues {
			if i.Priority == nil || *i.Priority != p.Key {
				continue
			}
			total++
			state := slaStateOf(i.SlaState)
			isCs := isCsStatus(statusOf(i.CurrentStatus))
			if state == "VIOLATED" {
				violated++
			}
			if state == "AT_RISK" {
				atRisk++
			}
			if state == "OK" && !isCs {
				onTrack++
			}
			if isCs {
				cs++
			}
		}
		matrixRows = append(matrixRows, MatrixRow{
			Key: p.Key, Code: p.Code,
			Cells: MatrixCells{Violated: violated, AtRisk: atRisk, OnTrack: onTrack, Cs: cs},
			Total: total,
		})
	}

	var grandTotal int
	var matrixTotals MatrixCells
	for _, i := range repoFilteredIssues {
		if i.Priority == nil {
			continue
		}
		if _, ok := budgetMap[*i.Priority]; !ok {
			continue
		}
		grandTotal++
		state := slaStateOf(i.SlaState)
		isCs := isCsStatus(statusOf(i.CurrentStatus))
		if state == "VIOLATED" {
			matrixTotals.Violated++
		}
		if state == "AT_RISK" {
			matrixTotals.AtRisk++
		}
		if state == "OK" && !isCs {
			matrixTotals.OnTrack++
		}
		if isCs {
			matrixTotals.Cs++
		}
	}

	// ── 7. Volume (ignores both filters — 12 UTC weeks of tracked issues) ───
	volume, err := buildVolume(ctx, pool, repoOrder, repoMap)
	if err != nil {
		return Overview{}, fmt.Errorf("metrics: build volume: %w", err)
	}

	heroCsByStatusWire := make([]HeroCsByStatus, len(csStatuses))
	for i, s := range csStatuses {
		heroCsByStatusWire[i] = HeroCsByStatus{Status: s, N: heroCsByStatus[s]}
	}

	return Overview{
		RefreshedAt: time.Now().UTC().Format(time.RFC3339),
		Filters:     Filters{Repo: repo, Priority: priority},
		Hero: Hero{
			Violated:    HeroMetric{N: heroViolated, Delta: violatedDelta, Spark: violatedSpark},
			AtRisk:      HeroMetric{N: heroAtRisk, Delta: atRiskDelta, Spark: atRiskSpark},
			Cs:          HeroCs{N: heroCs, ByStatus: heroCsByStatusWire},
			ProductSide: HeroMetric{N: heroProductSide, Delta: productSideDelta, Spark: productSideSpark},
		},
		Projects:   projects,
		Priorities: priorities,
		Matrix:     Matrix{Rows: matrixRows, Totals: matrixTotals, GrandTotal: grandTotal},
		Volume:     volume,
	}, nil
}

func fetchOverviewIssues(ctx context.Context, pool *pgxpool.Pool) ([]overviewIssue, error) {
	rows, err := pool.Query(ctx, `
		SELECT i.priority, i.current_status, r.id, r.owner, r.name, p.title, s.sla_state
		FROM issues i
		JOIN repositories r ON r.id = i.repository_id
		LEFT JOIN projects p ON p.id = r.sla_project_id
		LEFT JOIN issue_sla s ON s.issue_id = i.id
		WHERE i.state = 'OPEN' AND r.enabled = true AND s.sla_state IS DISTINCT FROM 'TERMINAL'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []overviewIssue
	for rows.Next() {
		var i overviewIssue
		if err := rows.Scan(&i.Priority, &i.CurrentStatus, &i.RepositoryID, &i.RepoOwner, &i.RepoName, &i.ProjectTitle, &i.SlaState); err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

type sparkRow struct {
	SnapshotDate time.Time
	N            int
}

func sparkQuery(ctx context.Context, pool *pgxpool.Pool, slaState string, repo, priority *string) ([]sparkRow, error) {
	sql := `
		SELECT s.snapshot_date, COUNT(*)::int AS n
		FROM sla_snapshots s
		JOIN issues i ON i.id = s.issue_id
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.sla_state = $1
		  AND s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - 15
		  AND i.state = 'OPEN'
		  AND r.enabled = true
	`
	args := []any{slaState}
	sql, args = appendRepoAndPriorityFilters(sql, args, repo, priority)
	sql += ` GROUP BY s.snapshot_date ORDER BY s.snapshot_date`
	return runSparkQuery(ctx, pool, sql, args)
}

// productSideSparkQuery mirrors sparkQuery but filters by current_status IN
// (...) instead of sla_state. An empty PRODUCT_SIDE category (a valid
// taxonomy.yaml edit) short-circuits to no rows rather than issuing a query
// with an empty IN-list.
func productSideSparkQuery(ctx context.Context, pool *pgxpool.Pool, productSideStatuses []string, repo, priority *string) ([]sparkRow, error) {
	if len(productSideStatuses) == 0 {
		return nil, nil
	}
	sql := `
		SELECT s.snapshot_date, COUNT(*)::int AS n
		FROM sla_snapshots s
		JOIN issues i ON i.id = s.issue_id
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.current_status = ANY($1)
		  AND s.snapshot_date >= (now() AT TIME ZONE 'UTC')::date - 15
		  AND i.state = 'OPEN'
		  AND r.enabled = true
	`
	args := []any{productSideStatuses}
	sql, args = appendRepoAndPriorityFilters(sql, args, repo, priority)
	sql += ` GROUP BY s.snapshot_date ORDER BY s.snapshot_date`
	return runSparkQuery(ctx, pool, sql, args)
}

func appendRepoAndPriorityFilters(sql string, args []any, repo, priority *string) (string, []any) {
	if repo != nil {
		owner, name, _ := strings.Cut(*repo, "/")
		args = append(args, owner, name)
		sql += fmt.Sprintf(" AND r.owner = $%d AND r.name = $%d", len(args)-1, len(args))
	}
	if priority != nil {
		args = append(args, *priority)
		sql += fmt.Sprintf(" AND s.priority = $%d", len(args))
	}
	return sql, args
}

func runSparkQuery(ctx context.Context, pool *pgxpool.Pool, sql string, args []any) ([]sparkRow, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sparkRow
	for rows.Next() {
		var r sparkRow
		if err := rows.Scan(&r.SnapshotDate, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// fillSpark builds a days-element array (oldest -> newest) for the last
// `days` days ending today (UTC), gap-filled with zero.
func fillSpark(rows []sparkRow, days int) []int {
	byDate := make(map[string]int, len(rows))
	for _, r := range rows {
		byDate[r.SnapshotDate.UTC().Format("2006-01-02")] = r.N
	}
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	out := make([]int, days)
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, -(days - 1 - i))
		out[i] = byDate[d.Format("2006-01-02")]
	}
	return out
}

type weekRow struct {
	RepositoryID int32
	WeekStart    time.Time
	Priority     string
	N            int
}

func fetchVolumeWeeks(ctx context.Context, pool *pgxpool.Pool) ([]weekRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			i.repository_id,
			date_trunc('week', i.github_created_at AT TIME ZONE 'UTC')::date AS wk,
			i.priority AS priority,
			COUNT(*)::int AS n
		FROM issues i
		JOIN repositories r ON r.id = i.repository_id
		WHERE i.github_created_at >= (date_trunc('week', now() AT TIME ZONE 'UTC') - INTERVAL '11 weeks') AT TIME ZONE 'UTC'
		  AND r.enabled = true
		  AND i.priority IS NOT NULL
		GROUP BY i.repository_id, wk, i.priority
		ORDER BY i.repository_id, wk
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []weekRow
	for rows.Next() {
		var r weekRow
		if err := rows.Scan(&r.RepositoryID, &r.WeekStart, &r.Priority, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// buildVolume is overview.ts section 7: ignores both repo/priority filters —
// last 12 UTC weeks (Monday-aligned) of tracked-issue creation volume, one
// row per project in repoOrder's insertion order.
func buildVolume(ctx context.Context, pool *pgxpool.Pool, repoOrder []int32, repoMap map[int32]*Project) ([]Volume, error) {
	rows, err := fetchVolumeWeeks(ctx, pool)
	if err != nil {
		return nil, err
	}

	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	daysToMonday := (int(today.Weekday()) + 6) % 7 // Weekday(): Sunday=0 ... Saturday=6
	currentMonday := today.AddDate(0, 0, -daysToMonday)

	weekKeys := make([]string, 12)
	for i := 0; i < 12; i++ {
		w := currentMonday.AddDate(0, 0, -(11-i)*7)
		weekKeys[i] = w.Format("2006-01-02")
	}

	weekByRepo := make(map[int32]map[string]*WeekBuckets)
	for _, row := range rows {
		wkMap, ok := weekByRepo[row.RepositoryID]
		if !ok {
			wkMap = make(map[string]*WeekBuckets)
			weekByRepo[row.RepositoryID] = wkMap
		}
		wkKey := row.WeekStart.UTC().Format("2006-01-02")
		b, ok := wkMap[wkKey]
		if !ok {
			b = &WeekBuckets{}
			wkMap[wkKey] = b
		}
		switch pCode(row.Priority) {
		case "P1":
			b.P1 += row.N
		case "P2":
			b.P2 += row.N
		case "P3":
			b.P3 += row.N
		case "P4":
			b.P4 += row.N
		}
	}

	volume := make([]Volume, 0, len(repoOrder))
	for _, id := range repoOrder {
		weeks := make([]Week, len(weekKeys))
		total := 0
		for i, wk := range weekKeys {
			b := WeekBuckets{}
			if m := weekByRepo[id]; m != nil {
				if existing, ok := m[wk]; ok {
					b = *existing
				}
			}
			weekTotal := b.P1 + b.P2 + b.P3 + b.P4
			weeks[i] = Week{WeekStart: wk, ByPriority: b, Total: weekTotal}
			total += weekTotal
		}
		volume = append(volume, Volume{RepoID: id, Name: repoMap[id].Name, Total: total, Weeks: weeks})
	}
	return volume, nil
}
