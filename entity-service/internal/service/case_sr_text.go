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
	"html"
	"log/slog"
	"sort"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// srCatalogReader is the part of CatalogService a service request's text is
// derived from.
type srCatalogReader interface {
	SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error)
	GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error)
}

// WithServiceRequestCatalog attaches the catalog CreateCase derives a service
// request's subject and description from (fillServiceRequestText), the same
// post-construction wiring as WithCSEngineerRole. A no-op if svc is not a
// *caseService or catalog is nil.
func WithServiceRequestCatalog(svc CaseService, catalog CatalogService) CaseService {
	if cs, ok := svc.(*caseService); ok && catalog != nil {
		cs.srCatalog = catalog
	}
	return svc
}

// fillServiceRequestText gives a service request raised from the catalog form
// a subject and a description when the caller sent none. The customer
// portal's form sends only the catalog item and the answers to its questions,
// so without this a natively created SR is stored with an empty subject and
// description: lists show no title, and csm-notification-service rejects its
// case.created and case.comment_added (both require the title), so no email
// goes out for it at all.
//
//   - Subject: the answer to the item's "Title" question -- the question the
//     customer portal's own form already treats as the title (isTitleField:
//     question text "Title", ignoring a leading "*" and case) -- else the
//     catalog item's name.
//   - Description: every other answered question, in the item's question
//     order, one "<question>: <answer>" paragraph each.
//
// A subject or description the caller did send is never replaced. Best
// effort: a catalog lookup failure is logged and the SR is created as sent.
func (s *caseService) fillServiceRequestText(ctx context.Context, req *domain.CreateCaseRequest) {
	if s.srCatalog == nil || req.Type != "service_request" {
		return
	}
	needSubject := strings.TrimSpace(req.Subject) == ""
	needDescription := strings.TrimSpace(req.Description) == ""
	if !needSubject && !needDescription {
		return
	}

	answers := make(map[string]string, len(req.Variables))
	for _, v := range req.Variables {
		if value := strings.TrimSpace(v.Value); value != "" {
			answers[v.ID] = value
		}
	}

	var questions []domain.CatalogItemVariable
	if vars, err := s.srCatalog.GetCatalogItemVariables(ctx, req.CatalogID, req.CatalogItemID); err != nil {
		slog.WarnContext(ctx, "create service request: catalog variables lookup failed; title/description not derived",
			"catalogId", req.CatalogID, "catalogItemId", req.CatalogItemID, "error", err)
	} else {
		questions = vars.Variables
	}
	sort.SliceStable(questions, func(i, j int) bool { return questions[i].Order < questions[j].Order })

	var title string
	var paragraphs []string
	for _, q := range questions {
		answer, ok := answers[q.ID]
		if !ok {
			continue
		}
		label := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(q.QuestionText), "*"))
		if title == "" && strings.EqualFold(label, "title") {
			title = answer
			continue
		}
		if label == "" {
			label = "Answer"
		}
		paragraphs = append(paragraphs,
			"<p><strong>"+html.EscapeString(label)+"</strong>: "+html.EscapeString(answer)+"</p>")
	}

	if needSubject {
		if title == "" {
			title = s.serviceRequestItemName(ctx, req)
		}
		req.Subject = title
	}
	if needDescription && len(paragraphs) > 0 {
		req.Description = strings.Join(paragraphs, "")
	}
}

// serviceRequestItemName is the catalog item's name, the subject of last
// resort for a service request whose form has no "Title" question. "" when it
// cannot be found.
func (s *caseService) serviceRequestItemName(ctx context.Context, req *domain.CreateCaseRequest) string {
	res, err := s.srCatalog.SearchCatalogs(ctx, domain.SearchCatalogsRequest{
		DeployedProductID: req.DeployedProductID,
		Pagination:        domain.Pagination{Limit: 100},
	})
	if err != nil {
		slog.WarnContext(ctx, "create service request: catalog lookup failed; no subject derived",
			"catalogItemId", req.CatalogItemID, "error", err)
		return ""
	}
	for _, c := range res.Catalogs {
		if c.ID != req.CatalogID {
			continue
		}
		for _, item := range c.CatalogItems {
			if item.ID == req.CatalogItemID {
				return item.Name
			}
		}
	}
	return ""
}
