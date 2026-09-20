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

// OutageHandler handles HTTP requests for the outages resource.
type OutageHandler struct {
	svc service.OutageService
}

// NewOutageHandler constructs an OutageHandler with the given service.
func NewOutageHandler(svc service.OutageService) *OutageHandler {
	return &OutageHandler{svc: svc}
}

// CreateOutage handles POST /outages.
func (h *OutageHandler) CreateOutage(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateOutageRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CreateOutage(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchOutages handles POST /outages/search.
func (h *OutageHandler) SearchOutages(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchOutagesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchOutages(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetOutage handles GET /outages/{id}.
func (h *OutageHandler) GetOutage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := h.svc.GetOutageByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// PatchOutage handles PATCH /outages/{id}.
func (h *OutageHandler) PatchOutage(w http.ResponseWriter, r *http.Request) {
	var req domain.PatchOutageRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	resp, err := h.svc.UpdateOutage(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// AddOutageCommunication handles POST /outages/{id}/communications.
func (h *OutageHandler) AddOutageCommunication(w http.ResponseWriter, r *http.Request) {
	var req domain.AddOutageCommunicationRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OutageID = r.PathValue("id")
	resp, err := h.svc.AddOutageCommunication(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchOutageCommunications handles POST /outages/{id}/communications/search.
func (h *OutageHandler) SearchOutageCommunications(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchOutageCommunicationsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.OutageID = r.PathValue("id")
	resp, err := h.svc.SearchOutageCommunications(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetOutageMetadata handles GET /outages/metadata.
func (h *OutageHandler) GetOutageMetadata(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetOutageMetadata(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
