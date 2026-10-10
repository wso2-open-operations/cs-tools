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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// changeRequestApprovalDecisionPayload is the strict shape accepted by
// DecideChangeRequestApproval: exactly one field, and Decision must be
// "approved" or "rejected".
type changeRequestApprovalDecisionPayload struct {
	Decision string `json:"decision"`
}

// entityChangeRequestClient abstracts the entity service change-request operations.
type entityChangeRequestClient interface {
	CreateChangeRequest(ctx context.Context, body []byte) ([]byte, error)
	SearchChangeRequests(ctx context.Context, body []byte) ([]byte, error)
	AggregateChangeRequests(ctx context.Context, body []byte) ([]byte, error)
	GetChangeRequest(ctx context.Context, id string) ([]byte, error)
	PatchChangeRequest(ctx context.Context, id string, body []byte) ([]byte, error)
	GetChangeRequestApprovals(ctx context.Context, id string) ([]byte, error)
	GetChangeRequestLinkOptions(ctx context.Context, body []byte) ([]byte, error)
	CreateComment(ctx context.Context, body []byte) ([]byte, error)
	SearchComments(ctx context.Context, body []byte) ([]byte, error)
	DecideChangeRequestApproval(ctx context.Context, id string, body []byte) ([]byte, error)
	ReplayChangeRequestMirrorFailure(ctx context.Context, failureID string) ([]byte, error)
}

// ChangeRequestHandler handles HTTP requests for change-request operations.
type ChangeRequestHandler struct {
	entity entityChangeRequestClient
	// access backs the inline-image redaction in every read response — see
	// WithAccessGuard and CaseHandler's own field of the same name/reasoning.
	// nil fails that check closed (redacts), never open.
	access *AccessGuard
}

// NewChangeRequestHandler creates a ChangeRequestHandler backed by the given entity client.
func NewChangeRequestHandler(entity entityChangeRequestClient) *ChangeRequestHandler {
	return &ChangeRequestHandler{entity: entity}
}

// WithAccessGuard wires the same guard that authorises every route into this
// handler, so every read response can redact an embedded raw base64 inline
// image (see redactRawBase64Images's own doc comment) for a caller who
// lacks PermDownloadAttachment. Returns h for chaining at the construction
// site.
func (h *ChangeRequestHandler) WithAccessGuard(g *AccessGuard) *ChangeRequestHandler {
	h.access = g
	return h
}

// CreateChangeRequest handles POST /change-requests.
func (h *ChangeRequestHandler) CreateChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	// The change type decides the whole approval flow (Standard: none; Normal:
	// peer then CAB; Emergency: CAB only), so a create without one of the
	// three is refused here with a message the form can show, rather than
	// forwarded to be rejected with a generic one.
	if msg := validateChangeRequestCreateType(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if msg := validateChangeRequestCustomerGateFlags(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if msg := validateChangeRequestScopeFields(body, false); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	result, err := h.entity.CreateChangeRequest(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateChangeRequest failed", "userID", user.UserID, "err", err)
		// mapUpstreamError, not the Generic variant: a 400 here is the entity
		// service explaining why the project / deployments
		// combination was refused ("deployment ... does not belong to the
		// selected project"), which the form has to show.
		mapUpstreamError(w, err, "Failed to create change request.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// payloadValues returns the value of EVERY key of payload that names field,
// whatever its case: the entity service decodes a request body with encoding/json,
// which matches a key to a field without regard to case, so "ISCUSTOMERAPPROVED",
// "iscustomerreviewed" and "isCustomerApproved" are one and the same field there
// (and when a body names it more than once, the last one wins, null included: a
// guard that looked at one spelling only, or at the winning one only, would let
// the other through). Every guard of this file looks fields up through it, so a
// guard judges the body as the decoder will read it. The keys are the decoded
// ones (a JSON escape such as "isCustomerApprov\u0065d" is already "isCustomerApproved").
// strings.EqualFold is the comparison encoding/json itself folds keys with.
func payloadValues(payload map[string]json.RawMessage, field string) []json.RawMessage {
	var out []json.RawMessage
	for key, raw := range payload {
		if strings.EqualFold(key, field) {
			out = append(out, raw)
		}
	}
	return out
}

// changeRequestCreatableTypes are the only types a change request may be
// created with (the entity service enforces the same set).
var changeRequestCreatableTypes = []string{"standard", "normal", "emergency"}

// validateChangeRequestCreateType returns a user-facing message when body does
// not carry a valid create-time "type" (standard, normal or emergency), or ""
// when it does. A body that is not a JSON object is left for the upstream to
// reject.
func validateChangeRequestCreateType(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	const required = "type is required: a change request must be one of standard, normal or emergency"
	values := payloadValues(payload, "type")
	if len(values) == 0 {
		return required
	}
	for _, raw := range values {
		var typ string
		if err := json.Unmarshal(raw, &typ); err != nil || typ == "" {
			return required
		}
		allowed := false
		for _, ok := range changeRequestCreatableTypes {
			if typ == ok {
				allowed = true
			}
		}
		if !allowed {
			return "type is not allowed: a change request must be one of standard, normal or emergency"
		}
	}
	return ""
}

// changeRequestCustomerGateFields are the creation form's two checkboxes,
// "Customer Approval" and "Customer Review": whether the change needs the
// customer's approval before it is scheduled / the customer's review before it
// is closed. They are forwarded to the entity service as-is; the only thing
// checked here is their type, so a stray string or null is refused with a
// message the form can show instead of a generic upstream decode failure.
var changeRequestCustomerGateFields = []string{"customerApprovalRequired", "customerReviewRequired"}

// validateChangeRequestCustomerGateFlags returns a user-facing message when
// body carries one of the customer gate checkboxes with a value that is not a
// JSON boolean, or "" when it carries none or only booleans. A body that is not
// a JSON object is left for the upstream to reject.
func validateChangeRequestCustomerGateFlags(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	for _, field := range changeRequestCustomerGateFields {
		for _, raw := range payloadValues(payload, field) {
			if v := string(bytes.TrimSpace(raw)); v != "true" && v != "false" {
				return field + " must be a boolean (true or false)"
			}
		}
	}
	return ""
}

// The customer's own answer is the customer's: nobody in the CSM portal (WSO2
// staff, every caller of this BFF) records the customer's approval or review on
// the customer's behalf -- the answer is the customer's decision and the change request's
// record of it is audited. isCustomerApproved / isCustomerReviewed are that answer,
// which the customer gives in the Customer Portal, so a PATCH that carries either
// (true or false, alone or with a state) is refused here with the entity service's
// own wording, before any upstream call. The entity service refuses it too on the
// PostgreSQL data source; this also covers the ServiceNow-backed one, where the
// entity service forwards the PATCH and cannot tell staff from the customer. The
// state half of the rule (no manual scheduled / closed out of Customer Approval /
// Customer Review) is the entity service's: it needs the change request's state.
const (
	errMsgCustomerApprovedByStaff = "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"
	errMsgCustomerReviewedByStaff = "isCustomerReviewed cannot be set on the customer's behalf: the customer's review can only be given by the customer in the Customer Portal"
)

// validateChangeRequestCustomerOutcomeFlags returns a user-facing message when
// body carries isCustomerApproved or isCustomerReviewed with any value but null
// (null is "absent" to the entity service), in any spelling of the key's case, or
// "" otherwise. A body that is not a JSON object is left for the upstream to reject.
func validateChangeRequestCustomerOutcomeFlags(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	// Present under ANY spelling of the key, and not null in any of them: the
	// entity service reads "ISCUSTOMERAPPROVED" as isCustomerApproved, and a body
	// that names the flag twice ({"isCustomerApproved":null,"ISCUSTOMERAPPROVED":true})
	// is read as whichever the decoder reaches last.
	present := func(field string) bool {
		for _, raw := range payloadValues(payload, field) {
			if string(bytes.TrimSpace(raw)) != "null" {
				return true
			}
		}
		return false
	}
	if present("isCustomerApproved") {
		return errMsgCustomerApprovedByStaff
	}
	if present("isCustomerReviewed") {
		return errMsgCustomerReviewedByStaff
	}
	return ""
}

// changeRequestTimeAnswerFields are the fields of WSO2's answer to a time the customer proposed
// (the entity service's "Accept proposed time" and "Propose a different time"):
// confirmCustomerUpdatedDate ("agree" is the only value) names the acceptance;
// expectedCustomerUpdatedOn names the proposal being answered, expectedPlannedStartOn /
// expectedPlannedEndOn the planned window the page showed. They are forwarded as-is -- the BFF
// has no change request data and decides nothing about them (whether a proposal waits, whether
// the window is still the one shown, whether the change is on hold are the entity service's
// to judge under its row lock, and its refusal text comes back verbatim) -- but a value that is
// not a string is refused with a message the page can show instead of the generic decode failure.
var changeRequestTimeAnswerFields = []string{
	"confirmCustomerUpdatedDate", "expectedCustomerUpdatedOn", "expectedPlannedStartOn", "expectedPlannedEndOn",
}

// validateChangeRequestTimeAnswerFields returns a user-facing message when body carries one of
// changeRequestTimeAnswerFields with a value that is not a JSON string (null is "absent" to the
// entity service), in any spelling of the key, or "" otherwise. A body that is not a JSON object
// is left for the upstream to reject.
func validateChangeRequestTimeAnswerFields(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	for _, field := range changeRequestTimeAnswerFields {
		for _, raw := range payloadValues(payload, field) {
			// A JSON null decodes into a string without error: it is "absent", as to the entity service.
			var v string
			if err := json.Unmarshal(raw, &v); err != nil {
				return field + " must be a string"
			}
		}
	}
	return ""
}

// maxChangeRequestScopeIDs caps each of deploymentIds / deploymentProductIds
// (the entity service enforces the same limit).
const maxChangeRequestScopeIDs = 100

// changeRequestScopeIDArrays are the create/PATCH fields that carry a list of
// UUIDs: the project's deployments the change touches, and the (read-only)
// deployment products that follow from them.
var changeRequestScopeIDArrays = []string{"deploymentIds", "deploymentProductIds"}

// The two fields the change request API no longer accepts, with the entity
// service's own messages (it refuses them too; the BFF refuses first so the
// form shows the same text and no upstream call is made). The Customer Group is
// derived, read-only, from the Customer Project's registered contacts
// (`customerContacts` on the detail and on the link-options lookup); a
// deployment carries its environment.
const (
	errMsgCustomerGroupIDRemoved = "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"
	errMsgEnvironmentIDsRemoved  = "environmentIds is no longer supported: deployments carry the environment"
)

// validateChangeRequestScopeFields returns a user-facing message when body
// carries a customer-scope / journal field of the wrong shape, or "" when the
// body is fine. It checks shape only -- projectId a UUID string; deploymentIds
// and deploymentProductIds arrays of UUID strings (null is not an array);
// category, comment and workNote strings; on a PATCH comment / workNote not
// blank -- so a stray string or null is refused with a message the form can
// show instead of a generic upstream decode failure. The relationships between
// them (deployments of the project, products of the deployments, ...) are the
// entity service's to judge and are surfaced as its 400 message. The exception
// is customerGroupId / environmentIds, which are no longer accepted at all and
// are refused here (any value, null included) with the same messages. A body
// that is not a JSON object is left for the upstream to reject.
func validateChangeRequestScopeFields(body []byte, patch bool) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	isUUIDString := func(raw json.RawMessage) bool {
		var v string
		return json.Unmarshal(raw, &v) == nil && uuidRe.MatchString(v)
	}
	isNull := func(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

	for _, raw := range payloadValues(payload, "projectId") {
		if !isUUIDString(raw) {
			return "projectId must be a UUID string"
		}
	}
	if len(payloadValues(payload, "customerGroupId")) > 0 {
		return errMsgCustomerGroupIDRemoved
	}
	if len(payloadValues(payload, "environmentIds")) > 0 {
		return errMsgEnvironmentIDsRemoved
	}
	for _, field := range changeRequestScopeIDArrays {
		for _, raw := range payloadValues(payload, field) {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil || items == nil {
				return field + " must be an array of UUID strings"
			}
			if len(items) > maxChangeRequestScopeIDs {
				return fmt.Sprintf("%s must contain at most %d entries", field, maxChangeRequestScopeIDs)
			}
			for _, item := range items {
				if !isUUIDString(item) {
					return field + " must be an array of UUID strings"
				}
			}
		}
	}
	for _, raw := range payloadValues(payload, "category") {
		if patch && isNull(raw) {
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return "category must be a string"
		}
	}
	for _, field := range []string{"comment", "workNote"} {
		for _, raw := range payloadValues(payload, field) {
			var v string
			if err := json.Unmarshal(raw, &v); err != nil {
				return field + " must be a string"
			}
			if patch && strings.TrimSpace(v) == "" {
				return field + " must not be empty"
			}
		}
	}
	return ""
}

// GetChangeRequestLinkOptions handles POST /change-requests/link-options: the
// lookup behind the change request form's Customer Project -> Deployments ->
// Deployment products cascade (and the read-only Customer Group). projectId is required;
// deploymentIds (the deployments chosen so far) is optional. Whether the
// deployments belong to the project is the entity service's to judge and
// comes back as its 400 message.
func (h *ChangeRequestHandler) GetChangeRequestLinkOptions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if len(payloadValues(payload, "projectId")) == 0 {
		writeError(w, http.StatusBadRequest, "projectId is required")
		return
	}
	if msg := validateChangeRequestScopeFields(body, false); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	result, err := h.entity.GetChangeRequestLinkOptions(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequestLinkOptions failed", "userID", user.UserID, "err", err)
		mapUpstreamError(w, err, "Failed to load change request options.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// mapApprovalDecisionError is mapUpstreamErrorGeneric, except a 403 or a 409
// that carries the entity service's own reason is shown to the caller. A
// refusal to decide ("the creator of a change request cannot approve it",
// "only active internal (WSO2) users can approve or reject the Peer Approval
// stage ...", or -- a 409 -- "this approval is no longer pending: the change
// request is in Closed, but the Review stage can only be decided while it is in
// Review") is only useful if the approver can read why; every other failure
// keeps the generic mapping. The refusal's machine-readable errorCode, when
// entity-service names one (upstreamErrorCode), goes on with it.
func mapApprovalDecisionError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusConflict) {
		if msg := upstreamErrorMessageStrict(apiErr.Body, ""); msg != "" {
			writeErrorCode(w, apiErr.StatusCode, msg, upstreamErrorCode(apiErr.Body))
			return
		}
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}

// PatchChangeRequest handles PATCH /change-requests/{id}.
func (h *ChangeRequestHandler) PatchChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if len(body) > 0 && !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if msg := validateChangeRequestCustomerOutcomeFlags(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if msg := validateChangeRequestCustomerGateFlags(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if msg := validateChangeRequestScopeFields(body, true); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if msg := validateChangeRequestTimeAnswerFields(body); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	result, err := h.entity.PatchChangeRequest(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchChangeRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update change request.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetChangeRequest handles GET /change-requests/{id}.
func (h *ChangeRequestHandler) GetChangeRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetChangeRequest(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequest failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve change request.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// GetChangeRequestApprovals handles GET /change-requests/{id}/approvals.
func (h *ChangeRequestHandler) GetChangeRequestApprovals(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetChangeRequestApprovals(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequestApprovals failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve change request approvals.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateChangeRequestComment handles POST /change-requests/{id}/comments.
// Injects referenceId and referenceType into the payload and forwards to the entity
// service's reference-generic POST /comments.
func (h *ChangeRequestHandler) CreateChangeRequestComment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCommentBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	if _, err := h.entity.GetChangeRequest(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "entity GetChangeRequest failed during comment guard", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create change request comment.")
		return
	}

	newBody, err := injectReferenceFields(body, id, "change_request")
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateComment(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create change request comment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// SearchChangeRequestComments handles POST /change-requests/{id}/comments/search.
// Injects referenceId and referenceType into the payload and forwards to the entity
// service's reference-generic POST /comments/search.
func (h *ChangeRequestHandler) SearchChangeRequestComments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	newBody, err := injectReferenceFields(body, id, "change_request")
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SearchComments(r.Context(), newBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search change request comments.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// DecideChangeRequestApproval handles POST /change-requests/{id}/approvals/decision. Any user
// with access to the change request may attempt a decision; ServiceNow itself enforces that
// only the caller's own pending approval can be acted on, so this is not a bypass-only endpoint.
func (h *ChangeRequestHandler) DecideChangeRequestApproval(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload changeRequestApprovalDecisionPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || decoder.More() {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if payload.Decision != "approved" && payload.Decision != "rejected" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.DecideChangeRequestApproval(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DecideChangeRequestApproval failed", "userID", user.UserID, "id", id, "err", err)
		mapApprovalDecisionError(w, err, "Failed to submit change request approval decision.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ReplayChangeRequestMirrorFailure handles
// POST /change-requests/{id}/mirror-failures/{failureId}/replay: re-sends one
// write of the change request the previous system is still missing (an entry
// of the detail's `mirrorFailures`, dual-write data source only) through the
// entity service's replay, which clears the entry only when the re-send
// succeeds. The entity's 409 (declined again: the previous system's change
// model does not allow the move from where its record is, or the records
// still diverge) reaches the caller with its message; a rejection by the
// previous system itself is a 500 whose reason is on the refreshed detail.
// The change request id is validated for shape only: the failure row is the
// entity service's, keyed by its own id.
func (h *ChangeRequestHandler) ReplayChangeRequestMirrorFailure(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	failureID := r.PathValue("failureId")
	if id == "" || !uuidRe.MatchString(id) || failureID == "" || !uuidRe.MatchString(failureID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.ReplayChangeRequestMirrorFailure(r.Context(), failureID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ReplayChangeRequestMirrorFailure failed", "userID", user.UserID, "id", id, "failureId", failureID, "err", err)
		mapUpstreamError(w, err, "Failed to replay the mirror write to the previous system.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchChangeRequests handles POST /change-requests/search.
func (h *ChangeRequestHandler) SearchChangeRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	result, err := h.entity.SearchChangeRequests(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchChangeRequests failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search change requests.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}

	writeJSON(w, http.StatusOK, result)
}

// AggregateChangeRequests handles POST /change-requests/aggregate.
// Server-side aggregation of change requests by a single field (e.g. state,
// assignmentGroup), capped to the top maxGroups buckets with the remainder
// folded into othersCount. The groupBy allowlist is validated upstream by
// the entity service; this layer only forwards the request and passes the
// response through as-is.
func (h *ChangeRequestHandler) AggregateChangeRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	result, err := h.entity.AggregateChangeRequests(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity AggregateChangeRequests failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to aggregate change requests.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
