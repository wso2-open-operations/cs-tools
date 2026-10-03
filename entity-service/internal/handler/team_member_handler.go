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
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// TeamMemberHandler serves the team-membership read the call-escalation ladder
// resolves its rungs from.
type TeamMemberHandler struct {
	svc service.TeamMemberService
}

func NewTeamMemberHandler(svc service.TeamMemberService) *TeamMemberHandler {
	return &TeamMemberHandler{svc: svc}
}

// GetTeamMembers handles
// GET /team-schedule/members?teamKeys=a,b[&roles=sub_lead,lead].
//
// A GET with query parameters rather than a POST /search: every other read in
// this module is shaped that way (on-duty, catalogue, my-lead-teams), the
// lookup has three scalar filters and no body worth speaking of, and the
// escalation resolver calls it once per rung on a hot path where a cacheable
// GET is worth having.
func (h *TeamMemberHandler) GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.MembersByTeamKeys(
		r.Context(),
		splitCommaList(r.URL.Query().Get("teamKeys")),
		splitCommaList(r.URL.Query().Get("roles")),
		// alertTiers narrows to the alert-duty nominees (T1/T2/T3), which is
		// what the ladder's first rung asks for. Empty means "any", so an
		// existing caller is unaffected.
		splitCommaList(r.URL.Query().Get("alertTiers")),
		// teamTypes selects whole ABTs -- cre-abt holds seven teams, sre-abt
		// two -- so a caller asking for "every team in this ABT" does not
		// carry a list of keys that goes stale when a team is added.
		splitCommaList(r.URL.Query().Get("teamTypes")),
	)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// splitCommaList turns "a,b" into two values and "" into none. The service
// trims and rejects what is left, so this stays a split and nothing more.
func splitCommaList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
