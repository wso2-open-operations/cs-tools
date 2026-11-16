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
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// generateCodeDashboardRequest is the body of POST /dashboards/generate-code.
//
// Prompt is the user's latest chat message, checked exactly as on
// POST /dashboards/generate. Current is the source of the dashboard being
// edited, if any (the previous reply's source), at most
// generatedCodeMaxLength characters; it is sent to the model with the prompt.
type generateCodeDashboardRequest struct {
	Prompt  string `json:"prompt"`
	Current string `json:"current,omitempty"`
}

// generateCodeDashboardResponse is the reply: the dashboard's source code, a
// chat message and the name the model suggests saving it under (left out
// when there is none).
type generateCodeDashboardResponse struct {
	Source string `json:"source"`
	Reply  string `json:"reply"`
	Name   string `json:"name,omitempty"`
}

// suggestedCodeDashboardName is the generator's suggested name, trimmed, or
// "" when it is over customDashboardNameMaxLength characters (the saved-name
// limit, as on PUT /dashboards/code/{id}): a suggestion the Save dialog could
// not save is no suggestion.
func suggestedCodeDashboardName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > customDashboardNameMaxLength {
		return ""
	}
	return name
}

// DashboardGenerateCodeHandler serves POST /dashboards/generate-code, the
// code approach's counterpart of POST /dashboards/generate (phase 2, off
// unless DASHBOARD_GENERATE_CODE_ENABLED is "true"): a chat message is turned
// into dashboard source code instead of a dashboard definition. It checks the
// request and asks its CodeDashboardGenerator (the AI integration service, or
// the sample when it is not configured; decided in main.go). It shares the
// prompt checks, the per-user limit (WithRateLimit, the same limiter as
// POST /dashboards/generate) and writeGenerateError's statuses with
// DashboardGenerateHandler, and changes nothing on that path. The source is
// not checked beyond defining a render function (the AI integration service
// does that): it runs only in the webapp's sandboxed frame, where its data
// comes through the fixed portal APIs with the viewer's own permissions.
// Nothing is stored.
type DashboardGenerateCodeHandler struct {
	generator CodeDashboardGenerator
	limiter   *GenerateRateLimiter
}

// NewDashboardGenerateCodeHandler returns the handler using generator.
func NewDashboardGenerateCodeHandler(generator CodeDashboardGenerator) *DashboardGenerateCodeHandler {
	return &DashboardGenerateCodeHandler{generator: generator}
}

// WithRateLimit returns the handler with limiter applied to every valid
// request, before the generator is asked.
func (h *DashboardGenerateCodeHandler) WithRateLimit(limiter *GenerateRateLimiter) *DashboardGenerateCodeHandler {
	c := *h
	c.limiter = limiter
	return &c
}

// GenerateCodeDashboard handles POST /dashboards/generate-code.
func (h *DashboardGenerateCodeHandler) GenerateCodeDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	var req generateCodeDashboardRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	// Checked before any work, so a refused request never reaches the model.
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeErrorCode(w, http.StatusBadRequest, errMsgGeneratePromptEmpty, errCodeGeneratePromptEmpty)
		return
	}
	if utf8.RuneCountInString(prompt) > generatePromptMaxLength {
		writeErrorCode(w, http.StatusBadRequest, errMsgGeneratePromptTooLong, errCodeGeneratePromptTooLong)
		return
	}
	if n := utf8.RuneCountInString(req.Current); n > generatedCodeMaxLength {
		writeGenerateError(w, r, fmt.Errorf("%w: %d characters of code", errDraftTooLarge, n))
		return
	}
	if h.limiter != nil {
		key := user.UserID
		if key == "" {
			key = strings.ToLower(user.Email)
		}
		if ok, retryAfter, daily := h.limiter.Allow(key); !ok {
			writeGenerateRateLimited(w, r, retryAfter, daily)
			return
		}
	}

	result, err := h.generator.Generate(r.Context(), prompt, req.Current)
	if err != nil {
		writeGenerateError(w, r, err)
		return
	}
	writeJSONValue(w, http.StatusOK, generateCodeDashboardResponse{
		Source: result.Source,
		Reply:  result.Reply,
		Name:   suggestedCodeDashboardName(result.Name),
	})
}
