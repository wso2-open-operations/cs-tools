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

// A transfer into a service request stores the catalog item and the answers in
// service_request.json_data, and the case detail reads them back. Same setup
// and skip rule as case_type_transfer_repo_integration_test.go.

package repository_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// seedCTCatalog adds the catalog, item and two of the three variables the plan
// names (the third is deliberately absent), and removes them afterwards.
func (f *ctFixture) seedCTCatalog(t *testing.T) {
	t.Helper()
	now := time.Now().UTC()
	cleanup := func() {
		for _, sql := range []string{
			`DELETE FROM catalog_variable WHERE catalog_item_id = '` + ctCatalogItemID + `'`,
			`DELETE FROM catalog_item_category WHERE catalog_item_id = '` + ctCatalogItemID + `'`,
			`DELETE FROM catalog_item WHERE id = '` + ctCatalogItemID + `'`,
			`DELETE FROM sr_category WHERE id = '` + ctCatalogID + `'`,
		} {
			_, _ = f.scoped.Exec(f.ctx, sql)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	f.exec(t, `INSERT INTO sr_category (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', 'Fixture Catalog')`, ctCatalogID, now)
	f.exec(t, `INSERT INTO catalog_item (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, $2, $2, 'test', 'test', 'Fixture Item')`, ctCatalogItemID, now)
	f.exec(t, `INSERT INTO catalog_variable (id, created_on, updated_on, created_by, updated_by, question_text, "order", catalog_item_id)
		VALUES ($1, $3, $3, 'test', 'test', 'Second question', 2, $2), ($4, $3, $3, 'test', 'test', 'First question', 1, $2)`,
		ctVarFirstID, ctCatalogItemID, now, ctVarSecondID)
}

func (f *ctFixture) jsonData(t *testing.T, id string) map[string]any {
	t.Helper()
	var raw []byte
	if err := f.scoped.QueryRow(f.ctx, `SELECT json_data FROM service_request WHERE id = $1`, id).Scan(&raw); err != nil {
		t.Fatalf("read json_data: %v", err)
	}
	if raw == nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json_data is not an object: %v", err)
	}
	return out
}

func TestCaseTypeTransferIntegration_IntoServiceRequest_StoresAndReadsBackTheCatalogRequest(t *testing.T) {
	for name, id := range map[string]string{"Incident (S2)": ctIncidentID, "Query (S4)": ctQueryID, "engagement": ctEngagementID, "security report analysis": ctSRAID} {
		t.Run(name, func(t *testing.T) {
			f := newCTFixture(t)
			f.seedCTCatalog(t)

			if _, err := f.repo.TransferCaseType(f.ctx, ctServiceRequestPlan(id), nil); err != nil {
				t.Fatalf("TransferCaseType: %v", err)
			}
			f.assertOnlyIn(t, id, "service_request")

			want := map[string]any{
				"catalogId":     ctCatalogID,
				"catalogItemId": ctCatalogItemID,
				"variables": map[string]any{
					ctVarFirstID:   "answer to the second question",
					ctVarSecondID:  "answer to the first question",
					ctVarUnknownID: "answer to a question no longer in the catalog",
				},
			}
			if got := f.jsonData(t, id); !reflect.DeepEqual(got, want) {
				t.Errorf("json_data = %v, want %v", got, want)
			}

			cv, err := repository.NewCaseRepository(f.scoped).GetCaseByID(f.ctx, id, repository.SearchScope{Unrestricted: true})
			if err != nil {
				t.Fatalf("GetCaseByID: %v", err)
			}
			if cv.Type == nil || *cv.Type != "service_request" {
				t.Fatalf("type = %v", cv.Type)
			}
			if cv.Catalog == nil || *cv.Catalog != (domain.EntityRef{ID: ctCatalogID, Name: "Fixture Catalog"}) {
				t.Errorf("catalog = %+v", cv.Catalog)
			}
			if cv.CatalogItem == nil || *cv.CatalogItem != (domain.EntityRef{ID: ctCatalogItemID, Name: "Fixture Item"}) {
				t.Errorf("catalogItem = %+v", cv.CatalogItem)
			}
			// Ordered by the question's own order; a question gone from the catalog
			// keeps its id as its name and goes last.
			wantVars := []domain.CaseVariable{
				{Name: "First question", Value: "answer to the first question"},
				{Name: "Second question", Value: "answer to the second question"},
				{Name: ctVarUnknownID, Value: "answer to a question no longer in the catalog"},
			}
			if !reflect.DeepEqual(cv.Variables, wantVars) {
				t.Errorf("variables = %+v, want %+v", cv.Variables, wantVars)
			}
		})
	}
}

// Every other target is exactly as before: no json_data to speak of, and a case
// detail with no catalog fields.
func TestCaseTypeTransferIntegration_OtherTargetsCarryNoCatalogRequest(t *testing.T) {
	f := newCTFixture(t)
	f.seedCTCatalog(t)
	repo := repository.NewCaseRepository(f.scoped)

	for target, plan := range map[string]repository.CaseTypeTransfer{
		"engagement":               ctEngagementPlan(ctIncidentID),
		"security_report_analysis": {CaseID: ctQueryID, TargetType: "security_report_analysis", ActorEmail: ctActor},
		"case":                     ctCasePlan(ctServiceReqID, domain.CaseSeverityHigh),
	} {
		if _, err := f.repo.TransferCaseType(f.ctx, plan, nil); err != nil {
			t.Fatalf("transfer to %s: %v", target, err)
		}
		cv, err := repo.GetCaseByID(f.ctx, plan.CaseID, repository.SearchScope{Unrestricted: true})
		if err != nil {
			t.Fatalf("GetCaseByID (%s): %v", target, err)
		}
		if cv.Catalog != nil || cv.CatalogItem != nil || cv.Variables != nil {
			t.Errorf("%s carries catalog fields: %+v %+v %+v", target, cv.Catalog, cv.CatalogItem, cv.Variables)
		}
	}
	// The service request that became a case left no json_data behind.
	if n := f.count(t, `SELECT COUNT(*) FROM service_request WHERE id = $1`, ctServiceReqID); n != 0 {
		t.Errorf("service_request still holds the transferred ticket")
	}
}

// A service request out and back in again holds the new request, not the old.
func TestCaseTypeTransferIntegration_ServiceRequestRoundTripReplacesTheCatalogRequest(t *testing.T) {
	f := newCTFixture(t)
	f.seedCTCatalog(t)
	if _, err := f.repo.TransferCaseType(f.ctx, ctServiceRequestPlan(ctIncidentID), nil); err != nil {
		t.Fatalf("into service_request: %v", err)
	}
	if _, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctIncidentID), nil); err != nil {
		t.Fatalf("out of service_request: %v", err)
	}
	plan := ctServiceRequestPlan(ctIncidentID)
	plan.Variables = []domain.Variable{{ID: ctVarFirstID, Value: "only answer"}}
	if _, err := f.repo.TransferCaseType(f.ctx, plan, nil); err != nil {
		t.Fatalf("into service_request again: %v", err)
	}
	vars, _ := f.jsonData(t, ctIncidentID)["variables"].(map[string]any)
	if len(vars) != 1 || vars[ctVarFirstID] != "only answer" {
		t.Errorf("variables = %v", vars)
	}
}

// A plan the service would have refused is refused by the repository too, and
// nothing moves.
func TestCaseTypeTransferIntegration_ServiceRequestWithoutCatalogChangesNothing(t *testing.T) {
	f := newCTFixture(t)
	_, err := f.repo.TransferCaseType(f.ctx, repository.CaseTypeTransfer{CaseID: ctIncidentID, TargetType: "service_request", ActorEmail: ctActor}, nil)
	if err == nil {
		t.Fatal("want an error for a service request with no catalog item")
	}
	f.assertOnlyIn(t, ctIncidentID, "case")
}
