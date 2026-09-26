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

// These tests exercise the real SQL in project_consumption_repo.go against a
// live PostgreSQL instance. The forward-only guard is a guarded UPDATE ... WHERE
// clause and the partial write is a COALESCE — neither has any behaviour a fake
// repository can reproduce, so the unit tests in internal/service cannot cover
// them and these fill that gap.
//
// They are skipped unless ENTITY_TEST_DATABASE_URL is set, so an ordinary
// `go test ./...` on a machine with no database stays green:
//
// Apply every migration in order rather than picking files by hand — these
// tests read columns and enum types spread across several of them (the
// choreo_application_status_enum and the project table from 000009, the two
// secret-key columns from 000075), and a hand-picked subset fails on the first
// query rather than at setup:
//
//	createdb entity_test
//	for f in migrations/*.up.sql; do psql -v ON_ERROR_STOP=1 -d entity_test -f "$f"; done
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestIntegration ./internal/repository/

package repository

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testIntegrationProjectID = "6fa0b42d-1bfa-4a69-a002-c9d3604bcb77"
	testIntegrationUserID    = "11111111-1111-4111-8111-111111111111"
	testIntegrationAccountID = "22222222-2222-4222-8222-222222222222"
)

// newIntegrationRepo connects to the test database, resets the fixtures this
// file depends on, and returns a repository over a real pool.
func newIntegrationRepo(t *testing.T) (ProjectConsumptionRepository, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Rebuild the fixture rows from scratch so the tests are order-independent
	// and rerunnable.
	seed := []string{
		`UPDATE project SET choreo_application_status = NULL, choreo_application_id = NULL, product_consumption_client_id = NULL, product_consumption_client_secret = NULL, product_consumption_primary_secret_key = NULL, product_consumption_secondary_secret_key = NULL, consumption_tracking_file_generated_on = NULL WHERE id = $1`,
	}
	for _, stmt := range seed {
		if _, err := pool.Exec(ctx, stmt, testIntegrationProjectID); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
	mustExec(t, pool, `
		INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, first_name, last_name, email)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', 'consumption.fixture', 'Consumption', 'Fixture', 'consumption.fixture@example.test')
		ON CONFLICT (id) DO NOTHING`, testIntegrationUserID)
	mustExec(t, pool, `
		INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, activation_date)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', 'Consumption Fixture Account', 'ACC-CONSUMPTION-FIXTURE', 'SF-CONSUMPTION-FIXTURE', NOW())
		ON CONFLICT (id) DO NOTHING`, testIntegrationAccountID)
	mustExec(t, pool, `
		INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, start_date, end_date)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', 'CONSUMPTION-FIXTURE', 'SF-PROJ-CONSUMPTION-FIXTURE', 'Consumption Fixture Project', $2,
		        NOW() - INTERVAL '1 day', NOW() + INTERVAL '365 days')
		ON CONFLICT (id) DO NOTHING`, testIntegrationProjectID, testIntegrationAccountID)

	return NewProjectConsumptionRepository(pool), pool
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func ptr(s string) *string { return &s }

// TestIntegration_GetUnprovisionedProject checks the LEFT JOIN branch: a
// project with no consumption row is step 1, not a missing resource.
func TestIntegration_GetUnprovisionedProject(t *testing.T) {
	repo, _ := newIntegrationRepo(t)

	state, name, key, err := repo.Get(context.Background(), testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.Status != domain.ConsumptionStatusPending {
		t.Fatalf("got status %d, want pending", state.Status)
	}
	if name == "" || key == "" {
		t.Fatalf("expected the project's name and key, got %q / %q", name, key)
	}
}

func TestIntegration_GetUnknownProjectIsNotFound(t *testing.T) {
	repo, _ := newIntegrationRepo(t)

	_, _, _, err := repo.Get(context.Background(), "99999999-9999-4999-8999-999999999999")
	var notFound *apierror.NotFoundError
	if !asNotFound(err, &notFound) {
		t.Fatalf("got %v, want a NotFoundError", err)
	}
}

// TestIntegration_SecretsAreStoredAsSupplied pins the format of the three
// secret-bearing columns, checked against the columns themselves rather than
// the Go API. They are shared with the ServiceNow sync, which writes them in
// the clear, so this service writes them the same way -- a second format in
// the same column could not be told apart from the sync's on read.
func TestIntegration_SecretsAreStoredAsSupplied(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()
	const (
		secret       = "consumer-secret-plaintext"
		primaryKey   = "primary-secret-key-plaintext"
		secondaryKey = "secondary-secret-key-plaintext"
	)

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})
	advance(t, repo, domain.ConsumptionStatusGeneratedCredentials, &domain.ProjectConsumption{
		ConsumerKey:    ptr("consumer-key"),
		ConsumerSecret: ptr(secret),
	})
	advance(t, repo, domain.ConsumptionStatusGeneratedSecretKeys, &domain.ProjectConsumption{
		PrimarySecretKey:   ptr(primaryKey),
		SecondarySecretKey: ptr(secondaryKey),
	})

	var storedSecret, storedPrimary, storedSecondary string
	if err := pool.QueryRow(ctx,
		`SELECT product_consumption_client_secret, product_consumption_primary_secret_key, product_consumption_secondary_secret_key FROM project WHERE id = $1`,
		testIntegrationProjectID).Scan(&storedSecret, &storedPrimary, &storedSecondary); err != nil {
		t.Fatalf("read column: %v", err)
	}
	for _, f := range []struct{ column, stored, supplied string }{
		{"product_consumption_client_secret", storedSecret, secret},
		{"product_consumption_primary_secret_key", storedPrimary, primaryKey},
		{"product_consumption_secondary_secret_key", storedSecondary, secondaryKey},
	} {
		if f.stored != f.supplied {
			t.Fatalf("%s was not stored as supplied: got %q, want %q", f.column, f.stored, f.supplied)
		}
	}

	// And they still round-trip back through the repository.
	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.ConsumerSecret == nil || *state.ConsumerSecret != secret {
		t.Fatalf("consumerSecret round trip failed: %+v", state.ConsumerSecret)
	}
	if state.PrimarySecretKey == nil || *state.PrimarySecretKey != primaryKey {
		t.Fatalf("primarySecretKey round trip failed: %+v", state.PrimarySecretKey)
	}
	if state.SecondarySecretKey == nil || *state.SecondarySecretKey != secondaryKey {
		t.Fatalf("secondarySecretKey round trip failed: %+v", state.SecondarySecretKey)
	}
}

// A row written by the ServiceNow sync must read back without error. The
// values the sync writes are 64-character keys and short client secrets, some
// of which contain characters outside the base64 alphabet -- an earlier
// revision of this repository decoded and decrypted these columns on read, so
// every already-provisioned project failed its read with either a decode or an
// authentication error.
func TestIntegration_SyncWrittenSecretsReadBack(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})

	// Shaped like the real synced values: a 64-character hex key, and a client
	// secret carrying a character base64 has no meaning for.
	const (
		syncedKey    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		syncedSecret = "abc-DEF_ghi.jkl~mno"
	)
	if _, err := pool.Exec(ctx,
		`UPDATE project SET product_consumption_client_secret = $2, product_consumption_primary_secret_key = $3, product_consumption_secondary_secret_key = $3 WHERE id = $1`,
		testIntegrationProjectID, syncedSecret, syncedKey); err != nil {
		t.Fatalf("seed: %v", err)
	}

	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get on sync-written secrets: %v", err)
	}
	if state.ConsumerSecret == nil || *state.ConsumerSecret != syncedSecret {
		t.Fatalf("consumerSecret: %+v", state.ConsumerSecret)
	}
	if state.PrimarySecretKey == nil || *state.PrimarySecretKey != syncedKey {
		t.Fatalf("primarySecretKey: %+v", state.PrimarySecretKey)
	}
}

// An empty column is "not set", not a present-but-blank secret -- otherwise
// the read response would report the project as having a secret it does not.
func TestIntegration_EmptySecretColumnReadsAsAbsent(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	if _, err := pool.Exec(ctx,
		`UPDATE project SET product_consumption_client_secret = '' WHERE id = $1`, testIntegrationProjectID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.ConsumerSecret != nil {
		t.Fatalf("an empty column must read as absent, got %q", *state.ConsumerSecret)
	}
}

// TestIntegration_PartialWriteKeepsEarlierSteps exercises the COALESCE: storing
// step 4's credentials must not clear step 2's application ID.
func TestIntegration_PartialWriteKeepsEarlierSteps(t *testing.T) {
	repo, _ := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})
	advance(t, repo, domain.ConsumptionStatusGeneratedCredentials, &domain.ProjectConsumption{
		ConsumerKey:    ptr("consumer-key"),
		ConsumerSecret: ptr("secret"),
	})

	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.ChoreoApplicationID == nil || *state.ChoreoApplicationID != "app-1" {
		t.Fatalf("step 2's application id was lost: %+v", state.ChoreoApplicationID)
	}
	if state.ConsumerKey == nil || *state.ConsumerKey != "consumer-key" {
		t.Fatalf("step 4's consumer key was not stored: %+v", state.ConsumerKey)
	}
}

// TestIntegration_BackwardsWriteIsRejected covers the UPDATE ... WHERE
// guard directly — the clause a fake repository cannot model.
func TestIntegration_BackwardsWriteIsRejected(t *testing.T) {
	repo, _ := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})

	_, err := repo.Upsert(ctx, testIntegrationProjectID, domain.ProjectConsumption{
		Status:              domain.ConsumptionStatusCreated,
		ChoreoApplicationID: ptr("app-2"),
	})
	if err != ErrConsumptionStatusStale {
		t.Fatalf("got %v, want ErrConsumptionStatusStale", err)
	}

	// The losing write must not have altered anything.
	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if *state.ChoreoApplicationID != "app-1" {
		t.Fatalf("a rejected write still changed the application id to %q", *state.ChoreoApplicationID)
	}
	if state.Status != domain.ConsumptionStatusSubscribed {
		t.Fatalf("status moved backwards to %d", state.Status)
	}
}

// TestIntegration_RepeatedSameStatusIsRejected — writing the status that is
// already stored is not "ahead of" it, so it must be reported as stale rather
// than silently re-running a step that already happened.
func TestIntegration_RepeatedSameStatusIsRejected(t *testing.T) {
	repo, _ := newIntegrationRepo(t)

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})

	_, err := repo.Upsert(context.Background(), testIntegrationProjectID, domain.ProjectConsumption{
		Status:              domain.ConsumptionStatusCreated,
		ChoreoApplicationID: ptr("app-2"),
	})
	if err != ErrConsumptionStatusStale {
		t.Fatalf("got %v, want ErrConsumptionStatusStale", err)
	}
}

// TestIntegration_ConcurrentFirstWriteHasOneWinner is the scenario the guard
// exists for: several license downloads racing on a project that has never been
// provisioned. Exactly one may create the Choreo application; every other
// caller must be told it lost, not silently overwrite the winner.
func TestIntegration_ConcurrentFirstWriteHasOneWinner(t *testing.T) {
	repo, _ := newIntegrationRepo(t)
	ctx := context.Background()

	const racers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		stale   int
		other   []error
	)
	wg.Add(racers)
	for i := range racers {
		go func(i int) {
			defer wg.Done()
			_, err := repo.Upsert(ctx, testIntegrationProjectID, domain.ProjectConsumption{
				Status:              domain.ConsumptionStatusCreated,
				ChoreoApplicationID: ptr("app-" + string(rune('a'+i))),
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
			case err == ErrConsumptionStatusStale:
				stale++
			default:
				other = append(other, err)
			}
		}(i)
	}
	wg.Wait()

	if len(other) > 0 {
		t.Fatalf("unexpected errors: %v", other)
	}
	if winners != 1 {
		t.Fatalf("got %d winners, want exactly 1 (stale=%d)", winners, stale)
	}
	if stale != racers-1 {
		t.Fatalf("got %d stale, want %d", stale, racers-1)
	}

	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.Status != domain.ConsumptionStatusCreated {
		t.Fatalf("got status %d, want created", state.Status)
	}
}

// TestIntegration_FullProvisioningWalk runs the real sequence a license
// download drives, 1 through 5, each step supplying only its own artefacts.
// This is the path that first exposed the CHECK-versus-upsert interaction: a
// step carrying no artefacts of its own proposes a row full of NULLs, and
// PostgreSQL checks the proposed tuple, not the merged result.
func TestIntegration_FullProvisioningWalk(t *testing.T) {
	repo, _ := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})
	advance(t, repo, domain.ConsumptionStatusGeneratedCredentials, &domain.ProjectConsumption{
		ConsumerKey:    ptr("consumer-key"),
		ConsumerSecret: ptr("consumer-secret"),
	})
	advance(t, repo, domain.ConsumptionStatusGeneratedSecretKeys, &domain.ProjectConsumption{
		PrimarySecretKey:   ptr("primary-key"),
		SecondarySecretKey: ptr("secondary-key"),
	})

	state, _, _, err := repo.Get(ctx, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if state.Status != domain.ConsumptionStatusGeneratedSecretKeys {
		t.Fatalf("got status %d, want 5", state.Status)
	}
	// Every earlier step's output must have survived every later step — and
	// every step's own output must have been stored at all. The secret keys
	// are the ones with no home in the ServiceNow-mirrored project table until
	// migration 000075 gave them one; without it the write silently succeeded
	// and these came back nil.
	for label, got := range map[string]*string{
		"choreoApplicationId": state.ChoreoApplicationID,
		"consumerKey":         state.ConsumerKey,
		"consumerSecret":      state.ConsumerSecret,
		"primarySecretKey":    state.PrimarySecretKey,
		"secondarySecretKey":  state.SecondarySecretKey,
	} {
		if got == nil {
			t.Fatalf("%s was lost during the walk", label)
		}
	}
	if *state.ChoreoApplicationID != "app-1" {
		t.Fatalf("application id changed to %q", *state.ChoreoApplicationID)
	}
	if *state.ConsumerKey != "consumer-key" || *state.ConsumerSecret != "consumer-secret" {
		t.Fatalf("credentials round-tripped incorrectly: %q / %q", *state.ConsumerKey, *state.ConsumerSecret)
	}
	if *state.PrimarySecretKey != "primary-key" || *state.SecondarySecretKey != "secondary-key" {
		t.Fatalf("secret keys round-tripped incorrectly: %q / %q", *state.PrimarySecretKey, *state.SecondarySecretKey)
	}
}

func TestIntegration_UnknownProjectOnWriteIsNotFound(t *testing.T) {
	repo, _ := newIntegrationRepo(t)

	_, err := repo.Upsert(context.Background(), "99999999-9999-4999-8999-999999999999",
		domain.ProjectConsumption{
			Status:              domain.ConsumptionStatusCreated,
			ChoreoApplicationID: ptr("app-1"),
		})
	var notFound *apierror.NotFoundError
	if !asNotFound(err, &notFound) {
		t.Fatalf("got %v, want a NotFoundError", err)
	}
}

func TestIntegration_GetSigningContext(t *testing.T) {
	repo, pool := newIntegrationRepo(t)

	// Set credentials on the test project
	_, err := pool.Exec(context.Background(), `
		UPDATE project
		SET product_consumption_client_id = 'test-client-id',
		    product_consumption_client_secret = 'test-client-secret',
		    product_consumption_primary_secret_key = 'test-primary-key',
		    product_consumption_secondary_secret_key = 'test-secondary-key',
		    product_consumption_license_secrets = 'test-license-secrets'
		WHERE id = $1
	`, testIntegrationProjectID)
	if err != nil {
		t.Fatalf("failed to update project credentials: %v", err)
	}

	// A real deployment on this project. It used to be enough to pass an id
	// that matched nothing: the joins are LEFT joins, so the name and number
	// came back as empty strings and the call succeeded. Both are part of the
	// signed payload, so that produced a signable-but-wrong context, and
	// GetSigningContext now refuses it.
	seedDeployment(t, pool, testIntegrationDeploymentID, testIntegrationProjectID, "DEP000000001", "QA")

	ctx, err := repo.GetSigningContext(context.Background(), testIntegrationProjectID, testIntegrationDeploymentID)
	if err != nil {
		t.Fatalf("GetSigningContext failed: %v", err)
	}
	if ctx.ClientID != "test-client-id" || ctx.PrimarySecretKey != "test-primary-key" || ctx.LicenseSecrets != "test-license-secrets" {
		t.Fatalf("unexpected signing context values: %+v", ctx)
	}
}

// advance moves the fixture project to the given status, failing the test if it
// cannot — used to reach the state a test actually cares about.
func advance(t *testing.T, repo ProjectConsumptionRepository, status domain.ConsumptionStatus, next *domain.ProjectConsumption) {
	t.Helper()
	next.Status = status
	if _, err := repo.Upsert(context.Background(), testIntegrationProjectID, *next); err != nil {
		t.Fatalf("advance to %d: %v", status, err)
	}
}

func asNotFound(err error, target **apierror.NotFoundError) bool {
	v, ok := err.(*apierror.NotFoundError)
	if ok {
		*target = v
	}
	return ok
}

// --- GetSigningContext ---------------------------------------------------
//
// These cover the two ways the query can hand the signer something unusable.
// Both matter because deploymentName and deploymentNumber are part of the
// signed licence payload, so a blank does not fail -- it produces a valid
// signature over the wrong payload.

const (
	testIntegrationDeploymentID    = "33333333-3333-4333-8333-333333333333"
	testIntegrationOtherProjectID  = "44444444-4444-4444-8444-444444444444"
	testIntegrationOtherDeployment = "55555555-5555-4555-8555-555555555555"
)

// seedDeployment attaches a deployment to the given project.
func seedDeployment(t *testing.T, pool *pgxpool.Pool, deploymentID, projectID, number, name string) {
	t.Helper()
	mustExec(t, pool, `
		INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', $3, $4, true, $2)
		ON CONFLICT (id) DO UPDATE SET project_id = EXCLUDED.project_id, number = EXCLUDED.number, name = EXCLUDED.name`,
		deploymentID, projectID, number, name)
}

// A fully provisioned project returns the deployment's NUMBER, not its id --
// the signed payload's deploymentId field carries the number.
func TestIntegration_SigningContextCarriesDeploymentNumber(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	seedDeployment(t, pool, testIntegrationDeploymentID, testIntegrationProjectID, "DEP000000001", "QA")
	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})
	advance(t, repo, domain.ConsumptionStatusGeneratedCredentials, &domain.ProjectConsumption{
		ConsumerKey:    ptr("consumer-key"),
		ConsumerSecret: ptr("consumer-secret"),
	})
	advance(t, repo, domain.ConsumptionStatusGeneratedSecretKeys, &domain.ProjectConsumption{
		PrimarySecretKey:   ptr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		SecondarySecretKey: ptr("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"),
	})

	sc, err := repo.GetSigningContext(ctx, testIntegrationProjectID, testIntegrationDeploymentID)
	if err != nil {
		t.Fatalf("GetSigningContext: %v", err)
	}
	if sc.DeploymentNumber != "DEP000000001" {
		t.Fatalf("deploymentNumber: got %q, want DEP000000001", sc.DeploymentNumber)
	}
	if sc.DeploymentName != "QA" {
		t.Fatalf("deploymentName: got %q, want QA", sc.DeploymentName)
	}
	if sc.DeploymentNumber == testIntegrationDeploymentID {
		t.Fatal("deploymentNumber must not be the deployment uuid")
	}
}

// A deployment that belongs to a DIFFERENT project must not resolve. Before the
// guard this returned empty strings for name and number, which are signable.
func TestIntegration_SigningContextRejectsForeignDeployment(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	mustExec(t, pool, `
		INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, start_date, end_date)
		VALUES ($1, NOW(), NOW(), 'fixture', 'fixture', 'CONSUMPTION-FIXTURE-OTHER', 'SF-PROJ-CONSUMPTION-OTHER', 'Other Fixture Project', $2,
		        NOW() - INTERVAL '1 day', NOW() + INTERVAL '365 days')
		ON CONFLICT (id) DO NOTHING`, testIntegrationOtherProjectID, testIntegrationAccountID)
	seedDeployment(t, pool, testIntegrationOtherDeployment, testIntegrationOtherProjectID, "DEP000000002", "Other")

	_, err := repo.GetSigningContext(ctx, testIntegrationProjectID, testIntegrationOtherDeployment)
	var notFound *apierror.NotFoundError
	if !asNotFound(err, &notFound) {
		t.Fatalf("got %v, want a NotFoundError for a deployment on another project", err)
	}
}

// A project whose credentials are incomplete must error rather than return a
// context with blanks in it -- the state 8 of 12 COMPLETED projects were
// observed in on the dev database.
func TestIntegration_SigningContextRejectsIncompleteCredentials(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	seedDeployment(t, pool, testIntegrationDeploymentID, testIntegrationProjectID, "DEP000000001", "QA")
	// Credentials but no secret keys.
	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	advance(t, repo, domain.ConsumptionStatusSubscribed, &domain.ProjectConsumption{})
	advance(t, repo, domain.ConsumptionStatusGeneratedCredentials, &domain.ProjectConsumption{
		ConsumerKey:    ptr("consumer-key"),
		ConsumerSecret: ptr("consumer-secret"),
	})

	if _, err := repo.GetSigningContext(ctx, testIntegrationProjectID, testIntegrationDeploymentID); err == nil {
		t.Fatal("expected an error when the primary secret key is absent, got none")
	}
}
