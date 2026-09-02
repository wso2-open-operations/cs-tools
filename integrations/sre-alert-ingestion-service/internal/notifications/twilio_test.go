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

package notifications

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

func newTwilioTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewTwilioClient_NeverFailsOnZeroValueConfig(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	if c == nil {
		t.Fatal("NewTwilioClient returned nil for zero-value TwilioConfig")
	}
	if err := c.MakeCall(context.Background(), "+15551234567", "hello"); err == nil {
		t.Fatal("expected error for unconfigured client, got nil")
	}
}

func TestMakeCall_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})

	t.Run("rejects empty to", func(t *testing.T) {
		if err := c.MakeCall(context.Background(), "", "hello"); err == nil {
			t.Fatal("expected error for empty to, got nil")
		}
	})
	t.Run("rejects empty message", func(t *testing.T) {
		if err := c.MakeCall(context.Background(), "+15551234567", ""); err == nil {
			t.Fatal("expected error for empty message, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestMakeCall_SendsExpectedRequest(t *testing.T) {
	var gotAuthUser, gotAuthPass string
	var gotForm url.Values
	var gotPath, gotContentType string
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret-token", FromNumber: "+15550000000", APIBaseURL: srv.URL})
	if err := c.MakeCall(context.Background(), "+15551234567", "SRE alert escalation: buffered alert exhausted retries"); err != nil {
		t.Fatalf("MakeCall returned error: %v", err)
	}

	if gotAuthUser != "AC123" || gotAuthPass != "secret-token" {
		t.Errorf("BasicAuth = (%q, %q), want (%q, %q)", gotAuthUser, gotAuthPass, "AC123", "secret-token")
	}
	if gotPath != "/Accounts/AC123/Calls.json" {
		t.Errorf("path = %q, want %q", gotPath, "/Accounts/AC123/Calls.json")
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
	if got := gotForm.Get("To"); got != "+15551234567" {
		t.Errorf("To = %q, want %q", got, "+15551234567")
	}
	if got := gotForm.Get("From"); got != "+15550000000" {
		t.Errorf("From = %q, want %q", got, "+15550000000")
	}
	wantTwiml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Response><Say>SRE alert escalation: buffered alert exhausted retries</Say></Response>"
	if got := gotForm.Get("Twiml"); got != wantTwiml {
		t.Errorf("Twiml = %q, want %q", got, wantTwiml)
	}
}

func TestMakeCall_SendsConfiguredVoiceAndLanguage(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{
		AccountSID: "AC123",
		AuthToken:  "secret-token",
		FromNumber: "+15550000000",
		Voice:      "Polly.Raveena",
		Language:   "en-IN",
		APIBaseURL: srv.URL,
	})
	if err := c.MakeCall(context.Background(), "+15551234567", "hello"); err != nil {
		t.Fatalf("MakeCall returned error: %v", err)
	}

	wantTwiml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Response><Say voice=\"Polly.Raveena\" language=\"en-IN\">hello</Say></Response>"
	if got := gotForm.Get("Twiml"); got != wantTwiml {
		t.Errorf("Twiml = %q, want %q", got, wantTwiml)
	}
}

// The call message is derived from alert data, not trusted markup — if it
// were embedded into the TwiML document unescaped, a message containing
// TwiML-shaped text could inject a different verb (e.g. <Dial> or
// <Redirect>) instead of just being spoken. Confirms sayTwiML escapes it.
func TestMakeCall_EscapesMessageInTwiML(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})
	malicious := `</Say><Redirect>https://evil.example/hijack</Redirect><Say>`
	if err := c.MakeCall(context.Background(), "+15551234567", malicious); err != nil {
		t.Fatalf("MakeCall returned error: %v", err)
	}

	twiml := gotForm.Get("Twiml")
	if strings.Contains(twiml, "<Redirect>") {
		t.Errorf("message was not escaped, TwiML contains an injected <Redirect> verb: %s", twiml)
	}
	if !strings.Contains(twiml, "&lt;Redirect&gt;") {
		t.Errorf("expected the injected markup to appear escaped in the TwiML: %s", twiml)
	}
}

func TestMakeCall_MapsUpstreamError(t *testing.T) {
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":21211,"message":"The 'To' number is not a valid phone number."}`))
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})
	err := c.MakeCall(context.Background(), "not-a-number", "hello")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *apierror.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
}

// Escalate is a thin wrapper around MakeCall using the configured static
// ToNumber (v1 scope) instead of a caller-supplied destination.
func TestEscalate_UsesConfiguredToNumber(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{
		AccountSID: "AC123",
		AuthToken:  "secret",
		FromNumber: "+15550000000",
		ToNumber:   "+15559998888",
		APIBaseURL: srv.URL,
	})
	if err := c.Escalate(context.Background(), "escalation message"); err != nil {
		t.Fatalf("Escalate returned error: %v", err)
	}
	if got := gotForm.Get("To"); got != "+15559998888" {
		t.Errorf("To = %q, want configured ToNumber %q", got, "+15559998888")
	}
}
