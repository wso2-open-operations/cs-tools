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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProblemHandler handles HTTP requests for the problems resource.
type ProblemHandler struct {
	svc service.ProblemService
}

// NewProblemHandler constructs a ProblemHandler with the given service.
func NewProblemHandler(svc service.ProblemService) *ProblemHandler {
	return &ProblemHandler{svc: svc}
}

// SearchProblems handles POST /problems/search.
func (h *ProblemHandler) SearchProblems(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProblemsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchProblems(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
