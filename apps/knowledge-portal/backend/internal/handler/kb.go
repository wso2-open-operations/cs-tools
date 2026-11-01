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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"context"
	"net/http"
	"sort"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/pinecone"
)

// Entity is the slice of the entity-service client the handlers use. It has no
// write method: this API cannot modify anything upstream.
type Entity interface {
	ListKnowledgeBases(ctx context.Context) ([]entity.KnowledgeBase, error)
	SearchKBArticles(ctx context.Context, req entity.SearchRequest) (entity.SearchResponse, error)
	GetKBArticle(ctx context.Context, id string) (entity.KBArticle, error)
}

// Similarity finds vectors similar to an article. nil disables semantic
// related-article search (the same-knowledge-base fallback still works).
type Similarity interface {
	Similar(ctx context.Context, id string, topK int) ([]pinecone.Match, error)
}

// KBHandler serves the public knowledge base routes.
type KBHandler struct {
	entity     Entity
	similarity Similarity
	activeKBs  *activeKBCache
}

// NewKBHandler constructs a KBHandler. similarity may be nil.
func NewKBHandler(e Entity, s Similarity) *KBHandler {
	return &KBHandler{entity: e, similarity: s, activeKBs: newActiveKBCache(e.ListKnowledgeBases)}
}

// ListKnowledgeBases handles GET /public/knowledge-bases. Only active knowledge
// bases are returned, as id and title only. The list comes from the same
// short-lived cache that gates article access, so it can lag a deactivation by
// up to activeKBTTL.
func (h *KBHandler) ListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	set, err := h.activeKBs.get(r.Context())
	if err != nil {
		writeUpstreamError(w, r, "list knowledge bases", err, false)
		return
	}
	out := listKnowledgeBasesDTO{KnowledgeBases: make([]knowledgeBaseDTO, 0, len(set))}
	for id, title := range set {
		out.KnowledgeBases = append(out.KnowledgeBases, knowledgeBaseDTO{ID: id, Title: title})
	}
	sort.Slice(out.KnowledgeBases, func(i, j int) bool {
		a, b := out.KnowledgeBases[i], out.KnowledgeBases[j]
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.ID < b.ID
	})
	writeJSONValue(w, http.StatusOK, out)
}

// SearchArticles handles POST /public/kb-articles/search. The state filter is
// forced to published here and is not part of the accepted request. Results are
// filtered to published and to active knowledge bases on the way out, so an
// upstream that ignored the filter still cannot leak a draft.
//
// Pagination tradeoff: entity-service can filter by one knowledge base or by
// none, not by "all active ones". With a knowledgeBaseId the request is checked
// against the active set (404 if inactive or unknown) and pagination is exact.
// Without one, articles of inactive knowledge bases are removed from the
// upstream page after it is fetched, so a page can hold fewer than limit items
// (even none while hasMore is true) and total is an upper bound, exact only when
// no knowledge base is inactive. offset is the upstream offset, so a client that
// pages by hasMore and offset+limit still visits every article.
func (h *KBHandler) SearchArticles(w http.ResponseWriter, r *http.Request) {
	req, status, msg := decodeSearchRequest(w, r)
	if status != 0 {
		writeError(w, status, msg)
		return
	}

	active, err := h.activeKBs.get(r.Context())
	if err != nil {
		writeUpstreamError(w, r, "search articles: load active knowledge bases", err, false)
		return
	}
	if req.KnowledgeBaseID != "" {
		if !active.has(req.KnowledgeBaseID) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
	}

	resp, err := h.entity.SearchKBArticles(r.Context(), entity.SearchRequest{
		KnowledgeBaseID: req.KnowledgeBaseID,
		States:          []string{entity.StatePublished},
		SearchQuery:     req.SearchQuery,
		Pagination:      entity.Pagination{Limit: req.Limit, Offset: req.Offset},
	})
	if err != nil {
		writeUpstreamError(w, r, "search articles", err, false)
		return
	}

	out := searchResponseDTO{
		Articles: make([]articleDTO, 0, len(resp.Articles)),
		Total:    resp.Total,
		Limit:    resp.Limit,
		Offset:   resp.Offset,
		HasMore:  resp.HasMore,
	}
	for _, a := range resp.Articles {
		if !active.has(a.KnowledgeBaseID) || a.State != entity.StatePublished {
			continue
		}
		out.Articles = append(out.Articles, toArticleDTO(a))
	}
	writeJSONValue(w, http.StatusOK, out)
}

// GetArticle handles GET /public/kb-articles/{id}. An unpublished article is
// reported as 404, not 403, so a caller cannot detect that a draft with that id
// exists. The same applies to an article whose knowledge base is inactive.
func (h *KBHandler) GetArticle(w http.ResponseWriter, r *http.Request) {
	id, ok := normalizeID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidID)
		return
	}
	a, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		writeUpstreamError(w, r, "get article", err, true)
		return
	}
	if a.State != entity.StatePublished {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	active, err := h.activeKBs.get(r.Context())
	if err != nil {
		writeUpstreamError(w, r, "get article: load active knowledge bases", err, false)
		return
	}
	if !active.has(a.KnowledgeBaseID) {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	writeJSONValue(w, http.StatusOK, toArticleDTO(a))
}
