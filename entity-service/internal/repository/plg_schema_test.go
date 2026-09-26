package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Tests against a real PostgreSQL.
//
// WHY THESE EXIST AT ALL. Most of what this schema enforces cannot be tested
// anywhere else, because the enforcement IS the database: a trigger that
// refuses a backwards stage move, a NOT NULL that refuses an unexplained
// history row, a function that decides which playbooks a pairing is offered.
// Mocking any of that would test the mock.
//
// They also guard a specific kind of mistake that unit tests are blind to. A
// query that no longer parses, a column removed from a SELECT but not its Scan,
// a view whose shape drifted from the struct reading it — every one of those
// compiles perfectly and fails at runtime on the first request.
//
// HOW TO RUN THEM. Set PLG_TEST_DATABASE_URL to a server this may create and
// drop a database on:
//
//	PLG_TEST_DATABASE_URL='postgres://plg:plg@localhost:5432/postgres' go test ./...
//
// or just `make test-db`, which starts from the docker-compose Postgres.
//
// Without it they skip, so `go test ./...` stays useful on a laptop with no
// database — the unit tests still run and still mean something.

const testDBName = "plgportal_v5_schema_test"

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	adminURL := os.Getenv("PLG_TEST_DATABASE_URL")
	if adminURL == "" {
		// Nothing to connect to. Every test below calls requireDB and skips.
		os.Exit(m.Run())
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot reach PLG_TEST_DATABASE_URL: %v\n", err)
		os.Exit(1)
	}

	// Dropped first, not just at the end: a previous run killed halfway through
	// would otherwise leave a half-migrated database and every run after it
	// would fail for a reason that has nothing to do with the code.
	if err := recreate(ctx, admin, testDBName); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create %s: %v\n", testDBName, err)
		os.Exit(1)
	}
	admin.Close()

	testPool, err = pgxpool.New(ctx, swapDatabase(adminURL, testDBName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect to %s: %v\n", testDBName, err)
		os.Exit(1)
	}
	if err := applyMigrations(ctx, testPool); err != nil {
		fmt.Fprintf(os.Stderr, "cannot apply migrations: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	testPool.Close()
	// The database is left behind on purpose when a test failed, so the state
	// that failed can be inspected. A passing run cleans up after itself.
	if code == 0 {
		if admin, err := pgxpool.New(ctx, adminURL); err == nil {
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+testDBName+" WITH (FORCE)")
			admin.Close()
		}
	} else {
		fmt.Fprintf(os.Stderr, "\ndatabase %s left in place for inspection\n", testDBName)
	}
	os.Exit(code)
}

func recreate(ctx context.Context, admin *pgxpool.Pool, name string) error {
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		return err
	}
	_, err := admin.Exec(ctx, "CREATE DATABASE "+name)
	return err
}

// swapDatabase rewrites the database segment of a postgres URL. The admin URL
// points somewhere that already exists (usually "postgres"); the tests need the
// same server with a different database.
func swapDatabase(url, name string) string {
	if i := strings.LastIndex(url, "/"); i > 0 {
		rest := ""
		if j := strings.Index(url[i:], "?"); j >= 0 {
			rest = url[i+j:]
		}
		return url[:i+1] + name + rest
	}
	return url
}

// applyMigrations runs the .up.sql files in order — the same files
// create-database.sh applies, read from disk rather than copied here, so a
// schema change cannot pass these tests without being in the migration.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no migrations found — has the directory moved?")
	}
	for _, f := range files { // Glob returns them sorted, and the names are ordered
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("set PLG_TEST_DATABASE_URL to run the schema tests")
	}
	return testPool
}

func sqlstate(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// ---------------------------------------------------------------------------
// The rules that are written down in more than one language
// ---------------------------------------------------------------------------

// The Go copy of a rule and the SQL copy must agree. This is the test the
// domain package's comment promises: plg_entity_test.go pins the Go side, and
// this asks the database the same questions.
func TestSchemaAgreesWithTheDomainAboutPlaybookKinds(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	// The enum itself.
	var labels []string
	if err := pool.QueryRow(ctx, `
		SELECT array_agg(enumlabel::TEXT ORDER BY enumsortorder)
		FROM   pg_enum e JOIN pg_type ty ON ty.oid = e.enumtypid
		WHERE  ty.typname = 'plg_playbook_type_enum'`).Scan(&labels); err != nil {
		t.Fatalf("read the enum: %v", err)
	}
	want := make([]string, 0, len(domain.PlaybookTypeOrder))
	for _, k := range domain.PlaybookTypeOrder {
		want = append(want, string(k))
	}
	if !reflect.DeepEqual(labels, want) {
		t.Errorf("the database has %v, the domain has %v", labels, want)
	}

	// The mapping. This is the rule from the design discussion — healthy offers
	// progressive and sustaining work, at-risk offers recovery — and it exists
	// in SQL, in Go and in TypeScript. Two of the three are compared here.
	for _, h := range []domain.HealthState{domain.HealthHealthy, domain.HealthAtRisk} {
		var fromSQL []string
		if err := pool.QueryRow(ctx,
			`SELECT plg_applicable_playbook_types($1::plg_health_enum)::TEXT[]`, string(h)).Scan(&fromSQL); err != nil {
			t.Fatalf("call the function for %s: %v", h, err)
		}
		fromGo := make([]string, 0, 2)
		for _, k := range domain.ApplicablePlaybookTypes(h) {
			fromGo = append(fromGo, string(k))
		}
		if !reflect.DeepEqual(fromSQL, fromGo) {
			t.Errorf("%s: SQL says %v, Go says %v", h, fromSQL, fromGo)
		}
	}
}

func TestSchemaAgreesWithTheDomainAboutStagesAndTaskTypes(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()

	for _, tc := range []struct {
		enum string
		want []string
	}{
		{"plg_lifecycle_stage_enum", stringsOf(domain.LifecycleStageOrder)},
		{"plg_task_value_type_enum", stringsOf(domain.TaskValueTypeOrder)},
		{"plg_health_enum", stringsOf(domain.HealthStateOrder)},
	} {
		t.Run(tc.enum, func(t *testing.T) {
			var got []string
			if err := pool.QueryRow(ctx, `
				SELECT array_agg(enumlabel::TEXT ORDER BY enumsortorder)
				FROM   pg_enum e JOIN pg_type ty ON ty.oid = e.enumtypid
				WHERE  ty.typname = $1`, tc.enum).Scan(&got); err != nil {
				t.Fatalf("read %s: %v", tc.enum, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("the database has %v, the domain has %v", got, tc.want)
			}
			inDomain := map[string]bool{}
			for _, v := range tc.want {
				inDomain[v] = true
			}
			for _, v := range got {
				if !inDomain[v] {
					t.Errorf("%s is in the database but not in the domain", v)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The rules only the database enforces
// ---------------------------------------------------------------------------

func TestStageTriggerRefusesBackwardsMoves(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	pairing := seedPairing(t, pool, "Backwards Ltd", "backwards@example.test")

	move := func(to domain.LifecycleStage) error {
		_, err := pool.Exec(ctx,
			`UPDATE plg_org_platform SET lifecycle_stage = $2::plg_lifecycle_stage_enum WHERE id = $1`,
			pairing, string(to))
		return err
	}

	if err := move(domain.StagePlgCsEligible); err != nil {
		t.Fatalf("a forward move was refused: %v", err)
	}
	if err := move(domain.StageRegistration); err == nil {
		t.Error("a backwards move was allowed")
	} else if sqlstate(err) != "23514" {
		t.Errorf("a backwards move failed with %s, want a check violation (23514)", sqlstate(err))
	} else if !strings.Contains(err.Error(), "forward only") {
		// The message is passed through to the caller as a 400 — see
		// checkViolationMessage in common.go — so it has to be readable.
		t.Errorf("the trigger's message is not something a caller can act on: %v", err)
	}

	// ABANDONED is reachable from anywhere, and nothing leaves it.
	if err := move(domain.StageAbandoned); err != nil {
		t.Fatalf("could not abandon a pairing: %v", err)
	}
	if err := move(domain.StageCommercial); err == nil {
		t.Error("a pairing left ABANDONED")
	}
}

func TestHistoryRowsMustRecordAndExplainSomething(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	pairing := seedPairing(t, pool, "History Ltd", "history@example.test")

	insert := func(toHealth any, reason any) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO plg_lifecycle_history (org_platform_id, to_health, reason)
			VALUES ($1, $2::plg_health_enum, $3)`, pairing, toHealth, reason)
		return err
	}

	t.Run("a null reason", func(t *testing.T) {
		// A health change recorded without a reason is exactly what the timeline
		// cannot be read through.
		err := insert("AT_RISK", nil)
		if err == nil {
			t.Fatal("a history row with no reason was accepted")
		}
		if sqlstate(err) != "23502" {
			t.Errorf("got %s, want a not-null violation (23502)", sqlstate(err))
		}
	})

	t.Run("a blank reason", func(t *testing.T) {
		// NOT NULL alone accepts a space. The column exists to be read.
		err := insert("AT_RISK", "   ")
		if err == nil {
			t.Fatal("a history row with a whitespace reason was accepted")
		}
		if sqlstate(err) != "23514" {
			t.Errorf("got %s, want a check violation (23514)", sqlstate(err))
		}
	})

	t.Run("nothing recorded", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO plg_lifecycle_history (org_platform_id, reason) VALUES ($1, 'a reason')`, pairing)
		if err == nil {
			t.Fatal("a history row that changed nothing was accepted")
		}
		if sqlstate(err) != "23514" {
			t.Errorf("got %s, want a check violation (23514)", sqlstate(err))
		}
	})

	t.Run("a real row", func(t *testing.T) {
		if err := insert("AT_RISK", "the sponsor went quiet"); err != nil {
			t.Fatalf("a well-formed history row was refused: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Reference data carries no customer data
// ---------------------------------------------------------------------------

// The lifecycle catalogue must not change when customers do.
//
// It once carried a count of pairings per stage, which the pairing view drew
// under each node — portfolio totals on a page about one customer, served by an
// endpoint that reads like static reference data. The rule replacing it is flat
// rather than conditional: reference endpoints return no counts of customers.
//
// Written as an invariance test rather than "assert the field is absent",
// because the failure it needs to catch is someone adding a DIFFERENT
// customer-derived field. Any of them makes the catalogue vary with the
// customer table, and that is what this notices.
func TestLifecycleCatalogueDoesNotVaryWithCustomerData(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewReferenceRepository(pool)

	before, err := repo.LifecycleCatalogue(ctx)
	if err != nil {
		t.Fatalf("load the catalogue: %v", err)
	}

	// Enough pairings, spread over enough stages, that any per-stage count
	// would move.
	for i, stage := range []domain.LifecycleStage{
		domain.StageRegistration, domain.StagePlgCsEligible, domain.StagePlgCsEligible,
		domain.StageActivated, domain.StageCommercial,
	} {
		p := seedPairing(t, pool, fmt.Sprintf("Invariance %d Ltd", i), fmt.Sprintf("inv%d@example.test", i))
		if stage != domain.StageRegistration {
			if _, err := pool.Exec(ctx,
				`UPDATE plg_org_platform SET lifecycle_stage = $2::plg_lifecycle_stage_enum WHERE id = $1`,
				p, string(stage)); err != nil {
				t.Fatalf("move a pairing to %s: %v", stage, err)
			}
		}
	}

	after, err := repo.LifecycleCatalogue(ctx)
	if err != nil {
		t.Fatalf("reload the catalogue: %v", err)
	}

	if reflect.DeepEqual(before, after) {
		return
	}
	// Dumping two structs here produced a wall of text with the one changed
	// number buried in it. Naming the field that moved is the whole value of
	// the failure.
	t.Errorf("the lifecycle catalogue changed when pairings were added — " +
		"reference data must not carry customer counts (see ER.md decision 17)")
	for i := range before.Stages {
		if i >= len(after.Stages) {
			t.Errorf("  stage %s disappeared", before.Stages[i].Stage)
			continue
		}
		for _, d := range diffStage(before.Stages[i], after.Stages[i]) {
			t.Errorf("  %s: %s", before.Stages[i].Stage, d)
		}
	}
}

// diffStage names the fields that differ between two catalogue entries, so a
// failure reads as "COMMERCIAL: PairingCount 0 -> 1" rather than as two
// hundred characters of struct.
func diffStage(a, b domain.LifecycleStageInfo) []string {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	var out []string
	for i := 0; i < va.NumField(); i++ {
		fa, fb := va.Field(i), vb.Field(i)
		if fa.Kind() == reflect.Ptr { // Description and NextStage: compare what they point at
			if !samePointee(fa, fb) {
				out = append(out, va.Type().Field(i).Name+" changed")
			}
			continue
		}
		if !fa.Equal(fb) {
			out = append(out, fmt.Sprintf("%s %v -> %v", va.Type().Field(i).Name, fa, fb))
		}
	}
	return out
}

func samePointee(a, b reflect.Value) bool {
	if a.IsNil() || b.IsNil() {
		return a.IsNil() == b.IsNil()
	}
	return a.Elem().Equal(b.Elem())
}

// The catalogue's playbook counts, by contrast, SHOULD move with the playbooks.
// A playbook is a template every engineer can already see in full; counting
// templates says nothing about any customer. Asserted so the test above is not
// accidentally satisfied by a catalogue that reports nothing at all.
func TestLifecycleCatalogueDoesCountPlaybooks(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewReferenceRepository(pool)

	var productID string
	if err := pool.QueryRow(ctx, `SELECT id::TEXT FROM plg_product WHERE code = 'IAM'`).Scan(&productID); err != nil {
		t.Fatalf("find a product: %v", err)
	}

	before, err := repo.LifecycleCatalogue(ctx)
	if err != nil {
		t.Fatalf("load the catalogue: %v", err)
	}
	countAt := func(cat *domain.LifecycleCatalogue, stage domain.LifecycleStage) int {
		for _, s := range cat.Stages {
			if s.Stage == stage {
				return s.SustainingPlaybooks
			}
		}
		t.Fatalf("stage %s is not in the catalogue", stage)
		return 0
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO plg_playbook (product_id, name, lifecycle_stage, playbook_type, display_order, active)
		VALUES ($1::UUID, 'A sustaining play', 'ACTIVATED', 'SUSTAINING', 1, TRUE)`, productID); err != nil {
		t.Fatalf("author a playbook: %v", err)
	}

	after, err := repo.LifecycleCatalogue(ctx)
	if err != nil {
		t.Fatalf("reload the catalogue: %v", err)
	}
	if got, want := countAt(after, domain.StageActivated), countAt(before, domain.StageActivated)+1; got != want {
		t.Errorf("sustaining playbooks at ACTIVATED = %d, want %d", got, want)
	}
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// seedPairing inserts an organisation and one pairing, and returns the pairing
// id. Each call uses its own names so tests do not collide.
func seedPairing(t *testing.T, pool *pgxpool.Pool, orgName, email string) string {
	t.Helper()
	ctx := context.Background()

	var personID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO plg_person (email) VALUES ($1) RETURNING id::TEXT`, email).Scan(&personID); err != nil {
		t.Fatalf("seed a person: %v", err)
	}

	var orgID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO plg_organization (organization_name, created_on, registered_user)
		VALUES ($1, NOW(), $2::UUID) RETURNING id::TEXT`, orgName, personID).Scan(&orgID); err != nil {
		t.Fatalf("seed an organisation: %v", err)
	}

	var pairingID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO plg_org_platform (organization_id, product_id, registered_on)
		VALUES ($1::UUID, (SELECT id FROM plg_product WHERE code = 'IAM'), NOW())
		RETURNING id::TEXT`, orgID).Scan(&pairingID); err != nil {
		t.Fatalf("seed a pairing: %v", err)
	}
	return pairingID
}

// ---------------------------------------------------------------------------
// PLG sees internal staff and nobody else
// ---------------------------------------------------------------------------

// The `"user"` table is shared with csm-portal, which holds customers,
// partners and system actors alongside CS engineers. Every PLG user query is
// restricted to INTERNAL, and that restriction is prepended in
// userConditions rather than left to the caller — so this test goes through
// the real repository rather than asserting on the SQL text.
//
// It matters more than it looks. An owner picker offering a customer, or a
// registration acknowledged by a partner contact, fails as a confusing name on
// a screen rather than as an error — and the identity resolver runs through
// this same path, so a lost filter would let a customer act as staff.
func TestUserSearchReturnsOnlyActiveInternalStaff(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewPlgUserRepository(pool)

	rows := []struct {
		userName, email, userType string
		active                    any
		want                      bool
	}{
		{"t.internal", "t.internal@wso2.test", "INTERNAL", true, true},
		{"t.external", "t.external@acme.test", "EXTERNAL", true, false},
		{"t.system", "t.system@wso2.test", "SYSTEM", true, false},
		{"t.norole", "t.norole@wso2.test", "NOT_AVAILABLE", true, false},
		{"t.offboarded", "t.offboarded@wso2.test", "INTERNAL", false, false},
		// NULL is upstream's default — the column is nullable and nothing
		// backfills it. It must read as "not active", not as "unknown, allow".
		{"t.nullactive", "t.nullactive@wso2.test", "INTERNAL", nil, false},
	}
	for _, r := range rows {
		if err := seedUser(ctx, pool, r.userName, r.email, r.userType, r.active); err != nil {
			t.Fatalf("seed %s: %v", r.userName, err)
		}
	}

	active := true
	found := map[string]bool{}
	users, _, err := repo.SearchUsers(ctx, domain.PlgSearchUsersRequest{
		Filters:    domain.UserSearchFilters{Active: &active},
		Pagination: domain.Pagination{Limit: 200},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, u := range users {
		found[u.Email] = true
	}

	for _, r := range rows {
		if got := found[r.email]; got != r.want {
			verb := "was returned"
			if r.want {
				verb = "was NOT returned"
			}
			t.Errorf("%s (%s, active=%v) %s", r.email, r.userType, r.active, verb)
		}
	}
}

// Asking for a non-internal type explicitly must not widen the result — the
// restriction is not a default the caller can override.
func TestUserSearchIgnoresAnAttemptToWidenTheType(t *testing.T) {
	pool := requireDB(t)
	ctx := context.Background()
	repo := NewPlgUserRepository(pool)

	if err := seedUser(ctx, pool, "t.widen", "t.widen@acme.test", "EXTERNAL", true); err != nil {
		t.Fatalf("seed: %v", err)
	}

	users, _, err := repo.SearchUsers(ctx, domain.PlgSearchUsersRequest{
		Filters:    domain.UserSearchFilters{UserTypes: []string{"EXTERNAL"}},
		Pagination: domain.Pagination{Limit: 200},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, u := range users {
		if u.Email == "t.widen@acme.test" {
			t.Fatal("an EXTERNAL user was returned when the caller asked for EXTERNAL — " +
				"the INTERNAL restriction must not be overridable")
		}
	}
}

// seedUser inserts a user the way csm-portal's application does, which is not
// the way PLG's standalone build did.
//
// TWO THINGS THE MERGE CHANGED, both learned by this test failing:
//
//  1. `id`, `created_on` and `updated_on` have NO DEFAULT on the real table.
//     PLG's stand-in gave them one so a standalone seed could be a bare INSERT;
//     csm-portal expects its application to supply all three.
//
//  2. `user_type` CANNOT BE WRITTEN. A trigger installed by
//     000007_users_add_user_type recomputes it from role membership on every
//     insert, so a value passed here is silently replaced by NOT_AVAILABLE.
//     The only way to make a user INTERNAL is to give them a role named
//     `internal` or `admin` — which is what this does, and what any real
//     environment bootstrap must do too.
func seedUser(ctx context.Context, pool *pgxpool.Pool, userName, email, userType string, active any) error {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO "user" (id, created_on, updated_on, user_name, first_name, last_name, email, is_active)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, 'T', 'User', $2, $3)
		ON CONFLICT (user_name) DO UPDATE SET email = EXCLUDED.email
		RETURNING id::TEXT`, userName, email, active).Scan(&id)
	if err != nil {
		return err
	}

	// SYSTEM is derived from a flag rather than a role; the rest come from one.
	if userType == "SYSTEM" {
		_, err = pool.Exec(ctx, `UPDATE "user" SET is_system_user = TRUE WHERE id = $1::UUID`, id)
		return err
	}
	roleName := map[string]string{"INTERNAL": "internal", "EXTERNAL": "external"}[userType]
	if roleName == "" {
		return nil // NOT_AVAILABLE: no role at all, which is the default state
	}

	var roleID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'test', 'test', $1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id::TEXT`, roleName).Scan(&roleID); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
		VALUES (gen_random_uuid(), NOW(), NOW(), 'test', 'test', $1::UUID, $2::UUID)
		ON CONFLICT DO NOTHING`, id, roleID)
	return err
}
