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
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxRequestBodySize caps the JSON body at 1 MiB to prevent memory exhaustion.
const maxRequestBodySize = int64(1 << 20)

// UserHandler handles HTTP requests for the user resource.
type UserHandler struct {
	svc service.UserService
}

// NewUserHandler constructs a UserHandler with the given service.
func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// SearchUsers handles POST /users/search for the postgres data source.
func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchUsersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchUsers(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SNUserHandler handles HTTP requests for the user resource backed by ServiceNow.
type SNUserHandler struct {
	svc service.SNUserService
}

// NewSNUserHandler constructs an SNUserHandler with the given service.
func NewSNUserHandler(svc service.SNUserService) *SNUserHandler {
	return &SNUserHandler{svc: svc}
}

// SearchUsers handles POST /users/search for the ServiceNow data source.
func (h *SNUserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchUsersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchUsers(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
