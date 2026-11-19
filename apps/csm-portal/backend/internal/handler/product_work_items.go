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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	// maxIdempotencyKeyChars is the longest idempotencyKey the entity service
	// accepts.
	maxIdempotencyKeyChars = 200
	idempotencyKeyHeader   = "Idempotency-Key"
)

// WithGitInternals makes POST /cases/{id}/github-issues forward to the entity
// service's POST /cases/{id}/product-work-items instead of creating the issue
// through the engineering client or the entity service's own github-issues
// route. Off (the default) leaves both of those exactly as they were. Returns h
// for chaining at the construction site.
func (h *CaseHandler) WithGitInternals(enabled bool) *CaseHandler {
	h.gitInternals = enabled
	return h
}

// validIdempotencyKey reports whether k is 1..200 characters, all printable
// (no control characters, no line breaks).
func validIdempotencyKey(k string) bool {
	if k == "" || !utf8.ValidString(k) || utf8.RuneCountInString(k) > maxIdempotencyKeyChars {
		return false
	}
	for _, r := range k {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return false
		}
	}
	return true
}

// derivedIdempotencyKey is the key used when the caller sent no Idempotency-Key
// header: a hash of the caller, the case and the create body, so a double click
// or a retry after a failed attempt (including a 503 "issue created but not
// recorded") sends the same key and the entity service replays or resumes the
// first attempt instead of filing a second issue. The body is hashed in its
// canonical form (the struct's fixed field order) with the key left empty.
func derivedIdempotencyKey(userID, caseID string, body productWorkItemCreateBody) (string, error) {
	body.IdempotencyKey = ""
	canonical, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, part := range [][]byte{[]byte(userID), []byte(caseID), canonical} {
		_, _ = h.Write(part)
		_, _ = h.Write([]byte{0})
	}
	return "bff-" + hex.EncodeToString(h.Sum(nil)), nil
}

// productWorkItemCreateBody is the exact body entity-service's
// POST /cases/{id}/product-work-items accepts: it rejects unknown fields, so the
// request is rebuilt from the parsed fields and the raw body is never forwarded.
// repoOverride is deliberately absent: the repository comes from the case product.
type productWorkItemCreateBody struct {
	IdempotencyKey       string `json:"idempotencyKey"`
	Title                string `json:"title"`
	Description          string `json:"description,omitempty"`
	Reason               string `json:"reason,omitempty"`
	UpdateLevel          string `json:"updateLevel,omitempty"`
	PublicIssueURL       string `json:"publicIssueUrl,omitempty"`
	Regression           bool   `json:"regression,omitempty"`
	HotFixRequired       bool   `json:"hotFixRequired,omitempty"`
	IssueTypeLabel       string `json:"issueTypeLabel,omitempty"`
	PriorityLevel        string `json:"priorityLevel,omitempty"`
	OnboardingInProgress bool   `json:"onboardingInProgress,omitempty"`
}

// createGitHubIssueViaProductWorkItems forwards the dialog's request to the
// entity service, which builds the issue body, footer and labels, writes the
// work note and (for a patch) the case tag itself; this handler writes none of
// those. The body is validated as on the other paths. The entity service's 201
// and its 200 replay of an earlier identical request both answer 201 here, so
// the webapp's contract is unchanged; its extra fields (kind, workItem) pass
// through. The signed-in user's token reaches it through the entity client like
// every other entity call, which is how "Opened by" is known.
func (h *CaseHandler) createGitHubIssueViaProductWorkItems(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, caseID string, body []byte) {
	var req caseGitHubIssueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxGitHubIssueTitleChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubTitleInvalid)
		return
	}
	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > maxGitHubIssueBodyChars {
		writeError(w, http.StatusBadRequest, errMsgGitHubBodyTooLong)
		return
	}

	createBody := productWorkItemCreateBody{
		Title:                title,
		Description:          description,
		Reason:               req.Reason,
		UpdateLevel:          req.UpdateLevel,
		PublicIssueURL:       req.PublicIssueURL,
		Regression:           req.Regression,
		HotFixRequired:       req.HotFixRequired,
		IssueTypeLabel:       req.IssueTypeLabel,
		PriorityLevel:        req.PriorityLevel,
		OnboardingInProgress: req.OnboardingInProgress,
	}
	if key := r.Header.Get(idempotencyKeyHeader); key != "" {
		if !validIdempotencyKey(key) {
			writeError(w, http.StatusBadRequest, "Idempotency-Key must be 1 to 200 printable characters.")
			return
		}
		createBody.IdempotencyKey = key
	} else {
		derived, err := derivedIdempotencyKey(user.UserID, caseID, createBody)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		createBody.IdempotencyKey = derived
	}
	forward, err := json.Marshal(createBody)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateCaseProductWorkItem(r.Context(), caseID, forward)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseProductWorkItem failed", "userID", user.UserID, "caseID", caseID, "err", err)
		// The entity service's 400/409 reasons (no repository mapped, case not
		// open, request in progress, key reused) are ones the dialog can show, but
		// only from a proper error envelope (mapUpstreamErrorEnvelope). Its 5xx
		// are generic. The mapper does not carry Retry-After, so a rate-limited
		// 503 loses it.
		mapUpstreamErrorEnvelope(w, err, "Failed to create GitHub issue.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// mapUpstreamErrorEnvelope is mapUpstreamError with one difference: a 409 or 422
// shows the upstream reason only when the body is a proper JSON error envelope
// (a non-empty message, with its errorCode when it has a plain one). Any other
// body, an HTML error page from a gateway say, is never echoed: the caller gets
// a fixed message for the status. Every other status is mapped as
// mapUpstreamError does (400 is already strict, 5xx generic).
func mapUpstreamErrorEnvelope(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusConflict:
			writeErrorCode(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, "The request conflicts with the current state."), upstreamErrorCode(apiErr.Body))
			return
		case http.StatusUnprocessableEntity:
			writeErrorCode(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, "The request could not be processed."), upstreamErrorCode(apiErr.Body))
			return
		}
	}
	mapUpstreamError(w, err, fallbackMsg)
}

// productWorkItemClient is the entity-service surface the product work item
// proxies use.
type productWorkItemClient interface {
	ListCaseProductWorkItems(ctx context.Context, caseID string) ([]byte, error)
	GetProductWorkItem(ctx context.Context, id string) ([]byte, error)
	SearchProductWorkItems(ctx context.Context, body []byte) ([]byte, error)
	GetProductIssueDashboard(ctx context.Context, query url.Values) ([]byte, error)
	LinkCaseProductWorkItem(ctx context.Context, caseID string, body []byte) ([]byte, error)
	UnlinkCaseProductWorkItem(ctx context.Context, caseID, workItemID string) ([]byte, error)
	CommentProductWorkItem(ctx context.Context, id string, body []byte, idempotencyKey string) (int, []byte, error)
	SetProductWorkItemBoardStatus(ctx context.Context, id string, body []byte, idempotencyKey string) (int, []byte, error)
	CloseProductWorkItem(ctx context.Context, id string, body []byte, idempotencyKey string) (int, []byte, error)
	RetryProductWorkItemCommand(ctx context.Context, id, commandID string, body []byte, idempotencyKey string) (int, []byte, error)
}

// ProductWorkItemHandler proxies the entity service's product work item read
// and link routes. Its routes are registered only when GIT_INTERNALS_ENABLED is
// on; the entity service re-checks that the caller is internal.
type ProductWorkItemHandler struct {
	entity productWorkItemClient
	access *AccessGuard
}

// NewProductWorkItemHandler creates a ProductWorkItemHandler.
func NewProductWorkItemHandler(entity productWorkItemClient) *ProductWorkItemHandler {
	return &ProductWorkItemHandler{entity: entity}
}

// WithAccessGuard supplies the guard that decides inline-image redaction. A nil
// guard redacts (fails closed).
func (h *ProductWorkItemHandler) WithAccessGuard(g *AccessGuard) *ProductWorkItemHandler {
	h.access = g
	return h
}

// readJSONObjectBody reads the request body under the body cap and requires one
// JSON object. It writes the error response and returns false otherwise.
func readJSONObjectBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// ListCaseProductWorkItems handles GET /cases/{id}/product-work-items.
func (h *ProductWorkItemHandler) ListCaseProductWorkItems(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	caseID := r.PathValue("id")
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	result, err := h.entity.ListCaseProductWorkItems(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCaseProductWorkItems failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve product issues.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetProductWorkItem handles GET /product-work-items/{id}. The detail's
// conversation carries GitHub comment bodies as markdown text, which can hold a
// pasted data:image URI, so the inline-image redaction runs on it like any other
// response that carries comment content.
func (h *ProductWorkItemHandler) GetProductWorkItem(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	result, err := h.entity.GetProductWorkItem(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProductWorkItem failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve product issue.")
		return
	}
	if shouldRedactInlineImages(h.access, user.Roles) {
		result = redactRawBase64Images(result)
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchProductWorkItems handles POST /product-work-items/search. The entity
// service validates the filters; the body must be one JSON object.
func (h *ProductWorkItemHandler) SearchProductWorkItems(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readJSONObjectBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.SearchProductWorkItems(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductWorkItems failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search product issues.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// productIssueDashboardParams are the only query parameters forwarded to the
// entity service's dashboard route; anything else is dropped.
var productIssueDashboardParams = []string{"from", "to", "bucket", "products"}

// GetProductIssueDashboard handles GET /product-work-items/dashboard (route
// permission PermViewProductIssueDashboard). It forwards only from, to, bucket
// and products, re-encoded, and passes the entity service's 200 body through.
// The entity service validates the values; its 400 reason reaches the caller
// (the reason names only the rejected parameter, never data); every other
// failure is the generic mapping.
func (h *ProductWorkItemHandler) GetProductIssueDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	in := r.URL.Query()
	q := url.Values{}
	for _, name := range productIssueDashboardParams {
		if vals, ok := in[name]; ok && len(vals) > 0 {
			q.Set(name, vals[0])
		}
	}
	result, err := h.entity.GetProductIssueDashboard(r.Context(), q)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProductIssueDashboard failed", "userID", user.UserID, "err", err)
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
			mapUpstreamError(w, err, "Failed to load the product issue dashboard.")
			return
		}
		mapUpstreamErrorGeneric(w, err, "Failed to load the product issue dashboard.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// LinkCaseProductWorkItem handles POST /cases/{id}/product-work-items/links.
// The entity service answers 201 (registered or newly linked) or 200 (already
// linked to this case); the entity client does not expose the status, so this
// handler answers 200 for both. Its 409 reasons (already_linked,
// request_in_progress) reach the caller.
func (h *ProductWorkItemHandler) LinkCaseProductWorkItem(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	caseID := r.PathValue("id")
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	body, ok := readJSONObjectBody(w, r)
	if !ok {
		return
	}
	result, err := h.entity.LinkCaseProductWorkItem(r.Context(), caseID, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity LinkCaseProductWorkItem failed", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorEnvelope(w, err, "Failed to link product issue.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UnlinkCaseProductWorkItem handles DELETE /cases/{id}/product-work-items/{workItemId}/link.
func (h *ProductWorkItemHandler) UnlinkCaseProductWorkItem(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	caseID, workItemID := r.PathValue("id"), r.PathValue("workItemId")
	if !uuidRe.MatchString(caseID) || !uuidRe.MatchString(workItemID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	if _, err := h.entity.UnlinkCaseProductWorkItem(r.Context(), caseID, workItemID); err != nil {
		slog.ErrorContext(r.Context(), "entity UnlinkCaseProductWorkItem failed", "userID", user.UserID, "caseID", caseID, "workItemID", workItemID, "err", err)
		mapUpstreamErrorEnvelope(w, err, "Failed to unlink product issue.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeBackAction is the shared body of the four write-back routes: validate the
// path ids, read the body (one JSON object; an empty body counts as {} when
// emptyOK), validate the optional Idempotency-Key, call the entity service and
// answer with its success status (202, or 200 for a replay) and body. Refusals
// reach the caller with their reason (mapUpstreamErrorEnvelope).
func (h *ProductWorkItemHandler) writeBackAction(w http.ResponseWriter, r *http.Request, op string, emptyOK bool, ids []string, call func(ctx context.Context, body []byte, key string) (int, []byte, error), fallbackMsg string) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	for _, id := range ids {
		if !uuidRe.MatchString(id) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
			return
		}
	}
	var body []byte
	if emptyOK {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
				return
			}
			writeError(w, http.StatusBadRequest, errMsgReadBody)
			return
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			r.Body = io.NopCloser(strings.NewReader("{}"))
		} else {
			r.Body = io.NopCloser(strings.NewReader(string(raw)))
		}
	}
	body, ok := readJSONObjectBody(w, r)
	if !ok {
		return
	}
	key := r.Header.Get(idempotencyKeyHeader)
	if key != "" && !validIdempotencyKey(key) {
		writeError(w, http.StatusBadRequest, "Idempotency-Key must be 1 to 200 printable characters.")
		return
	}
	status, result, err := call(r.Context(), body, key)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", user.UserID, "ids", ids, "err", err)
		mapUpstreamErrorEnvelope(w, err, fallbackMsg)
		return
	}
	if status != http.StatusOK && status != http.StatusAccepted {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

// CommentProductWorkItem handles POST /product-work-items/{id}/comments.
func (h *ProductWorkItemHandler) CommentProductWorkItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.writeBackAction(w, r, "CommentProductWorkItem", false, []string{id}, func(ctx context.Context, b []byte, k string) (int, []byte, error) {
		return h.entity.CommentProductWorkItem(ctx, id, b, k)
	}, "Failed to add comment.")
}

// SetProductWorkItemBoardStatus handles POST /product-work-items/{id}/board-status.
func (h *ProductWorkItemHandler) SetProductWorkItemBoardStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.writeBackAction(w, r, "SetProductWorkItemBoardStatus", false, []string{id}, func(ctx context.Context, b []byte, k string) (int, []byte, error) {
		return h.entity.SetProductWorkItemBoardStatus(ctx, id, b, k)
	}, "Failed to move product issue.")
}

// CloseProductWorkItem handles POST /product-work-items/{id}/close.
func (h *ProductWorkItemHandler) CloseProductWorkItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.writeBackAction(w, r, "CloseProductWorkItem", true, []string{id}, func(ctx context.Context, b []byte, k string) (int, []byte, error) {
		return h.entity.CloseProductWorkItem(ctx, id, b, k)
	}, "Failed to close product issue.")
}

// RetryProductWorkItemCommand handles POST /product-work-items/{id}/commands/{commandId}/retry.
func (h *ProductWorkItemHandler) RetryProductWorkItemCommand(w http.ResponseWriter, r *http.Request) {
	id, commandID := r.PathValue("id"), r.PathValue("commandId")
	h.writeBackAction(w, r, "RetryProductWorkItemCommand", true, []string{id, commandID}, func(ctx context.Context, b []byte, k string) (int, []byte, error) {
		return h.entity.RetryProductWorkItemCommand(ctx, id, commandID, b, k)
	}, "Failed to retry command.")
}
