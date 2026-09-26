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

package dto

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
)

// TestBuildEntityCreateCaseRequest_SeverityAndIssueTypeKeysTranslate guards
// the case-create numeric-key translation added when aligning this backend's
// response/request shapes to the old Ballerina backend's contract.
func TestBuildEntityCreateCaseRequest_SeverityAndIssueTypeKeysTranslate(t *testing.T) {
	sev, issue := 11, 4 // high, question
	got := BuildEntityCreateCaseRequest(CreateCaseRequest{
		Title:        "Cannot log in",
		ProjectID:    "proj-1",
		SeverityKey:  &sev,
		IssueTypeKey: &issue,
	})
	if got.Severity != "high" {
		t.Fatalf("Severity = %q, want \"high\"", got.Severity)
	}
	if got.IssueType != "question" {
		t.Fatalf("IssueType = %q, want \"question\"", got.IssueType)
	}
	if got.Subject != "Cannot log in" {
		t.Fatalf("Subject = %q, want title passthrough", got.Subject)
	}
}

// TestBuildEntityCreateCaseRequest_UnrecognizedKeysProduceEmptyEnum verifies
// an unrecognized numeric key never leaks through as a raw number.
func TestBuildEntityCreateCaseRequest_UnrecognizedKeysProduceEmptyEnum(t *testing.T) {
	bad := 999999
	got := BuildEntityCreateCaseRequest(CreateCaseRequest{SeverityKey: &bad})
	if got.Severity != "" {
		t.Fatalf("Severity = %q, want empty for unrecognized key", got.Severity)
	}
}

// TestBuildEntityUpdateCaseRequest_StateKeyTranslates verifies PATCH
// /cases/{id}'s stateKey (the only mutation field besides watchList) maps to
// entity-service's enum string.
func TestBuildEntityUpdateCaseRequest_StateKeyTranslates(t *testing.T) {
	state := 10 // work_in_progress
	got := BuildEntityUpdateCaseRequest("case-1", UpdateCaseRequest{StateKey: &state, WatchList: []string{"a@x.com"}})
	if got.ID != "case-1" {
		t.Fatalf("ID = %q, want case-1", got.ID)
	}
	if got.State == nil || *got.State != "work_in_progress" {
		t.Fatalf("State = %v, want work_in_progress", got.State)
	}
	if len(got.WatchList) != 1 || got.WatchList[0] != "a@x.com" {
		t.Fatalf("WatchList = %v", got.WatchList)
	}
}

// TestBuildEntitySearchChangeRequestsRequest_ScopesProjectAndTranslatesKeys
// verifies the change-request search translator forces project scope from
// the path and translates stateKeys/impactKeys via the numeric-id mirror
// table (entity-service's change-request search response is pre-normalized,
// unlike case search, so this is a direct lookup, not fuzzy label matching).
func TestBuildEntitySearchChangeRequestsRequest_ScopesProjectAndTranslatesKeys(t *testing.T) {
	got := BuildEntitySearchChangeRequestsRequest("proj-9", ChangeRequestSearchRequest{
		Filters: ChangeRequestSearchFilters{StateKeys: []int{5}, ImpactKeys: []int{1}}, // customer_approval, high
	})
	if len(got.Filters.ProjectIDs) != 1 || got.Filters.ProjectIDs[0] != "proj-9" {
		t.Fatalf("ProjectIDs = %v, want [proj-9]", got.Filters.ProjectIDs)
	}
	if len(got.Filters.States) != 1 || got.Filters.States[0] != "customer_approval" {
		t.Fatalf("States = %v, want [customer_approval]", got.Filters.States)
	}
	if len(got.Filters.Impacts) != 1 || got.Filters.Impacts[0] != "high" {
		t.Fatalf("Impacts = %v, want [high]", got.Filters.Impacts)
	}
}

// TestCrStateRef_KnownAndUnknownValues verifies the response-side {id,label}
// translation for change-request state.
func TestCrStateRef_KnownAndUnknownValues(t *testing.T) {
	state := "customer_approval"
	got := crStateRef(&state)
	if got == nil || got.ID != "5" || got.Label != "Customer Approval" {
		t.Fatalf("crStateRef(customer_approval) = %+v, want {id: 5, label: Customer Approval}", got)
	}
	if crStateRef(nil) != nil {
		t.Fatalf("crStateRef(nil) should be nil")
	}
}

// TestBuildEntitySearchConversationsRequest_ScopesProjectAndTranslatesStateKeys
// guards the fix for stateKeys being a complete no-op (the handler
// previously decoded straight into entity.SearchConversationsRequest, whose
// field names never matched the frontend's body at all).
func TestBuildEntitySearchConversationsRequest_ScopesProjectAndTranslatesStateKeys(t *testing.T) {
	got, err := BuildEntitySearchConversationsRequest("proj-3", ConversationSearchRequest{
		Filters: ConversationSearchFilters{StateKeys: []int{2}, CreatedByMe: true}, // ACTIVE
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Filters.ProjectIDs) != 1 || got.Filters.ProjectIDs[0] != "proj-3" {
		t.Fatalf("ProjectIDs = %v, want [proj-3]", got.Filters.ProjectIDs)
	}
	if len(got.Filters.States) != 1 || got.Filters.States[0] != "ACTIVE" {
		t.Fatalf("States = %v, want [ACTIVE]", got.Filters.States)
	}
	if !got.Filters.CreatedByMe {
		t.Fatalf("CreatedByMe = false, want true")
	}
}

// TestMapSearchTimeCards_WorkDateBecomesCreatedOn verifies the frontend's
// TimeCard.createdOn is sourced from entity-service's WorkDate field, and
// state/reportedBy/approvedBy/project all become {id,label} refs.
func TestMapSearchTimeCards_WorkDateBecomesCreatedOn(t *testing.T) {
	state := "approved"
	r := entity.SearchTimeCardsResponse{
		Total: 1,
		TimeCards: []entity.TimeCardView{
			{ID: "tc-1", TotalTime: 3.5, WorkDate: "2026-01-15", State: &state},
		},
	}
	got := MapSearchTimeCards(r)
	if got.TotalRecords != 1 {
		t.Fatalf("TotalRecords = %d, want 1", got.TotalRecords)
	}
	if len(got.TimeCards) != 1 || got.TimeCards[0].CreatedOn != "2026-01-15" {
		t.Fatalf("TimeCards[0].CreatedOn = %q, want entity-service's WorkDate value", got.TimeCards[0].CreatedOn)
	}
}

// TestBuildEntitySearchProjectsRequest_NestedFiltersUnwrapped verifies the
// frontend's nested filters.searchQuery body translates to entity-service's
// flat SearchQuery field.
func TestBuildEntitySearchProjectsRequest_NestedFiltersUnwrapped(t *testing.T) {
	got := BuildEntitySearchProjectsRequest(SearchProjectsRequest{
		Filters: SearchProjectsFilters{SearchQuery: "acme"},
	})
	if got.SearchQuery != "acme" {
		t.Fatalf("SearchQuery = %q, want acme", got.SearchQuery)
	}
}

// TestDeploymentTypeIDToEnumPtr_KnownAndUnknown verifies the deployment-type
// numeric-key translation used by both create and patch deployment.
func TestDeploymentTypeIDToEnumPtr_KnownAndUnknown(t *testing.T) {
	got := deploymentTypeIDToEnumPtr(6) // primary_production
	if got == nil || *got != "primary_production" {
		t.Fatalf("deploymentTypeIDToEnumPtr(6) = %v, want primary_production", got)
	}
	if deploymentTypeIDToEnumPtr(9999) != nil {
		t.Fatalf("deploymentTypeIDToEnumPtr(9999) should be nil for an unrecognized key")
	}
}

// TestBuildEntityCreateDeploymentRequest_ForcesProjectIDAndTranslatesType
// verifies POST /projects/{id}/deployments always scopes to the path's
// project id and translates deploymentTypeKey.
func TestBuildEntityCreateDeploymentRequest_ForcesProjectIDAndTranslatesType(t *testing.T) {
	got := BuildEntityCreateDeploymentRequest("proj-7", DeploymentCreateRequest{
		Name: "Prod", DeploymentTypeKey: 6, Description: "primary env",
	})
	if got.ProjectID != "proj-7" {
		t.Fatalf("ProjectID = %q, want proj-7", got.ProjectID)
	}
	if got.Type == nil || *got.Type != "primary_production" {
		t.Fatalf("Type = %v, want primary_production", got.Type)
	}
}

// TestParseCoresAndTPS_ParsesServiceNowStrings verifies entity-service's
// ServiceNow string Cores/TPS fields parse into numbers for the frontend's
// `typeof item.cores === "number"` checks, and that an absent/unparseable
// value maps to nil rather than 0 or a truncated garbage value.
func TestParseCoresAndTPS_ParsesServiceNowStrings(t *testing.T) {
	cores := "4"
	tps := "12.5"
	if got := parseCores(&cores); got == nil || *got != 4 {
		t.Fatalf("parseCores(4) = %v, want 4", got)
	}
	if got := parseTPS(&tps); got == nil || *got != 12.5 {
		t.Fatalf("parseTPS(12.5) = %v, want 12.5", got)
	}
	if parseCores(nil) != nil {
		t.Fatalf("parseCores(nil) should be nil")
	}
	bad := "not-a-number"
	if parseCores(&bad) != nil {
		t.Fatalf("parseCores(%q) should be nil, not a garbage parse", bad)
	}
}

// TestVulnerabilitySeverityRef_MatchesKnownLabelCaseInsensitively verifies
// the product-vulnerability severity translation resolves entity-service's
// raw upstream label (e.g. "High") to the numeric ServiceNow severity id
// mirrored from vulnerabilityPriorityToSeverityID.
func TestVulnerabilitySeverityRef_MatchesKnownLabelCaseInsensitively(t *testing.T) {
	got := vulnerabilitySeverityRef("High")
	if got == nil || got.ID != "3" || got.Label != "High" {
		t.Fatalf("vulnerabilitySeverityRef(High) = %+v, want {id: 3, label: High}", got)
	}
	if got := vulnerabilitySeverityRef("Unrecognized"); got == nil || got.ID != "" || got.Label != "Unrecognized" {
		t.Fatalf("vulnerabilitySeverityRef(Unrecognized) = %+v, want a label-only fallback ref", got)
	}
}

// TestBuildEntitySearchProductVulnerabilitiesRequest_SeverityIDTranslates
// verifies the frontend's numeric severityId filter becomes entity-service's
// lowercase Priority enum string.
func TestBuildEntitySearchProductVulnerabilitiesRequest_SeverityIDTranslates(t *testing.T) {
	sevID := 3 // high
	got := BuildEntitySearchProductVulnerabilitiesRequest(SearchProductVulnerabilitiesRequest{
		Filters: &SearchProductVulnerabilitiesFilters{SeverityID: &sevID},
	})
	if got.Filters == nil || got.Filters.Priority == nil || *got.Filters.Priority != "high" {
		t.Fatalf("Priority = %v, want high", got.Filters.Priority)
	}
}

// TestBuildEntitySearchCatalogsRequest_ScopesDeployedProductID verifies the
// catalog search translator always scopes to the deployed product in the
// URL, matching the corrected route
// POST /deployments/products/{deployedProductId}/catalogs/search.
func TestBuildEntitySearchCatalogsRequest_ScopesDeployedProductID(t *testing.T) {
	got := BuildEntitySearchCatalogsRequest("dp-1", SearchCatalogsRequest{Pagination: entity.Pagination{Limit: 10}})
	if got.DeployedProductID != "dp-1" {
		t.Fatalf("DeployedProductID = %q, want dp-1", got.DeployedProductID)
	}
	if got.Pagination.Limit != 10 {
		t.Fatalf("Pagination.Limit = %d, want 10", got.Pagination.Limit)
	}
}

// TestMapSearchCatalogs_ItemsUseLabelNotName verifies catalog items map to
// {id, label} (matching the frontend's CatalogItem = MetadataItem type)
// while the catalog container itself keeps its Name field.
func TestMapSearchCatalogs_ItemsUseLabelNotName(t *testing.T) {
	got := MapSearchCatalogs(entity.SearchCatalogsResponse{
		Total: 1,
		Catalogs: []entity.CatalogView{
			{ID: "cat-1", Name: "Support Catalog", CatalogItems: []entity.CatalogItem{{ID: "item-1", Name: "Restart Service"}}},
		},
	})
	if got.TotalRecords != 1 {
		t.Fatalf("TotalRecords = %d, want 1", got.TotalRecords)
	}
	if got.Catalogs[0].Name != "Support Catalog" {
		t.Fatalf("Catalog.Name = %q, want passthrough", got.Catalogs[0].Name)
	}
	if len(got.Catalogs[0].CatalogItems) != 1 || got.Catalogs[0].CatalogItems[0].Label != "Restart Service" {
		t.Fatalf("CatalogItems[0].Label = %q, want Restart Service", got.Catalogs[0].CatalogItems[0].Label)
	}
}

// TestMapRegistryToken_NumericIDAndExpiresAt verifies the registry token's
// ID/ExpiresAt fields decode as numbers, matching the frontend's
// RegistryToken type (registryTokenExpiresWithinDays does `token.expiresAt <
// nowSec`, which requires a number, not a string).
func TestMapRegistryToken_NumericIDAndExpiresAt(t *testing.T) {
	id := int64(42)
	expiresAt := int64(1893456000)
	got := MapRegistryToken(registry.Token{ID: &id, Name: "robot-1", ExpiresAt: &expiresAt})
	if got.ID == nil || *got.ID != 42 {
		t.Fatalf("ID = %v, want 42", got.ID)
	}
	if got.ExpiresAt == nil || *got.ExpiresAt != expiresAt {
		t.Fatalf("ExpiresAt = %v, want %d", got.ExpiresAt, expiresAt)
	}
}

// TestFilterProductsByClass_KeepsOnlyMatchingCaseInsensitive verifies
// GET /products' post-fetch class filter (entity-service has no class
// filter parameter to forward to, see FilterProductsByClass's doc comment)
// matches case-insensitively and drops products with no Class at all rather
// than treating them as a match.
func TestFilterProductsByClass_KeepsOnlyMatchingCaseInsensitive(t *testing.T) {
	software := "software"
	service := "Service"
	r := SearchProductsResponse{
		Products: []ProductSummary{
			{ID: "p1", Class: &software},
			{ID: "p2", Class: &service},
			{ID: "p3", Class: nil},
		},
		TotalRecords: 3,
	}

	got := FilterProductsByClass(r, "service")
	if len(got.Products) != 1 || got.Products[0].ID != "p2" {
		t.Fatalf("Products = %+v, want only p2 (case-insensitive match on Service)", got.Products)
	}

	// An empty want (no ?class= supplied) must pass every product through
	// unfiltered, not exclude everything.
	unfiltered := FilterProductsByClass(r, "")
	if len(unfiltered.Products) != 3 {
		t.Fatalf("FilterProductsByClass with empty want dropped items: got %d, want 3", len(unfiltered.Products))
	}
}

// TestBuildEntityUpdateDeploymentRequest_PreservesExplicitNullDescription
// guards against collapsing an explicit {"description": null} into "field
// absent" — entity-service's own UpdateDeploymentRequest.Description is
// json.RawMessage specifically to distinguish the three states (absent /
// null / value); a *string on this portal's request DTO couldn't represent
// the null case at all.
func TestBuildEntityUpdateDeploymentRequest_PreservesExplicitNullDescription(t *testing.T) {
	var absent DeploymentUpdateRequest
	if len(BuildEntityUpdateDeploymentRequest("dep-1", absent).Description) != 0 {
		t.Fatalf("absent Description must not populate entity.UpdateDeploymentRequest.Description")
	}

	explicitNull := DeploymentUpdateRequest{Description: []byte("null")}
	gotNull := BuildEntityUpdateDeploymentRequest("dep-1", explicitNull)
	if string(gotNull.Description) != "null" {
		t.Fatalf("Description = %q, want the literal JSON null preserved through", string(gotNull.Description))
	}

	withValue := DeploymentUpdateRequest{Description: []byte(`"Updated description"`)}
	gotValue := BuildEntityUpdateDeploymentRequest("dep-1", withValue)
	if string(gotValue.Description) != `"Updated description"` {
		t.Fatalf("Description = %q, want the raw JSON string value preserved through", string(gotValue.Description))
	}
}
