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

package dto

import (
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// userRefIdentity flattens entity-service's person reference to the identity
// string the portal contract exposes as createdBy.
//
// Email, not name: the frontend treats createdBy as an identity, not a label —
// isNoveraOrBotSender compares it against "novera" to decide whether a comment
// came from the assistant, and the Ballerina backend sent the email here too
// ("jane.doe@example.com", "system"). Using the display name would silently break
// bot attribution.
//
// Falls back to the name only when there is no email, so an unresolved author
// still renders as something rather than vanishing.
func userRefIdentity(u *entity.UserReference) string {
	if u == nil {
		return ""
	}
	if e := strings.TrimSpace(u.Email); e != "" {
		return e
	}
	return strings.TrimSpace(u.Name)
}

// userRefDisplayName flattens the same reference to a human-readable name, for
// the portal's createdByFullName.
//
// entity-service used to send createdByFullName as its own field and removed it
// in the same change that made createdBy an object, so the name now has to come
// from the reference. Returns "" when the upstream could not resolve the person,
// which keeps the field omitted rather than showing an email where a name belongs.
func userRefDisplayName(u *entity.UserReference) string {
	if u == nil {
		return ""
	}
	return strings.TrimSpace(u.Name)
}

// systemAuthorLabel is what the old customer-portal backend's entity-service
// literally sent as the string createdBy for automation/integration-authored
// activity (an activity with no real, resolvable author) before it switched to
// a per-field UserReference object. Case activities keep that same label so
// existing consumers (and any UI branching on it) don't regress.
const systemAuthorLabel = "system"

// userRefDisplayNameOrSystem is userRefDisplayName with a fallback: when the
// reference does not resolve to anyone (missing entirely, or present but with
// neither a name nor an email), or when it resolves to the platform's own
// automation account, it returns the "system" sentinel instead of an empty
// string.
//
// An empty createdBy is indistinguishable from a genuinely unknown author, so
// downstream consumers rendered "Unknown" for what used to render as "System".
// A resolved-but-nameless real account (email set, name blank) is left to
// userRefDisplayName's existing empty-string behavior — it is a person, not an
// automation actor, even though we cannot label them.
//
// The automation account has no real user record to resolve against, so the
// upstream data source reuses the "system" literal in the identity's email
// slot as the only signal that the actor is its own automation rather than an
// unresolvable person. That literal is the one confirmed shape; this check
// does not generalize to other unresolved-author patterns.
func userRefDisplayNameOrSystem(u *entity.UserReference) string {
	if u == nil {
		return systemAuthorLabel
	}
	if strings.TrimSpace(u.Name) == "" && strings.TrimSpace(u.Email) == "" {
		return systemAuthorLabel
	}
	if strings.EqualFold(strings.TrimSpace(u.Email), systemAuthorLabel) {
		return systemAuthorLabel
	}
	return userRefDisplayName(u)
}
