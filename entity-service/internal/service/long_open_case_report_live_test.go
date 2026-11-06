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

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// Builds the long-open cases report against a real deployment, seeding
// nothing and writing nothing: every statement it causes is a SELECT.
// Fixtures prove each condition; this proves the project types and tags
// exist under the configured names and that the report is the expected
// handful of rows rather than thousands.
//
//	LONG_OPEN_LIVE_DB_URL="postgres://..." LONG_OPEN_LIVE_OLDER_THAN_DAYS=30 \
//	LONG_OPEN_LIVE_JSON_OUT=/tmp/long-open.json \
//	  go test -v -run TestLongOpenCaseReportAgainstLiveDatabase ./internal/service/
//
// LONG_OPEN_LIVE_PROJECT_TYPES and LONG_OPEN_LIVE_EXCLUDED_TAGS (comma
// lists) override the defaults the service is configured with.
func TestLongOpenCaseReportAgainstLiveDatabase(t *testing.T) {
	dsn := os.Getenv("LONG_OPEN_LIVE_DB_URL")
	if dsn == "" {
		t.Skip("LONG_OPEN_LIVE_DB_URL is not set; skipping the live-deployment read")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	list := func(env string, def []string) []string {
		raw := os.Getenv(env)
		if raw == "" {
			return def
		}
		var out []string
		for _, v := range strings.Split(raw, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	cfg := service.LongOpenCaseReportConfig{
		ProjectTypes: list("LONG_OPEN_LIVE_PROJECT_TYPES", []string{"Subscription", "Managed Cloud Subscription", "Cloud Support", "Development Support", "Professional Services"}),
		ExcludedTags: list("LONG_OPEN_LIVE_EXCLUDED_TAGS", []string{"migrartion", "migration-assistance"}),
	}
	days := service.LongOpenCaseReportDefaultOlderThanDays
	if raw := os.Getenv("LONG_OPEN_LIVE_OLDER_THAN_DAYS"); raw != "" {
		if _, err := fmt.Sscan(raw, &days); err != nil {
			t.Fatalf("LONG_OPEN_LIVE_OLDER_THAN_DAYS: %v", err)
		}
	}

	svc := service.NewLongOpenCaseReportService(repository.NewLongOpenCaseReportRepository(repository.NewScoped(pool)), cfg)
	report, err := svc.BuildReport(ctx, domain.LongOpenCaseReportRequest{OlderThanDays: days, Now: time.Now()})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	t.Logf("older than %d days (created before %s): cases %d of %d, onboarding cases %d of %d",
		report.OlderThanDays, report.CreatedBefore.Format("2006-01-02"),
		len(report.Cases), report.CasesTotal, len(report.OnboardingCases), report.OnboardingCasesTotal)

	// A misspelt project type or tag name is invisible in the counts alone:
	// a type that matches nothing empties the report, a tag that matches
	// nothing lists cases the team has set aside.
	for _, name := range cfg.ProjectTypes {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM project_type WHERE name = $1`, name).Scan(&n); err != nil {
			t.Fatalf("count project types: %v", err)
		}
		t.Logf("project type rows named %q: %d", name, n)
	}
	for _, name := range cfg.ExcludedTags {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM tag WHERE LOWER(name) = LOWER($1)`, name).Scan(&n); err != nil {
			t.Fatalf("count tags: %v", err)
		}
		t.Logf("tag rows named %q: %d", name, n)
	}

	if out := os.Getenv("LONG_OPEN_LIVE_JSON_OUT"); out != "" {
		blob, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if err := os.WriteFile(out, blob, 0o600); err != nil {
			t.Fatalf("write %s: %v", out, err)
		}
		t.Logf("report written to %s", out)
	}
}
