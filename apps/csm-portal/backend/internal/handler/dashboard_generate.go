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
	errMsgGenerateTruncated    = "The dashboard was too large to create in one go. Ask for fewer widgets, or build it in steps."
	errMsgGenerateTimeout      = "The AI assistant took too long to answer. Try again."
	errMsgGenerateCall         = "The AI assistant can't be reached right now. Try again in a few minutes."
	errMsgGenerateUsageLimit   = "The AI assistant has reached its usage limit. Try again later."
)

// The errorCode on each message above, and on the prompt and rate limit
// messages. Every one starts with "generate_": the webapp shows a generate
// failure's message only when its errorCode does, so a proxy's or gateway's
// own JSON body (which can also carry a "message") is never shown as ours. A
// 500 has none; the webapp words it itself.
const (
	errCodeGenerateUnavailable   = "generate_unavailable"
	errCodeGeneratePromptEmpty   = "generate_prompt_empty"
	errCodeGeneratePromptTooLong = "generate_prompt_too_long"
	errCodeGenerateRateLimited   = "generate_rate_limited"
	errCodeGenerateDraftInvalid  = "generate_draft_invalid"
	errCodeGenerateDraftTooBig   = "generate_draft_too_large"
	errCodeGenerateDeclined      = "generate_declined"
	errCodeGenerateTeam          = "generate_team"
	errCodeGeneratePlaybook      = "generate_playbook"
	errCodeGenerateRefused       = "generate_refused"
	errCodeGenerateUnusable      = "generate_unusable"
	errCodeGenerateProduct       = "generate_product"
	errCodeGenerateProductLookup = "generate_product_lookup"
	errCodeGenerateProject       = "generate_project"
	errCodeGenerateProjectLookup = "generate_project_lookup"
	errCodeGenerateAccount       = "generate_account"
	errCodeGenerateAccountLookup = "generate_account_lookup"
	errCodeGenerateRegion        = "generate_region"
	errCodeGenerateRegionLookup  = "generate_region_lookup"
	errCodeGenerateTruncated     = "generate_truncated"
	errCodeGenerateUsageLimit    = "generate_usage_limit"
	errCodeGenerateTimeout       = "generate_timeout"
	errCodeGenerateCall          = "generate_call_failed"
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
//   - errDashboardPlaybook (a playbook answer        422, the reason (an unknown
//     filter no active playbook has)                    task or answer, the known ones)
//   - errGenerateRefused                             422
//   - errGenerateUnusable                            502
//   - errDashboardProduct (a product filter naming  422, the reason (no product,
//     no product or several, or a version the           the candidates, or the
//     product does not have: productResolver)           product's versions)
//   - errDashboardProductLookup                      502
//   - errDashboardProject (a project filter naming  422, the reason (no project,
//     no project or several: namedResolver)             or the candidates)
//   - errDashboardProjectLookup                      502
//   - errDashboardAccount (an account filter naming 422, the reason (no account,
//     no account or several: namedResolver)             or the candidates)
//   - errDashboardAccountLookup                      502
//   - errDashboardRegion (a region filter naming a  422, the reason (the
//     region no case's account has: regionResolver)     regions there are)
//   - errDashboardRegionLookup                       502
//   - errGenerateTruncated (the reply was cut off)   502, its own message
//   - errGenerateUsageLimit (the AI side's limit,    503 (429 stays this
//     not the per-user one)                              backend's own limit)
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
		writeErrorCode(w, http.StatusServiceUnavailable, errMsgGenerateUnavailable, errCodeGenerateUnavailable)
	case errors.Is(err, errDraftInvalid):
		slog.InfoContext(ctx, "dashboard generate: invalid draft", "err", err)
		writeErrorCode(w, http.StatusBadRequest, errMsgGenerateDraftInvalid, errCodeGenerateDraftInvalid)
	case errors.Is(err, errDraftTooLarge):
		slog.InfoContext(ctx, "dashboard generate: draft too large", "err", err)
		writeErrorCode(w, http.StatusRequestEntityTooLarge, errMsgGenerateDraftTooBig, errCodeGenerateDraftTooBig)
	case errors.As(err, &declined):
		slog.InfoContext(ctx, "dashboard generate: model declined")
		writeErrorCode(w, http.StatusUnprocessableEntity, declined.Reason, errCodeGenerateDeclined)
	case errors.Is(err, errDashboardTeam):
		slog.InfoContext(ctx, "dashboard generate: unusable team filter", "err", err)
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGenerateTeam)
	case errors.Is(err, errDashboardPlaybook):
		slog.InfoContext(ctx, "dashboard generate: unusable playbook answer filter", "err", err)
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGeneratePlaybook)
	case errors.Is(err, errGenerateRefused):
		slog.InfoContext(ctx, "dashboard generate: model refused")
		writeErrorCode(w, http.StatusUnprocessableEntity, errMsgGenerateRefused, errCodeGenerateRefused)
	case errors.Is(err, errDashboardProduct):
		// The reason names what the user typed and the records it matched
		// (a customer's name, say): the caller gets it, the log does not.
		slog.InfoContext(ctx, "dashboard generate: unusable product filter")
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGenerateProduct)
	case errors.Is(err, errDashboardProductLookup):
		slog.WarnContext(ctx, "dashboard generate: product lookup failed", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateProductLookup, errCodeGenerateProductLookup)
	case errors.Is(err, errDashboardProject):
		slog.InfoContext(ctx, "dashboard generate: unusable project filter")
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGenerateProject)
	case errors.Is(err, errDashboardProjectLookup):
		slog.WarnContext(ctx, "dashboard generate: project lookup failed", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateProjectLookup, errCodeGenerateProjectLookup)
	case errors.Is(err, errDashboardAccount):
		slog.InfoContext(ctx, "dashboard generate: unusable account filter")
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGenerateAccount)
	case errors.Is(err, errDashboardAccountLookup):
		slog.WarnContext(ctx, "dashboard generate: account lookup failed", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateAccountLookup, errCodeGenerateAccountLookup)
	case errors.Is(err, errDashboardRegion):
		slog.InfoContext(ctx, "dashboard generate: unusable region filter")
		writeErrorCode(w, http.StatusUnprocessableEntity, dashboardTeamMessage(err), errCodeGenerateRegion)
	case errors.Is(err, errDashboardRegionLookup):
		slog.WarnContext(ctx, "dashboard generate: region lookup failed", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateRegionLookup, errCodeGenerateRegionLookup)
	case errors.Is(err, errGenerateTruncated):
		slog.WarnContext(ctx, "dashboard generate: model reply cut off", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateTruncated, errCodeGenerateTruncated)
	case errors.Is(err, errGenerateUnusable):
		slog.WarnContext(ctx, "dashboard generate: unusable model reply", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateUnusable, errCodeGenerateUnusable)
	case errors.Is(err, errGenerateUsageLimit):
		slog.WarnContext(ctx, "dashboard generate: model usage limit reached", "err", err)
		writeErrorCode(w, http.StatusServiceUnavailable, errMsgGenerateUsageLimit, errCodeGenerateUsageLimit)
	case errors.Is(err, errGenerateCall) && isTimeout(err):
		slog.WarnContext(ctx, "dashboard generate: model call timed out", "err", err)
		writeErrorCode(w, http.StatusGatewayTimeout, errMsgGenerateTimeout, errCodeGenerateTimeout)
	case errors.Is(err, errGenerateCall):
		slog.ErrorContext(ctx, "dashboard generate: model call failed", "err", err)
		writeErrorCode(w, http.StatusBadGateway, errMsgGenerateCall, errCodeGenerateCall)
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
	writeErrorCode(w, http.StatusTooManyRequests, msg, errCodeGenerateRateLimited)
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
		writeErrorCode(w, http.StatusBadRequest, errMsgGeneratePromptEmpty, errCodeGeneratePromptEmpty)
		return
	}
	if utf8.RuneCountInString(prompt) > generatePromptMaxLength {
		writeErrorCode(w, http.StatusBadRequest, errMsgGeneratePromptTooLong, errCodeGeneratePromptTooLong)
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
