package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// OrganizationRepository owns the organisation list and the overview tab.
type OrganizationRepository interface {
	Search(ctx context.Context, req domain.SearchOrganizationsRequest) ([]domain.OrganizationSummary, int, error)
	Get(ctx context.Context, id string) (*domain.OrganizationDetail, error)
	Patch(ctx context.Context, req domain.PatchOrganizationRequest, actorID string) error
	AggregateByOwner(ctx context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error)
	AggregateByCountry(ctx context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error)
}

type organizationRepository struct{ db *pgxpool.Pool }

// NewOrganizationRepository builds an OrganizationRepository over the given pool.
func NewOrganizationRepository(db *pgxpool.Pool) OrganizationRepository {
	return &organizationRepository{db: db}
}

// buildConditions turns the search filters into SQL. Every caller-supplied value
// goes through argBuilder — nothing is interpolated.
//
// PlgProduct, stage and tier are EXISTS subqueries against the pairing, because
// they are facts about a pairing rather than an organisation: "organisations at
// Active" can only mean "organisations with something at Active".
func buildConditions(f *domain.OrganizationSearchFilters, b *argBuilder) []string {
	var conds []string

	if q := strings.TrimSpace(f.Query); q != "" {
		p := b.add("%" + strings.ToLower(q) + "%")
		conds = append(conds, `(LOWER(v.organization_name) LIKE `+p+
			` OR LOWER(v.registered_email) LIKE `+p+`)`)
	}
	if v := strings.TrimSpace(f.OrganizationName); v != "" {
		conds = append(conds, `LOWER(v.organization_name) LIKE `+b.add("%"+strings.ToLower(v)+"%"))
	}
	if v := strings.TrimSpace(f.RegisteredEmail); v != "" {
		conds = append(conds, `LOWER(v.registered_email) LIKE `+b.add("%"+strings.ToLower(v)+"%"))
	}
	if len(f.ProductCodes) > 0 {
		conds = append(conds, `EXISTS (SELECT 1 FROM plg_org_platform op
		                              JOIN plg_product pr ON pr.id = op.product_id
		                              WHERE op.organization_id = v.organization_id
		                                AND pr.code = ANY(`+b.add(f.ProductCodes)+`::TEXT[]))`)
	}
	if len(f.LifecycleStages) > 0 {
		conds = append(conds, `EXISTS (SELECT 1 FROM plg_org_platform op
		                              WHERE op.organization_id = v.organization_id
		                                AND op.lifecycle_stage::TEXT = ANY(`+
			b.add(stringsOf(f.LifecycleStages))+`::TEXT[]))`)
	}
	if len(f.SubscriptionTiers) > 0 {
		conds = append(conds, `EXISTS (SELECT 1 FROM plg_org_platform op
		                              WHERE op.organization_id = v.organization_id
		                                AND op.subscription_tier::TEXT = ANY(`+
			b.add(stringsOf(f.SubscriptionTiers))+`::TEXT[]))`)
	}
	// Cast the column, not the parameter — the idiom every id filter in this
	// package uses, including organization_id and playbook_id.
	if len(f.OwnerIDs) > 0 {
		conds = append(conds, `v.plg_cs_owner::TEXT = ANY(`+b.add(f.OwnerIDs)+`::TEXT[])`)
	}
	if f.Unowned != nil && *f.Unowned {
		conds = append(conds, `v.plg_cs_owner IS NULL`)
	}
	conds = append(conds, ownerEmailConditions(f, b, `v.plg_cs_owner_email`)...)
	if f.RegisteredFrom != nil {
		conds = append(conds, `v.created_on >= `+b.add(*f.RegisteredFrom))
	}
	if f.RegisteredTo != nil {
		conds = append(conds, `v.created_on <= `+b.add(*f.RegisteredTo))
	}
	conds = append(conds, countryConditions(f, b, `v.country_name`)...)
	for _, c := range playbookConditions(f, b, `op.id`) {
		conds = append(conds, `EXISTS (SELECT 1 FROM plg_org_platform op
		                              WHERE op.organization_id = v.organization_id
		                                AND `+c+`)`)
	}
	return conds
}

// playbookConditions are the playbook answer filters on pairing, a
// registration's id column: with playbookAnswers the task named by
// playbookTask has one of those answers on one of the registration's attached
// runs, with playbookUnanswered it has none on any of them. A detached run is
// history, not an answer. Codes match as written, trimmed.
func playbookConditions(f *domain.OrganizationSearchFilters, b *argBuilder, pairing string) []string {
	task := strings.TrimSpace(f.PlaybookTask)
	unanswered := f.PlaybookUnanswered != nil && *f.PlaybookUnanswered
	if task == "" || (len(f.PlaybookAnswers) == 0 && !unanswered) {
		return nil
	}
	answered := func(codes string) string {
		return `EXISTS (SELECT 1 FROM plg_run_task_reason_v a
		               JOIN plg_playbook_run r ON r.id = a.playbook_run_id
		               WHERE r.org_platform_id = ` + pairing + `
		                 AND r.detached_on IS NULL
		                 AND a.task_code = ` + b.add(task) + codes + `)`
	}
	var conds []string
	if len(f.PlaybookAnswers) > 0 {
		codes := make([]string, len(f.PlaybookAnswers))
		for i, c := range f.PlaybookAnswers {
			codes[i] = strings.TrimSpace(c)
		}
		conds = append(conds, answered(` AND a.reason_code = ANY(`+b.add(codes)+`::TEXT[])`))
	}
	if unanswered {
		conds = append(conds, `NOT `+answered(``))
	}
	return conds
}

// ownerEmailConditions is the ownerEmails filter on column, the owner's email
// address: any of them, trimmed and ignoring case.
func ownerEmailConditions(f *domain.OrganizationSearchFilters, b *argBuilder, column string) []string {
	if len(f.OwnerEmails) == 0 {
		return nil
	}
	emails := make([]string, len(f.OwnerEmails))
	for i, e := range f.OwnerEmails {
		emails[i] = strings.ToLower(strings.TrimSpace(e))
	}
	return []string{`LOWER(` + column + `) = ANY(` + b.add(emails) + `::TEXT[])`}
}

// countryConditions are the country filters on column, the organisation's
// resolved country: countryNames any of them, trimmed and ignoring case,
// and countryUnknown no country (NULL or blank), so that each group of a
// country grouping (countryGroupKey) is exactly what its filter lists.
func countryConditions(f *domain.OrganizationSearchFilters, b *argBuilder, column string) []string {
	var conds []string
	if len(f.CountryNames) > 0 {
		names := make([]string, len(f.CountryNames))
		for i, n := range f.CountryNames {
			names[i] = strings.ToLower(strings.TrimSpace(n))
		}
		conds = append(conds, `LOWER(TRIM(`+column+`)) = ANY(`+b.add(names)+`::TEXT[])`)
	}
	if f.CountryUnknown != nil && *f.CountryUnknown {
		conds = append(conds, `NULLIF(TRIM(`+column+`), '') IS NULL`)
	}
	return conds
}

// countryGroupKey and countryGroupLabel group rows by column the way
// countryConditions matches it: one group per country ignoring case and
// surrounding spaces, labelled with one of its spellings; the rows with no
// country are the group with key "".
func countryGroupKey(column string) string {
	return `COALESCE(LOWER(NULLIF(TRIM(` + column + `), '')), '')`
}

func countryGroupLabel(column string) string {
	return `COALESCE(MIN(TRIM(` + column + `)), '')`
}

// AggregateByOwner counts the organisations filters match per PLG CS owner,
// with the same conditions as Search, so a group's count is what the search
// for that owner lists. Each group's key is the owner's user id and its label
// the owner's name (or email when there is no name); the organisations with
// no owner are one group, PlgUnownedGroupKey labelled "Unowned". Groups come
// largest first, ties by label; with maxGroups > 0 the groups after the
// first maxGroups are summed into OthersCount.
func (r *organizationRepository) AggregateByOwner(ctx context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error) {
	b := &argBuilder{}
	conds := buildConditions(&filters, b)

	q := `
		SELECT COALESCE(v.plg_cs_owner::TEXT, ''),
		       COALESCE(NULLIF(MAX(v.plg_cs_owner_name), ''), MAX(v.plg_cs_owner_email), ''),
		       COUNT(*)
		FROM   plg_organization_v v` +
		whereClause(conds) + `
		GROUP  BY v.plg_cs_owner`

	rows, err := r.db.Query(ctx, q, b.list()...)
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate organizations by owner: %w", err)
	}
	defer rows.Close()

	groups := []domain.AggregateBucket{}
	total := 0
	for rows.Next() {
		var g domain.AggregateBucket
		if err := rows.Scan(&g.Key, &g.Label, &g.Count); err != nil {
			return domain.AggregateResponse{}, fmt.Errorf("scan organization owner group: %w", err)
		}
		switch {
		case g.Key == "":
			g.Key, g.Label = domain.PlgUnownedGroupKey, "Unowned"
		case g.Label == "":
			g.Label = g.Key
		}
		groups = append(groups, g)
		total += g.Count
	}
	if err := rows.Err(); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("iterate organization owner groups: %w", err)
	}
	return rankGroups(groups, total, maxGroups), nil
}

// AggregateByCountry counts the organisations filters match per country,
// with the same conditions as Search, so a group's count is what the search
// for its countryNames (or countryUnknown) lists. A group's key and label
// are the country as written; the organisations with no country are one
// group, PlgUnknownCountryGroupKey labelled "Unknown". Groups are ranked and
// folded as AggregateByOwner's are.
func (r *organizationRepository) AggregateByCountry(ctx context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error) {
	b := &argBuilder{}
	conds := buildConditions(&filters, b)

	q := `
		SELECT ` + countryGroupLabel(`v.country_name`) + `, COUNT(*)
		FROM   plg_organization_v v` +
		whereClause(conds) + `
		GROUP  BY ` + countryGroupKey(`v.country_name`)

	groups, total, err := scanCountryGroups(ctx, r.db, q, b.list())
	if err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("aggregate organizations by country: %w", err)
	}
	return rankGroups(groups, total, maxGroups), nil
}

// scanCountryGroups runs a country grouping query (each row a label and a
// count; the no-country row's label "") into groups and their total.
func scanCountryGroups(ctx context.Context, db *pgxpool.Pool, q string, args []any) ([]domain.AggregateBucket, int, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	groups := []domain.AggregateBucket{}
	total := 0
	for rows.Next() {
		var g domain.AggregateBucket
		if err := rows.Scan(&g.Label, &g.Count); err != nil {
			return nil, 0, err
		}
		g.Key = g.Label
		if g.Label == "" {
			g.Key, g.Label = domain.PlgUnknownCountryGroupKey, "Unknown"
		}
		groups = append(groups, g)
		total += g.Count
	}
	return groups, total, rows.Err()
}

// rankGroups orders groups largest first, ties by label, and with
// maxGroups > 0 sums the groups after the first maxGroups into OthersCount.
func rankGroups(groups []domain.AggregateBucket, total, maxGroups int) domain.AggregateResponse {
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		return groups[i].Label < groups[j].Label
	})

	resp := domain.AggregateResponse{Groups: groups, TotalRecords: total}
	if maxGroups > 0 && maxGroups < len(groups) {
		for _, g := range groups[maxGroups:] {
			resp.OthersCount += g.Count
		}
		resp.Groups = groups[:maxGroups]
	}
	return resp
}

func (r *organizationRepository) Search(ctx context.Context, req domain.SearchOrganizationsRequest) ([]domain.OrganizationSummary, int, error) {
	b := &argBuilder{}
	conds := buildConditions(&req.Filters, b)

	orderBy := "v.created_on"
	if req.SortBy == "name" {
		orderBy = "LOWER(v.organization_name)"
	}
	direction := "DESC"
	if strings.EqualFold(req.SortOrder, "asc") {
		direction = "ASC"
	}

	limit := b.add(req.Pagination.Limit)
	offset := b.add(req.Pagination.Offset)

	q := `
		SELECT v.organization_id, v.organization_name, v.registered_email, v.registered_name,
		       v.created_on,
		       v.plg_cs_owner::TEXT, v.plg_cs_owner_email, v.plg_cs_owner_name,
		       COUNT(*) OVER() AS total
		FROM   plg_organization_v v` +
		whereClause(conds) + `
		ORDER  BY ` + orderBy + ` ` + direction + `
		LIMIT  ` + limit + ` OFFSET ` + offset

	rows, err := r.db.Query(ctx, q, b.list()...)
	if err != nil {
		return nil, 0, fmt.Errorf("search organizations: %w", err)
	}
	defer rows.Close()

	var (
		out   []domain.OrganizationSummary
		total int
		ids   []string
	)
	for rows.Next() {
		var (
			s                              domain.OrganizationSummary
			ownerID, ownerEmail, ownerName *string
		)
		if err := rows.Scan(&s.ID, &s.OrganizationName, &s.RegisteredEmail, &s.RegisteredName,
			&s.RegisteredOn, &ownerID, &ownerEmail, &ownerName, &total); err != nil {
			return nil, 0, fmt.Errorf("scan organization: %w", err)
		}
		s.Owner = userRef(ownerID, ownerEmail, ownerName)
		s.PlatformStages = []domain.PlatformStage{}
		out = append(out, s)
		ids = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate organizations: %w", err)
	}
	if out == nil {
		return []domain.OrganizationSummary{}, 0, nil
	}

	if err := r.attachPlatformStages(ctx, out, ids); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// attachPlatformStages fills the "registered products and their lifecycle
// stages" column for one page of rows, in a single query rather than per row.
func (r *organizationRepository) attachPlatformStages(ctx context.Context, out []domain.OrganizationSummary, ids []string) error {
	pos := make(map[string]int, len(out))
	for i := range out {
		pos[out[i].ID] = i
	}

	const q = `
		SELECT v.organization_id, v.org_platform_id,
		       v.product_id, v.product_code, v.product_name,
		       v.lifecycle_stage::TEXT, v.lifecycle_stage_name, v.registered_on, v.is_new
		FROM   plg_org_platform_v v
		WHERE  v.organization_id::TEXT = ANY($1::TEXT[])
		ORDER  BY v.product_display_order`

	rows, err := r.db.Query(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("load platform stages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orgID                      string
			ps                         domain.PlatformStage
			prodID, prodCode, prodName string
		)
		if err := rows.Scan(&orgID, &ps.OrgPlatformID, &prodID, &prodCode, &prodName,
			&ps.LifecycleStage, &ps.StageName, &ps.RegisteredOn, &ps.IsNew); err != nil {
			return fmt.Errorf("scan platform stage: %w", err)
		}
		ps.Product = productRef(prodID, prodCode, prodName)
		if i, ok := pos[orgID]; ok {
			out[i].PlatformStages = append(out[i].PlatformStages, ps)
		}
	}
	return rows.Err()
}

// Get returns the overview tab: two cards, which is all it keeps.
//
// Reading through plg_organization_v means a sparse second registration shows
// its registrant's country and region rather than blanks, with FieldsInherited
// marking that it happened.
func (r *organizationRepository) Get(ctx context.Context, id string) (*domain.OrganizationDetail, error) {
	const q = `
		SELECT v.organization_id, v.organization_name, v.created_on,
		       v.registered_email, v.registered_name,
		       v.plg_cs_owner::TEXT, v.plg_cs_owner_email, v.plg_cs_owner_name,
		       v.country_name, v.company_name_from_domain,
		       v.moesif_company_id, v.fields_inherited
		FROM   plg_organization_v v
		WHERE  v.organization_id::TEXT = $1`

	var (
		d                              domain.OrganizationDetail
		ownerID, ownerEmail, ownerName *string
	)
	err := r.db.QueryRow(ctx, q, id).Scan(&d.ID, &d.OrganizationName, &d.CreatedOn,
		&d.RegisteredEmail, &d.RegisteredName, &ownerID, &ownerEmail, &ownerName,
		&d.CountryName, &d.CompanyNameFromDomain,
		&d.MoesifCompanyID, &d.FieldsInherited)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apierror.NotFoundError{Msg: "organization not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	d.Owner = userRef(ownerID, ownerEmail, ownerName)

	if d.PlatformCards, err = r.loadPlatformCards(ctx, id); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *organizationRepository) loadPlatformCards(ctx context.Context, orgID string) ([]domain.PlatformCard, error) {
	const q = `
		SELECT v.org_platform_id, v.product_id, v.product_code, v.product_name,
		       v.lifecycle_stage::TEXT, v.lifecycle_stage_name, v.stage_entered_on,
		       v.registered_on, v.subscription_tier::TEXT,
		       v.run_active::INT, v.run_total::INT, v.task_completed::INT, v.task_total::INT,
		       v.is_new
		FROM   plg_org_platform_v v
		WHERE  v.organization_id::TEXT = $1
		ORDER  BY v.product_display_order`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("load platform cards: %w", err)
	}
	defer rows.Close()

	cards := []domain.PlatformCard{}
	for rows.Next() {
		var (
			c                          domain.PlatformCard
			prodID, prodCode, prodName string
			tier                       *string
		)
		if err := rows.Scan(&c.OrgPlatformID, &prodID, &prodCode, &prodName,
			&c.LifecycleStage, &c.StageName, &c.StageEnteredOn, &c.RegisteredOn, &tier,
			&c.RunActive, &c.RunTotal, &c.TaskCompleted, &c.TaskTotal, &c.IsNew); err != nil {
			return nil, fmt.Errorf("scan platform card: %w", err)
		}
		c.Product = productRef(prodID, prodCode, prodName)
		if tier != nil {
			t := domain.SubscriptionTier(*tier)
			c.SubscriptionTier = &t
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// Patch updates the one organisation-level field the portal writes: the CS
// owner. A single owner handles every pairing for the customer.
func (r *organizationRepository) Patch(ctx context.Context, req domain.PatchOrganizationRequest, actorID string) error {
	// $2 is bound as a nullable UUID rather than wrapped in NULLIF: a UUID column
	// fails the cast before NULLIF can run. See uuidArg in common.go.
	// owner_updated_by/_on are set unconditionally: this statement ran, so
	// somebody reassigned the account, and that stays true even when the new
	// owner is the same person as the old one. plg_cs_owner says whose work it
	// is; these say who decided that and when.
	const q = `
		UPDATE plg_organization
		SET    plg_cs_owner     = $2::UUID,
		       owner_updated_by = $3::UUID,
		       owner_updated_on = NOW()
		WHERE  id::TEXT = $1`

	owner := ""
	if req.OwnerID != nil {
		owner = *req.OwnerID
	}

	tag, err := r.db.Exec(ctx, q, req.ID, uuidArg(owner), uuidArg(actorID))
	if err != nil {
		if isConstraintViolation(err) {
			return &apierror.ValidationError{Msg: "unknown CS user: " + owner}
		}
		return fmt.Errorf("patch organization: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "organization not found"}
	}
	return nil
}
