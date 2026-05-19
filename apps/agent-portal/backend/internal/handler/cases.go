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

package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
)

// entityCaseClient abstracts the entity service operations used by CaseHandler,
// allowing the handler to be tested independently of the real HTTP client.
type entityCaseClient interface {
	CreateCase(ctx context.Context, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	GetCase(ctx context.Context, caseID string) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access.
type CaseHandler struct {
	entity entityCaseClient
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify the caller is an authenticated agent user (validate bearer JWT,
	// extract agent identity, and enforce role-based access control).

	// TODO: Decode the request body into a typed CaseCreatePayload and run
	// field-level validation (required fields, enum values, etc.).

	// TODO: Apply agent-portal business rules before forwarding to entity
	// (e.g. restrict allowed case types, enforce project membership).

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	result, err := h.entity.CreateCase(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "err", err)
		// TODO: Translate entity error codes to appropriate HTTP status codes.
		http.Error(w, "failed to create case", http.StatusInternalServerError)
		return
	}

	// TODO: Unmarshal result into a typed CaseCreateResponse and transform it
	// to the agent-portal response shape before writing.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(result)
}

// SearchCases handles POST /cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify the caller is an authenticated agent user (validate bearer JWT,
	// extract agent identity, and enforce role-based access control).

	// TODO: Decode the request body into a typed CaseSearchPayload and validate.

	// TODO: Inject agent-specific filters into the search payload before forwarding
	// (e.g. restrict results to the agent's assigned projects or queues).

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "err", err)
		// TODO: Translate entity error codes to appropriate HTTP status codes.
		http.Error(w, "failed to search cases", http.StatusInternalServerError)
		return
	}

	// TODO: Unmarshal result into a typed CaseSearchResponse and transform it
	// to the agent-portal response shape before writing.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(result)
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify the caller is an authenticated agent user (validate bearer JWT,
	// extract agent identity, and enforce role-based access control).

	caseID := r.PathValue("id")
	if caseID == "" {
		http.Error(w, "missing case id", http.StatusBadRequest)
		return
	}

	result, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "caseID", caseID, "err", err)
		// TODO: Translate entity error codes to appropriate HTTP status codes
		// (e.g. 404 when entity returns a not-found error).
		http.Error(w, "failed to get case", http.StatusInternalServerError)
		return
	}

	// TODO: Unmarshal result into a typed CaseResponse and transform it
	// to the agent-portal response shape before writing.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(result)
}
