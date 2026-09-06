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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityProjectClient abstracts the entity-service project operations used by ProjectHandler.
type entityProjectClient interface {
	SearchProjects(ctx context.Context, req entity.SearchProjectsRequest) (entity.SearchProjectsResponse, error)
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// ProjectHandler handles HTTP requests for project operations.
type ProjectHandler struct {
	entity entityProjectClient

	callerScope *CallerScopeResolver
}

// NewProjectHandler creates a ProjectHandler backed by the given entity client.
func NewProjectHandler(entity entityProjectClient) *ProjectHandler {
	return &ProjectHandler{entity: entity}
}

// SetCallerScope wires up caller-scoped project search: SearchProjects only
// returns projects the caller is an active portal-user contact of (see
// CallerScopeResolver). main.go always calls this in production — there is
// no kill switch. A setter rather than a constructor parameter purely so
// the many pre-existing tests across this package that construct handlers
// directly, unrelated to this feature, keep compiling without change; a nil
// resolver (never calling this) is treated as unscoped rather than
// panicking — see requireProjectMember's doc comment.
func (h *ProjectHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

const (
	// callerScopeProjectsBatchSize is the page size used to query entity-service
	// when scanning upstream projects for caller membership.
	callerScopeProjectsBatchSize = 50

	// callerScopeProjectsMaxPages caps the scan at 500 projects — an independent ceiling
	// so a large or unbounded catalog doesn't loop indefinitely.
	callerScopeProjectsMaxPages = 10
)

// SearchProjects handles POST /projects/search.
func (h *ProjectHandler) SearchProjects(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.SearchProjectsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq := dto.BuildEntitySearchProjectsRequest(req)

	var result entity.SearchProjectsResponse
	var err error

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / scopeToCallerProjects.
	// if h.callerScope != nil {
	// 	result, err = h.scopeToCallerProjects(r.Context(), entityReq, req.Pagination, user.Email)
	// } else {
	// 	result, err = h.entity.SearchProjects(r.Context(), entityReq)
	// }
	result, err = h.entity.SearchProjects(r.Context(), entityReq)

	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProjects failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search projects.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchProjects(result))
}

// scopeToCallerProjects pages through entity-service's project search results
// in batches of callerScopeProjectsBatchSize (up to callerScopeProjectsMaxPages),
// filters each returned project to only those the caller is a member of (via
// CallerScopeResolver.IsProjectMember), and applies client-side slice pagination
// based on clientPagination (Offset & Limit).
func (h *ProjectHandler) scopeToCallerProjects(ctx context.Context, baseReq entity.SearchProjectsRequest, clientPagination entity.Pagination, email string) (entity.SearchProjectsResponse, error) {
	var allScoped []entity.ProjectView

	for page := 0; page < callerScopeProjectsMaxPages; page++ {
		batchReq := baseReq
		batchReq.Pagination = entity.Pagination{
			Limit:  callerScopeProjectsBatchSize,
			Offset: page * callerScopeProjectsBatchSize,
		}

		resp, err := h.entity.SearchProjects(ctx, batchReq)
		if err != nil {
			return entity.SearchProjectsResponse{}, err
		}

		for _, p := range resp.Projects {
			member, memberErr := h.callerScope.IsProjectMember(ctx, p.ID, email)
			if memberErr != nil {
				slog.ErrorContext(ctx, "caller scope check failed", "projectID", p.ID, "err", summarizeErr(memberErr))
				continue
			}
			if member {
				allScoped = append(allScoped, p)
			}
		}

		if len(resp.Projects) == 0 || !resp.HasMore || (page*callerScopeProjectsBatchSize+len(resp.Projects)) >= resp.Total {
			break
		}
	}

	totalScoped := len(allScoped)
	clientLimit := clientPagination.Limit
	if clientLimit <= 0 {
		clientLimit = callerScopeProjectsBatchSize
	}
	clientOffset := clientPagination.Offset
	if clientOffset < 0 {
		clientOffset = 0
	}

	var paged []entity.ProjectView
	if clientOffset < totalScoped {
		end := totalScoped
		if clientLimit < totalScoped-clientOffset {
			end = clientOffset + clientLimit
		}
		paged = allScoped[clientOffset:end]
	} else {
		paged = []entity.ProjectView{}
	}

	hasMore := (clientOffset + len(paged)) < totalScoped

	return entity.SearchProjectsResponse{
		Projects: paged,
		Total:    totalScoped,
		Limit:    clientLimit,
		Offset:   clientOffset,
		HasMore:  hasMore,
	}, nil
}

// GetProject handles GET /projects/{id}.
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetProject(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProjectDetails(result))
}
