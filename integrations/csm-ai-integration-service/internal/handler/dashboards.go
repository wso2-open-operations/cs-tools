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

// Package handler holds the service's HTTP handlers. Its one caller is the
// CSM portal backend, which has already authenticated and authorised the
// user, so no user identity is read here.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/dashboards"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-ai-integration-service/internal/teams"
)

const (
	// maxRequestBodyBytes caps a request body (1 MiB): a prompt, a draft
	// and the team list are far below it.
	maxRequestBodyBytes = 1 << 20
	// promptMaxLength caps a prompt in characters (runes), as the backend
	// and the webapp's text box do.
	promptMaxLength = 2000
	// maxTeams caps the team list.
	maxTeams = 500
	// draftMaxBytes caps the draft (64 KiB), the size entity-service allows
	// a saved definition: the draft is the definition being edited, so a real
	// one always fits, and it bounds what one call can send the model.
	draftMaxBytes = 64 << 10
)

// Error codes, in the body's "code", for the backend to map to its own
// statuses and messages. The "message" is for logs and people reading the
// response, never model output or user text.
const (
	CodeInvalidRequest  = "invalid_request"
	CodeNotConfigured   = "not_configured"
	CodeRefused         = "refused"
	CodeTruncated       = "truncated"
	CodeUnusable        = "unusable"
	CodeTimeout         = "timeout"
	CodeModelCallFailed = "model_call_failed"
	CodeInternal        = "internal"
)

// dashboardGenerator is dashboards.Generator.
type dashboardGenerator interface {
	Generate(ctx context.Context, req dashboards.Request) (dashboards.Result, error)
}

// DashboardHandler serves POST /ai/dashboards/generate.
type DashboardHandler struct {
	generator dashboardGenerator // nil when no API key is configured
}

// NewDashboardHandler returns the handler; a nil generator answers 503
// not_configured.
func NewDashboardHandler(generator *dashboards.Generator) *DashboardHandler {
	h := &DashboardHandler{}
	if generator != nil {
		h.generator = generator
	}
	return h
}

// generateRequest is the body of POST /ai/dashboards/generate. Draft is the
// dashboard being edited in the definition file format (the backend's
// draftToDefinition), absent or null on a first message. Teams are the teams
// a dashboard can filter by (only those with a group configured).
type generateRequest struct {
	Prompt string          `json:"prompt"`
	Draft  json.RawMessage `json:"draft,omitempty"`
	Teams  []teams.Team    `json:"teams"`
}

// generateResponse is a 200: either the model's decline sentence, or every
// JSON object in its reply, in order.
type generateResponse struct {
	Declined   string            `json:"declined,omitempty"`
	Candidates []json.RawMessage `json:"candidates,omitempty"`
	Usage      usage             `json:"usage"`
}

type usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Generate makes one model call for one chat message:
//
//   - 200 {"candidates": [...]} or {"declined": "<the model's sentence>"}
//   - 400 invalid_request         the body, prompt, draft or teams
//   - 503 not_configured          no API key
//   - 422 refused                 the model refused
//   - 502 truncated / unusable    the reply was cut off / holds no JSON object
//   - 504 timeout                 the model call ran out of time
//   - 502 model_call_failed       any other call failure (a gateway 401/403
//     is this service's key problem, never passed on as the caller's)
func (h *DashboardHandler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.generator == nil {
		writeError(w, http.StatusServiceUnavailable, CodeNotConfigured, "model calls are not configured")
		return
	}
	req, msg := decodeGenerateRequest(w, r)
	if msg != "" {
		slog.InfoContext(ctx, "dashboard generate: invalid request", "reason", msg)
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, msg)
		return
	}
	res, err := h.generator.Generate(ctx, req)
	if err != nil {
		writeGenerateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, generateResponse{
		Declined:   res.Declined,
		Candidates: res.Candidates,
		Usage:      usage{InputTokens: res.InputTokens, OutputTokens: res.OutputTokens},
	})
}

// decodeOne decodes exactly one JSON object with no unknown fields into v; a
// non-empty message says what is wrong.
func decodeOne(body io.Reader, v any) string {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return "body must be a JSON object with the documented fields"
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "body must hold one JSON object"
	}
	return ""
}

// decodeGenerateRequest reads and checks the body; a non-empty message says
// what is wrong (never echoing the user's text).
func decodeGenerateRequest(w http.ResponseWriter, r *http.Request) (dashboards.Request, string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var body generateRequest
	if msg := decodeOne(r.Body, &body); msg != "" {
		return dashboards.Request{}, msg
	}
	prompt := strings.TrimSpace(body.Prompt)
	switch {
	case prompt == "":
		return dashboards.Request{}, "prompt is required"
	case utf8.RuneCountInString(prompt) > promptMaxLength:
		return dashboards.Request{}, "prompt is over 2000 characters"
	}
	var draft json.RawMessage
	if d := strings.TrimSpace(string(body.Draft)); d != "" && d != "null" {
		if !strings.HasPrefix(d, "{") {
			return dashboards.Request{}, "draft must be a JSON object"
		}
		if len(d) > draftMaxBytes {
			return dashboards.Request{}, "draft is over 64 KiB"
		}
		draft = json.RawMessage(d)
	}
	if len(body.Teams) > maxTeams {
		return dashboards.Request{}, "too many teams"
	}
	for _, t := range body.Teams {
		if strings.TrimSpace(t.Key) == "" || strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Field) == "" {
			return dashboards.Request{}, "every team needs a key, a name and a field"
		}
		if t.Field != "creTeam" && t.Field != "sreTeam" {
			return dashboards.Request{}, "a team's field must be creTeam or sreTeam"
		}
		switch t.Family {
		case "", "cre-abt", "cre", "sre-abt", "sre":
		default:
			return dashboards.Request{}, "a team's family must be cre-abt, cre, sre-abt or sre"
		}
		if strings.ContainsAny(t.Key+t.Name+t.Family+t.Field, "|\n\r`") {
			return dashboards.Request{}, "a team's key, name, family or field holds a character the Teams table cannot hold"
		}
	}
	return dashboards.Request{Prompt: prompt, Draft: draft, Teams: body.Teams}, ""
}

// writeGenerateError maps a generator error to a status and code, and logs
// the cause (never the prompt or the model's reply).
func writeGenerateError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, dashboards.ErrRefused):
		slog.InfoContext(ctx, "dashboard generate: model refused")
		writeError(w, http.StatusUnprocessableEntity, CodeRefused, "the model refused the request")
	case errors.Is(err, dashboards.ErrTruncated):
		slog.WarnContext(ctx, "dashboard generate: reply cut off")
		writeError(w, http.StatusBadGateway, CodeTruncated, "the model's reply was cut off")
	case errors.Is(err, dashboards.ErrUnusable):
		slog.WarnContext(ctx, "dashboard generate: unusable reply", "err", err)
		writeError(w, http.StatusBadGateway, CodeUnusable, "the model's reply could not be used")
	case errors.Is(err, dashboards.ErrCall) && isTimeout(err):
		slog.WarnContext(ctx, "dashboard generate: model call timed out", "err", err)
		writeError(w, http.StatusGatewayTimeout, CodeTimeout, "the model call timed out")
	case errors.Is(err, dashboards.ErrCall):
		slog.ErrorContext(ctx, "dashboard generate: model call failed", "err", err)
		writeError(w, http.StatusBadGateway, CodeModelCallFailed, "the model could not be reached")
	default:
		slog.ErrorContext(ctx, "dashboard generate: failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternal, "internal error")
	}
}

// isTimeout reports whether err is a deadline running out: the generator's
// own bound, or a network timeout.
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Code: code, Message: message})
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "err", err)
	}
}
