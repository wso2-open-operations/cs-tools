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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityAnnouncementClient abstracts the entity service operation used by AnnouncementHandler.
type entityAnnouncementClient interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
}

// AnnouncementHandler handles HTTP requests scoped to announcement-audience
// resolution, delegating to the entity service for data access.
type AnnouncementHandler struct {
	entity              entityAnnouncementClient
	excludedProjectKeys []string
}

// NewAnnouncementHandler creates an AnnouncementHandler backed by the given
// entity client and the mandatory excluded-project-key list resolved once at
// startup from CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS (see main.go's
// loadAnnouncementExcludedProjectKeys).
func NewAnnouncementHandler(entity entityAnnouncementClient, excludedProjectKeys []string) *AnnouncementHandler {
	return &AnnouncementHandler{entity: entity, excludedProjectKeys: excludedProjectKeys}
}

// excludedProjectKeysResponse is GetExcludedProjectKeys' response shape.
type excludedProjectKeysResponse struct {
	// ExcludedProjectKeys is never null, even when nothing is configured —
	// callers can render it directly as a list with no extra nil-check.
	ExcludedProjectKeys []string `json:"excludedProjectKeys"`
}

// GetExcludedProjectKeys handles GET /announcements/audience/excluded-project-keys.
//
// Read-only: this list is deliberately not editable through the portal for
// now — configuration-only, via CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS
// (change it there and redeploy). This endpoint exists purely so
// AudienceScopeControls can display which specific projects are excluded
// (mirroring the real ServiceNow flow's own condition builder, where the
// excluded project keys are plainly visible), rather than leaving the
// mandatory exclusion an opaque "a configured list" with no way to see what
// it actually contains.
func (h *AnnouncementHandler) GetExcludedProjectKeys(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	keys := h.excludedProjectKeys
	if keys == nil {
		keys = []string{}
	}
	writeJSONValue(w, http.StatusOK, excludedProjectKeysResponse{ExcludedProjectKeys: keys})
}

// injectExcludeProjectKeys merges the configured mandatory excluded-project-
// key list into a JSON request body as excludeProjectKeys, unconditionally
// overwriting whatever the caller supplied for that field (if anything).
// This is a mandatory policy, not a caller-toggleable filter — mirrors the
// real ServiceNow flow this replaces, whose own "Create announcement for
// customers" flow hardcodes an equivalent Project Key exclusion with no way
// for whoever triggers it to opt out.
func injectExcludeProjectKeys(body []byte, excludedProjectKeys []string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	if excludedProjectKeys == nil {
		excludedProjectKeys = []string{}
	}
	keys, err := json.Marshal(excludedProjectKeys)
	if err != nil {
		return nil, err
	}
	m["excludeProjectKeys"] = keys
	return json.Marshal(m)
}

// SearchCustomerAnnouncementAudience handles POST /announcements/audience/search.
//
// Resolves the "All customer projects" audience for the "Create announcement
// for customers" flow (AudienceScopeControls' "all" scope): forwards to the
// entity service's general project search, but unconditionally injects the
// configured mandatory excluded-project-key list first. The caller-supplied
// excludeClosureStates/excludeSubscriptionTypes filters this endpoint also
// accepts stay exactly as toggleable as they already are on /projects/search
// (see AudienceScopeControls' own checkboxes) — only the project-key
// denylist is mandatory and invisible to the caller.
//
// This is a dedicated endpoint, not a change to /projects/search itself,
// because that endpoint is a general-purpose passthrough used by many
// unrelated features (project pickers, directory search, typeahead) —
// baking an announcement-specific exclusion into it would silently affect
// every other caller platform-wide. Only the announcement audience-
// resolution flow calls this endpoint.
func (h *AnnouncementHandler) SearchCustomerAnnouncementAudience(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
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

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	body, err = injectExcludeProjectKeys(body, h.excludedProjectKeys)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchProjects(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjects failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to resolve the announcement audience.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
