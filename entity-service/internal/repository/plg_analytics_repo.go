package repository

import (
	"context"
	"fmt"
	"strings"
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
		          BETWEEN CURRENT_DATE AND CURRENT_DATE + 14),
		  -- Registrations the ingest refused and nobody has cleared. The same
		  -- predicate failureRepository.OpenCount uses, and the one
		  -- idx_plg_ingest_failure_open is partial on. Not period-scoped: these
		  -- are a backlog, and bounding them by the dashboard's date range would
		  -- hide the oldest ones, which are the ones that have been waiting
		  -- longest.
		  (SELECT COUNT(*)::INT FROM plg_ingest_failure WHERE resolved_on IS NULL)`

	s := &out.Summary
	if err := r.db.QueryRow(ctx, summaryQ, from, to).Scan(
		&s.TotalOrganizations, &s.TotalRegistrations, &s.NewRegistrations,
		&s.PairingsNeedingAttention, &s.TrialsEndingSoon,
		&s.UnresolvedIngestFailures); err != nil {
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

	// One point per UTC day across the period, zeros included, counting each
	// organisation once (its entry registration) so the line is the day's signups.
	if out.RegistrationsOverTime, err = r.timeSeries(ctx, `
		SELECT TO_CHAR(d.day, 'YYYY-MM-DD'), 'Choreo Cloud Registrations', COALESCE(c.n, 0)::INT
		FROM   generate_series(($1::timestamptz AT TIME ZONE 'UTC')::date,
		                       ($2::timestamptz AT TIME ZONE 'UTC')::date,
		                       INTERVAL '1 day') AS d (day)
		LEFT   JOIN (SELECT (op.registered_on AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS n
		             FROM   plg_org_platform op
		             WHERE  op.registered_on BETWEEN $1 AND $2
		               `+entryRegistrationOnly+`
		             GROUP  BY 1) c ON c.day = d.day::date
		ORDER  BY d.day`, from, to); err != nil {
		return nil, err
	}

	// An organisation's entry product is its earliest registration's, whatever
	// the period; it counts when that registration falls in the period.
	if out.RegistrationsByEntryProduct, err = r.countBy(ctx, `
		SELECT pr.name, COUNT(*)::INT
		FROM  (SELECT DISTINCT ON (op.organization_id) op.product_id, op.registered_on
		       FROM   plg_org_platform op
		       ORDER  BY op.organization_id, op.registered_on, op.id) first
		JOIN   plg_product pr ON pr.id = first.product_id
		WHERE  first.registered_on BETWEEN $1 AND $2
		GROUP  BY pr.name ORDER BY 2 DESC, 1`, from, to); err != nil {
		return nil, err
	}

	// utm_source is free text a visitor sets in the landing URL, so the number of
	// distinct values is unbounded: the busiest are kept and the rest folded into OTHER.
	if out.UTMSources, err = r.countBy(ctx, `
		SELECT COALESCE(LOWER(m.utm_source), 'NOT_RECORDED'), COUNT(*)::INT
		FROM   plg_org_platform op
		LEFT   JOIN plg_registration_metadata m ON m.org_platform_id = op.id
		WHERE  op.registered_on BETWEEN $1 AND $2
		  `+entryRegistrationOnly+`
		GROUP  BY 1 ORDER BY 2 DESC, 1`, from, to); err != nil {
		return nil, err
	}
	out.UTMSources = foldRemainder(out.UTMSources, maxUTMSources)

	if out.EmailClassification, err = r.countBy(ctx, `
		SELECT COALESCE(x.value_text, 'NOT_CLASSIFIED'), COUNT(*)::INT
		FROM   plg_org_platform op
		`+registrationTaskValue+`
		WHERE  op.registered_on BETWEEN $1 AND $2
		  `+entryRegistrationOnly+`
		GROUP  BY 1 ORDER BY 2 DESC, 1`, from, to, domain.TaskCodeEmailClassification); err != nil {
		return nil, err
	}

	if out.QualificationOverview, err = r.countBy(ctx, `
		SELECT COALESCE(x.value_text, 'NOT_ASSESSED'), COUNT(*)::INT
		FROM   plg_org_platform op
		`+registrationTaskValue+`
		WHERE  op.registered_on BETWEEN $1 AND $2
		  `+entryRegistrationOnly+`
		GROUP  BY 1 ORDER BY 2 DESC, 1`, from, to, domain.TaskCodeEligibilityOutcome); err != nil {
		return nil, err
	}

	if out.QualifiedByUTM, err = r.qualifiedByUTM(ctx, from, to); err != nil {
		return nil, err
	}

	return out, nil
}

// entryRegistrationOnly keeps an organisation's earliest registration (its entry
// registration, as in the entry-product chart) so one organisation counts once
// across its products. It needs the pairing aliased as op.
const entryRegistrationOnly = `AND  op.id IN (SELECT DISTINCT ON (e.organization_id) e.id
		                 FROM   plg_org_platform e
		                 ORDER  BY e.organization_id, e.registered_on, e.id)`

// registrationTaskValue joins, as x, the value of the pairing's registration task
// with code $3; NULL when there is no run or no answer. A pairing has one attached
// run; should it ever have more, the latest one answers, as on the organisation page.
const registrationTaskValue = `LEFT JOIN LATERAL (
		    SELECT t.value_text
		    FROM   plg_playbook_run run
		    JOIN   plg_playbook pb ON pb.id = run.playbook_id
		    JOIN   plg_playbook_run_task t ON t.playbook_run_id = run.id
		    WHERE  run.org_platform_id = op.id AND run.detached_on IS NULL
		      AND  pb.lifecycle_stage = 'REGISTRATION' AND pb.playbook_type = 'PROGRESSIVE'
		      AND  t.code = $3
		    ORDER  BY run.created_at DESC, run.id DESC
		    LIMIT  1) x ON TRUE`

// maxUTMSources bounds the UTM charts: both keep the busiest sources and fold the
// rest into OTHER, so they stay readable and add up to the same cohort. Sources are
// compared lower-cased ("Google" and "google" are one).
const maxUTMSources = 10

// otherLabel is the label of the folded remainder of a capped breakdown. A real
// source that a visitor named the same is kept apart as realOtherLabel, so the
// two never share a label.
const (
	otherLabel     = "OTHER"
	realOtherLabel = "OTHER (utm_source)"
)

// foldRemainder keeps the first n rows (the query orders them busiest first) and
// adds the rest together as OTHER, so a breakdown's size stays bounded.
func foldRemainder(rows []domain.CountByLabel, n int) []domain.CountByLabel {
	if len(rows) <= n {
		return rows
	}
	kept := make([]domain.CountByLabel, n, n+1)
	copy(kept, rows[:n])
	for i := range kept {
		if strings.EqualFold(kept[i].Label, otherLabel) {
			kept[i].Label = realOtherLabel
		}
	}
	rest := 0
	for _, r := range rows[n:] {
		rest += r.Count
	}
	return append(kept, domain.CountByLabel{Label: otherLabel, Count: rest})
}

// foldQualificationRemainder is foldRemainder for the outcomes per source: the
// rows after the first n are added together, outcome by outcome, as OTHER.
func foldQualificationRemainder(rows []domain.UTMQualification, n int) []domain.UTMQualification {
	if len(rows) <= n {
		return rows
	}
	kept := make([]domain.UTMQualification, n, n+1)
	copy(kept, rows[:n])
	for i := range kept {
		if strings.EqualFold(kept[i].Label, otherLabel) {
			kept[i].Label = realOtherLabel
		}
	}
	rest := domain.UTMQualification{Label: otherLabel}
	for _, r := range rows[n:] {
		rest.Eligible += r.Eligible
		rest.Monitor += r.Monitor
		rest.Excluded += r.Excluded
		rest.NotAssessed += r.NotAssessed
		rest.Total += r.Total
	}
	return append(kept, rest)
}

// qualifiedByUTM counts eligibility outcomes per utm_source, over each
// organisation's entry registration only (its earliest, as in the entry-product chart).
func (r *analyticsRepository) qualifiedByUTM(ctx context.Context, from, to time.Time) ([]domain.UTMQualification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(LOWER(m.utm_source), 'NOT_RECORDED'),
		       COUNT(*) FILTER (WHERE x.value_text = 'ELIGIBLE')::INT,
		       COUNT(*) FILTER (WHERE x.value_text = 'MONITOR')::INT,
		       COUNT(*) FILTER (WHERE x.value_text = 'EXCLUDED')::INT,
		       COUNT(*) FILTER (WHERE x.value_text IS NULL
		                           OR x.value_text NOT IN ('ELIGIBLE', 'MONITOR', 'EXCLUDED'))::INT,
		       COUNT(*)::INT
		FROM   plg_org_platform op
		LEFT   JOIN plg_registration_metadata m ON m.org_platform_id = op.id
		`+registrationTaskValue+`
		WHERE  op.registered_on BETWEEN $1 AND $2
		  `+entryRegistrationOnly+`
		GROUP  BY 1 ORDER BY 6 DESC, 1`, from, to, domain.TaskCodeEligibilityOutcome)
	if err != nil {
		return nil, fmt.Errorf("analytics qualified by utm: %w", err)
	}
	defer rows.Close()

	out := []domain.UTMQualification{}
	for rows.Next() {
		var q domain.UTMQualification
		if err := rows.Scan(&q.Label, &q.Eligible, &q.Monitor, &q.Excluded, &q.NotAssessed, &q.Total); err != nil {
			return nil, fmt.Errorf("scan qualified by utm: %w", err)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return foldQualificationRemainder(out, maxUTMSources), nil
}

func (r *analyticsRepository) countBy(ctx context.Context, q string, from, to time.Time, extra ...any) ([]domain.CountByLabel, error) {
	rows, err := r.db.Query(ctx, q, append([]any{from, to}, extra...)...)
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
		       (SELECT o.moesif_company_id FROM plg_organization o WHERE o.id = w.organization_id),
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
		if err := rows.Scan(&it.OrgPlatformID, &it.OrganizationID, &it.OrganizationName, &it.MoesifCompanyID,
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
