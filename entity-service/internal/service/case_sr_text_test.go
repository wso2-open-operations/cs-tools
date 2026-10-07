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
	vars    []domain.CatalogItemVariable
	varsErr error
	items   []domain.Catalog
}

func (f fakeSRCatalog) SearchCatalogs(context.Context, domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	return domain.SearchCatalogsResponse{Catalogs: f.items}, nil
}
func (f fakeSRCatalog) GetCatalogItemVariables(context.Context, string, string) (domain.GetCatalogItemVariablesResponse, error) {
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

	t.Run("not wired is a no-op", func(t *testing.T) {
		s := &caseService{}
		req := srFormRequest(domain.Variable{ID: "v-title", Value: "t"})
		s.fillServiceRequestText(context.Background(), &req)
		if req.Subject != "" {
			t.Errorf("Subject = %q, want untouched", req.Subject)
		}
	})
}
