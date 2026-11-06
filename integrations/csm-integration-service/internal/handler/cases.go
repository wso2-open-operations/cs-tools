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
	"io"
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
	// the allowlist being server-configured rather than client-asserted. An
	// empty value here is a deploy-time misconfiguration, not something this
	// handler special-cases; the resulting entity-service 403 surfaces
	// normally. Despite the name (a holdover from this field's original,
	// UMT-specific introduction), it is now this service's single generic
	// M2M actor identity, used by any caller of these generic case
	// operations — renaming it is out of scope for the current change.
	umtActorEmail string
	// clientActorEmails maps the client id of a third-party consumer calling
	// through this service -- decoded (not verified; see
	// clientIDFromJWTAssertion's own doc comment) from the x-jwt-assertion
	// header Choreo's API Manager gateway forwards -- to that consumer's own
	// display email. Used only to attribute a markFixIssued PatchCase call
	// (see attributeMarkFixIssued): entity-service's work_item_activity feed
	// otherwise shows this service's own single, shared service-to-service
	// client id for every such call, regardless of which real third-party
	// integration actually triggered it, since every call through this
	// service authenticates to entity-service with the same one credential
	// (internal/entity/client.go). A client id with no entry here is simply
	// not attributed -- best-effort, never a validation gate. Nil/empty is
	// safe and disables the enrichment entirely.
	clientActorEmails map[string]string
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
// umtActorEmail is this service's configured M2M actor identity for
// CreateCaseComment and AddCaseTag (see the field's own doc comment); pass ""
// if unset. clientActorEmails is the per-client-id map used only by
// PatchCase's markFixIssued enrichment (see that field's own doc comment);
// pass nil if unset.
func NewCaseHandler(entity entityCaseClient, umtActorEmail string, clientActorEmails map[string]string) *CaseHandler {
	return &CaseHandler{entity: entity, umtActorEmail: umtActorEmail, clientActorEmails: clientActorEmails}
}

// patchCaseMarkFixIssuedUpstreamRequest mirrors the two fields
// attributeMarkFixIssued's enrichment cares about -- domain.UpdateCaseRequest's
// own MarkFixIssued/ActorEmail -- for tests to decode out of an upstream body
// that may also carry other, untouched fields.
type patchCaseMarkFixIssuedUpstreamRequest struct {
	MarkFixIssued *bool   `json:"markFixIssued"`
	ActorEmail    *string `json:"actorEmail,omitempty"`
}

// stripCallerActorEmail removes a caller-supplied "actorEmail" field from a
// PatchCase body, unconditionally and regardless of whether
// attributeMarkFixIssued goes on to set its own. actorEmail is meaningful to
// entity-service only because THIS service's own call to it is already
// trusted (the same M2MClientIDs allowlist AddCaseTag/CreateCaseComment's own
// actorEmail relies on) -- a caller-supplied value would therefore be
// accepted by entity-service at face value, letting any caller of this
// passthrough endpoint claim to be anyone, in a case's permanent activity
// history. Every actorEmail entity-service ever sees from this service must
// come from CaseHandler.clientActorEmails, never from the request body it
// received. Returns body unchanged if it carries no such field, or is not a
// JSON object at all (PatchCase's own json.Valid check already guards the
// latter; a non-object body fails there, never reaching this function).
//
// Matches the key case-insensitively, on purpose: encoding/json's own
// struct-field decoding (what entity-service's domain.UpdateCaseRequest
// uses) falls back to a case-insensitive match when no exact field-tag match
// is found, so "ActorEmail" or "ACTOREMAIL" reaches entity-service's
// ActorEmail field exactly as "actorEmail" would. A map keyed by the literal
// JSON key, which decodes case-sensitively unlike struct decoding, would
// leave those variants untouched and still forwarded.
func stripCallerActorEmail(body []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	changed := false
	for k := range m {
		if strings.EqualFold(k, "actorEmail") {
			delete(m, k)
			changed = true
		}
	}
	if !changed {
		return body
	}
	cleaned, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return cleaned
}

// attributeMarkFixIssued adds an actorEmail resolved from the caller's own
// x-jwt-assertion client id (via CaseHandler.clientActorEmails, see that
// field's own doc comment) to a markFixIssued PatchCase body, preserving
// every other field the body carried -- unlike an earlier revision of this
// function, which rebuilt the body from a struct holding only MarkFixIssued/
// ActorEmail and so silently dropped anything else a caller sent alongside
// markFixIssued (letting entity-service's own field-combination rejection
// never see it, and masking what should have been a 400 with a false
// success). Every other body shape passes through unchanged. A markFixIssued
// body this handler cannot attribute -- no x-jwt-assertion header, an
// undecodable one, or a client id with no entry in that map -- also passes
// through unchanged (no actorEmail added, leaving entity-service's own
// resolveActorOrM2M fallback to apply as before), but is logged at WARN so
// an unmapped/misconfigured caller is visible rather than silently never
// attributed. This is a best-effort enrichment, never a validation gate: it
// must never block or fail the call, since entity-service enforces its own
// field-combination and actorEmail-trust rules regardless of what this does
// here. Callers of this method must pass body through stripCallerActorEmail
// first -- this function only ever adds its own server-resolved actorEmail,
// never preserves one found in the input.
func (h *CaseHandler) attributeMarkFixIssued(r *http.Request, body []byte) []byte {
	if len(h.clientActorEmails) == 0 {
		return body
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	raw, ok := m["markFixIssued"]
	if !ok {
		return body
	}
	// A *bool, not bool: an explicit "markFixIssued": null must be treated
	// the same as the key being absent (not a markFixIssued update), not as
	// false -- matching domain.UpdateCaseRequest.MarkFixIssued's own *bool
	// shape and entity-service's own nil-check for it.
	var markFixIssued *bool
	if err := json.Unmarshal(raw, &markFixIssued); err != nil || markFixIssued == nil {
		return body
	}
	clientID := clientIDFromJWTAssertion(r.Header.Get(jwtAssertionHeader))
	if clientID == "" {
		slog.WarnContext(r.Context(), "patch case markFixIssued: no x-jwt-assertion client id found, forwarding with no actorEmail")
		return body
	}
	email, ok := h.clientActorEmails[clientID]
	if !ok {
		slog.WarnContext(r.Context(), "patch case markFixIssued: caller client id has no configured actor email, forwarding with no actorEmail", "clientId", clientID)
		return body
	}
	emailJSON, err := json.Marshal(email)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal patch case markFixIssued actorEmail failed", "err", err)
		return body
	}
	m["actorEmail"] = emailJSON
	upstream, err := json.Marshal(m)
	if err != nil {
		slog.ErrorContext(r.Context(), "marshal patch case markFixIssued body failed", "err", err)
		return body
	}
	return upstream
}

// PatchCase handles PATCH /cases/{id}. The request body is forwarded
// verbatim, except that any caller-supplied "actorEmail" is always stripped
// first (stripCallerActorEmail) and, for a markFixIssued update,
// attributeMarkFixIssued may then add back its own server-resolved one (see
// that method's own doc comment). The entity service enforces its own
// field-combination rules and
// 400s otherwise, so this handler does not re-validate that. A state/
// severity/workState-only update succeeds for this M2M-only service on a
// Postgres data source; every other field this shape accepts is
// ServiceNow-data-source-only and requires a forwarded end-user identity
// token this service cannot supply, so those calls receive a mapped 401 from
// upstream.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// stripCallerActorEmail runs unconditionally, before attributeMarkFixIssued
	// -- a caller must never be able to self-assert the field that attributes
	// a markFixIssued call's identity (see that function's own doc comment).
	body = stripCallerActorEmail(body)
	body = h.attributeMarkFixIssued(r, body)

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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(raw) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
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
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(raw) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
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
