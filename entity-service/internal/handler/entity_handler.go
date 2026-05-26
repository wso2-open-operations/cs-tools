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

// Package handler wires HTTP request parsing to the service layer and writes
// JSON responses. Handlers do not contain business logic.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxRequestBodySize caps the JSON body at 1 MiB to prevent memory exhaustion
// from oversized payloads.
const maxRequestBodySize = int64(1 << 20)

// UserHandler handles HTTP requests for the user resource.
type UserHandler struct {
	svc service.UserService
}

// NewUserHandler constructs a UserHandler with the given service.
func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// SearchUsers handles POST /users/search.
// It decodes a SearchUsersRequest from the request body and writes a paginated
// SearchUsersResponse as JSON. Validation errors produce 400; all other errors
// produce 500.
func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchUsersRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.SearchUsers(r.Context(), req)
	if err != nil {
		var ve *apierror.ValidationError
		switch {
		case errors.As(err, &ve):
			apierror.WriteJSON(w, http.StatusBadRequest, ve.Error())
		case errors.Is(err, context.DeadlineExceeded):
			apierror.WriteJSON(w, http.StatusRequestTimeout, "request timeout")
		case errors.Is(err, context.Canceled):
			// Client disconnected — connection is already gone, just log it.
			log.Printf("request canceled: %s %s", r.Method, r.URL.Path)
		default:
			apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
