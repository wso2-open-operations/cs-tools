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

package paging

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// LogID is how a log line names a recipient, under the key "recipientId":
// their CSM user id when the Team Schedule gave one, else "e:" and the first
// twelve hex digits of the SHA-256 of their lower-cased email (a head or lead
// named in configuration has no user id), else "unknown".
//
// Never the name, the email or the number -- this service logs ids only (see
// the Security section of CLAUDE.md). The email hash is a stable handle for
// following one person across log lines, not anonymisation: anyone holding
// the email can recompute it, which is also how a reader matches it to a
// person.
func (r Recipient) LogID() string {
	if id := strings.TrimSpace(r.UserID); id != "" {
		return id
	}
	email := strings.ToLower(strings.TrimSpace(r.Email))
	if email == "" {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(email))
	return "e:" + hex.EncodeToString(sum[:6])
}
