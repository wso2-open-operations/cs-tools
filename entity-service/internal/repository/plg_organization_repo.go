package repository

import (
	"context"
	"errors"
	"fmt"
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
	Patch(ctx context.Context, req domain.PatchOrganizationRequest) error
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
	if f.RegisteredFrom != nil {
		conds = append(conds, `v.created_on >= `+b.add(*f.RegisteredFrom))
	}
	if f.RegisteredTo != nil {
		conds = append(conds, `v.created_on <= `+b.add(*f.RegisteredTo))
	}
	return conds
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
func (r *organizationRepository) Patch(ctx context.Context, req domain.PatchOrganizationRequest) error {
	// $2 is bound as a nullable UUID rather than wrapped in NULLIF: a UUID column
	// fails the cast before NULLIF can run. See uuidArg in common.go.
	const q = `
		UPDATE plg_organization
		SET    plg_cs_owner = $2::UUID
		WHERE  id::TEXT = $1`

	owner := ""
	if req.OwnerID != nil {
		owner = *req.OwnerID
	}

	tag, err := r.db.Exec(ctx, q, req.ID, uuidArg(owner))
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
