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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductIssueWriteHandler serves the portal's write actions on a product issue:
// comment, move to WOW, close, and retry of a failed one. Each is queued (202) for
// the worker; an Idempotency-Key repeat answers 200 with the first result.
type ProductIssueWriteHandler struct {
	svc service.ProductIssueWriteService
}

// NewProductIssueWriteHandler constructs a ProductIssueWriteHandler.
func NewProductIssueWriteHandler(svc service.ProductIssueWriteService) *ProductIssueWriteHandler {
	return &ProductIssueWriteHandler{svc: svc}
}

func queuedStatus(replay bool) int {
	if replay {
		return http.StatusOK
	}
	return http.StatusAccepted
}

// AddComment handles POST /product-work-items/{id}/comments.
func (h *ProductIssueWriteHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	var req domain.AddProductIssueCommentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, replay, err := h.svc.AddComment(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, queuedStatus(replay), resp)
}

// SetBoardStatus handles POST /product-work-items/{id}/board-status.
func (h *ProductIssueWriteHandler) SetBoardStatus(w http.ResponseWriter, r *http.Request) {
	var req domain.SetProductIssueBoardStatusRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	cmd, replay, err := h.svc.SetBoardStatus(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, queuedStatus(replay), domain.ProductIssueCommandResponse{Command: cmd})
}

// CloseProductWorkItem handles POST /product-work-items/{id}/close.
func (h *ProductIssueWriteHandler) CloseProductWorkItem(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !decodeOptionalRequest(w, r, &body) {
		return
	}
	cmd, replay, err := h.svc.Close(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, queuedStatus(replay), domain.ProductIssueCommandResponse{Command: cmd})
}

// RetryCommand handles POST /product-work-items/{id}/commands/{commandId}/retry.
func (h *ProductIssueWriteHandler) RetryCommand(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !decodeOptionalRequest(w, r, &body) {
		return
	}
	cmd, err := h.svc.Retry(r.Context(), r.PathValue("id"), r.PathValue("commandId"))
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusAccepted, domain.ProductIssueCommandResponse{Command: cmd})
}
