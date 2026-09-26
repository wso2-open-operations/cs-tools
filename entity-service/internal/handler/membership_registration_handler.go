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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// MembershipRegistrationHandler handles POST /users/me/memberships/register.
type MembershipRegistrationHandler struct {
	svc service.MembershipRegistrationService
}

// NewMembershipRegistrationHandler constructs a MembershipRegistrationHandler with the given service.
func NewMembershipRegistrationHandler(svc service.MembershipRegistrationService) *MembershipRegistrationHandler {
	return &MembershipRegistrationHandler{svc: svc}
}

// RegisterInvitedMemberships handles POST /users/me/memberships/register for
// the postgres data source. The request has no body and the response has none
// either: there is nothing to report beyond "handled", and the common case
// did nothing at all.
func (h *MembershipRegistrationHandler) RegisterInvitedMemberships(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RegisterInvitedMemberships(r.Context()); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
