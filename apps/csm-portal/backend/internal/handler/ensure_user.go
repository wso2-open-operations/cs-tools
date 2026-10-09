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

// entityUserProvisioningClient is the narrow entity-service surface
// ensureUserProvisioned needs: resolve the caller's own platform user record,
// and create one if it doesn't exist yet.
type entityUserProvisioningClient interface {
	GetUserMe(ctx context.Context) ([]byte, error)
	CreateUser(ctx context.Context, body []byte) ([]byte, error)
}

// ensureUserProvisioned makes sure the caller has a "user" row in
// entity-service before a write that needs one to attribute itself to.
//
// A worknote_creator- or escalator-only caller reaches POST
// /cases/{id}/comments or POST /cases/{id}/escalations purely on the
// strength of an Asgardeo role grant (AUTH_WORKNOTE_CREATOR_ROLES /
// AUTH_ESCALATOR_ROLES) -- unlike the admin-only "Add User" flow, holding
// one of those roles never provisions a "user" row anywhere. Without one,
// entity-service's own identity resolution for the write (emailFromJWT ->
// GetUserByEmail) fails it outright. Call this immediately before such a
// write so a first-time worknote/escalation author is provisioned on demand
// instead of failing with no path forward.
//
// A cs_engineer or admin is assumed already provisioned -- call sites skip
// invoking this for them entirely (see hasFullWrite in CreateCaseComment) to
// avoid an extra round trip on the overwhelmingly common path. CreateCaseEscalation
// has no equivalent skip: a cs_engineer now holds PermEscalate too, but escalating
// is rare enough that calling this for every escalator costs nothing worth saving.
//
// FirstName/LastName/Email come straight off the caller's own validated token
// (middleware.UserInfo) -- never client-supplied. The created user is always
// granted the "internal" role: both work-note creation and escalation are
// internal-staff-only actions (their own AUTH_<ROLE>_ROLES grants are
// organisation-internal role names), so there is no "internal vs external"
// ambiguity to resolve the way CreateUser's own admin-facing "User type"
// selector has to.
//
// entity-service's own userService.CreateUser rejects a request whose
// firstName and lastName are BOTH blank with a 400 ValidationError -- a real
// risk here, since given_name/family_name are optional token claims (see
// UserInfo's own doc comment) that can legitimately be absent depending on
// the IdP's configured scopes. Falling into that 400 and then swallowing it
// (this function is best-effort) would leave the caller's write to fail with
// no path forward, exactly the gap this function exists to close. When both
// are blank, the email's local part is used as LastName instead -- a real,
// non-empty value entity-service accepts, not a fabricated name.
//
// Best-effort: a failure here is logged and otherwise swallowed. The write
// this precedes fails on its own terms immediately afterward if the user
// genuinely still doesn't exist -- the same failure mode as before this
// check existed -- and a transient GetUserMe/CreateUser error must not block
// a write that might otherwise have succeeded.
func ensureUserProvisioned(ctx context.Context, entity entityUserProvisioningClient, user *middleware.UserInfo) {
	_, err := entity.GetUserMe(ctx)
	if err == nil {
		return // already provisioned
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		slog.ErrorContext(ctx, "ensureUserProvisioned: entity GetUserMe failed", "userID", user.UserID, "err", err)
		return
	}

	firstName, lastName := user.FirstName, user.LastName
	if strings.TrimSpace(firstName) == "" && strings.TrimSpace(lastName) == "" {
		lastName = emailLocalPart(user.Email)
	}

	body, err := json.Marshal(struct {
		FirstName string   `json:"firstName"`
		LastName  string   `json:"lastName"`
		Email     string   `json:"email"`
		Roles     []string `json:"roles"`
	}{
		FirstName: firstName,
		LastName:  lastName,
		Email:     user.Email,
		Roles:     []string{"internal"},
	})
	if err != nil {
		slog.ErrorContext(ctx, "ensureUserProvisioned: marshal create-user request failed", "userID", user.UserID, "err", err)
		return
	}

	if _, err := entity.CreateUser(ctx, body); err != nil {
		slog.ErrorContext(ctx, "ensureUserProvisioned: entity CreateUser failed", "userID", user.UserID, "err", err)
	}
}

// emailLocalPart returns the portion of email before "@", or email unchanged
// if it carries none -- used only as ensureUserProvisioned's last-resort name
// fallback, never as a validated/canonical form of the address.
func emailLocalPart(email string) string {
	if i := strings.IndexByte(email, '@'); i >= 0 {
		return email[:i]
	}
	return email
}
