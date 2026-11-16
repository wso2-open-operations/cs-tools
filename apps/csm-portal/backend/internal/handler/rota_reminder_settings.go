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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityRotaReminderSettingsClient is the entity-service calls the Rota
// Access tab's Rota Reminders card needs.
type entityRotaReminderSettingsClient interface {
	GetRotaReminderSettings(ctx context.Context, family string) ([]byte, error)
	PutRotaReminderSettings(ctx context.Context, family string, body []byte) ([]byte, error)
}

// RotaReminderSettingsHandler proxies a family's rota reminder settings.
// Who may change them -- a CSM admin or the family's rota admin -- is
// entity-service's decision, so this checks only the shape of the request,
// forwards the known fields and the caller's identity.
type RotaReminderSettingsHandler struct {
	entity entityRotaReminderSettingsClient
}

// NewRotaReminderSettingsHandler creates a RotaReminderSettingsHandler.
func NewRotaReminderSettingsHandler(entity entityRotaReminderSettingsClient) *RotaReminderSettingsHandler {
	return &RotaReminderSettingsHandler{entity: entity}
}

// rotaReminderSettingsBody is the card's whole form: only these fields are
// forwarded.
type rotaReminderSettingsBody struct {
	WeekdayEnabled  *bool                     `json:"weekdayEnabled"`
	WeekendEnabled  *bool                     `json:"weekendEnabled"`
	InvitesEnabled  *bool                     `json:"invitesEnabled"`
	WeekendRequired map[string]int            `json:"weekendRequired"`
	Windows         map[string]reminderWindow `json:"windows"`
	// InviteRotations is forwarded as sent, absent (null) included: an older
	// card sends none and entity-service then keeps what is stored, while an
	// empty object is a save of no exceptions. So no omitempty.
	InviteRotations map[string]bool `json:"inviteRotations"`
	CopyTo          []string        `json:"copyTo"`
	// WeekendCopyTo likewise: null from an older card keeps what is stored.
	WeekendCopyTo []string `json:"weekendCopyTo"`
	// InviteCopyTo likewise.
	InviteCopyTo []string `json:"inviteCopyTo"`
}

type reminderWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	TimeZone string `json:"timeZone"`
}

// GetRotaReminderSettings handles GET /team-schedule/reminder-settings/{family}.
// A 404 means nothing was saved yet and is passed on as such: the card then
// shows the reminders' built-in settings.
func (h *RotaReminderSettingsHandler) GetRotaReminderSettings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	family := r.PathValue("family")
	if !rotaFamilyRe.MatchString(family) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	h.forward(w, r, user.UserID, "GetRotaReminderSettings", "Failed to load the rota reminder settings.",
		func(ctx context.Context) ([]byte, error) {
			return h.entity.GetRotaReminderSettings(ctx, strings.ToUpper(family))
		})
}

// PutRotaReminderSettings handles PUT /team-schedule/reminder-settings/{family}.
func (h *RotaReminderSettingsHandler) PutRotaReminderSettings(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	family := r.PathValue("family")
	var in rotaReminderSettingsBody
	if !rotaFamilyRe.MatchString(family) || json.Unmarshal(body, &in) != nil ||
		in.WeekdayEnabled == nil || in.WeekendEnabled == nil || in.InvitesEnabled == nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	h.forward(w, r, userID, "PutRotaReminderSettings", "Failed to save the rota reminder settings.",
		func(ctx context.Context) ([]byte, error) {
			return h.entity.PutRotaReminderSettings(ctx, strings.ToUpper(family), mustJSON(in))
		})
}

// forward makes the call and answers with its result. A refusal keeps
// entity-service's own reason ("only a CSM admin or the CRE rota admin can
// ...", "windows: CRE_WEEKEND_NIGHT: ... is not a time zone") so the card can
// say why; a 404 stays a 404 so the card can tell "nothing saved yet".
func (h *RotaReminderSettingsHandler) forward(w http.ResponseWriter, r *http.Request, callerID, op, fallbackMsg string,
	call func(ctx context.Context) ([]byte, error)) {
	result, err := call(r.Context())
	if err != nil {
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusNotFound:
				writeError(w, http.StatusNotFound, upstreamErrorMessageStrict(apiErr.Body, "No rota reminder settings saved."))
				return
			case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict:
				slog.WarnContext(r.Context(), "entity "+op+" refused", "userID", callerID, "status", apiErr.StatusCode)
				writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
				return
			}
		}
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", callerID, "err", err)
		mapUpstreamErrorGeneric(w, err, fallbackMsg)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// entityRotaReminderRequestClient is the entity-service calls "Send now" on
// the Rota Reminders card needs. The scheduled task's claim and report are not
// proxied: they are for its service credential only.
type entityRotaReminderRequestClient interface {
	ListRotaReminderRequests(ctx context.Context, family string) ([]byte, error)
	CreateRotaReminderRequest(ctx context.Context, family string, body []byte) ([]byte, error)
}

// RotaReminderRequestHandler proxies "Send now": asking for a family's rota
// reminders to be sent at the scheduled task's next run, and the latest
// requests. Who may ask is entity-service's decision.
type RotaReminderRequestHandler struct {
	entity entityRotaReminderRequestClient
}

// NewRotaReminderRequestHandler creates a RotaReminderRequestHandler.
func NewRotaReminderRequestHandler(entity entityRotaReminderRequestClient) *RotaReminderRequestHandler {
	return &RotaReminderRequestHandler{entity: entity}
}

// ListRotaReminderRequests handles GET /team-schedule/reminder-requests/{family}.
func (h *RotaReminderRequestHandler) ListRotaReminderRequests(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	family := r.PathValue("family")
	if !rotaFamilyRe.MatchString(family) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := h.entity.ListRotaReminderRequests(r.Context(), strings.ToUpper(family))
	if err != nil {
		reminderRequestError(w, r, user.UserID, "ListRotaReminderRequests", "Failed to load the rota reminder requests.", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// rotaReminderKinds are what "Send now" may ask for: the Morning (with its
// on-call), the Evening or the weekend reminder, or "weekday", both of the
// next working day's at once (what the tab sent before it offered the two
// apart).
var rotaReminderKinds = map[string]bool{"weekday": true, "morning": true, "evening": true, "weekend": true}

// CreateRotaReminderRequest handles POST /team-schedule/reminder-requests/{family}.
func (h *RotaReminderRequestHandler) CreateRotaReminderRequest(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	family := r.PathValue("family")
	var in struct {
		Kind string `json:"kind"`
		Test bool   `json:"test"`
	}
	if !rotaFamilyRe.MatchString(family) || json.Unmarshal(body, &in) != nil || !rotaReminderKinds[in.Kind] {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := h.entity.CreateRotaReminderRequest(r.Context(), strings.ToUpper(family), mustJSON(in))
	if err != nil {
		reminderRequestError(w, r, userID, "CreateRotaReminderRequest", "Failed to ask for the rota reminders.", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// reminderRequestError keeps entity-service's reason for a refusal ("a
// weekend reminder was asked for less than 10 minutes ago", "only a CSM admin
// or the CRE rota admin can ...") so the card can say why.
func reminderRequestError(w http.ResponseWriter, r *http.Request, userID, op, fallbackMsg string, err error) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusBadRequest, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests:
			slog.WarnContext(r.Context(), "entity "+op+" refused", "userID", userID, "status", apiErr.StatusCode)
			writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
			return
		}
	}
	slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", userID, "err", err)
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}
