package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// AnalyticsRepository serves the dashboard and the work queue.
type AnalyticsRepository interface {
	Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error)
	WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error)
}

type analyticsRepository struct{ db *pgxpool.Pool }

// NewAnalyticsRepository builds an AnalyticsRepository over the given pool.
func NewAnalyticsRepository(db *pgxpool.Pool) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

// rangeBounds turns the optional range into two non-null timestamps, so every
// query below binds the same two parameters unconditionally.
func rangeBounds(rng domain.AnalyticsRange) (time.Time, time.Time) {
	to := time.Now()
	if rng.To != nil {
		to = *rng.To
	}
	from := to.AddDate(0, 0, -180)
	if rng.From != nil {
		from = *rng.From
	}
	return from, to
}

func (r *analyticsRepository) Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error) {
	from, to := rangeBounds(rng)
	out := &domain.DashboardAnalytics{}

	// Two of these are deliberately not period-scoped. A backlog that shrinks
	// because someone narrowed the date range is a backlog nobody clears, and the
	// same goes for work currently in flight.
	const summaryQ = `
		SELECT
		  (SELECT COUNT(*)::INT FROM plg_organization WHERE created_on BETWEEN $1 AND $2),
		  (SELECT COUNT(*)::INT FROM plg_org_platform WHERE registered_on BETWEEN $1 AND $2),
		  (SELECT COUNT(*)::INT FROM plg_org_platform WHERE acknowledged_on IS NULL),
		  -- The same rule the work queue uses, read from the same view, so the
		  -- tile cannot disagree with the page it links to.
		  (SELECT COUNT(*)::INT FROM plg_work_queue_v),
		  -- Counts extensions as well as first trials, and counts each pairing
		  -- by the date its CURRENT tier makes live — so a pairing whose
		  -- original trial has passed but whose extension runs to next week is
		  -- due next week, not overdue. plg_current_period_end_date owns that
		  -- rule; this query does not restate it. It returns NULL for FREE and
		  -- PAYG, which drops them without needing a tier filter here.
		  (SELECT COUNT(*)::INT FROM plg_org_platform
		    WHERE plg_current_period_end_date(subscription_tier, trial_end_date,
		                                      trial_extended_date)
		          BETWEEN CURRENT_DATE AND CURRENT_DATE + 14)`

	s := &out.Summary
	if err := r.db.QueryRow(ctx, summaryQ, from, to).Scan(
		&s.TotalOrganizations, &s.TotalRegistrations, &s.NewRegistrations,
		&s.PairingsNeedingAttention, &s.TrialsEndingSoon); err != nil {
		return nil, fmt.Errorf("load dashboard summary: %w", err)
	}

	var err error
	if out.RegistrationsByProduct, err = r.countBy(ctx, `
		SELECT v.product_name, COUNT(*)::INT
		FROM   plg_org_platform_v v
		WHERE  v.registered_on BETWEEN $1 AND $2
		GROUP  BY v.product_name ORDER BY 2 DESC`, from, to); err != nil {
		return nil, err
	}

	// Ordered by the stage catalogue rather than by count, so the donut always
	// reads Registration → Abandoned instead of reshuffling as data moves.
	if out.RegistrationsByLifecycleStage, err = r.countBy(ctx, `
		SELECT ls.stage::TEXT, COUNT(op.id)::INT
		FROM   plg_lifecycle_stage ls
		LEFT   JOIN plg_org_platform op
		       ON op.lifecycle_stage = ls.stage AND op.registered_on BETWEEN $1 AND $2
		GROUP  BY ls.stage, ls.display_order
		HAVING COUNT(op.id) > 0
		ORDER  BY ls.display_order`, from, to); err != nil {
		return nil, err
	}

	if out.SubscriptionMix, err = r.countBy(ctx, `
		SELECT COALESCE(op.subscription_tier::TEXT, 'NOT_RECORDED'), COUNT(*)::INT
		FROM   plg_org_platform op
		WHERE  op.registered_on BETWEEN $1 AND $2
		GROUP  BY 1 ORDER BY 2 DESC`, from, to); err != nil {
		return nil, err
	}

	if out.RegistrationsOverTime, err = r.timeSeries(ctx, `
		SELECT TO_CHAR(DATE_TRUNC('week', v.registered_on), 'YYYY-MM-DD') AS bucket,
		       v.product_name AS series, COUNT(*)::INT
		FROM   plg_org_platform_v v
		WHERE  v.registered_on BETWEEN $1 AND $2
		GROUP  BY 1, 2 ORDER BY 1`, from, to); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *analyticsRepository) countBy(ctx context.Context, q string, from, to time.Time) ([]domain.CountByLabel, error) {
	rows, err := r.db.Query(ctx, q, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics breakdown: %w", err)
	}
	defer rows.Close()

	out := []domain.CountByLabel{}
	for rows.Next() {
		var c domain.CountByLabel
		if err := rows.Scan(&c.Label, &c.Count); err != nil {
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// timeSeries runs a (bucket, series, count) query and pivots it onto the shared
// bucket axis the charts expect.
func (r *analyticsRepository) timeSeries(ctx context.Context, q string, from, to time.Time) (domain.TimeSeries, error) {
	rows, err := r.db.Query(ctx, q, from, to)
	if err != nil {
		return domain.TimeSeries{}, fmt.Errorf("analytics time series: %w", err)
	}
	defer rows.Close()

	type point struct {
		bucket, series string
		count          int
	}
	var (
		points     []point
		buckets    []string
		seriesKeys []string
		seenBucket = map[string]bool{}
		seenSeries = map[string]bool{}
	)
	for rows.Next() {
		var p point
		if err := rows.Scan(&p.bucket, &p.series, &p.count); err != nil {
			return domain.TimeSeries{}, fmt.Errorf("scan time series: %w", err)
		}
		points = append(points, p)
		if !seenBucket[p.bucket] {
			seenBucket[p.bucket] = true
			buckets = append(buckets, p.bucket)
		}
		if !seenSeries[p.series] {
			seenSeries[p.series] = true
			seriesKeys = append(seriesKeys, p.series)
		}
	}
	if err := rows.Err(); err != nil {
		return domain.TimeSeries{}, fmt.Errorf("iterate time series: %w", err)
	}

	bucketIndex := make(map[string]int, len(buckets))
	for i, b := range buckets {
		bucketIndex[b] = i
	}
	out := domain.TimeSeries{Buckets: buckets, Series: make([]domain.TimeBucketSeries, 0, len(seriesKeys))}
	seriesIndex := make(map[string]int, len(seriesKeys))
	for i, k := range seriesKeys {
		seriesIndex[k] = i
		out.Series = append(out.Series, domain.TimeBucketSeries{Label: k, Counts: make([]int, len(buckets))})
	}
	for _, p := range points {
		out.Series[seriesIndex[p.series]].Counts[bucketIndex[p.bucket]] = p.count
	}
	if out.Buckets == nil {
		out.Buckets = []string{}
	}
	return out, nil
}

// queueConditions turns the queue filters into SQL.
//
// EVERY FILTER THE UI EXPOSES APPLIES TO BOTH the tiles and the list, so every
// number on the page counts the same set of pairings. Product, stage, health and
// reason are all filters an engineer can see and clear, and a tile that
// contradicts the list under it is read as a bug — it was reported as one: a
// product chip saying "4 waiting" above a list of 2, because the reason chips
// narrowed the list alone.
//
// Playbook and task stay list-only. They are not exposed on the queue page, so
// no tile can disagree with anything on screen because of them, and they are
// fine enough to fragment a stage tile into counts nobody asked for.
func queueConditions(f *domain.WorkQueueFilters, b *argBuilder, includeNarrow bool) []string {
	var conds []string
	// Health narrows the TILES as well as the list, alongside product and stage.
	// "Show me what is at risk" is a different queue, not a subset of the one on
	// screen, so the numbers have to move with it.
	if len(f.HealthStates) > 0 {
		conds = append(conds, `w.health_state::TEXT = ANY(`+b.add(stringsOf(f.HealthStates))+`::TEXT[])`)
	}
	if len(f.OwnerIDs) > 0 {
		conds = append(conds, `w.plg_cs_owner::TEXT = ANY(`+b.add(f.OwnerIDs)+`::TEXT[])`)
	}
	if len(f.OrganizationIDs) > 0 {
		conds = append(conds, `w.organization_id::TEXT = ANY(`+b.add(f.OrganizationIDs)+`::TEXT[])`)
	}
	if len(f.ProductCodes) > 0 {
		conds = append(conds, `w.product_code = ANY(`+b.add(f.ProductCodes)+`::TEXT[])`)
	}
	if len(f.LifecycleStages) > 0 {
		conds = append(conds, `w.lifecycle_stage::TEXT = ANY(`+
			b.add(stringsOf(f.LifecycleStages))+`::TEXT[])`)
	}
	if len(f.Reasons) > 0 {
		conds = append(conds, `w.reason = ANY(`+b.add(stringsOf(f.Reasons))+`::TEXT[])`)
	}
	if includeNarrow {
		if len(f.PlaybookIDs) > 0 {
			conds = append(conds, `w.playbook_id::TEXT = ANY(`+b.add(f.PlaybookIDs)+`::TEXT[])`)
		}
		if len(f.TaskCodes) > 0 {
			conds = append(conds, `w.next_task_code = ANY(`+b.add(f.TaskCodes)+`::TEXT[])`)
		}
	}
	return conds
}

// WorkQueue returns the tiles and the pairing list.
func (r *analyticsRepository) WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error) {
	resp := &domain.WorkQueueResponse{
		ProductGroups: []domain.ProductQueueGroup{},
		Items:         []domain.WorkQueueItem{},
		ByReason:      map[domain.QueueReason]int{},
	}

	groups, err := r.stageTiles(ctx, f)
	if err != nil {
		return nil, err
	}
	resp.ProductGroups = groups

	byReason, err := r.reasonTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	resp.ByReason = byReason

	b := &argBuilder{}
	conds := queueConditions(&f, b, true)

	q := `
		SELECT w.org_platform_id::TEXT, w.organization_id::TEXT, w.organization_name,
		       w.product_id::TEXT, w.product_code, w.product_name,
		       w.lifecycle_stage::TEXT, w.lifecycle_stage_name, w.reason,
		       w.run_active::INT, w.run_total::INT,
		       w.task_completed::INT, w.task_total::INT, w.available_total::INT,
		       w.playbook_id::TEXT, w.playbook_name, w.next_task_code, w.next_task_name,
		       w.plg_cs_owner::TEXT, w.plg_cs_owner_email, w.plg_cs_owner_name,
		       w.acknowledged_by::TEXT, w.acknowledged_by_email, w.acknowledged_by_name,
		       w.acknowledged_on, w.registered_on,
		       w.health_state::TEXT, w.health_entered_on
		-- Both LEFT JOINs on plg_cs_user are gone. plg_work_queue_v resolves the
		-- owner and the acknowledger itself, so "the owner's name" has one
		-- definition in the schema rather than one per query that needed it.
		FROM   plg_work_queue_v w` +
		whereClause(conds) + `
		-- At risk first, then work needing a decision, then oldest — so nothing
		-- sits unnoticed at the bottom.
		--
		-- Health outranks reason deliberately. A paying customer slipping away
		-- is worth more attention than a registration nobody has picked up.
		ORDER  BY CASE w.health_state WHEN 'AT_RISK' THEN 0 ELSE 1 END,
		          CASE w.reason
		              WHEN 'NO_PLAYBOOK' THEN 1
		              WHEN 'NOT_STARTED' THEN 2
		              ELSE 3
		          END,
		          w.registered_on
		LIMIT  500`

	rows, err := r.db.Query(ctx, q, b.list()...)
	if err != nil {
		return nil, fmt.Errorf("load work queue: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			it                             domain.WorkQueueItem
			prodID, prodCode, prodName     string
			ownerID, ownerEmail, ownerName *string
			ackID, ackEmail, ackName       *string
		)
		if err := rows.Scan(&it.OrgPlatformID, &it.OrganizationID, &it.OrganizationName,
			&prodID, &prodCode, &prodName,
			&it.CurrentStage, &it.StageName, &it.Reason,
			&it.RunActive, &it.RunTotal,
			&it.TaskCompleted, &it.TaskTotal, &it.AvailablePlaybooks,
			&it.PlaybookID, &it.PlaybookName, &it.NextTaskCode, &it.NextTaskName,
			&ownerID, &ownerEmail, &ownerName,
			&ackID, &ackEmail, &ackName,
			&it.AcknowledgedOn, &it.RegisteredOn,
			&it.HealthState, &it.HealthEnteredOn); err != nil {
			return nil, fmt.Errorf("scan work queue item: %w", err)
		}
		it.Product = productRef(prodID, prodCode, prodName)
		it.Owner = userRef(ownerID, ownerEmail, ownerName)
		it.AcknowledgedBy = userRef(ackID, ackEmail, ackName)
		resp.Items = append(resp.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work queue: %w", err)
	}
	resp.Total = len(resp.Items)
	return resp, nil
}

// stageTiles counts pairings by product and lifecycle stage.
//
// PlgProduct and stage partition the queue — every pairing has exactly one of each
// — so these tiles sum to the list total. Grouping by playbook did not: a
// pairing running two playbooks counted twice, and one running none counted
// nowhere.
func (r *analyticsRepository) stageTiles(ctx context.Context, f domain.WorkQueueFilters) (
	[]domain.ProductQueueGroup, error) {

	b := &argBuilder{}
	conds := queueConditions(&f, b, false)

	q := `
		SELECT w.product_id::TEXT, w.product_code, w.product_name, w.product_display_order,
		       w.lifecycle_stage::TEXT, w.lifecycle_stage_name, w.lifecycle_stage_order,
		       COUNT(*)::INT,
		       COUNT(*) FILTER (WHERE w.health_state = 'AT_RISK')::INT,
		       COUNT(*) FILTER (WHERE w.reason = 'NO_PLAYBOOK')::INT,
		       COUNT(*) FILTER (WHERE w.reason = 'NOT_STARTED')::INT,
		       COUNT(*) FILTER (WHERE w.reason = 'IN_PROGRESS')::INT
		FROM   plg_work_queue_v w` +
		whereClause(conds) + `
		GROUP  BY w.product_id, w.product_code, w.product_name, w.product_display_order,
		          w.lifecycle_stage, w.lifecycle_stage_name, w.lifecycle_stage_order
		ORDER  BY w.product_display_order, w.lifecycle_stage_order`

	rows, err := r.db.Query(ctx, q, b.list()...)
	if err != nil {
		return nil, fmt.Errorf("load work queue tiles: %w", err)
	}
	defer rows.Close()

	groups := []domain.ProductQueueGroup{}
	index := map[string]int{}

	for rows.Next() {
		var (
			prodID, prodCode, prodName string
			prodOrder, stageOrder      int
			tile                       domain.StageTile
		)
		if err := rows.Scan(&prodID, &prodCode, &prodName, &prodOrder,
			&tile.LifecycleStage, &tile.StageName, &stageOrder,
			&tile.Pairings, &tile.AtRisk, &tile.NoPlaybook,
			&tile.NotStarted, &tile.InProgress); err != nil {
			return nil, fmt.Errorf("scan work queue tile: %w", err)
		}

		i, seen := index[prodCode]
		if !seen {
			groups = append(groups, domain.ProductQueueGroup{
				Product: productRef(prodID, prodCode, prodName),
				Tiles:   []domain.StageTile{},
			})
			i = len(groups) - 1
			index[prodCode] = i
		}
		groups[i].Tiles = append(groups[i].Tiles, tile)
		groups[i].Pairings += tile.Pairings
	}
	return groups, rows.Err()
}

// reasonTotals counts the queue by reason, IGNORING any reason the caller
// selected — and that exception is the whole point of it being its own query.
//
// The reason chips are how an engineer moves between kinds of work, so each one
// has to keep showing what it would find. Narrowed alongside everything else,
// selecting "Not started" would zero the other two chips and the row would stop
// saying where the remaining work is. Every other filter still applies, so the
// breakdown describes the queue being looked at rather than the whole database.
func (r *analyticsRepository) reasonTotals(ctx context.Context, f domain.WorkQueueFilters) (
	map[domain.QueueReason]int, error) {

	broad := f
	broad.Reasons = nil

	b := &argBuilder{}
	conds := queueConditions(&broad, b, false)

	q := `
		SELECT COUNT(*) FILTER (WHERE w.reason = 'NO_PLAYBOOK')::INT,
		       COUNT(*) FILTER (WHERE w.reason = 'NOT_STARTED')::INT,
		       COUNT(*) FILTER (WHERE w.reason = 'IN_PROGRESS')::INT
		FROM   plg_work_queue_v w` + whereClause(conds)

	var noPlaybook, notStarted, inProgress int
	if err := r.db.QueryRow(ctx, q, b.list()...).
		Scan(&noPlaybook, &notStarted, &inProgress); err != nil {
		return nil, fmt.Errorf("load work queue reason totals: %w", err)
	}
	return map[domain.QueueReason]int{
		domain.ReasonNoPlaybook: noPlaybook,
		domain.ReasonNotStarted: notStarted,
		domain.ReasonInProgress: inProgress,
	}, nil
}
