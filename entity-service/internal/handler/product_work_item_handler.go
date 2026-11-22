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
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductWorkItemHandler serves POST /cases/{id}/product-work-items.
type ProductWorkItemHandler struct {
	svc service.ProductWorkItemService
}

// NewProductWorkItemHandler returns a handler bound to svc.
func NewProductWorkItemHandler(svc service.ProductWorkItemService) *ProductWorkItemHandler {
	return &ProductWorkItemHandler{svc: svc}
}

// CreateProductWorkItem handles POST /cases/{id}/product-work-items: 201 with
// the new result, 200 with the stored result when the same request is repeated.
func (h *ProductWorkItemHandler) CreateProductWorkItem(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateProductWorkItemRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, created, err := h.svc.CreateProductWorkItem(r.Context(), req)
	if err != nil {
		var ufe *service.UpstreamFailureError
		if errors.As(err, &ufe) {
			// The upstream's own words never reach the caller; Retry-After does.
			log.Printf("Upstream failure: %s %s: %d", r.Method, sanitizeLog(r.URL.Path), ufe.Status) // #nosec G706 -- path sanitized
			if ufe.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(ufe.RetryAfter.Seconds()))))
			}
			apierror.WriteJSON(w, ufe.Status, ufe.Msg)
			return
		}
		writeServiceError(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
