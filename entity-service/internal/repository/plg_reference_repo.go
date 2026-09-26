package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ReferenceRepository serves the data that changes only with a migration:
// products and the lifecycle catalogue.
//
// The CS user queries are NOT here; they live on PlgUserRepository. `"user"` is
// not PLG reference data — it is a table entity-service owns and csm-portal
// reads. Products and the lifecycle catalogue really are
// PLG's own vocabulary, and stay.
type ReferenceRepository interface {
	ListProducts(ctx context.Context) ([]domain.PlgProduct, error)
	LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error)
}

type referenceRepository struct{ db *pgxpool.Pool }

// NewReferenceRepository builds a ReferenceRepository over the given pool.
func NewReferenceRepository(db *pgxpool.Pool) ReferenceRepository {
	return &referenceRepository{db: db}
}

func (r *referenceRepository) ListProducts(ctx context.Context) ([]domain.PlgProduct, error) {
	const q = `
		SELECT id::TEXT, code, name, display_order, active
		FROM   plg_product
		ORDER  BY display_order`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := []domain.PlgProduct{}
	for rows.Next() {
		var p domain.PlgProduct
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.DisplayOrder, &p.Active); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// LifecycleCatalogue returns the six stages.
//
// There is no path table to return alongside them: the progression is the stage
// order, with ABANDONED hanging off all of it, so a window function and two
// counts produce what would otherwise need a second query and a join.
//
// Everything here is derived rather than stored, which is why the diagram, the
// stage picker and the database cannot drift apart.
func (r *referenceRepository) LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error) {
	const q = `
		SELECT ls.stage::TEXT, ls.name, ls.display_order, ls.description,

		       -- Where a progressive playbook here carries a pairing. NULL at
		       -- COMMERCIAL, which has nowhere further to go, and at ABANDONED,
		       -- which nothing leaves. LEAD over the progression gives it
		       -- directly — ABANDONED is excluded from the window by the
		       -- display_order filter, so it cannot become anyone's "next".
		       (SELECT nx.stage::TEXT FROM (
		            SELECT stage, LEAD(stage) OVER (ORDER BY display_order) AS next_stage
		            FROM   plg_lifecycle_stage WHERE display_order < 99
		        ) w JOIN plg_lifecycle_stage nx ON nx.stage = w.next_stage
		          WHERE w.stage = ls.stage) AS next_stage,

		       (ls.display_order >= 99) AS terminal,

		       -- One pass over the table rather than three correlated counts.
		       -- The editor shows all three side by side, so they are always
		       -- wanted together.
		       COALESCE(pc.progressive, 0)                     AS progressive_playbooks,
		       COALESCE(pc.recovery, 0)                        AS recovery_playbooks,
		       COALESCE(pc.sustaining, 0)                      AS sustaining_playbooks

		       -- NO PAIRING COUNT. This query once returned how many pairings
		       -- sat at each stage, and the only consumer was the lifecycle
		       -- diagram on the pairing view — a page about ONE customer,
		       -- reporting a total across every customer in the portfolio.
		       --
		       -- It was wrong on the page before it was anything else: the
		       -- diagram is there to show where this pairing stands, and a
		       -- caption reading "2 pairings" invites the reader to think it
		       -- says something about the customer they are looking at.
		       --
		       -- It is also the wrong shape for an endpoint like this one.
		       -- /lifecycle reads as a static catalogue — six stages and their
		       -- names — so nobody scoping organisation visibility later would
		       -- think to look here, and portfolio totals would keep being
		       -- served to callers who had been cut off from the rows they are
		       -- computed from. Reference data should hold no customer counts
		       -- at all; that is the analytics endpoint's job, and it has one.
		FROM   plg_lifecycle_stage ls
		LEFT   JOIN (
		           SELECT lifecycle_stage,
		                  COUNT(*) FILTER (WHERE playbook_type = 'PROGRESSIVE')::INT AS progressive,
		                  COUNT(*) FILTER (WHERE playbook_type = 'RECOVERY')::INT    AS recovery,
		                  COUNT(*) FILTER (WHERE playbook_type = 'SUSTAINING')::INT  AS sustaining
		           FROM   plg_playbook WHERE active GROUP BY lifecycle_stage
		       ) pc ON pc.lifecycle_stage = ls.stage
		ORDER  BY ls.display_order`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("load lifecycle stages: %w", err)
	}
	defer rows.Close()

	cat := &domain.LifecycleCatalogue{Stages: []domain.LifecycleStageInfo{}}
	for rows.Next() {
		var (
			s    domain.LifecycleStageInfo
			next *string
		)
		if err := rows.Scan(&s.Stage, &s.Name, &s.DisplayOrder, &s.Description,
			&next, &s.Terminal,
			&s.ProgressivePlaybooks, &s.RecoveryPlaybooks, &s.SustainingPlaybooks); err != nil {
			return nil, fmt.Errorf("scan lifecycle stage: %w", err)
		}
		if next != nil {
			stage := domain.LifecycleStage(*next)
			s.NextStage = &stage
		}
		cat.Stages = append(cat.Stages, s)
	}
	return cat, rows.Err()
}
