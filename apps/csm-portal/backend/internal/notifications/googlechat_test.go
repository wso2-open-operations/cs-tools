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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func TestSendIncidentAlert_ValidatesArgumentsBeforeCallingUpstream(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Run("rejects unconfigured product", func(t *testing.T) {
		c := NewGoogleChatClient(GoogleChatConfig{})
		if err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com"); err == nil {
			t.Fatal("expected error for unconfigured product, got nil")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})
		if err := c.SendIncidentAlert(context.Background(), "api-manager", "", "desc", "https://example.com"); err == nil {
			t.Fatal("expected error for empty title, got nil")
		}
	})

	if called {
		t.Error("upstream should not have been called for invalid arguments")
	}
}

func TestSendIncidentAlert_RoutesToTheConfiguredProductsSpace(t *testing.T) {
	var apimCalled, isCalled bool
	apimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apimCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apimSrv.Close()
	isSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer isSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "API-Manager", WebhookURL: apimSrv.URL},
		{Product: "identity-server", WebhookURL: isSrv.URL},
	}})

	// Product matching is case- and whitespace-insensitive.
	if err := c.SendIncidentAlert(context.Background(), " api-manager ", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !apimCalled || isCalled {
		t.Errorf("apimCalled = %v, isCalled = %v, want true, false", apimCalled, isCalled)
	}

	if err := c.SendIncidentAlert(context.Background(), "unknown-product", "title", "desc", "https://example.com"); err == nil {
		t.Fatal("expected error for a product with no configured space, got nil")
	}
}

func TestNewGoogleChatClient_MarksDuplicateNormalizedProductsUnconfigured(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "API-Manager", WebhookURL: "https://chat.example.com/a"},
		{Product: " api-manager ", WebhookURL: "https://chat.example.com/b"},
	}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected error for a product with a duplicate (now unconfigured) mapping, got nil")
	}
	if !strings.Contains(err.Error(), "no google chat space configured") {
		t.Errorf("expected unconfigured space error, got: %v", err)
	}
}

func TestSendIncidentAlert_RedactsWebhookURLOnNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	webhookURL := srv.URL + "/messages?key=SECRET_KEY&token=SECRET_TOKEN"
	srv.Close() // subsequent requests to webhookURL now fail to connect

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: webhookURL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected an error from an unreachable webhook, got nil")
	}
	if strings.Contains(err.Error(), "SECRET_KEY") || strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Errorf("error leaked webhook credentials: %v", err)
	}
}

func TestSendIncidentAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "P1 Incident - CASE-001", "Asgardeo Add Customer", "https://portal.example.com/operations/incidents/CASE-001")
	if err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header.Title != "P1 Incident - CASE-001" {
		t.Errorf("Header.Title = %q, want %q", card.Header.Title, "P1 Incident - CASE-001")
	}
	if len(card.Sections) != 2 {
		t.Fatalf("Sections length = %d, want 2", len(card.Sections))
	}
	if card.Sections[0].Header != "Short Description" {
		t.Errorf("Sections[0].Header = %q, want %q", card.Sections[0].Header, "Short Description")
	}
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != "Asgardeo Add Customer" {
		t.Errorf("short description text = %q, want %q", got, "Asgardeo Add Customer")
	}
	button := card.Sections[1].Widgets[0].ButtonList.Buttons[0]
	if button.Text != "Open in CSM Portal" {
		t.Errorf("button text = %q, want %q", button.Text, "Open in CSM Portal")
	}
	if button.OnClick.OpenLink.URL != "https://portal.example.com/operations/incidents/CASE-001" {
		t.Errorf("button URL = %q, want %q", button.OnClick.OpenLink.URL, "https://portal.example.com/operations/incidents/CASE-001")
	}
}

func TestSendIncidentAlert_MapsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid webhook payload"})
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com")
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

func TestSendIncidentAlert_ConstructsWithZeroValueConfig(t *testing.T) {
	// NewGoogleChatClient must never fail or panic even when the Google Chat
	// channel has not been configured for a given deployment.
	c := NewGoogleChatClient(GoogleChatConfig{})
	if c == nil {
		t.Fatal("NewGoogleChatClient returned nil for zero-value GoogleChatConfig")
	}
}
