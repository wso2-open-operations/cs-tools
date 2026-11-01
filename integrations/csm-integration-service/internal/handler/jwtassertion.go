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
	"encoding/base64"
	"encoding/json"
	"strings"
)

// jwtAssertionHeader is the header Choreo's API Manager gateway forwards
// naming which application it already authenticated (subscription + client
// credentials) before the request ever reached this service. See
// CaseHandler.clientActorEmails' own doc comment for what this is used for.
const jwtAssertionHeader = "x-jwt-assertion"

type jwtAssertionClaims struct {
	ClientID string `json:"client_id"`
	AZP      string `json:"azp"`
}

// clientIDFromJWTAssertion reads the client id (client_id claim, falling
// back to azp) out of raw -- a client-credentials assertion -- WITHOUT
// verifying its signature, issuer, or expiry. This is a plain decode, not a
// validation, mirroring entity-service's own auth.Validator.ExtractClientID
// and for the identical reason: the Choreo gateway in front of this service
// has already authenticated the calling application by whatever means it
// uses, over a path this service already trusts -- see this package's own
// "Why no Auth middleware" doc comment (cases.go). The value is used only to
// look up a configured display email in CaseHandler.clientActorEmails, never
// to grant access or bypass any check, so trusting it at face value here
// carries no more risk than entity-service's own identical decode of the
// same kind of header.
//
// Returns "" for a missing header, a malformed token, or a token with
// neither claim -- every caller treats that the same way: skip the
// enrichment and forward the request unchanged.
func clientIDFromJWTAssertion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var c jwtAssertionClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return ""
	}
	if id := strings.TrimSpace(c.ClientID); id != "" {
		return id
	}
	return strings.TrimSpace(c.AZP)
}
