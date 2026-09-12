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

// entityVulnerabilityClient abstracts the entity service product-vulnerability
// operations used by VulnerabilityHandler.
type entityVulnerabilityClient interface {
	SyncProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error)
}

// VulnerabilityHandler handles HTTP requests for product-vulnerability operations,
// delegating to the entity service for data access. See AccountHandler's doc
// comment: there is no end-user identity checked here — Choreo's API Manager
// gateway is the trust boundary for this service's M2M/third-party consumers.
type VulnerabilityHandler struct {
	entity entityVulnerabilityClient
}

// NewVulnerabilityHandler creates a VulnerabilityHandler backed by the given entity client.
func NewVulnerabilityHandler(entity entityVulnerabilityClient) *VulnerabilityHandler {
	return &VulnerabilityHandler{entity: entity}
}

// SyncProductVulnerabilities handles POST /vulnerabilities/sync. The request body is
// forwarded to the entity service verbatim. This is a full-replace sync — the caller
// must submit the complete current set of product-vulnerability records on every
// call, not an incremental delta; the entity service's downstream ServiceNow-backed
// operation deletes any existing record not present in the submitted set. Unlike
// UpdateProject, this entity-service operation accepts pure M2M calls with no
// forwarded end-user token, so this call is expected to actually succeed.
func (h *VulnerabilityHandler) SyncProductVulnerabilities(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.SyncProductVulnerabilities(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SyncProductVulnerabilities failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to sync product vulnerabilities.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
