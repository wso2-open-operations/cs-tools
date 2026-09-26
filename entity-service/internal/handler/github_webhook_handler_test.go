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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/github"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const ghSecret = "webhook-secret"

type fakeSync struct {
	called  int
	outcome service.Outcome
	err     error
}

func (f *fakeSync) CreateServiceRequestFromIssue(context.Context, domain.CreateServiceRequestFromIssueRequest) (domain.CreateServiceRequestFromIssueResponse, error) {
	return domain.CreateServiceRequestFromIssueResponse{}, nil
}

func (f *fakeSync) HandleWebhook(context.Context, service.Delivery) (service.Outcome, error) {
	f.called++
	return f.outcome, f.err
}

func ghSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func ghRequest(t *testing.T, body, signature, delivery, event string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", strings.NewReader(body))
	if signature != "" {
		req.Header.Set(github.SignatureHeader, signature)
	}
	if delivery != "" {
		req.Header.Set(github.DeliveryHeader, delivery)
	}
	if event != "" {
		req.Header.Set(github.EventHeader, event)
	}
	return req
}

const ghBody = `{"action":"labeled","issue":{"html_url":"https://github.com/wso2/choreo/issues/1"}}`

func TestGithubWebhook_ValidSignature(t *testing.T) {
	sync := &fakeSync{outcome: service.Outcome{Action: "would_create"}}
	h := NewGithubWebhookHandler(sync, ghSecret)

	rec := httptest.NewRecorder()
	h.Handle(rec, ghRequest(t, ghBody, ghSign(ghSecret, []byte(ghBody)), "d-1", "issues"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if sync.called != 1 {
		t.Fatalf("service called %d times, want 1", sync.called)
	}
}

// THE SECURITY BOUNDARY. Nothing reaches the service without a valid
// signature, and every failure looks identical from outside.
func TestGithubWebhook_RejectsBadSignatures(t *testing.T) {
	cases := map[string]string{
		"absent":         "",
		"wrong secret":   ghSign("not-the-secret", []byte(ghBody)),
		"wrong body":     ghSign(ghSecret, []byte(`{"action":"closed"}`)),
		"not hex":        "sha256=nothex",
		"sha1 downgrade": "sha1=" + hex.EncodeToString([]byte("x")),
		"no prefix":      hex.EncodeToString([]byte("x")),
	}
	var bodies []string
	for name, sig := range cases {
		t.Run(name, func(t *testing.T) {
			sync := &fakeSync{}
			h := NewGithubWebhookHandler(sync, ghSecret)
			rec := httptest.NewRecorder()
			h.Handle(rec, ghRequest(t, ghBody, sig, "d-1", "issues"))

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if sync.called != 0 {
				t.Fatal("the service was reached without a valid signature")
			}
			bodies = append(bodies, rec.Body.String())
		})
	}
	// Every rejection must read the same: a caller must not be able to tell
	// "malformed" from "wrong", which would otherwise narrow a forgery search.
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Fatalf("rejection responses differ:\n%s\n%s", bodies[0], bodies[i])
		}
	}
}

// The response must never contain the expected signature. ServiceNow's did,
// which made its secret irrelevant.
func TestGithubWebhook_NeverEchoesExpectedSignature(t *testing.T) {
	h := NewGithubWebhookHandler(&fakeSync{}, ghSecret)
	rec := httptest.NewRecorder()
	h.Handle(rec, ghRequest(t, ghBody, ghSign("guess", []byte(ghBody)), "d-1", "issues"))

	expected := strings.TrimPrefix(ghSign(ghSecret, []byte(ghBody)), "sha256=")
	if strings.Contains(rec.Body.String(), expected) {
		t.Fatal("the response leaked the expected signature")
	}
}

// An unconfigured secret must refuse everything, not accept everything.
func TestGithubWebhook_NoSecretRefuses(t *testing.T) {
	sync := &fakeSync{}
	h := NewGithubWebhookHandler(sync, "")
	rec := httptest.NewRecorder()
	h.Handle(rec, ghRequest(t, ghBody, ghSign("anything", []byte(ghBody)), "d-1", "issues"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if sync.called != 0 {
		t.Fatal("an unconfigured secret let a webhook through")
	}
}

func TestGithubWebhook_RequiresHeaders(t *testing.T) {
	for name, req := range map[string]*http.Request{
		"no delivery": ghRequest(t, ghBody, ghSign(ghSecret, []byte(ghBody)), "", "issues"),
		"no event":    ghRequest(t, ghBody, ghSign(ghSecret, []byte(ghBody)), "d-1", ""),
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewGithubWebhookHandler(&fakeSync{}, ghSecret).Handle(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

// A redelivery of something already applied is a success, so GitHub stops
// retrying it.
func TestGithubWebhook_DuplicateIsOK(t *testing.T) {
	sync := &fakeSync{err: repository.ErrDeliverySeen}
	rec := httptest.NewRecorder()
	NewGithubWebhookHandler(sync, ghSecret).
		Handle(rec, ghRequest(t, ghBody, ghSign(ghSecret, []byte(ghBody)), "d-1", "issues"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 so GitHub stops retrying", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "duplicate") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

// A real failure must be a 500 so GitHub retries it.
func TestGithubWebhook_FailureIsRetryable(t *testing.T) {
	sync := &fakeSync{err: errors.New("database is down")}
	rec := httptest.NewRecorder()
	NewGithubWebhookHandler(sync, ghSecret).
		Handle(rec, ghRequest(t, ghBody, ghSign(ghSecret, []byte(ghBody)), "d-1", "issues"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 so GitHub retries", rec.Code)
	}
	// The upstream error must not reach the caller.
	if strings.Contains(rec.Body.String(), "database is down") {
		t.Fatalf("the internal error leaked: %s", rec.Body.String())
	}
}

func TestGithubWebhook_MalformedPayload(t *testing.T) {
	body := `{not json`
	rec := httptest.NewRecorder()
	NewGithubWebhookHandler(&fakeSync{}, ghSecret).
		Handle(rec, ghRequest(t, body, ghSign(ghSecret, []byte(body)), "d-1", "issues"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
