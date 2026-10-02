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
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CustomerEngagementAllocationHandler handles POST /customer-engagements/allocation-events.
type CustomerEngagementAllocationHandler struct {
	svc service.CustomerEngagementAllocationService
	// internalClientIDs is config.Config.AuthInternalClientIDs.
	internalClientIDs map[string]bool
}

// NewCustomerEngagementAllocationHandler constructs the handler.
func NewCustomerEngagementAllocationHandler(svc service.CustomerEngagementAllocationService, internalClientIDs map[string]bool) *CustomerEngagementAllocationHandler {
	return &CustomerEngagementAllocationHandler{svc: svc, internalClientIDs: internalClientIDs}
}

// ProcessAllocationEvent applies one Allocation-app event. A skip is still 200, so the
// caller logs it without retrying; only allow-listed internal clients may call.
func (h *CustomerEngagementAllocationHandler) ProcessAllocationEvent(w http.ResponseWriter, r *http.Request) {
	id := auth.IdentityFromContext(r.Context())
	if !id.Validated || id.ClientID == "" || !h.internalClientIDs[id.ClientID] {
		apierror.WriteJSON(w, http.StatusUnauthorized, "an authorized internal client credential is required")
		return
	}

	// Unknown fields are ignored: the body is the Finance Entity's whole allocation record.
	var ev domain.AllocationEvent
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err := dec.Decode(&ev); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, decodeErrMsg(err))
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		apierror.WriteJSON(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return
	}

	res, err := h.svc.ProcessAllocationEvent(repository.WithSystemIdentity(r.Context()), ev)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "allocation event processed",
		"allocationId", sanitizeLog(ev.ID), "result", res.Result, "reason", res.Reason,
		"engagementCreated", res.EngagementCreated)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}
