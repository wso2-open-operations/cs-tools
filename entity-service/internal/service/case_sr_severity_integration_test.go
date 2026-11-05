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

// A service request's internal severity (S1-S4) end to end on a real database:
// the real caseService over the real repositories, row-level security enforced.
// Every assertion that something was stored reads the row back from
// service_request, not the response. Skipped unless ENTITY_TEST_DATABASE_URL is
// set, to a login that does not bypass row-level security.
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/service/ -run SRSeverityIntegration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	srsAccountID  = "5e5e5e00-0000-4000-8000-000000000001"
	srsContactID  = "5e5e5e00-0000-4000-8000-000000000002"
	srsProjectID  = "5e5e5e00-0000-4000-8000-000000000003"
	srsCustomerID = "5e5e5e00-0000-4000-8000-000000000004"
	srsStaffID    = "5e5e5e00-0000-4000-8000-000000000005"
	srsDeployment = "5e5e5e00-0000-4000-8000-000000000006"
	srsDeployed   = "5e5e5e00-0000-4000-8000-000000000007"
	srsSRHigh     = "5e5e5e00-0000-4000-8000-000000000011" // an existing SR, S2
	srsSRLow      = "5e5e5e00-0000-4000-8000-000000000012" // an existing SR, S4
	srsCaseCrit   = "5e5e5e00-0000-4000-8000-000000000013" // a case at S1
	srsCaseCata   = "5e5e5e00-0000-4000-8000-000000000014" // a case at S0
	srsCaseNone   = "5e5e5e00-0000-4000-8000-000000000015" // a case with no severity
	srsCaseMed    = "5e5e5e00-0000-4000-8000-000000000016" // a case at S3
	srsEngagement = "5e5e5e00-0000-4000-8000-000000000017"
	srsCustomer   = "srs-customer@example.com"
	srsStaff      = "srs-staff@example.com"
)

var srsWorkItems = []string{srsSRHigh, srsSRLow, srsCaseCrit, srsCaseCata, srsCaseNone, srsCaseMed, srsEngagement}

func srsInternalCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, srsStaff))
	return repository.WithCallerIdentity(ctx, repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: srsStaff})
}

// srsM2MCtx is an integration client: unrestricted, no end user.
func srsM2MCtx() context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true})
}

func srsCustomerCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, srsCustomer))
	return repository.WithCallerIdentity(ctx, repository.SearchScope{ProjectIDs: []string{srsProjectID}, ViewerEmail: srsCustomer, ViaCustomerPortal: true})
}

// srsStaffViaPortalCtx is a staff user whose request came through the customer
// portal's backend.
func srsStaffViaPortalCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, srsStaff))
	return repository.WithCallerIdentity(ctx, repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: srsStaff, ViaCustomerPortal: true})
}

func seedSRSeverityFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	ids := "('" + srsSRHigh + "','" + srsSRLow + "','" + srsCaseCrit + "','" + srsCaseCata + "','" + srsCaseNone + "','" + srsCaseMed + "','" + srsEngagement + "')"
	cleanup := func() {
		for _, stmt := range []string{
			`DELETE FROM work_item_activity WHERE work_item_id IN (SELECT id FROM work_item WHERE project_id = '` + srsProjectID + `')`,
			`DELETE FROM "case" WHERE id IN (SELECT id FROM work_item WHERE project_id = '` + srsProjectID + `')`,
			`DELETE FROM service_request WHERE id IN (SELECT id FROM work_item WHERE project_id = '` + srsProjectID + `')`,
			`DELETE FROM engagement WHERE id IN ` + ids,
			`DELETE FROM work_item WHERE project_id = '` + srsProjectID + `'`,
			`DELETE FROM deployed_product WHERE id = '` + srsDeployed + `'`,
			`DELETE FROM deployment WHERE id = '` + srsDeployment + `'`,
			`DELETE FROM project_contact WHERE project_id = '` + srsProjectID + `'`,
			`DELETE FROM project WHERE id = '` + srsProjectID + `'`,
			`DELETE FROM account_contact WHERE id = '` + srsContactID + `'`,
			`DELETE FROM account WHERE id = '` + srsAccountID + `'`,
			`DELETE FROM "user" WHERE id IN ('` + srsCustomerID + `','` + srsStaffID + `')`,
		} {
			_, _ = scoped.Exec(sys, stmt)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed (%.80s): %v", sql, err)
		}
	}
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, $2, $2, 't', 't', 'SRS Account', 'SRS-ACC')`, srsAccountID, now)
	exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id) VALUES ($1, $2, $2, 't', 't', 'SRSPROJ', 'SRS', $3)`, srsProjectID, now, srsAccountID)
	exec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, is_active, project_id) VALUES ($1, $2, $2, 't', 't', 'SRS-DEP', 'SRS prod', true, $3)`, srsDeployment, now, srsProjectID)
	exec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, deployment_id) VALUES ($1, $2, $2, 't', 't', 'SRS-DP', $3)`, srsDeployed, now, srsDeployment)
	exec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email, user_type) VALUES ($1, $2, $2, 'srs-customer', $3, 'EXTERNAL'), ($4, $2, $2, 'srs-staff', $5, 'INTERNAL')`, srsCustomerID, now, srsCustomer, srsStaffID, srsStaff)
	exec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id) VALUES ($1, $2, $2, 't', 't', 'Jane Doe', $3)`, srsContactID, now, srsAccountID)
	exec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state) VALUES (gen_random_uuid(), $1, $1, 't', 't', $2, $3, $4, 'REGISTERED')`, now, srsCustomer, srsContactID, srsProjectID)

	workItem := func(id, number, typ string) {
		exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id, deployment_id, deployed_product_id)
			VALUES ($1, $2, $2, 't', 't', $3, $4, 'sr severity fixture', $5::work_item_type_enum, $6, $7, $8)`,
			id, now, number, "WSO2-"+number, typ, srsProjectID, srsDeployment, srsDeployed)
	}
	workItem(srsSRHigh, "SRS-SR-HIGH", "SERVICE_REQUEST")
	exec(`INSERT INTO service_request (id, state, severity) VALUES ($1, 'OPEN', 'S2')`, srsSRHigh)
	workItem(srsSRLow, "SRS-SR-LOW", "SERVICE_REQUEST")
	exec(`INSERT INTO service_request (id, state, severity) VALUES ($1, 'OPEN', 'S4')`, srsSRLow)
	workItem(srsCaseCrit, "SRS-CASE-S1", "CASE")
	exec(`INSERT INTO "case" (id, state, severity, issue_type) VALUES ($1, 'OPEN', 'S1', 'ERROR')`, srsCaseCrit)
	workItem(srsCaseCata, "SRS-CASE-S0", "CASE")
	exec(`INSERT INTO "case" (id, state, severity, issue_type) VALUES ($1, 'OPEN', 'S0', 'ERROR')`, srsCaseCata)
	workItem(srsCaseNone, "SRS-CASE-NONE", "CASE")
	exec(`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, srsCaseNone)
	workItem(srsCaseMed, "SRS-CASE-S3", "CASE")
	exec(`INSERT INTO "case" (id, state, severity, issue_type) VALUES ($1, 'OPEN', 'S3', 'ERROR')`, srsCaseMed)
	workItem(srsEngagement, "SRS-ENG", "ENGAGEMENT")
	exec(`INSERT INTO engagement (id, state, type, payment_type) VALUES ($1, 'OPEN', 'CONSULTANCY', 'PAID')`, srsEngagement)
}

// srsStoredSeverity reads service_request.severity of id back from the table.
func srsStoredSeverity(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var sev *string
	if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT severity::TEXT FROM service_request WHERE id = $1`, id).Scan(&sev); err != nil {
		t.Fatalf("read service_request.severity of %s: %v", id, err)
	}
	if sev == nil {
		return "<NULL>"
	}
	return *sev
}

func srsNewService(pool *pgxpool.Pool, defaultLabel string) CaseService {
	scoped := repository.NewScoped(pool)
	repo := repository.NewCaseRepository(scoped)
	svc := NewCaseService(repo, repository.NewUserRepository(pool), nil, ctxIdentityAccess{}, repository.NewProjectContactRepository(pool))
	svc = WithServiceRequestDefaultSeverity(svc, defaultLabel)
	return WithCaseTypeTransfer(svc, repo)
}

func srsCreate(severity domain.CaseSeverity) domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type: "service_request", CreatedBy: srsStaffID, ProjectID: srsProjectID,
		DeploymentID: srsDeployment, DeployedProductID: srsDeployed,
		Subject: "sr severity test", Description: "d", Severity: severity,
		CatalogID: "00000000-0000-0000-0000-000000000001", CatalogItemID: "00000000-0000-0000-0000-000000000002",
		Variables: []domain.Variable{{ID: "00000000-0000-0000-0000-000000000003", Value: "v"}},
	}
}

func TestSRSeverityIntegration_Create(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	seedSRSeverityFixture(t, pool)

	t.Run("explicit severity from an internal user is stored", func(t *testing.T) {
		resp, err := srsNewService(pool, "").CreateCase(srsInternalCtx(t), srsCreate(domain.CaseSeverityHigh))
		if err != nil {
			t.Fatalf("CreateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S2" {
			t.Errorf("stored severity = %s, want S2", got)
		}
	})
	t.Run("explicit severity from an integration client is stored", func(t *testing.T) {
		resp, err := srsNewService(pool, "").CreateCase(srsM2MCtx(), srsCreate(domain.CaseSeverityCritical))
		if err != nil {
			t.Fatalf("CreateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S1" {
			t.Errorf("stored severity = %s, want S1", got)
		}
	})
	t.Run("the configured default applies when none is sent", func(t *testing.T) {
		resp, err := srsNewService(pool, "s2").CreateCase(srsInternalCtx(t), srsCreate(""))
		if err != nil {
			t.Fatalf("CreateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S2" {
			t.Errorf("stored severity = %s, want S2 (the configured default)", got)
		}
	})
	t.Run("no severity and no configured default is S4, never NULL", func(t *testing.T) {
		resp, err := srsNewService(pool, "").CreateCase(srsInternalCtx(t), srsCreate(""))
		if err != nil {
			t.Fatalf("CreateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S4" {
			t.Errorf("stored severity = %s, want S4", got)
		}
	})
	t.Run("S0 is refused and nothing is stored", func(t *testing.T) {
		var before int
		_ = repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()), `SELECT count(*) FROM work_item WHERE project_id = $1`, srsProjectID).Scan(&before)
		_, err := srsNewService(pool, "").CreateCase(srsInternalCtx(t), srsCreate(domain.CaseSeverityCatastrophic))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("CreateCase(catastrophic) = %v, want a ValidationError", err)
		}
		var after int
		_ = repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()), `SELECT count(*) FROM work_item WHERE project_id = $1`, srsProjectID).Scan(&after)
		if after != before {
			t.Errorf("work items in the project went from %d to %d on a refused create", before, after)
		}
	})
	t.Run("a customer's severity is dropped and the default stored", func(t *testing.T) {
		req := srsCreate(domain.CaseSeverityCritical)
		req.CreatedBy = srsCustomerID
		resp, err := srsNewService(pool, "S3").CreateCase(srsCustomerCtx(t), req)
		if err != nil {
			t.Fatalf("CreateCase as a customer: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S3" {
			t.Errorf("stored severity = %s, want S3: the customer's critical must not be stored", got)
		}
	})
	t.Run("a customer's catastrophic is dropped, not refused", func(t *testing.T) {
		req := srsCreate(domain.CaseSeverityCatastrophic)
		req.CreatedBy = srsCustomerID
		resp, err := srsNewService(pool, "").CreateCase(srsCustomerCtx(t), req)
		if err != nil {
			t.Fatalf("CreateCase as a customer: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S4" {
			t.Errorf("stored severity = %s, want S4", got)
		}
	})
	t.Run("a staff user through the customer portal cannot set it either", func(t *testing.T) {
		resp, err := srsNewService(pool, "").CreateCase(srsStaffViaPortalCtx(t), srsCreate(domain.CaseSeverityCritical))
		if err != nil {
			t.Fatalf("CreateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, resp.Case.ID); got != "S4" {
			t.Errorf("stored severity = %s, want S4", got)
		}
	})
	t.Run("a service request written without the column reads back S4", func(t *testing.T) {
		scoped := repository.NewScoped(pool)
		sys := repository.WithSystemIdentity(context.Background())
		if _, err := scoped.Exec(sys, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
			VALUES ('5e5e5e00-0000-4000-8000-0000000000f1', now(), now(), 't', 't', 'SRS-SR-RAW', 'WSO2-SRS-SR-RAW', 'raw', 'SERVICE_REQUEST', $1)`, srsProjectID); err != nil {
			t.Fatalf("seed work item: %v", err)
		}
		if _, err := scoped.Exec(sys, `INSERT INTO service_request (id, state) VALUES ('5e5e5e00-0000-4000-8000-0000000000f1', 'OPEN')`); err != nil {
			t.Fatalf("seed service request: %v", err)
		}
		if got := srsStoredSeverity(t, pool, "5e5e5e00-0000-4000-8000-0000000000f1"); got != "S4" {
			t.Errorf("severity of an SR inserted without the column = %s, want S4", got)
		}
		if _, err := scoped.Exec(sys, `UPDATE service_request SET severity = 'S0' WHERE id = '5e5e5e00-0000-4000-8000-0000000000f1'`); err == nil {
			t.Error("the database accepted S0 on a service request")
		}
	})
}

func TestSRSeverityIntegration_Update(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	seedSRSeverityFixture(t, pool)
	svc := srsNewService(pool, "")
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	sev := func(s domain.CaseSeverity) domain.UpdateCaseRequest {
		return domain.UpdateCaseRequest{ID: srsSRHigh, Severity: &s}
	}

	t.Run("an internal user changes it and the row says so", func(t *testing.T) {
		resp, err := svc.UpdateCase(srsInternalCtx(t), sev(domain.CaseSeverityCritical))
		if err != nil {
			t.Fatalf("UpdateCase: %v", err)
		}
		if resp.Case.Severity == nil || *resp.Case.Severity != domain.CaseSeverityCritical {
			t.Errorf("response severity = %v, want critical", resp.Case.Severity)
		}
		if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S1" {
			t.Errorf("stored severity = %s, want S1", got)
		}
		// The activity feed records old and new.
		var oldV, newV string
		if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()),
			`SELECT old_value, new_value FROM work_item_activity WHERE work_item_id = $1 AND field_name = 'severity'`, srsSRHigh).Scan(&oldV, &newV); err != nil {
			t.Fatalf("read the activity entry: %v", err)
		}
		if oldV != "High" || newV != "Critical" {
			t.Errorf("activity = %q -> %q, want High -> Critical", oldV, newV)
		}
	})
	t.Run("the repository reports the previous severity", func(t *testing.T) {
		low := domain.CaseSeverityLow
		_, prev, _, err := repo.UpdateCase(srsInternalCtx(t), domain.UpdateCaseRequest{ID: srsSRHigh, Severity: &low}, nil)
		if err != nil {
			t.Fatalf("UpdateCase: %v", err)
		}
		if prev == nil || *prev != domain.CaseSeverityCritical {
			t.Errorf("previous severity = %v, want critical (the value written by the step before)", prev)
		}
		if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S4" {
			t.Errorf("stored severity = %s, want S4", got)
		}
	})
	t.Run("an integration client may", func(t *testing.T) {
		if _, err := svc.UpdateCase(srsM2MCtx(), sev(domain.CaseSeverityMedium)); err != nil {
			t.Fatalf("UpdateCase: %v", err)
		}
		if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S3" {
			t.Errorf("stored severity = %s, want S3", got)
		}
	})
	t.Run("S0 is a validation error and changes nothing", func(t *testing.T) {
		_, err := svc.UpdateCase(srsInternalCtx(t), sev(domain.CaseSeverityCatastrophic))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("UpdateCase(catastrophic) = %v, want a ValidationError", err)
		}
		if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S3" {
			t.Errorf("stored severity = %s, want it to stay S3", got)
		}
	})
	for name, ctx := range map[string]func(*testing.T) context.Context{
		"a customer":                           srsCustomerCtx,
		"a staff user through customer portal": srsStaffViaPortalCtx,
	} {
		t.Run(name+" is refused and the row is untouched", func(t *testing.T) {
			_, err := svc.UpdateCase(ctx(t), sev(domain.CaseSeverityCritical))
			var fe *apierror.ForbiddenError
			if !errors.As(err, &fe) {
				t.Fatalf("UpdateCase = %v, want a ForbiddenError", err)
			}
			if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S3" {
				t.Errorf("stored severity = %s, want it to stay S3", got)
			}
		})
	}
	t.Run("the database refuses a non-internal change even if the service were bypassed", func(t *testing.T) {
		scoped := repository.NewScoped(pool)
		_, err := scoped.Exec(srsCustomerCtx(t), `UPDATE service_request SET severity = 'S1' WHERE id = $1`, srsSRHigh)
		if err == nil {
			t.Fatal("a customer-identity UPDATE of severity succeeded")
		}
		if got := srsStoredSeverity(t, pool, srsSRHigh); got != "S3" {
			t.Errorf("stored severity = %s, want S3", got)
		}
		// Another column of the same row is still theirs to change.
		if _, err := scoped.Exec(srsCustomerCtx(t), `UPDATE service_request SET close_notes = 'thanks' WHERE id = $1`, srsSRHigh); err != nil {
			t.Errorf("a customer-identity UPDATE of another column failed: %v", err)
		}
	})
	t.Run("a case's severity update is untouched", func(t *testing.T) {
		low := domain.CaseSeverityLow
		if _, err := svc.UpdateCase(srsInternalCtx(t), domain.UpdateCaseRequest{ID: srsCaseMed, Severity: &low}); err != nil {
			t.Fatalf("UpdateCase on a case: %v", err)
		}
		var got string
		_ = repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()), `SELECT severity::TEXT FROM "case" WHERE id = $1`, srsCaseMed).Scan(&got)
		if got != "S4" {
			t.Errorf("case severity = %s, want S4", got)
		}
	})
}

func TestSRSeverityIntegration_ReadFilterSortGroup(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	seedSRSeverityFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	internal := repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: srsStaff}
	customer := repository.SearchScope{ProjectIDs: []string{srsProjectID}, ViewerEmail: srsCustomer}
	portalStaff := repository.SearchScope{Unrestricted: true, ViaCustomerPortal: true, ViewerEmail: srsStaff}

	search := func(t *testing.T, scope repository.SearchScope, sevs []domain.CaseSeverity, field domain.CaseSortField) []domain.SearchCaseView {
		t.Helper()
		req := domain.SearchCasesRequest{
			Parsed:     domain.ParsedCaseFilters{ProjectIDs: []string{srsProjectID}, Severities: sevs},
			SortBy:     domain.CaseSort{Field: field, Order: domain.CaseSortOrderAsc},
			Pagination: domain.Pagination{Limit: 50},
		}
		views, _, err := repo.SearchCases(repository.WithCallerIdentity(context.Background(), scope), req, scope)
		if err != nil {
			t.Fatalf("SearchCases: %v", err)
		}
		return views
	}
	numbers := func(vs []domain.SearchCaseView) map[string]string {
		out := map[string]string{}
		for _, v := range vs {
			sev := "<nil>"
			if v.Severity != nil {
				sev = *v.Severity
			}
			out[v.Number] = sev
		}
		return out
	}

	t.Run("an internal filter by severity includes service requests", func(t *testing.T) {
		got := numbers(search(t, internal, []domain.CaseSeverity{domain.CaseSeverityHigh}, domain.CaseSortFieldCreatedOn))
		if len(got) != 1 || got["SRS-SR-HIGH"] != "high" {
			t.Errorf("filter high = %v, want exactly the high service request, reported as high", got)
		}
		got = numbers(search(t, internal, []domain.CaseSeverity{domain.CaseSeverityCritical, domain.CaseSeverityLow}, domain.CaseSortFieldCreatedOn))
		if got["SRS-CASE-S1"] != "critical" || got["SRS-SR-LOW"] != "low" || len(got) != 2 {
			t.Errorf("filter critical+low = %v, want the S1 case and the S4 service request", got)
		}
	})
	t.Run("an internal sort by severity orders service requests with cases", func(t *testing.T) {
		views := search(t, internal, nil, domain.CaseSortFieldSeverity)
		var order []string
		for _, v := range views {
			if v.Severity != nil {
				order = append(order, v.Number)
			}
		}
		want := []string{"SRS-CASE-S0", "SRS-CASE-S1", "SRS-SR-HIGH", "SRS-CASE-S3", "SRS-SR-LOW"}
		if len(order) != len(want) {
			t.Fatalf("rows with a severity = %v, want %v", order, want)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("severity order = %v, want %v", order, want)
			}
		}
	})
	t.Run("group by severity counts service requests for an internal caller", func(t *testing.T) {
		req := domain.SearchCasesRequest{Parsed: domain.ParsedCaseFilters{ProjectIDs: []string{srsProjectID}}}
		buckets, err := repo.AggregateCases(repository.WithCallerIdentity(context.Background(), internal), req, "severity", internal)
		if err != nil {
			t.Fatalf("AggregateCases: %v", err)
		}
		got := map[string]int{}
		for _, b := range buckets {
			got[b.Key] = b.Count
		}
		want := map[string]int{"catastrophic": 1, "critical": 1, "high": 1, "medium": 1, "low": 1}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("bucket %s = %d, want %d (all buckets %v)", k, got[k], v, got)
			}
		}
	})
	for name, scope := range map[string]repository.SearchScope{"a customer": customer, "a staff user through the customer portal": portalStaff} {
		t.Run(name+" cannot see, filter, sort or count a service request's severity", func(t *testing.T) {
			if got := numbers(search(t, scope, []domain.CaseSeverity{domain.CaseSeverityHigh}, domain.CaseSortFieldCreatedOn)); len(got) != 0 {
				t.Errorf("filter high matched %v: a service request's severity leaked through the filter", got)
			}
			if got := numbers(search(t, scope, []domain.CaseSeverity{domain.CaseSeverityCritical}, domain.CaseSortFieldCreatedOn)); len(got) != 1 || got["SRS-CASE-S1"] != "critical" {
				t.Errorf("filter critical = %v, want only the case", got)
			}
			for number, sevStr := range numbers(search(t, scope, nil, domain.CaseSortFieldSeverity)) {
				if (number == "SRS-SR-HIGH" || number == "SRS-SR-LOW") && sevStr != "<nil>" {
					t.Errorf("%s reports severity %s to a caller that may not see it", number, sevStr)
				}
			}
			req := domain.SearchCasesRequest{Parsed: domain.ParsedCaseFilters{ProjectIDs: []string{srsProjectID}}}
			buckets, err := repo.AggregateCases(repository.WithCallerIdentity(context.Background(), scope), req, "severity", scope)
			if err != nil {
				t.Fatalf("AggregateCases: %v", err)
			}
			for _, b := range buckets {
				if b.Key == "high" || (b.Key == "low" && b.Count != 0) {
					t.Errorf("bucket %s = %d counts a service request", b.Key, b.Count)
				}
			}
		})
	}
	t.Run("get by id: internal sees it, a customer and customer-portal staff do not", func(t *testing.T) {
		cv, err := repo.GetCaseByID(context.Background(), srsSRHigh, internal)
		if err != nil {
			t.Fatalf("GetCaseByID: %v", err)
		}
		if cv.Severity == nil || *cv.Severity != domain.CaseSeverityHigh {
			t.Errorf("internal view severity = %v, want high", cv.Severity)
		}
		for name, scope := range map[string]repository.SearchScope{"customer": customer, "portal staff": portalStaff} {
			cv, err := repo.GetCaseByID(context.Background(), srsSRHigh, scope)
			if err != nil {
				t.Fatalf("GetCaseByID as %s: %v", name, err)
			}
			if cv.Severity != nil {
				t.Errorf("%s sees severity %v of a service request", name, *cv.Severity)
			}
		}
	})
}

func TestSRSeverityIntegration_TypeTransfer(t *testing.T) {
	pool := newCustomerReplyIntegrationPool(t)
	svc := srsNewService(pool, "")
	toSR := func(t *testing.T, id string) domain.UpdateCaseResponse {
		t.Helper()
		typ := "service_request"
		cat, item := "00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"
		resp, err := svc.UpdateCase(srsInternalCtx(t), domain.UpdateCaseRequest{
			ID: id, Type: &typ, CatalogID: &cat, CatalogItemID: &item,
			Variables: []domain.Variable{{ID: "00000000-0000-0000-0000-000000000003", Value: "v"}},
		})
		if err != nil {
			t.Fatalf("transfer %s: %v", id, err)
		}
		return resp
	}
	cases := []struct {
		name     string
		id       string
		want     string
		warnFrom string
	}{
		{"S1 carries unchanged", srsCaseCrit, "S1", ""},
		{"S3 carries unchanged", srsCaseMed, "S3", ""},
		{"no severity becomes S4", srsCaseNone, "S4", ""},
		{"a non-case source becomes S4", srsEngagement, "S4", ""},
		{"S0 is stored as S1 and reported", srsCaseCata, "S1", "catastrophic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedSRSeverityFixture(t, pool)
			resp := toSR(t, tc.id)
			if got := srsStoredSeverity(t, pool, tc.id); got != tc.want {
				t.Errorf("stored severity = %s, want %s", got, tc.want)
			}
			if tc.warnFrom == "" {
				if len(resp.Warnings) != 0 {
					t.Errorf("warnings = %+v, want none", resp.Warnings)
				}
				return
			}
			if len(resp.Warnings) != 1 || resp.Warnings[0].Code != domain.ResponseWarningSeverityDowngraded ||
				resp.Warnings[0].From != tc.warnFrom || resp.Warnings[0].To != "critical" {
				t.Errorf("warnings = %+v, want one SEVERITY_DOWNGRADED catastrophic -> critical", resp.Warnings)
			}
			if resp.Case.Severity == nil || *resp.Case.Severity != domain.CaseSeverityCritical {
				t.Errorf("response severity = %v, want critical", resp.Case.Severity)
			}
		})
	}
	t.Run("a service request moved to a case needs the case's own severity", func(t *testing.T) {
		seedSRSeverityFixture(t, pool)
		typ := "case"
		issue := domain.CaseIssueTypeError
		if _, err := svc.UpdateCase(srsInternalCtx(t), domain.UpdateCaseRequest{ID: srsSRHigh, Type: &typ, IssueType: &issue}); err == nil {
			t.Fatal("transfer to case without a severity succeeded")
		}
		low := domain.CaseSeverityLow
		if _, err := svc.UpdateCase(srsInternalCtx(t), domain.UpdateCaseRequest{ID: srsSRHigh, Type: &typ, IssueType: &issue, Severity: &low}); err != nil {
			t.Fatalf("transfer to case with a severity: %v", err)
		}
		var got string
		if err := repository.NewScoped(pool).QueryRow(repository.WithSystemIdentity(context.Background()), `SELECT severity::TEXT FROM "case" WHERE id = $1`, srsSRHigh).Scan(&got); err != nil {
			t.Fatalf("read the case row: %v", err)
		}
		if got != "S4" {
			t.Errorf("case severity = %s, want S4", got)
		}
	})
}
