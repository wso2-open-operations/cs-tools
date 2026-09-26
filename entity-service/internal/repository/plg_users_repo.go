package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// PlgUserRepository reads the shared `"user"` table.
//
// Read-only on purpose. `"user"` is entity-service's table and the same rows
// csm-portal reads, so PLG is one reader among several and has no business
// writing them —
// the CS team is administered elsewhere. There is deliberately no Insert,
// Update or Delete here, and adding one later should be an argument, not a
// convenience.
type PlgUserRepository interface {
	// SearchUsers returns matching engineers and the total before pagination.
	SearchUsers(ctx context.Context, req domain.PlgSearchUsersRequest) ([]domain.UserRef, int, error)
}

type userRepository struct{ db *pgxpool.Pool }

// NewPlgUserRepository builds a PlgUserRepository over the given pool.
func NewPlgUserRepository(db *pgxpool.Pool) PlgUserRepository {
	return &userRepository{db: db}
}

// userConditions turns the filters into SQL. Every caller-supplied value goes
// through argBuilder — nothing is interpolated.
func userConditions(f *domain.UserSearchFilters, b *argBuilder) []string {
	// EVERY user query PLG issues is restricted to internal staff, and this is
	// not a filter the caller can turn off — it is prepended before any of them.
	//
	// The table is shared with csm-portal, which holds customers, partners and
	// system actors alongside CS engineers. Nothing in PLG has any business
	// seeing those: an owner picker offering a customer, or a registration
	// acknowledged by a partner contact, is a data-integrity problem that would
	// surface as a confusing name on a screen rather than as an error.
	//
	// Upstream derives user_type from the role tables — SYSTEM from
	// is_system_user, INTERNAL from the admin/internal roles — so this tracks
	// role membership without PLG reading the role tables itself.
	//
	// Note the UPPERCASE value. csm-portal's enum is uppercase while its Go
	// constants are lowercase ("internal"), with nothing normalising between
	// them; the database is what this query talks to, so the database's
	// spelling is what it uses.
	conds := []string{`u.user_type = 'INTERNAL'`}

	if len(f.IDs) > 0 {
		conds = append(conds, `u.id::TEXT = ANY(`+b.add(f.IDs)+`::TEXT[])`)
	}
	// The resolution path: the BFF holds the caller's email from their token and
	// needs the id it maps to. Compared case-insensitively — an exact match
	// would not find "Akilaf@wso2.com".
	if len(f.Emails) > 0 {
		conds = append(conds, `LOWER(u.email) = ANY(`+b.add(lowerAll(f.Emails))+`::TEXT[])`)
	}
	if len(f.UserTypes) > 0 {
		conds = append(conds, `u.user_type::TEXT = ANY(`+b.add(f.UserTypes)+`::TEXT[])`)
	}
	// Nil means "either", which is not the same as false. An attribution lookup
	// wants inactive engineers included — a note written by someone who has
	// since left must still render their name — while the owner pickers never do.
	// is_active, not active: csm-portal's column name. It is also NULLABLE
	// upstream, so `is_active = false` and `is_active IS NULL` are different
	// things — IS NOT DISTINCT FROM treats a NULL as "not true", which is the
	// reading that keeps an unset engineer out of an owner picker rather than
	// silently into it.
	if f.Active != nil {
		conds = append(conds, `COALESCE(u.is_active, FALSE) = `+b.add(*f.Active))
	}
	return conds
}

// SearchUsers runs the count and the page concurrently, on separate pool
// connections — the pattern entity-service uses for every search (see
// SearchCases). total may differ from the page by at most one concurrent write,
// which is accepted for search-style pagination.
func (r *userRepository) SearchUsers(ctx context.Context, req domain.PlgSearchUsersRequest) ([]domain.UserRef, int, error) {
	// Both queries are built, and both argument slices frozen, BEFORE either
	// goroutine starts. argBuilder is not safe for concurrent use, and adding
	// the LIMIT/OFFSET placeholders inside a goroutine would mutate it while the
	// count query was reading the slice it had already taken. Cheap to get right
	// here; a genuine race to debug later.
	b := &argBuilder{}
	where := whereClause(userConditions(&req.Filters, b))

	countQ := `SELECT COUNT(*)::INT FROM "user" u` + where
	countArgs := append([]any(nil), b.list()...)

	// The display name is assembled here rather than in the caller, the same
	// way the views assemble plg_cs_owner_name — so "the owner's name" has one
	// definition across the whole slice.
	pageQ := `
		SELECT u.id::TEXT, COALESCE(u.email, ''),
		       COALESCE(
		           NULLIF(TRIM(BOTH ' ' FROM COALESCE(u.name, '')), ''),
		           NULLIF(TRIM(BOTH ' ' FROM COALESCE(u.first_name, '') || ' ' ||
		                                     COALESCE(u.last_name, '')), ''))
		FROM   "user" u` + where + `
		-- u.id last as a tiebreaker. csm-portal's email column carries no unique
		-- index, so two rows can agree on every other sort key; without a total
		-- order the "first match" the BFF resolves an address to could differ
		-- between two identical requests.
		ORDER  BY u.first_name, u.last_name, u.email, u.id
		LIMIT  ` + b.add(req.Pagination.Limit) + ` OFFSET ` + b.add(req.Pagination.Offset)
	pageArgs := append([]any(nil), b.list()...)

	var (
		users []domain.UserRef
		total int
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := r.db.QueryRow(gctx, countQ, countArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		rows, err := r.db.Query(gctx, pageQ, pageArgs...)
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		defer rows.Close()

		users = []domain.UserRef{}
		for rows.Next() {
			var id, email string
			var name *string
			if err := rows.Scan(&id, &email, &name); err != nil {
				return fmt.Errorf("scan user: %w", err)
			}
			if ref := userRef(&id, &email, name); ref != nil {
				users = append(users, *ref)
			}
		}
		return rows.Err()
	})

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
