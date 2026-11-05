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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityFeatureOverrideClient is the entity-service calls Feature visibility needs.
type entityFeatureOverrideClient interface {
	ListFeatureOverrides(ctx context.Context) ([]byte, error)
	PutFeatureOverride(ctx context.Context, body []byte) ([]byte, error)
}

// FeatureOverrideHandler proxies the shared per-page feature state. Reading is
// PermView (every signed-in staff member's shell needs it). Writing is
// PermAdmin, checked by the route; entity-service only accepts the write from
// this backend and records who made it.
type FeatureOverrideHandler struct {
	entity entityFeatureOverrideClient
}

// NewFeatureOverrideHandler creates a FeatureOverrideHandler.
func NewFeatureOverrideHandler(entity entityFeatureOverrideClient) *FeatureOverrideHandler {
	return &FeatureOverrideHandler{entity: entity}
}

// featureNavNodeID is the shape of a portal nav-node id (operations.incidents,
// kb-articles.to-review). Entity-service enforces the same pattern.
var featureNavNodeID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

const maxFeatureNavNodeIDLen = 200

type featureOverrideSave struct {
	NavNodeID string `json:"navNodeId"`
	State     string `json:"state"`
	MarkSaved bool   `json:"markSaved"`
}

// ListFeatureOverrides handles GET /feature-overrides.
func (h *FeatureOverrideHandler) ListFeatureOverrides(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, "ListFeatureOverrides", "Failed to load feature visibility.", h.entity.ListFeatureOverrides)
}

// PutFeatureOverride handles PUT /feature-overrides.
func (h *FeatureOverrideHandler) PutFeatureOverride(w http.ResponseWriter, r *http.Request) {
	body, ok := readFeatureOverrideSave(w, r)
	if !ok {
		return
	}
	h.forward(w, r, "PutFeatureOverride", "Failed to save feature visibility.",
		func(ctx context.Context) ([]byte, error) { return h.entity.PutFeatureOverride(ctx, body) })
}

func readFeatureOverrideSave(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req featureOverrideSave
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	// A second decode must find nothing left, including a stray ] or }.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	req.NavNodeID = strings.TrimSpace(req.NavNodeID)
	if req.NavNodeID == "" || len(req.NavNodeID) > maxFeatureNavNodeIDLen || !featureNavNodeID.MatchString(req.NavNodeID) {
		writeError(w, http.StatusBadRequest, "A page id such as operations.incidents is required.")
		return nil, false
	}
	req.State = strings.TrimSpace(req.State)
	if req.State != "enabled" && req.State != "preview" {
		writeError(w, http.StatusBadRequest, "State must be enabled or preview.")
		return nil, false
	}
	return mustJSON(req), true
}

func (h *FeatureOverrideHandler) forward(w http.ResponseWriter, r *http.Request, op, fallback string, call func(context.Context) ([]byte, error)) {
	w.Header().Set("Cache-Control", "no-store")
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := call(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "err", err)
		mapUpstreamErrorGeneric(w, err, fallback)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
