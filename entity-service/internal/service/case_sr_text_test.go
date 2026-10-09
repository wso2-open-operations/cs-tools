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

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeSRCatalog struct {
	vars              []domain.CatalogItemVariable
	varsErr           error
	requiredCatalogID string
	items             []domain.Catalog
}

func (f fakeSRCatalog) SearchCatalogs(context.Context, domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	return domain.SearchCatalogsResponse{Catalogs: f.items}, nil
}
func (f fakeSRCatalog) GetCatalogItemVariables(_ context.Context, catalogID, _ string) (domain.GetCatalogItemVariablesResponse, error) {
	if f.requiredCatalogID != "" && catalogID != f.requiredCatalogID {
		return domain.GetCatalogItemVariablesResponse{}, errors.New("invalid catalog ID: " + catalogID)
	}
	return domain.GetCatalogItemVariablesResponse{Variables: f.vars}, f.varsErr
}

func srFormRequest(answers ...domain.Variable) domain.CreateCaseRequest {
	return domain.CreateCaseRequest{Type: "service_request", CatalogID: "cat-1", CatalogItemID: "item-1", DeployedProductID: "dp-1", Variables: answers}
}

var srFormQuestions = []domain.CatalogItemVariable{
	{ID: "v-details", QuestionText: "Details of the request", Order: 2},
	{ID: "v-title", QuestionText: "* Title", Order: 1},
	{ID: "v-env", QuestionText: "Environment", Order: 3},
}

func TestFillServiceRequestText(t *testing.T) {
	catalog := fakeSRCatalog{
		vars:  srFormQuestions,
		items: []domain.Catalog{{ID: "cat-1", CatalogItems: []domain.CatalogItem{{ID: "item-1", Name: "Rotate API Gateway Key"}}}},
	}

	t.Run("title answer and the rest as description", func(t *testing.T) {
		s := &caseService{srCatalog: catalog}
		req := srFormRequest(
			domain.Variable{ID: "v-title", Value: " Rotate the prod key "},
			domain.Variable{ID: "v-env", Value: "prod <eu>"},
			domain.Variable{ID: "v-details", Value: "before Friday"},
		)
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Rotate the prod key" {
			t.Errorf("Subject = %q", req.Subject)
		}
		want := "<p><strong>Details of the request</strong>: before Friday</p><p><strong>Environment</strong>: prod &lt;eu&gt;</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q (question order, escaped, title excluded)", req.Description, want)
		}
	})

	t.Run("no title answer falls back to the catalog item name", func(t *testing.T) {
		s := &caseService{srCatalog: catalog}
		req := srFormRequest(domain.Variable{ID: "v-details", Value: "x"})
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Rotate API Gateway Key" {
			t.Errorf("Subject = %q, want the catalog item name", req.Subject)
		}
	})

	t.Run("what the caller sent is kept", func(t *testing.T) {
		s := &caseService{srCatalog: catalog}
		req := srFormRequest(domain.Variable{ID: "v-title", Value: "from form"})
		req.Subject, req.Description = "sent subject", "sent description"
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "sent subject" || req.Description != "sent description" {
			t.Errorf("got %q / %q, want the caller's own values", req.Subject, req.Description)
		}
	})

	t.Run("only service requests", func(t *testing.T) {
		s := &caseService{srCatalog: catalog}
		req := srFormRequest(domain.Variable{ID: "v-title", Value: "t"})
		req.Type = "case"
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "" {
			t.Errorf("Subject = %q for a case, want untouched", req.Subject)
		}
	})

	t.Run("variables lookup failure still names the SR after its item", func(t *testing.T) {
		s := &caseService{srCatalog: fakeSRCatalog{varsErr: errors.New("down"), items: catalog.items}}
		req := srFormRequest(domain.Variable{ID: "v-title", Value: "t"})
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Rotate API Gateway Key" || req.Description != "" {
			t.Errorf("got %q / %q", req.Subject, req.Description)
		}
	})

	t.Run("short description becomes title when title is absent", func(t *testing.T) {
		shortDescQuestions := []domain.CatalogItemVariable{
			{ID: "v-short", QuestionText: "Short Description", Order: 1},
			{ID: "v-env", QuestionText: "Environment", Order: 2},
		}
		cat := fakeSRCatalog{
			vars:  shortDescQuestions,
			items: catalog.items,
		}
		s := &caseService{srCatalog: cat}
		req := srFormRequest(
			domain.Variable{ID: "v-short", Value: "Need urgent patch"},
			domain.Variable{ID: "v-env", Value: "Production"},
		)
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Need urgent patch" {
			t.Errorf("Subject = %q, want 'Need urgent patch'", req.Subject)
		}
		want := "<p><strong>Environment</strong>: Production</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q", req.Description, want)
		}
	})

	t.Run("request details becomes title when title is absent", func(t *testing.T) {
		reqDetailsQuestions := []domain.CatalogItemVariable{
			{ID: "v-details", QuestionText: "Request Details", Order: 1},
			{ID: "v-desc", QuestionText: "Description", Order: 2},
		}
		cat := fakeSRCatalog{
			vars:  reqDetailsQuestions,
			items: catalog.items,
		}
		s := &caseService{srCatalog: cat}
		req := srFormRequest(
			domain.Variable{ID: "v-details", Value: "Restart LB instance"},
			domain.Variable{ID: "v-desc", Value: "Instance 2 is degraded"},
		)
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Restart LB instance" {
			t.Errorf("Subject = %q, want 'Restart LB instance'", req.Subject)
		}
		want := "<p><strong>Description</strong>: Instance 2 is degraded</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q", req.Description, want)
		}
	})

	t.Run("title takes precedence over short description and request details", func(t *testing.T) {
		mixedQuestions := []domain.CatalogItemVariable{
			{ID: "v-title", QuestionText: "Title", Order: 1},
			{ID: "v-short", QuestionText: "Short Description", Order: 2},
			{ID: "v-details", QuestionText: "Request Details", Order: 3},
		}
		cat := fakeSRCatalog{
			vars:  mixedQuestions,
			items: catalog.items,
		}
		s := &caseService{srCatalog: cat}
		req := srFormRequest(
			domain.Variable{ID: "v-title", Value: "Primary Title"},
			domain.Variable{ID: "v-short", Value: "Secondary Short"},
			domain.Variable{ID: "v-details", Value: "Tertiary Details"},
		)
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Primary Title" {
			t.Errorf("Subject = %q, want 'Primary Title'", req.Subject)
		}
		want := "<p><strong>Short Description</strong>: Secondary Short</p><p><strong>Request Details</strong>: Tertiary Details</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q", req.Description, want)
		}
	})

	t.Run("catalog item name found across catalogs even when catalog id differs", func(t *testing.T) {
		// In ServiceNow, the catalog id is sc_catalog sys_id (e.g. "sn-cat-service-catalog"),
		// while the caller passes category sys_id ("caller-cat-information-request").
		catWithDifferentID := fakeSRCatalog{
			requiredCatalogID: "sn-cat-service-catalog",
			vars:              []domain.CatalogItemVariable{{ID: "v-purpose", QuestionText: "Purpose", Order: 1}},
			items: []domain.Catalog{
				{
					ID:           "sn-cat-service-catalog",
					CatalogItems: []domain.CatalogItem{{ID: "item-1", Name: "Request Product Logs"}},
				},
			},
		}
		s := &caseService{srCatalog: catWithDifferentID}
		req := srFormRequest(domain.Variable{ID: "v-purpose", Value: "Debugging issue"})
		req.CatalogID = "caller-cat-information-request"
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Request Product Logs" {
			t.Errorf("Subject = %q, want 'Request Product Logs' from item name", req.Subject)
		}
		want := "<p><strong>Purpose</strong>: Debugging issue</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q", req.Description, want)
		}
	})

	t.Run("resolves sc_catalog id for variable lookup when caller sends category id", func(t *testing.T) {
		// Verify that variable questions (including title) are successfully fetched and used
		// even when the caller passes a category id that differs from the sc_catalog id.
		catWithDifferentID := fakeSRCatalog{
			requiredCatalogID: "sn-sc-catalog-uuid",
			vars: []domain.CatalogItemVariable{
				{ID: "v-title", QuestionText: "Title", Order: 1},
				{ID: "v-details", QuestionText: "Details", Order: 2},
			},
			items: []domain.Catalog{
				{
					ID:           "sn-sc-catalog-uuid",
					CatalogItems: []domain.CatalogItem{{ID: "item-1", Name: "Information Request"}},
				},
			},
		}
		s := &caseService{srCatalog: catWithDifferentID}
		req := srFormRequest(
			domain.Variable{ID: "v-title", Value: "Need staging DB access"},
			domain.Variable{ID: "v-details", Value: "For troubleshooting issue"},
		)
		req.CatalogID = "caller-category-uuid"
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "Need staging DB access" {
			t.Errorf("Subject = %q, want 'Need staging DB access'", req.Subject)
		}
		want := "<p><strong>Details</strong>: For troubleshooting issue</p>"
		if req.Description != want {
			t.Errorf("Description = %q, want %q", req.Description, want)
		}
	})

	t.Run("WithServiceRequestCatalog wires snCaseService", func(t *testing.T) {
		sn := &snCaseService{}
		mockCat := fakeSRCatalog{}
		wired := WithServiceRequestCatalog(sn, mockCat)
		if wired != sn {
			t.Errorf("expected same service instance returned")
		}
		if sn.srCatalog == nil {
			t.Errorf("expected sn.srCatalog to be wired, got nil")
		}
	})
}

