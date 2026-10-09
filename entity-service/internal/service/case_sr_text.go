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
// post-construction wiring as WithCSEngineerRole. A no-op if svc is neither a
// *caseService nor a *snCaseService, or if catalog is nil.
func WithServiceRequestCatalog(svc CaseService, catalog CatalogService) CaseService {
	if cs, ok := svc.(*caseService); ok && catalog != nil {
		cs.srCatalog = catalog
	}
	if sn, ok := svc.(*snCaseService); ok && catalog != nil {
		sn.srCatalog = catalog
	}
	return svc
}

// titleQuestionRank returns a priority rank (lower is better, >0 means it is a title question)
// for question labels that serve as the case title/topic.
func titleQuestionRank(label string) int {
	clean := strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(label), "*")))
	switch clean {
	case "title":
		return 1
	case "short description":
		return 2
	case "request details":
		return 3
	case "summary", "subject":
		return 4
	default:
		return 0
	}
}

// fillServiceRequestText gives a service request raised from the catalog form
// a subject and a description when the caller sent none. The customer
// portal's form sends only the catalog item and the answers to its questions,
// so without this a natively created SR is stored with an empty subject and
// description: lists show no title, and csm-notification-service rejects its
// case.created and case.comment_added (both require the title), so no email
// goes out for it at all.
//
//   - Subject: the answer to the item's "Title" question (or, if absent,
//     "Short Description", "Request Details", "Summary", or "Subject") -- else
//     the catalog item's name.
//   - Description: every other answered question, in the item's question
//     order, one "<question>: <answer>" paragraph each.
//
// A subject or description the caller did send is never replaced. Best
// effort: a catalog lookup failure is logged and the SR is created as sent.
func fillServiceRequestText(ctx context.Context, srCatalog srCatalogReader, req *domain.CreateCaseRequest) {
	if srCatalog == nil || req.Type != "service_request" {
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

	catalogID := req.CatalogID
	resolvedCatalogID, itemName := findCatalogItem(ctx, srCatalog, req)
	if resolvedCatalogID != "" {
		catalogID = resolvedCatalogID
	}

	var questions []domain.CatalogItemVariable
	if vars, err := srCatalog.GetCatalogItemVariables(ctx, catalogID, req.CatalogItemID); err != nil {
		if resolvedCatalogID != "" && req.CatalogID != "" && req.CatalogID != resolvedCatalogID {
			if fallbackVars, fallbackErr := srCatalog.GetCatalogItemVariables(ctx, req.CatalogID, req.CatalogItemID); fallbackErr == nil {
				questions = fallbackVars.Variables
			} else {
				slog.WarnContext(ctx, "create service request: catalog variables lookup failed; title/description not derived",
					"catalogId", catalogID, "catalogItemId", req.CatalogItemID, "error", err)
			}
		} else {
			slog.WarnContext(ctx, "create service request: catalog variables lookup failed; title/description not derived",
				"catalogId", catalogID, "catalogItemId", req.CatalogItemID, "error", err)
		}
	} else {
		questions = vars.Variables
	}
	sort.SliceStable(questions, func(i, j int) bool { return questions[i].Order < questions[j].Order })

	var bestTitleRank int
	var titleQuestionID string
	var title string
	for _, q := range questions {
		answer, ok := answers[q.ID]
		if !ok || answer == "" {
			continue
		}
		rank := titleQuestionRank(q.QuestionText)
		if rank > 0 && (bestTitleRank == 0 || rank < bestTitleRank) {
			bestTitleRank = rank
			titleQuestionID = q.ID
			title = answer
		}
	}

	var paragraphs []string
	for _, q := range questions {
		answer, ok := answers[q.ID]
		if !ok {
			continue
		}
		if q.ID == titleQuestionID {
			continue
		}
		label := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(q.QuestionText), "*"))
		if label == "" {
			label = "Answer"
		}
		paragraphs = append(paragraphs,
			"<p><strong>"+html.EscapeString(label)+"</strong>: "+html.EscapeString(answer)+"</p>")
	}

	if needSubject {
		if title == "" {
			title = itemName
		}
		req.Subject = title
	}
	if needDescription && len(paragraphs) > 0 {
		req.Description = strings.Join(paragraphs, "")
	}
}

func (s *caseService) fillServiceRequestText(ctx context.Context, req *domain.CreateCaseRequest) {
	fillServiceRequestText(ctx, s.srCatalog, req)
}

// findCatalogItem searches catalogs for the request's deployed product to locate
// the catalog item. It returns the enclosing catalog's ID (for ServiceNow, the
// sc_catalog sys_id rather than the category ID in req.CatalogID) and the item's
// name. Both return values are "" if not found.
func findCatalogItem(ctx context.Context, srCatalog srCatalogReader, req *domain.CreateCaseRequest) (catalogID string, itemName string) {
	if req.DeployedProductID == "" || req.CatalogItemID == "" {
		return "", ""
	}
	limit := 50
	offset := 0
	for {
		res, err := srCatalog.SearchCatalogs(ctx, domain.SearchCatalogsRequest{
			DeployedProductID: req.DeployedProductID,
			Pagination:        domain.Pagination{Limit: limit, Offset: offset},
		})
		if err != nil {
			slog.WarnContext(ctx, "create service request: catalog lookup failed",
				"catalogItemId", req.CatalogItemID, "error", err)
			return "", ""
		}
		for _, c := range res.Catalogs {
			for _, item := range c.CatalogItems {
				if item.ID == req.CatalogItemID {
					return c.ID, item.Name
				}
			}
		}
		offset += len(res.Catalogs)
		if offset >= res.Total || len(res.Catalogs) == 0 {
			break
		}
	}
	return "", ""
}
