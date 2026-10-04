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
	"log/slog"
	"net/http"
)

// entityInvoiceClient abstracts the entity service invoice operations used by
// InvoiceHandler. Read-only and M2M-safe on both data sources.
type entityInvoiceClient interface {
	SearchInvoices(ctx context.Context, body []byte) ([]byte, error)
	GetInvoice(ctx context.Context, id string) ([]byte, error)
}

// InvoiceHandler handles HTTP requests for invoice operations, delegating to the
// entity service for data access. See AccountHandler's doc comment: there is no
// end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type InvoiceHandler struct {
	entity entityInvoiceClient
}

// NewInvoiceHandler creates an InvoiceHandler backed by the given entity client.
func NewInvoiceHandler(entity entityInvoiceClient) *InvoiceHandler {
	return &InvoiceHandler{entity: entity}
}

// SearchInvoices handles POST /invoices/search.
func (h *InvoiceHandler) SearchInvoices(w http.ResponseWriter, r *http.Request) {
	body, ok := readJSONBody(w, r, bodyOptional)
	if !ok {
		return
	}

	result, err := h.entity.SearchInvoices(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchInvoices failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search invoices.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetInvoice handles GET /invoices/{id}.
func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetInvoice(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetInvoice failed", "invoiceID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve invoice.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
