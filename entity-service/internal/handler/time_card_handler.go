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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// TimeCardHandler handles HTTP requests for the time-cards resource.
type TimeCardHandler struct {
	svc service.TimeCardService
}

// NewTimeCardHandler constructs a TimeCardHandler with the given service.
func NewTimeCardHandler(svc service.TimeCardService) *TimeCardHandler {
	return &TimeCardHandler{svc: svc}
}

// SearchTimeCards handles POST /time-cards/search.
func (h *TimeCardHandler) SearchTimeCards(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchTimeCardsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchTimeCards(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateTimeCard handles POST /time-cards.
func (h *TimeCardHandler) CreateTimeCard(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateTimeCardRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateTimeCard(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateTimeCard handles PATCH /time-cards/{id}.
func (h *TimeCardHandler) UpdateTimeCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		apierror.WriteJSON(w, http.StatusBadRequest, "time card ID is required")
		return
	}
	var req domain.UpdateTimeCardRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = id
	resp, err := h.svc.UpdateTimeCard(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
