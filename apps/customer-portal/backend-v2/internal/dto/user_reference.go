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
