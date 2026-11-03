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
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// errMsgGenerateUnavailable is the 503 when the generator cannot answer at all
// (ErrGenerateUnavailable, e.g. the sample is not loaded), so the caller can
// tell it apart from a failed attempt.
const errMsgGenerateUnavailable = "Dashboard generation is not available right now."

// generatePromptMaxLength caps a prompt, counted in characters (runes), not
// bytes, so non-Latin text is not cut off early. Matches the webapp's text box
// (GENERATE_PROMPT_MAX_LENGTH in GenerateChatPanel.tsx), so a normal user
// never reaches it; it guards direct API calls before any model is called.
const generatePromptMaxLength = 2000

// Messages for a refused prompt, written to be shown to the user as they are.
const (
	errMsgGeneratePromptEmpty   = "Describe the dashboard you'd like."
	errMsgGeneratePromptTooLong = "The description is too long: keep it under 2,000 characters."
)

// generateDashboardRequest is the body of POST /dashboards/generate.
//
// Prompt is the user's latest chat message: required, trimmed, at most
// generatePromptMaxLength characters. Current is the dashboard being edited,
// if any (the previous reply's dashboard), so a follow-up message can change
// it. Both are passed to the DashboardGenerator.
type generateDashboardRequest struct {
	Prompt  string          `json:"prompt"`
	Current json.RawMessage `json:"current,omitempty"`
}

// Messages for a generation that did not give a dashboard. Shown to the user
// as they are; none carries upstream or model detail except the model's own
// decline sentence (generateDeclinedError), which is about the request.
const (
	errMsgGenerateDraftInvalid = "The dashboard being edited could not be read. Start a new one."
	errMsgGenerateDraftTooBig  = "The dashboard being edited is too large to change in the chat. Start a new one."
	errMsgGenerateRefused      = "This request can't be turned into a dashboard. Try rephrasing it."
	errMsgGenerateUnusable     = "The answer couldn't be turned into a dashboard. Try again, or rephrase the request."
	errMsgGenerateTimeout      = "Generating the dashboard took too long. Try again."
	errMsgGenerateCall         = "The dashboard model couldn't be reached. Try again in a moment."
)

// writeGenerateError maps a generator error to a status and message, and
// logs the cause (never the prompt or the model's reply: user text):
//
//   - ErrGenerateUnavailable                         503
//   - errDraftInvalid (the request's "current")      400
//   - errDraftTooLarge (over generateDraftMaxBytes)  413
//   - generateDeclinedError (the model's {"error"})  422, the model's sentence
//   - errDashboardTeam (a team filter that cannot   422, the reason (an unknown
//     work: see dashboardTeams)                         team, the wrong field, ...)
//   - errGenerateRefused                             422
//   - errGenerateUnusable, errGenerateTruncated      502
//   - errGenerateCall, timed out                     504
//   - errGenerateCall, otherwise                     502 (a gateway 401/403 is
//     this backend's key problem, never passed on as the caller's)
//   - anything else                                  500
func writeGenerateError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	var declined *generateDeclinedError
	switch {
	case errors.Is(err, ErrGenerateUnavailable):
		slog.WarnContext(ctx, "dashboard generate: generator unavailable", "err", err)
		writeError(w, http.StatusServiceUnavailable, errMsgGenerateUnavailable)
	case errors.Is(err, errDraftInvalid):
		slog.InfoContext(ctx, "dashboard generate: invalid draft", "err", err)
		writeError(w, http.StatusBadRequest, errMsgGenerateDraftInvalid)
	case errors.Is(err, errDraftTooLarge):
		slog.InfoContext(ctx, "dashboard generate: draft too large", "err", err)
		writeError(w, http.StatusRequestEntityTooLarge, errMsgGenerateDraftTooBig)
	case errors.As(err, &declined):
		slog.InfoContext(ctx, "dashboard generate: model declined")
		writeError(w, http.StatusUnprocessableEntity, declined.Reason)
	case errors.Is(err, errDashboardTeam):
		slog.InfoContext(ctx, "dashboard generate: unusable team filter", "err", err)
		writeError(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err))
	case errors.Is(err, errGenerateRefused):
		slog.InfoContext(ctx, "dashboard generate: model refused")
		writeError(w, http.StatusUnprocessableEntity, errMsgGenerateRefused)
	case errors.Is(err, errGenerateUnusable), errors.Is(err, errGenerateTruncated):
		slog.WarnContext(ctx, "dashboard generate: unusable model reply", "err", err)
		writeError(w, http.StatusBadGateway, errMsgGenerateUnusable)
	case errors.Is(err, errGenerateCall) && isTimeout(err):
		slog.WarnContext(ctx, "dashboard generate: model call timed out", "err", err)
		writeError(w, http.StatusGatewayTimeout, errMsgGenerateTimeout)
	case errors.Is(err, errGenerateCall):
		slog.ErrorContext(ctx, "dashboard generate: model call failed", "err", err)
		writeError(w, http.StatusBadGateway, errMsgGenerateCall)
	default:
		slog.ErrorContext(ctx, "dashboard generate: generator failed", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
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

// generatedDashboardID is the id of the dashboard a reply returns: the
// draft's own id when it is a UUID, so a follow-up message edits the same
// dashboard, else a new random (version 4) UUID. The id is the backend's to
// give, never the generator's: whatever id a generator (the model) put on its
// dashboard is replaced. A draft that does not decode just gets a new id.
func generatedDashboardID(current json.RawMessage) (string, error) {
	var draft struct {
		ID string `json:"id"`
	}
	if len(current) > 0 && json.Unmarshal(current, &draft) == nil && uuidRe.MatchString(draft.ID) {
		return strings.ToLower(draft.ID), nil
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// generateDashboardResponse is the reply: the dashboard to preview, in the
// same shape as GET /dashboards/{dashboardId}, and a chat message.
type generateDashboardResponse struct {
	Dashboard dashboardDetailView `json:"dashboard"`
	Reply     string              `json:"reply"`
}

// DashboardGenerateHandler serves POST /dashboards/generate, which turns a
// chat message into a dashboard definition for the Custom Dashboards
// generate page.
//
// It checks the request, asks its DashboardGenerator for a dashboard, and
// converts the result with the same widgetViews GET /dashboards/{dashboardId}
// uses, so the preview matches the dashboard everywhere else. Which generator
// answers (the sample today) is decided in main.go. The dashboard's id is set
// here (generatedDashboardID), not by the generator. Nothing is stored. With a
// GenerateRateLimiter (WithRateLimit), a user over its limits gets a 429
// before the generator is asked.
type DashboardGenerateHandler struct {
	generator DashboardGenerator
	limiter   *GenerateRateLimiter
}

// NewDashboardGenerateHandler returns the handler using generator.
func NewDashboardGenerateHandler(generator DashboardGenerator) *DashboardGenerateHandler {
	return &DashboardGenerateHandler{generator: generator}
}

// WithRateLimit returns the handler with limiter applied to every valid
// request, before the generator is asked.
func (h *DashboardGenerateHandler) WithRateLimit(limiter *GenerateRateLimiter) *DashboardGenerateHandler {
	c := *h
	c.limiter = limiter
	return &c
}

// writeGenerateRateLimited is the 429 for a user over a GenerateRateLimiter
// limit, with Retry-After in whole seconds.
func writeGenerateRateLimited(w http.ResponseWriter, r *http.Request, retryAfter time.Duration, daily bool) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	slog.InfoContext(r.Context(), "dashboard generate: rate limited", "daily", daily, "retryAfterSeconds", secs)
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	msg := fmt.Sprintf("You've sent a lot of messages in a short time. Try again in %d seconds.", secs)
	if daily {
		msg = "You've reached the limit of dashboard generations for the last 24 hours. Try again later."
	}
	writeError(w, http.StatusTooManyRequests, msg)
}

// GenerateDashboard handles POST /dashboards/generate.
func (h *DashboardGenerateHandler) GenerateDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	var req generateDashboardRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	// Checked before any work, so a refused prompt never reaches the model.
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, errMsgGeneratePromptEmpty)
		return
	}
	if utf8.RuneCountInString(prompt) > generatePromptMaxLength {
		writeError(w, http.StatusBadRequest, errMsgGeneratePromptTooLong)
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

	d := result.Dashboard
	d.ID, err = generatedDashboardID(req.Current)
	if err != nil {
		slog.ErrorContext(r.Context(), "dashboard generate: new dashboard id", "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSONValue(w, http.StatusOK, generateDashboardResponse{
		Dashboard: dashboardDetailView{
			ID:          d.ID,
			DisplayName: d.DisplayName,
			Type:        d.Type,
			IsDefault:   d.IsDefault,
			TargetTeam:  d.TargetTeam,
			IsTeamBased: d.IsTeamBased,
			Widgets:     widgetViews(d.Widgets),
		},
		Reply: result.Reply,
	})
}
