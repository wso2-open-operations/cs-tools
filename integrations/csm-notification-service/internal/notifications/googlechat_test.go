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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
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

// TestSendCard_FallsBackToDefaultSpaceWhenProductUnconfigured verifies the
// opt-in "default" GOOGLE_CHAT_SPACES entry: a product with no space of its
// own routes to the space configured under the reserved "default" key
// instead of erroring.
func TestSendCard_FallsBackToDefaultSpaceWhenProductUnconfigured(t *testing.T) {
	var defaultCalled bool
	defaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer defaultSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: "https://chat.example.com/apim"},
		{Product: "default", WebhookURL: defaultSrv.URL},
	}})

	if err := c.SendIncidentAlert(context.Background(), "identity-server", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !defaultCalled {
		t.Error("expected the alert to be posted to the default space's webhook, but it wasn't called")
	}
}

// TestSendCard_MatchedProductTakesPriorityOverDefault verifies the default
// space fallback never overrides a product that does have its own
// configured space.
func TestSendCard_MatchedProductTakesPriorityOverDefault(t *testing.T) {
	var apimCalled, defaultCalled bool
	apimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apimCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer apimSrv.Close()
	defaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer defaultSrv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: apimSrv.URL},
		{Product: "default", WebhookURL: defaultSrv.URL},
	}})

	if err := c.SendIncidentAlert(context.Background(), "api-manager", "title", "desc", "https://example.com"); err != nil {
		t.Fatalf("SendIncidentAlert returned error: %v", err)
	}
	if !apimCalled || defaultCalled {
		t.Errorf("apimCalled = %v, defaultCalled = %v, want true, false", apimCalled, defaultCalled)
	}
}

// TestSendCard_StillErrorsWithNoDefaultConfigured verifies the fallback is
// opt-in: with no "default" GOOGLE_CHAT_SPACES entry at all, an unmatched
// product still errors exactly as before this change.
func TestSendCard_StillErrorsWithNoDefaultConfigured(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{
		{Product: "api-manager", WebhookURL: "https://chat.example.com/apim"},
	}})

	err := c.SendIncidentAlert(context.Background(), "identity-server", "title", "desc", "https://example.com")
	if err == nil {
		t.Fatal("expected error for an unmatched product with no default space configured, got nil")
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

// TestSendCaseCreatedAlert_SendsExpectedCard verifies the redesigned
// case.created card: the case reference leads the header, prefixed with a
// "🆕" marker (no separate "New case" row — see SendCaseCreatedAlert's own
// doc comment for why); the header subtitle is the case title alone,
// unstyled. The body leads with team (muted gray, via teamPart) as its
// own first line — right under the header — then up to two more
// plain-text lines, no leading icon on either (dropped for width on
// mobile — see SendCaseCreatedAlert's own doc comment): severity alone,
// then product (bold) on its own line, then a visible "View case" link
// on its own line below it — no button, since navigating to the case
// isn't a genuine action.
func TestSendCaseCreatedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001001", "WSO2-1000", "WSO2 API Manager",
		`Tom & Jerry <script>`, "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header == nil {
		t.Fatal("Header = nil, want a header leading with the case reference")
	}
	if card.Header.Title != "🆕 CS0001001 · WSO2-1000" {
		t.Errorf("Header.Title = %q, want %q", card.Header.Title, "🆕 CS0001001 · WSO2-1000")
	}
	if card.Header.Subtitle != `Tom & Jerry <script>` {
		t.Errorf("Header.Subtitle = %q, want the case title verbatim (headers aren't HTML, and team no longer lives here)", card.Header.Subtitle)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v — want a single text widget, no button", card.Sections)
	}
	want := `<font color="#5F6368">Team Nova</font><br><font color="#DC2626"><b>Critical (P1)</b></font><br><b>WSO2 API Manager</b><br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

// TestSendCaseCreatedAlert_OmitsEmptyOptionalParts verifies severity/
// product/team are each dropped entirely (no stray " · " separators) when
// the caller doesn't supply one — the "View case" link is never omitted.
func TestSendCaseCreatedAlert_OmitsEmptyOptionalParts(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseCreatedAlert(context.Background(), "api-manager",
		"", "", "CS0001001", "", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendCaseCreatedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header.Title != "🆕 CS0001001" {
		t.Errorf("Header.Title = %q, want just the 🆕 marker and case number with no WSO2CaseID separator", card.Header.Title)
	}
	if card.Header.Subtitle != "" {
		t.Errorf("Header.Subtitle = %q, want empty when title and team are both empty", card.Header.Subtitle)
	}
	want := `<a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

func TestSendCaseCreatedAlert_RejectsEmptyCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseCreatedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendSecurityReportAnalysisAlert_SendsExpectedCard verifies the
// security_report_analysis card's shape: same header convention as
// SendCaseCreatedAlert (case reference + "🆕" marker as the title, case
// title as the subtitle, unstyled), but a fixed "Security Report Analysis"
// label in place of a severity line, since severity is never set for this
// case type.
func TestSendSecurityReportAnalysisAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager",
		"CS0001001", "WSO2-1000", "WSO2 API Manager",
		`Tom & Jerry <script>`, "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSecurityReportAnalysisAlert returned error: %v", err)
	}

	if len(capturedBody.CardsV2) != 1 {
		t.Fatalf("CardsV2 length = %d, want 1", len(capturedBody.CardsV2))
	}
	card := capturedBody.CardsV2[0].Card
	if card.Header == nil {
		t.Fatal("Header = nil, want a header leading with the case reference")
	}
	if card.Header.Title != "🆕 CS0001001 · WSO2-1000" {
		t.Errorf("Header.Title = %q, want %q", card.Header.Title, "🆕 CS0001001 · WSO2-1000")
	}
	if card.Header.Subtitle != `Tom & Jerry <script>` {
		t.Errorf("Header.Subtitle = %q, want the case title verbatim", card.Header.Subtitle)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v — want a single text widget, no button", card.Sections)
	}
	want := `<font color="#5F6368">Team Nova</font><br><b>Security Report Analysis</b><br><b>WSO2 API Manager</b><br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

// TestSendSecurityReportAnalysisAlert_OmitsEmptyOptionalParts verifies
// team/product are each dropped entirely when the caller doesn't supply
// one — the fixed "Security Report Analysis" label and the "View case"
// link are never omitted.
func TestSendSecurityReportAnalysisAlert_OmitsEmptyOptionalParts(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager",
		"CS0001001", "", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSecurityReportAnalysisAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header.Title != "🆕 CS0001001" {
		t.Errorf("Header.Title = %q, want just the 🆕 marker and case number with no WSO2CaseID separator", card.Header.Title)
	}
	if card.Header.Subtitle != "" {
		t.Errorf("Header.Subtitle = %q, want empty when title is empty", card.Header.Subtitle)
	}
	want := `<b>Security Report Analysis</b><br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

func TestSendSecurityReportAnalysisAlert_RejectsEmptyCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendSecurityReportAnalysisAlert(context.Background(), "api-manager", "", "WSO2-1000", "WSO2 API Manager", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}

// TestSendCaseAcknowledgedAlert_SendsExpectedCard verifies the
// case.acknowledged card's three lines — no header, no button, no leading
// icon on any of them (dropped for width on mobile — see
// SendCaseCreatedAlert's own doc comment), and deliberately no team line
// at all, unlike the other two cards: severity alone, "<caseNumber>
// <wso2CaseID>" together, then "Ack by <name> · View case" — caseNumber
// is plain text; "View case" (not the case number) is the link, matching
// the other two cards' explicit-link-text convention.
func TestSendCaseAcknowledgedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"Critical (P1)", "#DC2626", "CS0001002", "WSO2-1001", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header != nil {
		t.Errorf("Header = %+v, want nil (this card stays one line, no header)", card.Header)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v", card.Sections)
	}
	want := `<font color="#DC2626"><b>Critical (P1)</b></font><br>CS0001002 · WSO2-1001<br>Ack by Jane Doe · <a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

// TestSendCaseAcknowledgedAlert_OmitsEmptyWSO2CaseID verifies it's dropped
// from the line entirely (no stray separator) when the caller doesn't
// supply one.
func TestSendCaseAcknowledgedAlert_OmitsEmptyWSO2CaseID(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager",
		"High (P2)", "#EA580C", "CS0001003", "", "https://csm.example.com/cases/CASE-1", "Jane Doe")
	if err != nil {
		t.Fatalf("SendCaseAcknowledgedAlert returned error: %v", err)
	}

	text := capturedBody.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	want := `<font color="#EA580C"><b>High (P2)</b></font><br>CS0001003<br>Ack by Jane Doe · <a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if text != want {
		t.Errorf("card text = %q, want %q", text, want)
	}
}

func TestSendCaseAcknowledgedAlert_RejectsMissingRequiredArgs(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "", "WSO2-1000", "https://example.com/cases/1", "Jane Doe"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
	if err := c.SendCaseAcknowledgedAlert(context.Background(), "api-manager", "Critical (P1)", "#DC2626", "CS0001", "WSO2-1000", "https://example.com/cases/1", ""); err == nil {
		t.Fatal("expected error for empty acknowledgerName, got nil")
	}
}

// TestSendSeverityChangedAlert_SendsExpectedCard verifies the redesigned
// case.severity_changed card: case reference in the header, the case
// title alone as subtitle (unstyled). The body leads with team (muted
// gray, via teamPart) as its own first line, then up to two more
// plain-text lines, no leading icon on either (dropped for width on
// mobile — see SendCaseCreatedAlert's own doc comment): an old→new
// severity transition — both severities colored, not just the new one —
// on its own line, then a visible "View case" link alone on the next, no
// button.
func TestSendSeverityChangedAlert_SendsExpectedCard(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSeverityChangedAlert(context.Background(), "api-manager",
		"High (P2)", "#EA580C", "Low (P4)", "#6B7280", "CS0001002", "WSO2-1001", "Gateway returns 502", "Team Nova", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSeverityChangedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header == nil {
		t.Fatal("Header = nil, want a header leading with the case reference")
	}
	if card.Header.Title != "CS0001002 · WSO2-1001" {
		t.Errorf("Header.Title = %q, want %q", card.Header.Title, "CS0001002 · WSO2-1001")
	}
	if card.Header.Subtitle != "Gateway returns 502" {
		t.Errorf("Header.Subtitle = %q, want the case title (team no longer lives here)", card.Header.Subtitle)
	}
	if len(card.Sections) != 1 || len(card.Sections[0].Widgets) != 1 {
		t.Fatalf("unexpected sections/widgets shape: %+v — want a single text widget, no button", card.Sections)
	}
	want := `<font color="#5F6368">Team Nova</font><br><font color="#EA580C"><b>High (P2)</b></font> → <font color="#6B7280"><b>Low (P4)</b></font><br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if got := card.Sections[0].Widgets[0].TextParagraph.Text; got != want {
		t.Errorf("card text = %q, want %q", got, want)
	}
}

func TestSendSeverityChangedAlert_OmitsEmptyWSO2CaseIDAndTeam(t *testing.T) {
	var capturedBody chatCardMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: srv.URL}}})

	err := c.SendSeverityChangedAlert(context.Background(), "api-manager",
		"Medium (P3)", "#7C3AED", "Critical (P1)", "#DC2626", "CS0001003", "", "", "", "https://csm.example.com/cases/CASE-1")
	if err != nil {
		t.Fatalf("SendSeverityChangedAlert returned error: %v", err)
	}

	card := capturedBody.CardsV2[0].Card
	if card.Header.Title != "CS0001003" {
		t.Errorf("Header.Title = %q, want just the case number with no WSO2CaseID separator", card.Header.Title)
	}
	if card.Header.Subtitle != "" {
		t.Errorf("Header.Subtitle = %q, want empty when no title was sent", card.Header.Subtitle)
	}
	text := card.Sections[0].Widgets[0].TextParagraph.Text
	want := `<font color="#7C3AED"><b>Medium (P3)</b></font> → <font color="#DC2626"><b>Critical (P1)</b></font><br><a href="https://csm.example.com/cases/CASE-1">View case</a>`
	if text != want {
		t.Errorf("card text = %q, want %q", text, want)
	}
}

func TestSendSeverityChangedAlert_RejectsMissingCaseNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{Spaces: []GoogleChatSpace{{Product: "api-manager", WebhookURL: "https://example.com"}}})
	if err := c.SendSeverityChangedAlert(context.Background(), "api-manager", "High (P2)", "#EA580C", "Low (P4)", "#6B7280", "", "WSO2-1000", "title", "Team Nova", "https://example.com/cases/1"); err == nil {
		t.Fatal("expected error for empty caseNumber, got nil")
	}
}
