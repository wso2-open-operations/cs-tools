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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// AppConfigHandler serves small, static values resolved once at startup from
// an env var, for chrome the webapp shows that has no page/resource of its
// own to carry the value — today just the header's "Report an issue" link.
type AppConfigHandler struct {
	reportIssueURL string
}

// NewAppConfigHandler creates an AppConfigHandler backed by the given
// report-issue URL, resolved once at startup from CSM_PORTAL_REPORT_ISSUE_URL
// (see main.go's loadReportIssueURL).
func NewAppConfigHandler(reportIssueURL string) *AppConfigHandler {
	return &AppConfigHandler{reportIssueURL: reportIssueURL}
}

// appConfigResponse is GetAppConfig's response shape — a small, growing set
// of static, env-var-configured values the webapp reads once. A future
// second value is a new field here, not a new route.
type appConfigResponse struct {
	// ReportIssueURL is "" when CSM_PORTAL_REPORT_ISSUE_URL is unset — never
	// null, so callers can read it directly with no nil-check. The webapp
	// hides its "Report an issue" control when this is empty.
	ReportIssueURL string `json:"reportIssueUrl"`
}

// GetAppConfig handles GET /app-config.
//
// Registered on PermAuthenticated, not a portal-role permission: the header
// control this backs is shown to every signed-in user, including one not yet
// granted a portal role (AuthGuard's "not yet entitled" screen in the
// webapp) — requiring a portal role here would 403 exactly the audience that
// screen exists for.
func (h *AppConfigHandler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	writeJSONValue(w, http.StatusOK, appConfigResponse{ReportIssueURL: h.reportIssueURL})
}
