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

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-webhooks/github/internal/webhook"
)

const testSecret = "test-secret"

func sign(t *testing.T, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// *** THIS IS THE TEST THAT WOULD HAVE CAUGHT THE TRUNCATION BUG. ***
// The first version read the body through an io.LimitReader, which stops at
// the cap and reports success. An oversized delivery therefore reached the
// HMAC check as partial bytes, failed it, and was answered 401 "invalid
// signature" -- sending anyone who read that log at the secret rather than
// at the size. The assertion that the status is NOT 401 is the whole point;
// asserting only "not 200" would have passed against the bug.
func TestHandle_OversizedBodyIs413NotASignatureFailure(t *testing.T) {
	body := []byte(`{"action":"opened","padding":"` + strings.Repeat("x", maxWebhookBody+1024) + `"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	// A VALID signature over the full body. The request is correct in every
	// way except its size, so a 401 here could only mean the truncation.
	req.Header.Set(webhook.SignatureHeader, sign(t, body))
	req.Header.Set(webhook.DeliveryHeader, "delivery-1")
	req.Header.Set(webhook.EventHeader, "issues")

	rec := httptest.NewRecorder()
	// client is nil: an oversized body must be rejected before anything is
	// forwarded, so reaching the client at all would panic the test.
	handle(testSecret, nil)(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("oversized body reported as a signature failure (401); the body was truncated before verification")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandle_BadSignatureIs401(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	req.Header.Set(webhook.SignatureHeader, sign(t, []byte(`{"action":"closed"}`)))
	req.Header.Set(webhook.DeliveryHeader, "delivery-2")
	req.Header.Set(webhook.EventHeader, "issues")

	rec := httptest.NewRecorder()
	handle(testSecret, nil)(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// A body at exactly the cap must pass. MaxBytesReader errors when the limit
// is EXCEEDED, so an off-by-one in the cap would show up here and nowhere
// else -- the oversize test above passes either way.
func TestHandle_BodyAtTheCapIsNotRejectedForSize(t *testing.T) {
	prefix := `{"action":"opened","padding":"`
	suffix := `"}`
	body := []byte(prefix + strings.Repeat("x", maxWebhookBody-len(prefix)-len(suffix)) + suffix)
	if len(body) != maxWebhookBody {
		t.Fatalf("test body is %d bytes, want exactly %d", len(body), maxWebhookBody)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	// Signature deliberately wrong, so the request stops at the HMAC check
	// and never needs a client. A 401 proves the body was read in full.
	req.Header.Set(webhook.SignatureHeader, sign(t, []byte("something else")))
	req.Header.Set(webhook.DeliveryHeader, "delivery-3")
	req.Header.Set(webhook.EventHeader, "issues")

	rec := httptest.NewRecorder()
	handle(testSecret, nil)(rec, req)

	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("a body of exactly %d bytes was rejected as too large", maxWebhookBody)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
