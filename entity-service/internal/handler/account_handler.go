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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// AccountHandler handles HTTP requests for the account resource.
type AccountHandler struct {
	svc service.AccountService
}

// NewAccountHandler constructs an AccountHandler with the given service.
func NewAccountHandler(svc service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// SearchAccounts handles POST /accounts/search.
func (h *AccountHandler) SearchAccounts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAccountsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchAccounts(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetAccount handles GET /accounts/{id}.
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := h.svc.GetAccountByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(account)
}

// SNAccountHandler handles HTTP requests for account operations backed by ServiceNow.
type SNAccountHandler struct {
	svc service.SNAccountService
}

// NewSNAccountHandler constructs an SNAccountHandler with the given service.
func NewSNAccountHandler(svc service.SNAccountService) *SNAccountHandler {
	return &SNAccountHandler{svc: svc}
}

// SearchAccounts handles POST /accounts/search for the ServiceNow data source.
func (h *SNAccountHandler) SearchAccounts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAccountsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchAccounts(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetAccount handles GET /accounts/{id} for the ServiceNow data source.
func (h *SNAccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := h.svc.GetAccountByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(account)
}
