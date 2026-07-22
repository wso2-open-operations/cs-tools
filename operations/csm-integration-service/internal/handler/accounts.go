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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// entityAccountClient abstracts the entity service account operations used by AccountHandler.
type entityAccountClient interface {
	GetAccount(ctx context.Context, id string) ([]byte, error)
	SearchAccounts(ctx context.Context, body []byte) ([]byte, error)
	SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error)
}

// AccountHandler handles HTTP requests for account operations, delegating to the
// entity service for data access. There is no end-user identity to check here —
// this service has no inbound auth of its own (see the package doc note on
// response.go); the trust boundary is Choreo's API Manager gateway, which fronts
// this service for its M2M/third-party consumers.
type AccountHandler struct {
	entity entityAccountClient
}

// NewAccountHandler creates an AccountHandler backed by the given entity client.
func NewAccountHandler(entity entityAccountClient) *AccountHandler {
	return &AccountHandler{entity: entity}
}

// GetAccount handles GET /accounts/{id}.
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetAccount(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAccount failed", "accountID", id, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve account.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchAccounts handles POST /accounts/search.
func (h *AccountHandler) SearchAccounts(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchAccounts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAccounts failed", "err", err)
		mapUpstreamError(w, err, "Failed to search accounts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchAccountContacts handles POST /accounts/{id}/contacts/search.
// The endpoint is path-scoped, so the request body is capped and forwarded to the
// entity service as-is (no fields are injected) and the response is returned verbatim.
func (h *AccountHandler) SearchAccountContacts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchAccountContacts(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAccountContacts failed", "accountID", id, "err", err)
		mapUpstreamError(w, err, "Failed to search account contacts.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
