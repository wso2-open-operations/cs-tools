// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type pineconeQueryRequest struct {
	ID              string `json:"id"`
	TopK            int    `json:"topK"`
	IncludeMetadata bool   `json:"includeMetadata"`
	Namespace       string `json:"namespace"`
}

type pineconeMatch struct {
	ID       string            `json:"id"`
	Score    float64           `json:"score"`
	Metadata map[string]string `json:"metadata"`
}

type pineconeQueryResponse struct {
	Matches []pineconeMatch `json:"matches"`
}

type relatedArticle struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// RelatedArticles handles GET /public/kb-articles/{id}/related. Queries
// Pinecone by the article's vector id for semantically similar articles.
// Falls back to same-knowledge-base articles when Pinecone is unavailable,
// unconfigured, or returns nothing.
func (h *KBHandler) RelatedArticles(w http.ResponseWriter, r *http.Request) {
	articleID := r.PathValue("id")
	if articleID == "" {
		writeError(w, http.StatusBadRequest, "Article id is required.")
		return
	}

	var article struct {
		KnowledgeBaseID string `json:"knowledgeBaseId"`
	}
	if entResp, err := h.do(r, http.MethodGet, "/kb-articles/"+articleID, nil); err == nil {
		defer entResp.Body.Close()
		_ = json.NewDecoder(entResp.Body).Decode(&article)
	}

	if h.pineconeHost == "" || h.pineconeKey == "" {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}

	qBody, err := json.Marshal(pineconeQueryRequest{ID: articleID, TopK: 6, IncludeMetadata: true, Namespace: ""})
	if err != nil {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}

	host := h.pineconeHost
	if host[len(host)-1] == '/' {
		host = host[:len(host)-1]
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, host+"/query", bytes.NewReader(qBody))
	if err != nil {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", h.pineconeKey)

	resp, err := h.http.Do(req)
	if err != nil {
		slog.ErrorContext(r.Context(), "pinecone query failed", "id", articleID, "err", err)
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}

	var pResp pineconeQueryResponse
	if err := json.Unmarshal(payload, &pResp); err != nil {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}

	result := make([]relatedArticle, 0, 5)
	for _, m := range pResp.Matches {
		if m.ID == articleID || m.Metadata["title"] == "" {
			continue
		}
		result = append(result, relatedArticle{ID: m.ID, Title: m.Metadata["title"], Score: m.Score})
		if len(result) >= 5 {
			break
		}
	}
	if len(result) == 0 {
		h.fallbackSameKB(w, r, articleID, article.KnowledgeBaseID)
		return
	}

	out, _ := json.Marshal(map[string]any{"articles": result})
	writeJSON(w, http.StatusOK, out)
}

// fallbackSameKB returns up to 5 published articles from the same knowledge
// base, excluding the current one.
func (h *KBHandler) fallbackSameKB(w http.ResponseWriter, r *http.Request, excludeID, knowledgeBaseID string) {
	if knowledgeBaseID == "" {
		writeJSON(w, http.StatusOK, []byte(`{"articles":[]}`))
		return
	}
	searchBody, _ := json.Marshal(map[string]any{
		"knowledgeBaseId": knowledgeBaseID,
		"states":          []string{"published"},
		"pagination":      map[string]int{"limit": 6, "offset": 0},
	})
	resp, err := h.do(r, http.MethodPost, "/kb-articles/search", searchBody)
	if err != nil {
		writeJSON(w, http.StatusOK, []byte(`{"articles":[]}`))
		return
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, []byte(`{"articles":[]}`))
		return
	}

	var searchResp struct {
		Articles []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(payload, &searchResp); err != nil {
		writeJSON(w, http.StatusOK, []byte(`{"articles":[]}`))
		return
	}

	result := make([]relatedArticle, 0, 5)
	for _, a := range searchResp.Articles {
		if a.ID == excludeID {
			continue
		}
		result = append(result, relatedArticle{ID: a.ID, Title: a.Title, Score: 0})
		if len(result) >= 5 {
			break
		}
	}
	out, _ := json.Marshal(map[string]any{"articles": result})
	writeJSON(w, http.StatusOK, out)
}
