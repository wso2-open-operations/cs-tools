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

// FeedbackHandler handles HTTP requests for the case-feedback resource.
type FeedbackHandler struct {
	svc service.FeedbackService
}

// NewFeedbackHandler constructs a FeedbackHandler with the given service.
func NewFeedbackHandler(svc service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{svc: svc}
}

// SearchFeedback handles POST /cases/feedback/search.
func (h *FeedbackHandler) SearchFeedback(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchFeedbackRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchFeedback(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// AggregateFeedback handles POST /cases/feedback/aggregate.
func (h *FeedbackHandler) AggregateFeedback(w http.ResponseWriter, r *http.Request) {
	var req domain.AggregateFeedbackRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AggregateFeedback(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
