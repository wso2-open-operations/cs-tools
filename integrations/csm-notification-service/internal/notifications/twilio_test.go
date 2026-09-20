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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// newTwilioTestServer starts a local mock Twilio server and returns it — the
// caller passes srv.URL as TwilioConfig.APIBaseURL to point a TwilioClient
// at it instead of the real Twilio API.
func newTwilioTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestSendSMS_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})

	t.Run("rejects empty to", func(t *testing.T) {
		if err := c.SendSMS(context.Background(), "", "hello"); err == nil {
			t.Fatal("expected error for empty to, got nil")
		}
	})
	t.Run("rejects empty body", func(t *testing.T) {
		if err := c.SendSMS(context.Background(), "+15551234567", ""); err == nil {
			t.Fatal("expected error for empty body, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestSendSMS_RejectsWhenUnconfigured(t *testing.T) {
	// NewTwilioClient must never fail or panic even when the SMS channel has
	// not been configured for a given deployment; the error only surfaces on
	// the first real send attempt.
	c := NewTwilioClient(TwilioConfig{})
	if c == nil {
		t.Fatal("NewTwilioClient returned nil for zero-value TwilioConfig")
	}
	if err := c.SendSMS(context.Background(), "+15551234567", "hello"); err == nil {
		t.Fatal("expected error for unconfigured client, got nil")
	}
}

func TestSendSMS_SendsExpectedRequest_FromNumber(t *testing.T) {
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
	if err := c.SendSMS(context.Background(), "+15551234567", "On-call page: P1 incident"); err != nil {
		t.Fatalf("SendSMS returned error: %v", err)
	}

	if gotAuthUser != "AC123" || gotAuthPass != "secret-token" {
		t.Errorf("BasicAuth = (%q, %q), want (%q, %q)", gotAuthUser, gotAuthPass, "AC123", "secret-token")
	}
	if gotPath != "/Accounts/AC123/Messages.json" {
		t.Errorf("path = %q, want %q", gotPath, "/Accounts/AC123/Messages.json")
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
	if gotForm.Has("MessagingServiceSid") {
		t.Error("MessagingServiceSid should not be set when only FromNumber is configured")
	}
	if got := gotForm.Get("Body"); got != "On-call page: P1 incident" {
		t.Errorf("Body = %q, want %q", got, "On-call page: P1 incident")
	}
}

func TestSendSMS_SendsExpectedRequest_MessagingServiceSid(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret-token", MessagingServiceSid: "MG123", APIBaseURL: srv.URL})
	if err := c.SendSMS(context.Background(), "+15551234567", "On-call page: P1 incident"); err != nil {
		t.Fatalf("SendSMS returned error: %v", err)
	}

	if got := gotForm.Get("MessagingServiceSid"); got != "MG123" {
		t.Errorf("MessagingServiceSid = %q, want %q", got, "MG123")
	}
	if gotForm.Has("From") {
		t.Error("From should not be set when only MessagingServiceSid is configured")
	}
}

func TestSendSMS_PrefersMessagingServiceSidWhenBothConfigured(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{
		AccountSID:          "AC123",
		AuthToken:           "secret-token",
		FromNumber:          "+15550000000",
		MessagingServiceSid: "MG123",
		APIBaseURL:          srv.URL,
	})
	if err := c.SendSMS(context.Background(), "+15551234567", "hello"); err != nil {
		t.Fatalf("SendSMS returned error: %v", err)
	}

	if got := gotForm.Get("MessagingServiceSid"); got != "MG123" {
		t.Errorf("MessagingServiceSid = %q, want %q", got, "MG123")
	}
	if gotForm.Has("From") {
		t.Error("From should not be sent alongside MessagingServiceSid")
	}
}

func TestSendSMS_MapsUpstreamError(t *testing.T) {
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":21211,"message":"The 'To' number is not a valid phone number."}`))
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})
	err := c.SendSMS(context.Background(), "not-a-number", "hello")
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

func TestMakeCall_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", FromNumber: "+15550000000", APIBaseURL: srv.URL})

	t.Run("rejects empty to", func(t *testing.T) {
		if _, err := c.MakeCall(context.Background(), "", "hello"); err == nil {
			t.Fatal("expected error for empty to, got nil")
		}
	})
	t.Run("rejects empty message", func(t *testing.T) {
		if _, err := c.MakeCall(context.Background(), "+15551234567", ""); err == nil {
			t.Fatal("expected error for empty message, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestMakeCall_RejectsWhenUnconfigured(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{})
	if _, err := c.MakeCall(context.Background(), "+15551234567", "hello"); err == nil {
		t.Fatal("expected error for unconfigured client, got nil")
	}
}

// MakeCall always needs FromNumber for caller ID — MessagingServiceSid is an
// SMS-only concept with no Voice equivalent, so it can't substitute here the
// way it does for SendSMS.
func TestMakeCall_RejectsMessagingServiceSidOnlyConfig(t *testing.T) {
	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret", MessagingServiceSid: "MG123"})
	if _, err := c.MakeCall(context.Background(), "+15551234567", "hello"); err == nil {
		t.Fatal("expected error: MakeCall requires FromNumber even when MessagingServiceSid is set")
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
	if _, err := c.MakeCall(context.Background(), "+15551234567", "On-call page: P1 incident"); err != nil {
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
	wantTwiml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Response><Say>On-call page: P1 incident</Say></Response>"
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
	if _, err := c.MakeCall(context.Background(), "+15551234567", "hello"); err != nil {
		t.Fatalf("MakeCall returned error: %v", err)
	}

	wantTwiml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Response><Say voice=\"Polly.Raveena\" language=\"en-IN\">hello</Say></Response>"
	if got := gotForm.Get("Twiml"); got != wantTwiml {
		t.Errorf("Twiml = %q, want %q", got, wantTwiml)
	}
}

func TestMakeCall_OmitsVoiceAndLanguageAttributesWhenUnset(t *testing.T) {
	var gotForm url.Values
	srv := newTwilioTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form
		w.WriteHeader(http.StatusCreated)
	})

	c := NewTwilioClient(TwilioConfig{AccountSID: "AC123", AuthToken: "secret-token", FromNumber: "+15550000000", APIBaseURL: srv.URL})
	if _, err := c.MakeCall(context.Background(), "+15551234567", "hello"); err != nil {
		t.Fatalf("MakeCall returned error: %v", err)
	}

	twiml := gotForm.Get("Twiml")
	if strings.Contains(twiml, "voice=") || strings.Contains(twiml, "language=") {
		t.Errorf("expected no voice/language attributes when unconfigured, got: %s", twiml)
	}
}

// The call message is caller-supplied text, not trusted markup — if it were
// embedded into the TwiML document unescaped, a message body containing
// TwiML-shaped text could inject a different verb (e.g. <Dial> or
// <Redirect>) instead of just being spoken. Confirms sayTwiML actually
// escapes it.
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
	if _, err := c.MakeCall(context.Background(), "+15551234567", malicious); err != nil {
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
	_, err := c.MakeCall(context.Background(), "not-a-number", "hello")
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
