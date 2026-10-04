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
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// entityCaseClient abstracts the entity service case operations used by CaseHandler.
type entityCaseClient interface {
	PatchCase(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error)
	SearchCases(ctx context.Context, body []byte) ([]byte, error)
	AddCaseTag(ctx context.Context, caseID string, body []byte) ([]byte, error)
}

// CaseHandler handles HTTP requests for case operations, delegating to the
// entity service for data access. See AccountHandler's doc comment: there is no
// end-user identity checked here — Choreo's API Manager gateway is the trust
// boundary for this service's M2M/third-party consumers.
type CaseHandler struct {
	entity entityCaseClient
	// umtActorEmail is this service's own trusted M2M actor identity, asserted
	// on CreateCaseComment as entity-service's
	// CreateCaseCommentRequest.ActorEmail, and on AddCaseTag as
	// entity-service's AddCaseTagRequest.ActorEmail. It must match an entry
	// in entity-service's M2M_TRUSTED_ACTOR_EMAILS allowlist or every call
	// 403s. Never accepted from the caller — that would defeat the point of
	// the allowlist being server-configured rather than client-asserted. The
	// server refuses to start without a valid value (see cmd/server's
	// actorEmail); a value that is set but not on the allowlist still
	// surfaces normally as the entity service's 403. Despite the name (a
	// holdover from this field's original,
	// UMT-specific introduction), it is now this service's single generic
	// M2M actor identity, used by any caller of these generic case
	// operations — renaming it is out of scope for the current change.
	umtActorEmail string
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
// umtActorEmail is this service's configured M2M actor identity for
// CreateCaseComment and AddCaseTag (see the field's own doc comment); pass ""
// if unset.
func NewCaseHandler(entity entityCaseClient, umtActorEmail string) *CaseHandler {
	return &CaseHandler{entity: entity, umtActorEmail: umtActorEmail}
}

// PatchCase handles PATCH /cases/{id}. The request body is forwarded verbatim;
// the entity service enforces its own field-combination rules and 400s
// otherwise, so this handler does not re-validate that. A state/severity/
// workState-only update succeeds for this M2M-only service on a Postgres data
// source; every other field this shape accepts is external-data-source-only
// and requires a forwarded end-user identity token this service cannot
// supply, so those calls receive a mapped 401 from upstream.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r, bodyRequired)
	if !ok {
		return
	}

	result, err := h.entity.PatchCase(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchCase failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// createCaseCommentRequest is the caller-facing request body for
// CreateCaseComment: only "type" and "content" are ever accepted from the
// caller. actorEmail is never a caller input -- see
// CaseHandler.umtActorEmail's doc comment.
type createCaseCommentRequest struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// createCaseCommentUpstreamRequest is the entity-service POST
// /cases/{id}/comments body this handler builds, mirroring
// domain.CreateCaseCommentRequest. ActorEmail is always this service's own
// configured M2M identity.
type createCaseCommentUpstreamRequest struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	ActorEmail string `json:"actorEmail"`
}

// CreateCaseComment handles POST /cases/{id}/comments. The caller supplies
// only "type" and "content"; this handler supplies entity-service's
// actorEmail field itself, from this service's own configured trusted M2M
// identity (CaseHandler.umtActorEmail) -- it is never taken from the caller,
// the same way AddCaseTag injects it. If
// umtActorEmail is unset or not on entity-service's
// M2M_TRUSTED_ACTOR_EMAILS allowlist, entity-service rejects the call with
// 403, which is surfaced normally rather than special-cased here.
//
// Before entity-service grew its own actorEmail allowlist path for this
// operation, this endpoint always received a mapped 401 (comment-author
// resolution unconditionally required a forwarded end-user identity token,
// which this strictly-M2M service has no mechanism to supply). That gap is
// now closed for the configured M2M actor.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	raw, ok := readJSONBody(w, r, bodyRequired)
	if !ok {
		return
	}

	var req createCaseCommentRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, ErrMsgContentRequired)
		return
	}

	upstreamBody, err := json.Marshal(createCaseCommentUpstreamRequest{
		Type:       req.Type,
		Content:    req.Content,
		ActorEmail: h.umtActorEmail,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal create case comment body failed", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateCaseComment(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create case comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchCases handles POST /cases/search. A generic passthrough to
// entity-service's own POST /cases/search — the request body is forwarded
// verbatim (callers build whatever filter/pagination shape entity-service's
// contract accepts, e.g. an exact-match filter on "number" to look up a case
// by case number) and the response is returned as-is. No case-number lookup
// or other special-casing lives here; mirrors SearchAccounts's shape.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	body, ok := readJSONBody(w, r, bodyRequired)
	if !ok {
		return
	}

	result, err := h.entity.SearchCases(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// addCaseTagRequest is the caller-facing request body for AddCaseTag: only
// "label" is ever accepted from the caller. actorEmail is never a caller
// input -- see CaseHandler.umtActorEmail's doc comment.
type addCaseTagRequest struct {
	Label string `json:"label"`
}

// addCaseTagUpstreamRequest is the entity-service POST /cases/{id}/tags body
// this handler builds, mirroring domain.AddCaseTagRequest. ActorEmail is
// always this service's own configured M2M identity.
type addCaseTagUpstreamRequest struct {
	Label      string `json:"label"`
	ActorEmail string `json:"actorEmail"`
}

// AddCaseTag handles POST /cases/{id}/tags. The caller supplies only "label";
// this handler supplies entity-service's actorEmail field itself, from this
// service's own configured trusted M2M identity (CaseHandler.umtActorEmail)
// -- it is never taken from the caller, the same way CreateCaseComment
// injects it. If umtActorEmail is unset or not on entity-service's
// M2M_TRUSTED_ACTOR_EMAILS allowlist, entity-service rejects the call with
// 403, which is surfaced normally rather than special-cased here.
func (h *CaseHandler) AddCaseTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	raw, ok := readJSONBody(w, r, bodyRequired)
	if !ok {
		return
	}

	var req addCaseTagRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		writeError(w, http.StatusBadRequest, ErrMsgLabelRequired)
		return
	}

	upstreamBody, err := json.Marshal(addCaseTagUpstreamRequest{
		Label:      req.Label,
		ActorEmail: h.umtActorEmail,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal add case tag body failed", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.AddCaseTag(r.Context(), id, upstreamBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AddCaseTag failed", "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to add case tag.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
