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

package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrInvalidSignature is returned for any signature that does not verify --
// missing, malformed or wrong.
//
// ONE ERROR FOR EVERY FAILURE, ON PURPOSE. A caller must not be able to learn
// which of those it was, and must never be told what the expected signature
// would have been. ServiceNow's equivalent returned the computed HMAC in the
// 401 body, which let anyone who could reach the endpoint sign arbitrary
// payloads.
var ErrInvalidSignature = errors.New("github: invalid webhook signature")

// SignatureHeader is the header GitHub signs with, SHA-256.
//
// GitHub also sends the older X-Hub-Signature (SHA-1). This verifies the
// SHA-256 one only: accepting SHA-1 would let a caller choose the weaker
// algorithm, and GitHub has sent both for years.
const SignatureHeader = "X-Hub-Signature-256"

// DeliveryHeader uniquely identifies a delivery. GitHub reuses it across
// redeliveries of the same event, which makes it the natural idempotency key
// for anything that must not be applied twice.
const DeliveryHeader = "X-GitHub-Delivery"

// EventHeader names the event type (issues, issue_comment, ...).
const EventHeader = "X-GitHub-Event"

// VerifySignature checks a webhook body against the shared secret.
//
// Constant-time comparison: a byte-by-byte compare leaks, through timing, how
// much of a guessed signature was correct, which turns forgery into a search
// rather than a brute force.
func VerifySignature(secret string, body []byte, header string) error {
	if secret == "" {
		// Refuse rather than skip. A deployment with no secret configured
		// would otherwise accept anything that reached the endpoint.
		return errors.New("github: webhook secret is not configured")
	}
	if header == "" {
		return ErrInvalidSignature
	}

	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return ErrInvalidSignature
	}
	sent, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(sent, mac.Sum(nil)) {
		return ErrInvalidSignature
	}
	return nil
}
