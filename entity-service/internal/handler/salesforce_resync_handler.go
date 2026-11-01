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

// SalesforceResyncHandler handles POST /salesforce/resync/preview and
// POST /salesforce/resync/apply. The service enforces the admin check.
type SalesforceResyncHandler struct {
	svc service.SalesforceResyncService
}

// NewSalesforceResyncHandler constructs a SalesforceResyncHandler.
func NewSalesforceResyncHandler(svc service.SalesforceResyncService) *SalesforceResyncHandler {
	return &SalesforceResyncHandler{svc: svc}
}

// Preview handles POST /salesforce/resync/preview: compares the Salesforce
// data of one account or project with the database and writes nothing.
func (h *SalesforceResyncHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req domain.SalesforceResyncRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.Preview(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// Apply handles POST /salesforce/resync/apply: overwrites the chosen
// components from Salesforce, never deleting a row.
func (h *SalesforceResyncHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var req domain.SalesforceResyncApplyRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	res, err := h.svc.Apply(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
