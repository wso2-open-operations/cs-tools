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

// Integration test: PATCH of a project's four onboarding dates on the Postgres
// data source, read back through GetProjectByID. Runs against a real Postgres,
// skipped without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ProjectOnboardingDatesIntegration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const podProject = "92000000-0000-0000-0000-0000000000b1"

func TestProjectOnboardingDatesIntegration(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, podProject) }
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, sf_id)
		VALUES ($1, now(), now(), 'pod-test', 'pod-test', 'PODPROJ', 'PODPROJ', 'sf-PODPROJ')`, podProject); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := repository.NewProjectRepository(repository.NewScoped(pool))
	scope := repository.SearchScope{Unrestricted: true}
	str := func(s string) *string { return &s }
	patch := func(req domain.ProjectUpdateRequest) (domain.ProjectUpdateResult, error) {
		return repo.UpdateProject(ctx, podProject, req, "jane.doe@example.com")
	}
	day := func(s string) string { return s + "T00:00:00Z" }
	fmtDate := func(d *time.Time) string {
		if d == nil {
			return "<nil>"
		}
		return d.Format(time.RFC3339)
	}
	check := func(label string, got [4]*time.Time, start, plan, live, expiry string) {
		t.Helper()
		want := [4]string{start, plan, live, expiry}
		for i, g := range got {
			w := want[i]
			if w != "<nil>" {
				w = day(w)
			}
			if fmtDate(g) != w {
				t.Errorf("%s: date[%d] (start,plan,live,expiry) = %s, want %s", label, i, fmtDate(g), w)
			}
		}
	}

	// All four set; the result and a fresh read agree.
	res, err := patch(domain.ProjectUpdateRequest{
		OnboardingStartDate: str("2026-03-01"), OnboardingGoLivePlanDate: str("2026-05-01"),
		OnboardingGoLiveDate: str("2026-05-15"), OnboardingExpiryDate: str("2027-03-01"),
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	check("result", [4]*time.Time{res.OnboardingStartDate, res.GoLivePlanDate, res.GoLiveDate, res.OnboardingExpiryDate},
		"2026-03-01", "2026-05-01", "2026-05-15", "2027-03-01")
	got, err := repo.GetProjectByID(ctx, podProject, scope)
	if err != nil {
		t.Fatalf("GetProjectByID: %v", err)
	}
	check("read", [4]*time.Time{got.OnboardingStartDate, got.GoLivePlanDate, got.GoLiveDate, got.OnboardingExpiryDate},
		"2026-03-01", "2026-05-01", "2026-05-15", "2027-03-01")

	// A start-only patch leaves the others.
	if _, err := patch(domain.ProjectUpdateRequest{OnboardingStartDate: str("2026-02-01")}); err != nil {
		t.Fatalf("start-only: %v", err)
	}
	got, _ = repo.GetProjectByID(ctx, podProject, scope)
	check("after start-only", [4]*time.Time{got.OnboardingStartDate, got.GoLivePlanDate, got.GoLiveDate, got.OnboardingExpiryDate},
		"2026-02-01", "2026-05-01", "2026-05-15", "2027-03-01")

	// Rejections leave the row untouched: malformed, in-request order, and
	// order against the stored start / stored dates.
	for name, req := range map[string]domain.ProjectUpdateRequest{
		"malformed":               {OnboardingGoLiveDate: str("15-05-2026")},
		"in-request order":        {OnboardingStartDate: str("2026-06-01"), OnboardingExpiryDate: str("2026-05-01")},
		"go-live before stored":   {OnboardingGoLiveDate: str("2026-01-31")},
		"start after stored plan": {OnboardingStartDate: str("2026-06-01")},
	} {
		_, err := patch(req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
	}
	got, _ = repo.GetProjectByID(ctx, podProject, scope)
	check("after rejections", [4]*time.Time{got.OnboardingStartDate, got.GoLivePlanDate, got.GoLiveDate, got.OnboardingExpiryDate},
		"2026-02-01", "2026-05-01", "2026-05-15", "2027-03-01")

	// Stored data that already violates the order (e.g. overwritten by a sync)
	// must not block an edit to a different date; the supplied date is still
	// checked against the stored start, and moving the start re-checks all.
	if _, err := pool.Exec(ctx, `UPDATE project SET onboarding_go_live_date = '2026-01-10' WHERE id = $1`, podProject); err != nil {
		t.Fatalf("seed violation: %v", err)
	}
	if _, err := patch(domain.ProjectUpdateRequest{OnboardingExpiryDate: str("2027-04-01")}); err != nil {
		t.Fatalf("expiry-only edit blocked by a stored violation: %v", err)
	}
	var ve *apierror.ValidationError
	if _, err := patch(domain.ProjectUpdateRequest{OnboardingGoLivePlanDate: str("2026-01-05")}); !errors.As(err, &ve) {
		t.Errorf("supplied plan before the stored start: err = %v, want ValidationError", err)
	}
	if _, err := patch(domain.ProjectUpdateRequest{OnboardingStartDate: str("2026-02-02")}); !errors.As(err, &ve) {
		t.Errorf("start change over a stored violation: err = %v, want ValidationError", err)
	}
	got, _ = repo.GetProjectByID(ctx, podProject, scope)
	check("after stored-violation edits", [4]*time.Time{got.OnboardingStartDate, got.GoLivePlanDate, got.GoLiveDate, got.OnboardingExpiryDate},
		"2026-02-01", "2026-05-01", "2026-01-10", "2027-04-01")
}
