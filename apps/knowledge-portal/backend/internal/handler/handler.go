// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

// Package handler serves the public Knowledge Base API. It talks to
// entity-service with a plain HTTP client (no OAuth, no user token) and
// only ever exposes published articles -- drafts and in-review articles
// never leave the CSM Portal.
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type KBHandler struct {
	http         *http.Client
	baseURL      string
	pineconeHost string
	pineconeKey  string
}

func NewKBHandler() *KBHandler {
	return &KBHandler{
		http:         &http.Client{Timeout: 10 * time.Second},
		baseURL:      os.Getenv("ENTITY_SERVICE_BASE_URL"),
		pineconeHost: os.Getenv("PINECONE_HOST"),
		pineconeKey:  os.Getenv("PINECONE_KEY"),
	}
}

type searchRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId,omitempty"`
	SearchQuery     string `json:"searchQuery,omitempty"`
	SortBy          string `json:"sortBy,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Offset          int    `json:"offset,omitempty"`
}

// ListKnowledgeBases handles GET /public/knowledge-bases.
func (h *KBHandler) ListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, http.MethodGet, "/knowledge-bases", nil)
}

// SearchArticles handles POST /public/kb-articles/search. The state filter
// is forced to published server-side and never taken from the request.
func (h *KBHandler) SearchArticles(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	}

	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	upstream := map[string]any{
		"states":     []string{"published"},
		"pagination": map[string]int{"limit": limit, "offset": offset},
	}
	if req.KnowledgeBaseID != "" {
		upstream["knowledgeBaseId"] = req.KnowledgeBaseID
	}
	if req.SearchQuery != "" {
		upstream["searchQuery"] = req.SearchQuery
	}
	if req.SortBy != "" {
		upstream["sortBy"] = req.SortBy
	}

	body, err := json.Marshal(upstream)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to build the search request.")
		return
	}
	h.forward(w, r, http.MethodPost, "/kb-articles/search", body)
}

// GetArticle handles GET /public/kb-articles/{id}. An unpublished article
// is reported as 404, not 403, so a caller can't detect that a draft with
// that id exists.
func (h *KBHandler) GetArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Article id is required.")
		return
	}

	resp, err := h.do(r, http.MethodGet, "/kb-articles/"+id, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "get article failed", "id", id, "err", err)
		writeError(w, http.StatusBadGateway, "Failed to retrieve the article.")
		return
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the article.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, resp.StatusCode, payload)
		return
	}

	var article struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(payload, &article); err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the article.")
		return
	}
	if article.State != "published" {
		writeError(w, http.StatusNotFound, "Article not found.")
		return
	}

	writeJSON(w, http.StatusOK, payload)
}

// RecordView handles POST /public/kb-articles/{id}/view. Always 204, even
// on upstream failure: a lost view count is not worth erroring a page over.
func (h *KBHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Article id is required.")
		return
	}
	resp, err := h.do(r, http.MethodPost, "/kb-articles/"+id+"/views", []byte("{}"))
	if err != nil {
		slog.ErrorContext(r.Context(), "record view failed", "id", id, "err", err)
	} else {
		defer resp.Body.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KBHandler) forward(w http.ResponseWriter, r *http.Request, method, path string, body []byte) {
	resp, err := h.do(r, method, path, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity-service call failed", "path", path, "err", err)
		writeError(w, http.StatusBadGateway, "The knowledge base is temporarily unavailable.")
		return
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to read the response.")
		return
	}
	writeJSON(w, resp.StatusCode, payload)
}

func (h *KBHandler) do(r *http.Request, method, path string, body []byte) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, h.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return h.http.Do(req)
}

func writeJSON(w http.ResponseWriter, status int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// CORS allows the webapp (a different origin in dev) to call this API.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
