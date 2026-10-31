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
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
)

const (
	// relatedMax is how many related articles are returned.
	relatedMax = 5
	// relatedCandidates is how many vector matches are fetched; one extra covers
	// the source article matching itself, the rest cover unpublished matches.
	relatedCandidates = 8
	// RelatedBudget bounds the whole related-articles lookup. It must fit inside
	// the server's write timeout (REST_WRITE_TIMEOUT) or the lookup is cut off
	// mid-flight and the client sees a dropped connection instead of a clean
	// error: config loading rejects a write timeout below this value.
	RelatedBudget = 20 * time.Second
)

// RelatedArticles handles GET /public/kb-articles/{id}/related.
//
// The source article must itself be published (404 otherwise). Semantic matches
// come back from the vector index as ids only; each is re-read from
// entity-service and kept only if it is published, and the title served is the
// one entity-service holds, never one stored in the index. When no semantic
// match survives (index disabled, down, or all unpublished) it falls back to
// other published articles in the same knowledge base.
func (h *KBHandler) RelatedArticles(w http.ResponseWriter, r *http.Request) {
	id, ok := normalizeID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidID)
		return
	}
	// Related articles chain several upstream calls; bound the whole lookup so it
	// finishes inside the server's write timeout and can still answer cleanly.
	ctx, cancel := context.WithTimeout(r.Context(), RelatedBudget)
	defer cancel()
	r = r.WithContext(ctx)

	src, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		writeUpstreamError(w, r, "related: get source article", err, true)
		return
	}
	if src.State != entity.StatePublished {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	active, err := h.activeKBs.get(r.Context())
	if err != nil {
		writeUpstreamError(w, r, "related: load active knowledge bases", err, false)
		return
	}
	if !active.has(src.KnowledgeBaseID) {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	if h.similarity != nil {
		if out := h.semanticRelated(r, id, active); len(out) > 0 {
			writeJSONValue(w, http.StatusOK, relatedArticlesDTO{Articles: out})
			return
		}
	}
	writeJSONValue(w, http.StatusOK, relatedArticlesDTO{Articles: h.sameKnowledgeBase(r, src, active)})
}

// semanticRelated returns verified, published semantic matches in index order.
// Any failure yields an empty result so the caller falls back.
func (h *KBHandler) semanticRelated(r *http.Request, id string, active kbSet) []relatedArticleDTO {
	matches, err := h.similarity.Similar(r.Context(), id, relatedCandidates)
	if err != nil {
		slog.ErrorContext(r.Context(), "related: similarity query failed", "err", err)
		return nil
	}

	// Candidate ids come from an external index: they must be UUIDs, are
	// normalised before use in an upstream URL, and the source itself and
	// duplicates are dropped.
	type candidate struct {
		id    string
		score float64
	}
	seen := map[string]bool{id: true}
	var cands []candidate
	for _, m := range matches {
		mid, ok := normalizeID(m.ID)
		if !ok || seen[mid] {
			continue
		}
		seen[mid] = true
		cands = append(cands, candidate{id: mid, score: m.Score})
	}

	// Published-state lookup, concurrent (bounded by relatedCandidates).
	found := make([]*entity.KBArticle, len(cands))
	var wg sync.WaitGroup
	for i, c := range cands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := h.entity.GetKBArticle(r.Context(), c.id)
			if err != nil {
				var apiErr *apierror.Error
				if !(errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusBadRequest)) {
					slog.WarnContext(r.Context(), "related: candidate lookup failed", "err", err)
				}
				return
			}
			if active.has(a.KnowledgeBaseID) && a.State == entity.StatePublished && strings.EqualFold(a.ID, c.id) {
				found[i] = &a
			}
		}()
	}
	wg.Wait()

	out := make([]relatedArticleDTO, 0, relatedMax)
	for i, a := range found {
		if a == nil || a.Title == "" {
			continue
		}
		out = append(out, relatedArticleDTO{ID: cands[i].id, Title: a.Title, Score: cands[i].score})
		if len(out) >= relatedMax {
			break
		}
	}
	return out
}

// sameKnowledgeBase returns up to relatedMax other published articles from the
// source article's knowledge base. Failure yields an empty list: related
// articles are decoration and must not fail the page.
func (h *KBHandler) sameKnowledgeBase(r *http.Request, src entity.KBArticle, active kbSet) []relatedArticleDTO {
	out := []relatedArticleDTO{}
	kbID, ok := normalizeID(src.KnowledgeBaseID)
	if !ok {
		return out
	}
	resp, err := h.entity.SearchKBArticles(r.Context(), entity.SearchRequest{
		KnowledgeBaseID: kbID,
		States:          []string{entity.StatePublished},
		Pagination:      entity.Pagination{Limit: relatedMax + 1},
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "related: same-knowledge-base search failed", "err", err)
		return out
	}
	for _, a := range resp.Articles {
		if !active.has(a.KnowledgeBaseID) || strings.EqualFold(a.ID, src.ID) || a.State != entity.StatePublished || a.Title == "" {
			continue
		}
		out = append(out, relatedArticleDTO{ID: a.ID, Title: a.Title})
		if len(out) >= relatedMax {
			break
		}
	}
	return out
}
