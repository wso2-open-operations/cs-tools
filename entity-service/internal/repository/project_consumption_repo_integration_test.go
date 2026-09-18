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
// live PostgreSQL instance. The forward-only guard is an ON CONFLICT ... WHERE
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
// secret-key columns from 000067), and a hand-picked subset fails on the first
// query rather than at setup:
//
//	createdb entity_test
//	for f in migrations/*.up.sql; do psql -v ON_ERROR_STOP=1 -d entity_test -f "$f"; done
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestIntegration ./internal/repository/

package repository

import (
	"context"
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/crypto"
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
		`UPDATE project SET choreo_application_status = NULL, choreo_application_id = NULL, client_id = NULL, client_secret = NULL, primary_secret_key = NULL, secondary_secret_key = NULL, consumption_tracking_file_generated_on = NULL WHERE id = $1`,
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

	codec, err := crypto.NewAESGCMCodec(make([]byte, 32))
	if err != nil {
		t.Fatalf("codec: %v", err)
	}
	return NewProjectConsumptionRepository(pool, codec), pool
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

// TestIntegration_SecretsAreCiphertextInTheColumn is the claim the PR makes
// about data at rest, checked against the column itself rather than the Go API.
func TestIntegration_SecretsAreCiphertextInTheColumn(t *testing.T) {
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

	// Each of the three secret-bearing columns, read as the database holds it.
	var storedSecret, storedPrimary, storedSecondary string
	if err := pool.QueryRow(ctx,
		`SELECT client_secret, primary_secret_key, secondary_secret_key FROM project WHERE id = $1`,
		testIntegrationProjectID).Scan(&storedSecret, &storedPrimary, &storedSecondary); err != nil {
		t.Fatalf("read column: %v", err)
	}
	for _, f := range []struct{ column, stored, plaintext string }{
		{"client_secret", storedSecret, secret},
		{"primary_secret_key", storedPrimary, primaryKey},
		{"secondary_secret_key", storedSecondary, secondaryKey},
	} {
		raw, err := base64.StdEncoding.DecodeString(f.stored)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s was not written as base64 ciphertext: %v", f.column, err)
		}
		if string(raw) == f.plaintext {
			t.Fatalf("%s is stored in the clear", f.column)
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

// A secret column that is present but not decodable must surface, not silently
// read back as "this project has no secret" — these columns are also written by
// the ServiceNow sync, so an unexpected format is exactly the drift worth
// knowing about.
func TestIntegration_UndecodableSecretIsAnError(t *testing.T) {
	repo, pool := newIntegrationRepo(t)
	ctx := context.Background()

	advance(t, repo, domain.ConsumptionStatusCreated, &domain.ProjectConsumption{ChoreoApplicationID: ptr("app-1")})
	if _, err := pool.Exec(ctx,
		`UPDATE project SET client_secret = $2 WHERE id = $1`,
		testIntegrationProjectID, "not base64 at all!!"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, _, _, err := repo.Get(ctx, testIntegrationProjectID); err == nil {
		t.Fatal("expected an error for an undecodable stored secret, got none")
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

// TestIntegration_BackwardsWriteIsRejected covers the ON CONFLICT ... WHERE
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
	// migration 000067 gave them one; without it the write silently succeeded
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

func asValidation(err error, target **apierror.ValidationError) bool {
	v, ok := err.(*apierror.ValidationError)
	if ok {
		*target = v
	}
	return ok
}
