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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityTeamSearchClient is the subset of internal/entity.CustomerEntityClient
// the team search needs.
type entityTeamSearchClient interface {
	SearchTeams(ctx context.Context, body []byte) ([]byte, error)
}

// ReferenceHandler serves the role catalogue and the team list, both of which
// back the user-directory filters and the dashboard team picker.
//
// The role catalogue is deployment configuration this service resolves once at
// startup (see package directory), so it is a memory read. The team list comes
// from the entity service's `team` table when an entity client is wired (see
// WithEntityClient), with the configured registry supplying each matching
// team's key, family and backing group ids; without one it is the registry alone.
type ReferenceHandler struct {
	dir    *directory.Directory
	entity entityTeamSearchClient
}

// NewReferenceHandler creates a ReferenceHandler backed by the startup-resolved
// directory.
func NewReferenceHandler(dir *directory.Directory) *ReferenceHandler {
	return &ReferenceHandler{dir: dir}
}

// WithEntityClient makes POST /teams/search read the `team` table through the
// entity service instead of serving the configured registry alone.
func (h *ReferenceHandler) WithEntityClient(c entityTeamSearchClient) *ReferenceHandler {
	h.entity = c
	return h
}

// SearchRoles handles POST /roles/search.
func (h *ReferenceHandler) SearchRoles(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeSearch(w, r)
	if !ok {
		return
	}
	writeJSONValue(w, http.StatusOK, h.dir.SearchRoles(req))
}

// SearchTeams handles POST /teams/search.
func (h *ReferenceHandler) SearchTeams(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeSearch(w, r)
	if !ok {
		return
	}

	// Family exists only in the configured registry, not on the `team` table, so
	// a family-scoped request (the discipline pickers) is answered from it, as
	// is every request when no entity client is wired.
	if h.entity == nil || strings.TrimSpace(req.Filters.Family) != "" {
		writeJSONValue(w, http.StatusOK, h.dir.SearchTeams(req))
		return
	}

	page := directory.ClampPagination(req.Pagination)
	upstreamBody, err := json.Marshal(map[string]any{
		"filters":    map[string]string{"searchQuery": strings.TrimSpace(req.Filters.SearchQuery)},
		"pagination": page,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	raw, err := h.entity.SearchTeams(r.Context(), upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTeams failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search teams.")
		return
	}

	var upstream struct {
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &upstream); err != nil {
		slog.ErrorContext(r.Context(), "entity SearchTeams returned an unreadable body", "err", err)
		writeError(w, http.StatusBadGateway, "Failed to search teams.")
		return
	}

	// A team the registry knows keeps its registry key as id, plus its family and
	// group ids, because every consumer of a team id resolves it by key. One it
	// does not know is listed by its `team` id and name alone.
	teams := make([]directory.TeamResult, 0, len(upstream.Teams))
	for _, t := range upstream.Teams {
		if known, ok := h.dir.TeamResultByGroupName(t.Name); ok {
			teams = append(teams, known)
			continue
		}
		teams = append(teams, directory.TeamResult{ID: t.ID, Name: t.Name})
	}
	writeJSONValue(w, http.StatusOK, directory.SearchTeamsResponse{
		Teams: teams, Total: upstream.Total, Offset: page.Offset, Limit: page.Limit,
	})
}

// decodeSearch carries the shared auth / read-body / decode sequence for both
// catalogue endpoints. It writes the error response itself and reports false
// when the caller should stop.
func (h *ReferenceHandler) decodeSearch(w http.ResponseWriter, r *http.Request) (directory.SearchRequest, bool) {
	var req directory.SearchRequest

	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return req, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return req, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return req, false
	}

	// Both endpoints accept an absent body, meaning "no filters, default page".
	// Only a non-empty body has to be valid JSON.
	if len(body) == 0 {
		return req, true
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return directory.SearchRequest{}, false
	}
	return req, true
}
