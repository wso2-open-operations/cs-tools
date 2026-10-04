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

package dispatch

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

type sentEmail struct {
	from        string
	to          []string
	cc          []string
	bcc         []string
	replyTo     []string
	subject     string
	htmlBody    string
	attachments []notifications.EmailAttachment
}

type mockEmailSender struct {
	err error
	// errFor, when set, overrides err on a per-call basis — lets a test make
	// one recipient group fail while another succeeds, to exercise
	// sendPerGroup's per-group idempotency tracking.
	errFor func(to []string) error
	// mu guards calls — SendEmail is called concurrently by
	// TestDispatcher_Handle_ConcurrentCallsClaimEachChannelOnce, unlike every
	// other test here, which drives Handle sequentially.
	mu    sync.Mutex
	calls []sentEmail
	// block, when non-nil, holds every send open until it is closed, so a
	// test can have a second Handle call arrive mid-send.
	block chan struct{}
	// onSend, when set, runs as the send completes -- a seam for a test
	// that needs something to happen between the e-mail going out and the
	// step being recorded.
	onSend func()
}

func (m *mockEmailSender) FromAddress() string { return "noreply@wso2.com" }

func (m *mockEmailSender) SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	return m.SendEmailFrom(ctx, "", to, cc, bcc, replyTo, subject, htmlBody, attachments)
}

func (m *mockEmailSender) SendEmailFrom(ctx context.Context, from string, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	if m.block != nil {
		<-m.block
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, sentEmail{from: from, to: to, cc: cc, bcc: bcc, replyTo: replyTo, subject: subject, htmlBody: htmlBody, attachments: attachments})
	if m.onSend != nil {
		m.onSend()
	}
	if m.errFor != nil {
		return m.errFor(to)
	}
	return m.err
}

type sentCaseCreatedAlert struct {
	audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string
}

type sentSecurityReportAnalysisAlert struct {
	audience, caseNumber, wso2CaseID, productName, title, team, caseLink string
}

type sentCaseAcknowledgedAlert struct {
	audience, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string
}

type sentSeverityChangedAlert struct {
	audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string
}

type mockGoogleChatSender struct {
	err error
	// mu guards calls — see mockEmailSender.mu's doc comment.
	mu                          sync.Mutex
	caseCreatedCalls            []sentCaseCreatedAlert
	caseAcknowledgedCalls       []sentCaseAcknowledgedAlert
	severityChangedCalls        []sentSeverityChangedAlert
	securityReportAnalysisCalls []sentSecurityReportAnalysisAlert
}

func (m *mockGoogleChatSender) SendCaseCreatedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caseCreatedCalls = append(m.caseCreatedCalls, sentCaseCreatedAlert{audience, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink})
	return m.err
}

func (m *mockGoogleChatSender) SendSecurityReportAnalysisAlert(ctx context.Context, audience, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.securityReportAnalysisCalls = append(m.securityReportAnalysisCalls, sentSecurityReportAnalysisAlert{audience, caseNumber, wso2CaseID, productName, title, team, caseLink})
	return m.err
}

func (m *mockGoogleChatSender) SendCaseAcknowledgedAlert(ctx context.Context, audience, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caseAcknowledgedCalls = append(m.caseAcknowledgedCalls, sentCaseAcknowledgedAlert{audience, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName})
	return m.err
}

func (m *mockGoogleChatSender) SendSeverityChangedAlert(ctx context.Context, audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.severityChangedCalls = append(m.severityChangedCalls, sentSeverityChangedAlert{audience, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink})
	return m.err
}

type sentCall struct {
	to, message string
}

type mockCallSender struct {
	err error
	// mu guards calls — see mockEmailSender.mu's doc comment.
	mu    sync.Mutex
	calls []sentCall
}

func (m *mockCallSender) MakeCall(ctx context.Context, to, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, sentCall{to, message})
	return m.err
}

// mockLinkResolver defaults to resolving every recipient to the same fixed
// link (https://csm.example/cases/<caseID>) so tests that don't care about
// link resolution itself don't need their own resolver setup. linkFor, when
// set, lets a test give different recipients different links (e.g. to
// exercise groupByLink's grouping).
type mockLinkResolver struct {
	linkFor func(email string) string
	err     error

	gotEmails               []string
	gotProjectID, gotCaseID string
}

// CSMLink mirrors recipientlinks.Resolver.CSMLink's own shape closely enough
// for tests that check the Google Chat alert's portal link — a fixed base
// (no configuration plumbed through this mock) plus the caseID.
func (m *mockLinkResolver) CSMLink(caseID string) string {
	return "https://csm.example/cases/" + caseID
}

// ChangeRequestLink mirrors the real resolver's audience split: a customer
// notice links into the customer portal, under the project; everyone else
// links into the CSM portal.
func (m *mockLinkResolver) ChangeRequestLink(audience, changeRequestID, projectID string) string {
	if audience == "customer" && projectID != "" {
		return "https://customer.example/projects/" + projectID + "/operations/change-requests/" + changeRequestID
	}
	return "https://csm.example/operations/change-requests/" + changeRequestID
}

func (m *mockLinkResolver) ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]recipientlinks.RecipientLink, error) {
	m.gotEmails = emails
	m.gotProjectID = projectID
	m.gotCaseID = caseID
	if m.err != nil {
		return nil, m.err
	}
	links := make([]recipientlinks.RecipientLink, len(emails))
	for i, email := range emails {
		link := "https://csm.example/cases/" + caseID
		if m.linkFor != nil {
			link = m.linkFor(email)
		}
		links[i] = recipientlinks.RecipientLink{Email: email, CaseLink: link}
	}
	return links, nil
}

const testRecipient = "test-recipient@example.com"

func newTestDispatcher(email emailSender, chat googleChatSender, call callSender) *Dispatcher {
	return NewDispatcher(Deps{Email: email, GoogleChat: chat, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})
}

func TestDispatcher_Handle_CaseCreated(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	got := mock.calls[0]
	if len(got.to) != 1 || got.to[0] != testRecipient {
		t.Errorf("to = %v, want [%s]", got.to, testRecipient)
	}
	if !strings.Contains(got.subject, "Something broke") {
		t.Errorf("subject = %q, want it to contain the case title", got.subject)
	}
	if !strings.Contains(got.htmlBody, "Something broke") {
		t.Error("htmlBody does not contain the case title")
	}

	if len(chat.caseCreatedCalls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.caseCreatedCalls))
	}
	gotChat := chat.caseCreatedCalls[0]
	if gotChat.title != "Something broke" || gotChat.caseLink != "https://csm.example/cases/CASE-1" {
		t.Errorf("unexpected SendCaseCreatedAlert args: %+v", gotChat)
	}
}

// TestDispatcher_Handle_CaseCreated_NonCaseTypesSkipChatButStillEmail verifies
// handleCaseCreated's CaseType gate: every non-"CASE" type (engagement,
// service_request, security_report_analysis, announcement) sends no Google
// Chat alert at all — explicit product direction, those types notify their
// audience by email only — while the email reaction is unaffected.
func TestDispatcher_Handle_CaseCreated_NonCaseTypesSkipChatButStillEmail(t *testing.T) {
	for _, caseType := range []string{"ENGAGEMENT", "SERVICE_REQUEST", "SECURITY_REPORT_ANALYSIS", "ANNOUNCEMENT"} {
		t.Run(caseType, func(t *testing.T) {
			email := &mockEmailSender{}
			chat := &mockGoogleChatSender{}
			d := newTestDispatcher(email, chat, &mockCallSender{})

			record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"` + caseType + `","priority":"","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

			if err := d.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			if len(chat.caseCreatedCalls) != 0 {
				t.Errorf("expected SendCaseCreatedAlert NOT to be called for a %s case, got %d call(s)", caseType, len(chat.caseCreatedCalls))
			}
			if len(chat.securityReportAnalysisCalls) != 0 {
				t.Errorf("expected SendSecurityReportAnalysisAlert NOT to be called for a %s case, got %d call(s)", caseType, len(chat.securityReportAnalysisCalls))
			}
			if len(email.calls) != 1 {
				t.Errorf("expected the email reaction to still fire for a %s case, got %d call(s)", caseType, len(email.calls))
			}
		})
	}
}

// TestDispatcher_Handle_CaseCreated_LowSeveritySkipsChatButStillEmail verifies
// a LOW/S4-severity "case" sends no Google Chat alert — S4 is WSO2's own
// best-efforts support tier and doesn't warrant one — while the email
// reaction still fires normally, and a non-LOW severity is unaffected.
func TestDispatcher_Handle_CaseCreated_LowSeveritySkipsChatButStillEmail(t *testing.T) {
	testCases := []struct {
		name     string
		priority string
		wantChat bool
	}{
		{"LOW skips chat", "LOW", false},
		{"lowercase low still matches (case-insensitive)", "low", false},
		{"HIGH still sends chat", "HIGH", true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := &mockEmailSender{}
			chat := &mockGoogleChatSender{}
			d := newTestDispatcher(email, chat, &mockCallSender{})

			record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"C-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"C-1","caseTitle":"Something broke","caseType":"CASE","priority":"` + tc.priority + `","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

			if err := d.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			gotChat := len(chat.caseCreatedCalls) > 0
			if gotChat != tc.wantChat {
				t.Errorf("SendCaseCreatedAlert called = %v, want %v", gotChat, tc.wantChat)
			}
			if len(email.calls) != 1 {
				t.Errorf("expected the email reaction to still fire, got %d call(s)", len(email.calls))
			}
		})
	}
}

// TestDispatcher_Handle_CaseCreated_EmailShowsHumanReadableCaseType verifies
// the "Case Type" row in the case-created email shows a reader-friendly
// label (e.g. "Security Report Analysis"), not entity-service's raw
// UPPER_SNAKE_CASE wire value — a real reported issue where a recipient saw
// "SECURITY_REPORT_ANALYSIS" verbatim in their inbox.
func TestDispatcher_Handle_CaseCreated_EmailShowsHumanReadableCaseType(t *testing.T) {
	testCases := []struct {
		wire  string
		label string
	}{
		{"CASE", "Case"},
		{"ENGAGEMENT", "Engagement"},
		{"SERVICE_REQUEST", "Service Request"},
		{"SECURITY_REPORT_ANALYSIS", "Security Report Analysis"},
		{"ANNOUNCEMENT", "Announcement"},
	}
	for _, tc := range testCases {
		t.Run(tc.wire, func(t *testing.T) {
			email := &mockEmailSender{}
			d := newTestDispatcher(email, &mockGoogleChatSender{}, &mockCallSender{})

			// entityId/caseId deliberately avoid the substring "CASE" (unlike
			// this file's other fixtures), so the "no raw wire value" check
			// below can't false-positive against it when tc.wire is "CASE".
			record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"C-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"C-1","caseTitle":"Something broke","caseType":"` + tc.wire + `","priority":"","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

			if err := d.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(email.calls) != 1 {
				t.Fatalf("expected 1 email sent, got %d", len(email.calls))
			}
			body := email.calls[0].htmlBody
			if !strings.Contains(body, tc.label) {
				t.Errorf("rendered email doesn't contain the human-readable label %q", tc.label)
			}
			if strings.Contains(body, tc.wire) {
				t.Errorf("rendered email still contains the raw wire value %q", tc.wire)
			}
		})
	}
}

// TestDispatcher_Handle_CaseCreated_ChatFailureStillSendsEmail verifies the
// two reactions are independent, the same as
// TestDispatcher_Handle_IncidentCreated_ChatFailureStillPlacesCall.
func TestDispatcher_Handle_CaseCreated_ChatFailureStillSendsEmail(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the chat error to propagate")
	}
	if len(mock.calls) != 1 {
		t.Fatal("expected the email to still be sent despite the chat failure")
	}
}

// TestDispatcher_Handle_CaseCreated_RetryDoesNotResendSucceededChatAlert is a
// regression test for a real production incident: a case.created record
// whose email step keeps failing exhausts eventbus.Consumer's 3 main-topic
// attempts and gets dead-lettered — same Value, but a brand new
// topic/partition/offset (see cmd/server/main.go's OnExhausted and
// recordBaseKey's own doc comment) — then goes through the DLQ topic's own
// 3 attempts. The already-succeeded Chat alert must be sent exactly once
// across all 6 attempts, on both topics — not resent once the DLQ takes
// over, which is exactly the bug this reproduces: an earlier version keyed
// idempotency off Kafka coordinates and force-forgot on the main topic's
// own IsFinalAttempt, so the DLQ's first attempt saw an unclaimed key and
// reposted the alert.
func TestDispatcher_Handle_CaseCreated_RetryDoesNotResendSucceededChatAlert(t *testing.T) {
	mock := &mockEmailSender{err: errors.New("email service unreachable")}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	value := []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)

	mainRecord := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: value}
	for attempt := 1; attempt <= 3; attempt++ {
		// NoMoreRetries stays false throughout: the main topic's own
		// OnExhausted dead-letters on exhaustion, so there is always one
		// more tier of attempts coming for this content, even on the main
		// topic's own last attempt.
		mainRecord.IsFinalAttempt = attempt == 3
		if err := d.Handle(context.Background(), mainRecord); err == nil {
			t.Fatalf("main attempt %d: expected the email error to still propagate", attempt)
		}
	}
	if len(chat.caseCreatedCalls) != 1 {
		t.Fatalf("chat sent %d times across 3 main-topic attempts, want exactly 1", len(chat.caseCreatedCalls))
	}
	if len(d.done) == 0 {
		t.Fatal("done map is empty after the main topic's final attempt — the Chat claim must survive to protect the upcoming DLQ delivery")
	}

	// Same content, dead-lettered: new topic/partition/offset, identical
	// Value — recordBaseKey must resolve to the same key as mainRecord's.
	dlqRecord := eventbus.Record{Topic: "case-events-dlq", Partition: 0, Offset: 99, Value: value}
	for attempt := 1; attempt <= 3; attempt++ {
		dlqRecord.IsFinalAttempt = attempt == 3
		dlqRecord.NoMoreRetries = attempt == 3 // the DLQ topic's Consumer has no OnExhausted of its own.
		if err := d.Handle(context.Background(), dlqRecord); err == nil {
			t.Fatalf("dlq attempt %d: expected the email error to still propagate", attempt)
		}
	}

	if len(chat.caseCreatedCalls) != 1 {
		t.Errorf("chat sent %d times across main+DLQ attempts, want exactly 1 (the DLQ redelivery must not resend an already-succeeded Chat alert)", len(chat.caseCreatedCalls))
	}
	if len(mock.calls) != 6 {
		t.Errorf("email attempted %d times across main+DLQ attempts, want 6 (3 main + 3 DLQ, the genuinely failing channel should keep retrying on both)", len(mock.calls))
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the DLQ's own final attempt, has %d entries (leaked tracking for a channel that never succeeded anywhere)", len(d.done))
	}
}

// TestDispatcher_Handle_CaseCreated_ForgetsAfterFullSuccess is a regression
// test for the other direction: once both the email and the Chat alert
// succeed, the tracking entry must not leak forever, and a later,
// unrelated record must not be affected by stale tracking.
func TestDispatcher_Handle_CaseCreated_ForgetsAfterFullSuccess(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

// TestDispatcher_Handle_EmailDebugMode_RedirectsToConfiguredRecipients
// verifies EMAIL_DEBUG_MODE: Handle still succeeds and still resolves
// recipient links against the event's real recipients (a broken link
// resolver should still surface as an error, debug mode or not), but
// SendEmail is called with emailDebugRecipients instead of the real
// resolved recipients.
func TestDispatcher_Handle_EmailDebugMode_RedirectsToConfiguredRecipients(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{}
	debugRecipients := []string{"debug-1@example.com", "debug-2@example.com"}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: debugRecipients, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email still sent in debug mode, got %d", len(mock.calls))
	}
	if got := mock.calls[0].to; len(got) != 2 || got[0] != "debug-1@example.com" || got[1] != "debug-2@example.com" {
		t.Errorf("to = %v, want the configured debug recipients %v", got, debugRecipients)
	}
	if len(links.gotEmails) != 1 || links.gotEmails[0] != testRecipient {
		t.Errorf("expected link resolution to still run against the real recipient in debug mode, gotEmails = %v", links.gotEmails)
	}
}

// TestDispatcher_Handle_EmailDebugMode_MultipleGroups_SendsOnePerGroup
// verifies that a case with recipients spanning two portals (customer-role +
// CSM-role) still sends one real, separately-rendered email per group even
// in EMAIL_DEBUG_MODE, each redirected to the configured debug recipients
// independently — mirroring exactly what production would send to the two
// real, different audiences. An earlier attempt at this service merged every
// group's already-fully-rendered HTML document into one email body, which
// produced invalid HTML (multiple concatenated <html> documents) and broke
// the logo image — reverted in favor of this simpler, correct-by-mirroring
// behavior instead.
func TestDispatcher_Handle_EmailDebugMode_MultipleGroups_SendsOnePerGroup(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{
		linkFor: func(email string) string {
			if email == "customer@example.com" {
				return "https://customer.example/projects/PROJ-1/support/cases/CASE-1"
			}
			return "https://csm.example/cases/CASE-1"
		},
	}
	debugRecipients := []string{"debug-1@example.com", "debug-2@example.com"}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: debugRecipients, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["customer@example.com","csm-agent@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 separate emails (one per real group) in debug mode, got %d", len(mock.calls))
	}
	for _, call := range mock.calls {
		if len(call.to) != 2 || call.to[0] != "debug-1@example.com" || call.to[1] != "debug-2@example.com" {
			t.Errorf("to = %v, want the configured debug recipients", call.to)
		}
	}
}

// TestDispatcher_Handle_EmailDebugMode_NoRecipientsConfigured_SkipsSend
// verifies that EMAIL_DEBUG_MODE=true with an empty EMAIL_DEBUG_RECIPIENTS
// (misconfigured) skips SendEmail entirely rather than calling it with zero
// recipients.
func TestDispatcher_Handle_EmailDebugMode_NoRecipientsConfigured_SkipsSend(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, EmailDebugMode: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("expected 0 emails sent with no debug recipients configured, got %d", len(mock.calls))
	}
}

func TestDispatcher_Handle_CommentAdded(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	if !strings.Contains(mock.calls[0].htmlBody, "fixed it") {
		t.Error("htmlBody does not contain the comment text")
	}
}

// TestDispatcher_Handle_CommentAdded_InlineImage verifies a comment
// containing an inline (data: URI) image ends up sent as a real inline
// EmailAttachment with a matching Content-ID, referenced from the email
// body as cid:<contentId> — never as the original data: URI, which Gmail
// and most major webmail clients strip from received HTML on render.
func TestDispatcher_Handle_CommentAdded_InlineImage(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	const dataURI = "data:image/png;base64,aGVsbG8="
	payload := `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"<p>see attached<img src=\"` + dataURI + `\"></p>","commentId":"C-1","recipients":["test-recipient@example.com"]}}`
	record := eventbus.Record{Value: []byte(payload)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	if strings.Contains(call.htmlBody, "data:image") {
		t.Error("htmlBody must never contain the original data: URI")
	}
	if len(call.attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(call.attachments))
	}
	att := call.attachments[0]
	if !att.Inline || att.ContentID == "" {
		t.Errorf("attachment not marked inline with a Content-ID: %+v", att)
	}
	if !strings.Contains(call.htmlBody, "cid:"+att.ContentID) {
		t.Errorf("htmlBody does not reference cid:%s", att.ContentID)
	}
	if att.ContentType != "image/png" || string(att.Attachment) != "hello" {
		t.Errorf("attachment bytes/type not carried through correctly: %+v", att)
	}
}

// TestDispatcher_Handle_CommentAdded_InternalNote_UsesInternalNoteLayout
// verifies that isInternalNote:true routes through RenderInternalNoteEmail
// instead of RenderCommentAddedEmail: the "added work note" wording (not
// "commented on case"), no "Re: <title>" strap, and wso2CaseId (not
// caseNumber) as the case reference — see
// events.CommentAddedPayload.IsInternalNote's own doc comment for why.
func TestDispatcher_Handle_CommentAdded_InternalNote_UsesInternalNoteLayout(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Agent","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","caseComment":"internal only","commentId":"C-1","isInternalNote":true,"recipients":["agent@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	body := mock.calls[0].htmlBody
	if !strings.Contains(body, "added work note") {
		t.Error("htmlBody does not use the internal-note wording")
	}
	if strings.Contains(body, "Re: Something broke") {
		t.Error("htmlBody carries the regular layout's \"Re: <title>\" strap, which the internal-note layout must not have")
	}
	if !strings.Contains(body, "WSO2-1000") {
		t.Error("htmlBody does not use wso2CaseId as the case reference")
	}
	if strings.Contains(body, "CS0001001") {
		t.Error("htmlBody uses caseNumber as the case reference, want wso2CaseId for an internal note")
	}
	if !strings.Contains(body, "internal only") {
		t.Error("htmlBody does not contain the note's own content")
	}
}

// TestDispatcher_Handle_CommentAdded_LinksToCommentFragment verifies
// commentLinkFor's suffix actually reaches the rendered email: the "Add
// Comment" CTA must link to <resolved case link>#<commentId>, matching the
// CSM portal frontend's own comment-permalink format.
func TestDispatcher_Handle_CommentAdded_LinksToCommentFragment(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(string) string { return "https://csm.example.com/cases/CASE-1" }}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	want := "https://csm.example.com/cases/CASE-1#C-1"
	if !strings.Contains(mock.calls[0].htmlBody, want) {
		t.Errorf("htmlBody does not contain the comment permalink %q", want)
	}
}

func TestDispatcher_Handle_StatusChanged(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Work In Progress","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(mock.calls[0].subject, "Work In Progress") {
		t.Errorf("subject = %q, want it to contain the new status", mock.calls[0].subject)
	}
}

func TestDispatcher_Handle_CaseAssigned(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.assigned","entityId":"CASE-1","payload":{"assigneeName":"Assignee","assigneeEmail":"assignee@example.com","projectId":"PROJ-1","caseId":"CASE-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(mock.calls[0].htmlBody, "assignee@example.com") {
		t.Error("htmlBody does not contain the assignee's email")
	}
}

// TestDispatcher_Handle_CaseAcknowledged verifies the happy path: a
// case.acknowledged event posts exactly one Google Chat alert (no email —
// see events.CaseAcknowledgedPayload's own doc comment) with the severity
// mapped to its display label/color, the case reference, and the
// acknowledger's name.
func TestDispatcher_Handle_CaseAcknowledged(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","severity":"CRITICAL","product":"api-manager","acknowledgerName":"Jane Doe"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Errorf("expected no email sent for case.acknowledged, got %d", len(mock.calls))
	}
	if len(chat.caseAcknowledgedCalls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.caseAcknowledgedCalls))
	}
	got := chat.caseAcknowledgedCalls[0]
	if got.audience != chataudience.IncidentMonitor || got.severityLabel != "Critical (P1)" || got.caseNumber != "CS0001001" ||
		got.wso2CaseID != "WSO2-1000" || got.acknowledgerName != "Jane Doe" {
		t.Errorf("unexpected SendCaseAcknowledgedAlert args: %+v", got)
	}
}

// TestDispatcher_Handle_CaseAcknowledged_BlankSeverityRendersUnknownNotEmpty
// is a regression test: severity is optional on CaseAcknowledgedPayload, and
// severityLabelAndColor used to return the empty string verbatim for a blank
// input, rendering an empty <font>...</font> element in the Chat card
// instead of something readable.
func TestDispatcher_Handle_CaseAcknowledged_BlankSeverityRendersUnknownNotEmpty(t *testing.T) {
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","product":"api-manager","acknowledgerName":"Jane Doe"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(chat.caseAcknowledgedCalls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.caseAcknowledgedCalls))
	}
	if got := chat.caseAcknowledgedCalls[0].severityLabel; got != "Unknown" {
		t.Errorf("severityLabel = %q, want %q for a blank severity", got, "Unknown")
	}
}

// TestDispatcher_Handle_CaseAcknowledged_ForgetsAfterFullSuccess mirrors
// TestDispatcher_Handle_CaseCreated_ForgetsAfterFullSuccess: once
// case.acknowledged's one and only reaction (the Chat alert) succeeds, its
// claim must not leak in d.done forever.
func TestDispatcher_Handle_CaseAcknowledged_ForgetsAfterFullSuccess(t *testing.T) {
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","severity":"HIGH","product":"api-manager","acknowledgerName":"Jane Doe"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

// TestDispatcher_Handle_CaseAcknowledged_RetryAfterChatFailureResends
// verifies the actual retry contract for this single-channel handler: a
// Chat send that fails releases its claim (via claim's own failure path,
// not record.NoMoreRetries — see handleCaseAcknowledged's own doc comment
// for why there's no cross-call release race to guard against here), so a
// subsequent retry of the exact same record content genuinely re-attempts
// it, and eventually succeeds once the failure clears.
func TestDispatcher_Handle_CaseAcknowledged_RetryAfterChatFailureResends(t *testing.T) {
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","severity":"HIGH","product":"api-manager","acknowledgerName":"Jane Doe"}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the chat error to propagate on the first attempt")
	}
	chat.err = nil
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("retry: Handle() error = %v, want nil once the failure clears", err)
	}
	// Both the failed first attempt and the succeeding retry actually
	// invoked SendCaseAcknowledgedAlert — claim's failure path releases the
	// key precisely so the retry genuinely re-attempts it, rather than
	// silently treating the record as already handled.
	if len(chat.caseAcknowledgedCalls) != 2 {
		t.Errorf("expected 2 send attempts (1 failed, 1 succeeded), got %d", len(chat.caseAcknowledgedCalls))
	}
}

func TestDispatcher_Handle_SeverityChanged(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","oldSeverity":"HIGH","newSeverity":"LOW","product":"api-manager","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	gotEmail := mock.calls[0]
	if len(gotEmail.to) != 1 || gotEmail.to[0] != testRecipient {
		t.Errorf("to = %v, want [%s]", gotEmail.to, testRecipient)
	}
	if !strings.Contains(gotEmail.htmlBody, "High(S2)") || !strings.Contains(gotEmail.htmlBody, "Low(S4)") {
		t.Error("htmlBody does not contain both the old and new severity labels")
	}

	if len(chat.severityChangedCalls) != 1 {
		t.Fatalf("expected 1 Google Chat alert sent, got %d", len(chat.severityChangedCalls))
	}
	gotChat := chat.severityChangedCalls[0]
	if gotChat.audience != chataudience.IncidentMonitor || gotChat.oldSeverityLabel != "High (P2)" || gotChat.newSeverityLabel != "Low (P4)" || gotChat.caseLink != "https://csm.example/cases/CASE-1" {
		t.Errorf("unexpected SendSeverityChangedAlert args: %+v", gotChat)
	}
}

// TestDispatcher_Handle_SeverityChanged_ChatFailureStillSendsEmail verifies
// the two reactions are independent, mirroring
// TestDispatcher_Handle_CaseCreated_ChatFailureStillSendsEmail.
func TestDispatcher_Handle_SeverityChanged_ChatFailureStillSendsEmail(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{err: errors.New("webhook unreachable")}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","product":"api-manager","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the chat error to propagate")
	}
	if len(mock.calls) != 1 {
		t.Fatal("expected the email to still be sent despite the chat failure")
	}
}

// TestDispatcher_Handle_SeverityChanged_EmailFailureStillSendsChat is the
// mirror image of ChatFailureStillSendsEmail above — a failing email
// (groupByLink itself, here) must not suppress the independent Chat alert,
// same "both attempted even if one fails" reasoning handleCaseCreated's
// own Chat block uses.
func TestDispatcher_Handle_SeverityChanged_EmailFailureStillSendsChat(t *testing.T) {
	chat := &mockGoogleChatSender{}
	links := &mockLinkResolver{err: errors.New("entity-service unreachable")}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: chat, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","product":"api-manager","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the link-resolution error to propagate")
	}
	if len(chat.severityChangedCalls) != 1 {
		t.Fatalf("expected the chat alert to still be sent despite the email failure, got %d calls", len(chat.severityChangedCalls))
	}
}

// TestDispatcher_Handle_SeverityChanged_ForgetsAfterFullSuccess mirrors
// TestDispatcher_Handle_CaseCreated_ForgetsAfterFullSuccess.
func TestDispatcher_Handle_SeverityChanged_ForgetsAfterFullSuccess(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := newTestDispatcher(mock, chat, &mockCallSender{})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(`{"type":"case.severity_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","product":"api-manager","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

// TestDispatcher_Handle_TwoRecipientsTwoLinks_SendsTwoEmails is the core
// regression test for the recipientlinks migration: a comment-added event
// with two recipients that resolve to two different portal links must
// result in two separate SendEmail calls, each to only the recipient(s) who
// resolved to that link, each body carrying that link — not one shared
// email with one link for everyone.
func TestDispatcher_Handle_TwoRecipientsTwoLinks_SendsTwoEmails(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(email string) string {
		if email == "customer@acme.com" {
			return "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1"
		}
		return "https://csm.example.com/cases/CASE-1"
	}}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["customer@acme.com","agent@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 emails sent (one per resolved link), got %d", len(mock.calls))
	}
	byRecipient := make(map[string]sentEmail)
	for _, call := range mock.calls {
		if len(call.to) != 1 {
			t.Fatalf("each group should have exactly 1 recipient here, got %v", call.to)
		}
		byRecipient[call.to[0]] = call
	}
	customerEmail, ok := byRecipient["customer@acme.com"]
	if !ok {
		t.Fatal("no email sent to customer@acme.com")
	}
	if !strings.Contains(customerEmail.htmlBody, "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1") {
		t.Errorf("customer email body = %q, want the customer portal link", customerEmail.htmlBody)
	}
	agentEmail, ok := byRecipient["agent@wso2.com"]
	if !ok {
		t.Fatal("no email sent to agent@wso2.com")
	}
	if !strings.Contains(agentEmail.htmlBody, "https://csm.example.com/cases/CASE-1") {
		t.Errorf("agent email body = %q, want the CSM portal link", agentEmail.htmlBody)
	}
}

// TestDispatcher_Handle_DefaultCSMEmailCC_OnlyOnCSMGroup is the regression
// test for Dispatcher.defaultCSMEmailCC: a comment-added event with
// recipients spanning both the customer-portal and CSM-portal groups must
// CC the configured default only on the CSM-portal group's own SendEmail
// call — the customer-portal group's call must carry no cc at all.
func TestDispatcher_Handle_DefaultCSMEmailCC_OnlyOnCSMGroup(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(email string) string {
		if email == "customer@acme.com" {
			return "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1"
		}
		// Must match (&mockLinkResolver{}).CSMLink("CASE-1") exactly --
		// that's the value sendPerGroup compares caseLink against to
		// decide which group is the CSM one.
		return "https://csm.example/cases/CASE-1"
	}}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true, DefaultCSMEmailCC: []string{"cc@wso2.com"}})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["customer@acme.com","agent@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 emails sent, got %d", len(mock.calls))
	}
	byRecipient := make(map[string]sentEmail)
	for _, call := range mock.calls {
		byRecipient[call.to[0]] = call
	}
	csmEmail, ok := byRecipient["agent@wso2.com"]
	if !ok {
		t.Fatal("no email sent to agent@wso2.com")
	}
	if !slices.Equal(csmEmail.cc, []string{"cc@wso2.com"}) {
		t.Errorf("CSM group cc = %v, want [cc@wso2.com]", csmEmail.cc)
	}
	customerEmail, ok := byRecipient["customer@acme.com"]
	if !ok {
		t.Fatal("no email sent to customer@acme.com")
	}
	if customerEmail.cc != nil {
		t.Errorf("customer group cc = %v, want nil -- the default CC must never reach the customer-portal group", customerEmail.cc)
	}
}

// TestDispatcher_Handle_DefaultCSMEmailCC_SuppressedInDebugMode: even for
// the CSM-portal group, EMAIL_DEBUG_MODE must suppress the default CC
// entirely rather than redirect it — a debug run must never leak to the
// real, shared default inbox just because `to` is being redirected to a
// safe test list.
func TestDispatcher_Handle_DefaultCSMEmailCC_SuppressedInDebugMode(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{linkFor: func(string) string { return "https://csm.example/cases/CASE-1" }}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: []string{"debug@wso2.com"}, CallSendingEnabled: true, DefaultCSMEmailCC: []string{"cc@wso2.com"}})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["agent@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
	}
	if mock.calls[0].cc != nil {
		t.Errorf("cc = %v, want nil -- EMAIL_DEBUG_MODE must suppress the default CC, not redirect it", mock.calls[0].cc)
	}
}

// TestDispatcher_Handle_CommentAdded_RetryDoesNotResendSucceededGroup is a
// regression test: eventbus.Consumer retries the whole Handle call on any
// error. When one recipient group's SendEmail persistently fails while
// another group's already succeeded, the succeeded group must not be
// resent on every retry — only the genuinely failing group should keep
// being attempted. record.NoMoreRetries (not IsFinalAttempt — see
// recordBaseKey/NoMoreRetries' own doc comments) is set true on the last of
// these 3 attempts to simulate this being the last tier with nowhere
// further to retry (e.g. the DLQ topic's own final attempt), so the test
// can also verify no tracking leaks once that's genuinely true.
func TestDispatcher_Handle_CommentAdded_RetryDoesNotResendSucceededGroup(t *testing.T) {
	mock := &mockEmailSender{errFor: func(to []string) error {
		if len(to) == 1 && to[0] == "agent@wso2.com" {
			return errors.New("email service unreachable")
		}
		return nil
	}}
	links := &mockLinkResolver{linkFor: func(email string) string {
		if email == "customer@acme.com" {
			return "https://customer.example.com/projects/PROJ-1/support/cases/CASE-1"
		}
		return "https://csm.example.com/cases/CASE-1"
	}}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["customer@acme.com","agent@wso2.com"]}}`)}

	for attempt := 1; attempt <= 3; attempt++ {
		record.NoMoreRetries = attempt == 3
		if err := d.Handle(context.Background(), record); err == nil {
			t.Fatalf("attempt %d: expected the agent group's error to still propagate", attempt)
		}
	}

	customerSends, agentSends := 0, 0
	for _, call := range mock.calls {
		if len(call.to) == 1 && call.to[0] == "customer@acme.com" {
			customerSends++
		}
		if len(call.to) == 1 && call.to[0] == "agent@wso2.com" {
			agentSends++
		}
	}
	if customerSends != 1 {
		t.Errorf("customer group sent %d times across 3 retries, want exactly 1 (already succeeded, should not be resent)", customerSends)
	}
	if agentSends != 3 {
		t.Errorf("agent group attempted %d times across 3 retries, want 3 (the genuinely failing group should keep retrying)", agentSends)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the final attempt, has %d entries (leaked tracking for a group that never succeeded)", len(d.done))
	}
}

// TestDispatcher_Handle_TwoRecipientsSameLink_SendsOneEmail proves
// groupByLink batches recipients sharing a resolved link into a single
// SendEmail call, rather than fanning out one call per recipient
// regardless of link.
func TestDispatcher_Handle_TwoRecipientsSameLink_SendsOneEmail(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["agent1@wso2.com","agent2@wso2.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 email sent (both recipients share a link), got %d", len(mock.calls))
	}
	if len(mock.calls[0].to) != 2 {
		t.Errorf("to = %v, want both recipients batched into the one call", mock.calls[0].to)
	}
}

// TestDispatcher_Handle_ResolveLinksFails_NoEmailSent verifies a
// recipientlinks failure (e.g. entity-service unreachable) fails the whole
// record rather than silently sending to a wrong/default link.
func TestDispatcher_Handle_ResolveLinksFails_NoEmailSent(t *testing.T) {
	mock := &mockEmailSender{}
	links := &mockLinkResolver{err: errors.New("entity-service unreachable")}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: links}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the resolver error to propagate")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called when link resolution fails")
	}
}

// TestDispatcher_Handle_EmptyRecipients exercises events.Validate, the only
// validation boundary this service has left (see Handle's doc comment) —
// Dispatcher.groupByLink has its own defensive backstop for the same case,
// but Validate should reject this before groupByLink is ever reached.
func TestDispatcher_Handle_EmptyRecipients(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":[]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error when the payload's recipients list is empty")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called when recipients is empty")
	}
}

func TestDispatcher_Handle_InvalidPayload_MissingRequiredField(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for a payload missing newStatus")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called for an invalid payload")
	}
}

func TestDispatcher_Handle_InvalidPayload_EntityIDMismatch(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-2","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error when the payload's caseId disagrees with the envelope's entityId")
	}
	if len(mock.calls) != 0 {
		t.Error("SendEmail should not be called for a mismatched entityId/caseId")
	}
}

func TestDispatcher_Handle_UnknownType(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.deleted","entityId":"CASE-1","payload":{}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for an unknown event type")
	}
}

func TestDispatcher_Handle_MalformedEnvelope(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`not json`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected an error for a malformed envelope")
	}
}

func TestDispatcher_Handle_SendFailurePropagates(t *testing.T) {
	mock := &mockEmailSender{err: context.DeadlineExceeded}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err == nil {
		t.Fatal("expected the underlying SendEmail error to propagate")
	}
}

const validIncidentRecord = `{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","callTo":"+15551234567"}}`

func TestDispatcher_Handle_IncidentCreated(t *testing.T) {
	call := &mockCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(call.calls) != 1 {
		t.Fatalf("expected 1 call placed, got %d", len(call.calls))
	}
	gotCall := call.calls[0]
	if gotCall.to != "+15551234567" {
		t.Errorf("call to = %q, want %q", gotCall.to, "+15551234567")
	}
	if !strings.Contains(gotCall.message, "P1 outage") || !strings.Contains(gotCall.message, "Everything is down") {
		t.Errorf("call message = %q, want it to mention the title and description", gotCall.message)
	}
}

// TestDispatcher_Handle_IncidentCreated_RetryDoesNotResendSucceededCall is a
// regression test: eventbus.Consumer retries the whole Handle call on any
// error. A call that fails releases its claim (via claim's own failure
// path), so a retry genuinely re-attempts it, and once there's truly no
// further retry coming, record.NoMoreRetries (not IsFinalAttempt — see
// NoMoreRetries' own doc comment for why) releases its tracking entry so it
// doesn't stay in d.done forever.
func TestDispatcher_Handle_IncidentCreated_RetryDoesNotResendSucceededCall(t *testing.T) {
	call := &mockCallSender{err: errors.New("twilio unreachable")}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(validIncidentRecord)}

	for attempt := 1; attempt <= 3; attempt++ {
		record.NoMoreRetries = attempt == 3
		if err := d.Handle(context.Background(), record); err == nil {
			t.Fatalf("attempt %d: expected the call error to still propagate", attempt)
		}
	}

	if len(call.calls) != 3 {
		t.Errorf("call attempted %d times across 3 retries, want 3 (the genuinely failing channel should keep retrying)", len(call.calls))
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the final attempt, has %d entries (leaked tracking for a channel that never succeeded)", len(d.done))
	}
}

// TestDispatcher_Handle_IncidentCreated_ForgetsAfterFullSuccess is a
// regression test for the other direction: once the call succeeds, its
// tracking entry must not leak in d.done forever.
func TestDispatcher_Handle_IncidentCreated_ForgetsAfterFullSuccess(t *testing.T) {
	call := &mockCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after full success, has %d entries", len(d.done))
	}
}

// TestDispatcher_Handle_IncidentCreated_UsesDefaultWhenCallToOmitted
// verifies a publisher that can't determine the on-call number itself
// (e.g. entity-service) can omit callTo, and Dispatcher's own configured
// default is used instead.
func TestDispatcher_Handle_IncidentCreated_UsesDefaultWhenCallToOmitted(t *testing.T) {
	call := &mockCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true, DefaultOnCallNumber: "+15559998888"})

	record := eventbus.Record{Value: []byte(`{"type":"incident.created","entityId":"INC-1","payload":{"title":"P1 outage","shortDescription":"Everything is down"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(call.calls) != 1 || call.calls[0].to != "+15559998888" {
		t.Fatalf("expected the call to use the default callTo, got %+v", call.calls)
	}
}

// TestDispatcher_Handle_IncidentCreated_SkipsCallWhenNoCallTo verifies that
// when both the payload's callTo and INCIDENT_DEFAULT_CALL_TO are empty (and
// calling is otherwise enabled), the call is skipped instead of being
// attempted with an empty destination, and Handle still succeeds.
func TestDispatcher_Handle_IncidentCreated_SkipsCallWhenNoCallTo(t *testing.T) {
	call := &mockCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"incident.created","entityId":"INC-1","payload":{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down"}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(call.calls) != 0 {
		t.Errorf("expected no call with no callTo resolved, got %d calls", len(call.calls))
	}
}

// TestDispatcher_Handle_IncidentCreated_CallSendingDisabled verifies the
// CALL_SENDING_ENABLED killswitch: Handle still succeeds, but MakeCall is
// never invoked.
func TestDispatcher_Handle_IncidentCreated_CallSendingDisabled(t *testing.T) {
	call := &mockCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true})

	record := eventbus.Record{Value: []byte(validIncidentRecord)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(call.calls) != 0 {
		t.Errorf("expected MakeCall to never be invoked while disabled, got %d calls", len(call.calls))
	}
}

// TestDispatcher_Handle_CaseCreated_EmailSendingDisabled verifies the
// EMAIL_SENDING_ENABLED killswitch: Handle still succeeds and the Google
// Chat alert still sends, but SendEmail is never invoked, and the group is
// still tracked as done (a retry doesn't send it once re-enabled either,
// matching CALL_SENDING_ENABLED's own disable-entirely shape) — unlike
// EMAIL_DEBUG_MODE, which still sends, just redirected.
func TestDispatcher_Handle_CaseCreated_EmailSendingDisabled(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: chat, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Errorf("expected SendEmail to never be invoked while disabled, got %d calls", len(mock.calls))
	}
	if len(chat.caseCreatedCalls) != 1 {
		t.Errorf("expected the chat alert to still be sent, got %d calls", len(chat.caseCreatedCalls))
	}
}

// TestDispatcher_Handle_IgnoresSLATierReached verifies that a
// sla.tier_reached record — published by internal/slaengine's own poller,
// which this dispatcher's consumers still get a full copy of via the shared
// topic — is a silent no-op here, not an error. Erroring would burn this
// consumer's retries and dead-letter an event that was never broken.
func TestDispatcher_Handle_IgnoresSLATierReached(t *testing.T) {
	mock := &mockEmailSender{}
	chat := &mockGoogleChatSender{}
	call := &mockCallSender{}
	d := newTestDispatcher(mock, chat, call)

	record := `{"type":"sla.tier_reached","entityId":"CASE-1","payload":{"caseId":"CASE-1","clockType":"response","tier":"50"}}`
	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(record)}); err != nil {
		t.Errorf("Handle(%s) error = %v, want nil", record, err)
	}
	chatCalls := len(chat.caseCreatedCalls) + len(chat.caseAcknowledgedCalls) + len(chat.severityChangedCalls) + len(chat.securityReportAnalysisCalls)
	if len(mock.calls) != 0 || chatCalls != 0 || len(call.calls) != 0 {
		t.Errorf("expected no notification sent, got email=%d chat=%d call=%d", len(mock.calls), chatCalls, len(call.calls))
	}
}

// TestMaskPhone verifies only the last 4 characters of a phone number
// survive in a log line — the rest must never be recoverable from the
// masked output.
func TestMaskPhone(t *testing.T) {
	tests := []struct {
		phone string
		want  string
	}{
		{"+15551234567", "********4567"},
		{"1234", "****"},
		{"12", "**"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := maskPhone(tt.phone); got != tt.want {
			t.Errorf("maskPhone(%q) = %q, want %q", tt.phone, got, tt.want)
		}
	}
}

// concurrencyProbeCallSender is a callSender that records the highest number
// of MakeCall calls it ever had in flight at once — the invariant
// TestDispatcher_Handle_ConcurrentClaimNeverOverlaps checks — by sleeping
// briefly inside the "critical section" to widen the window a race would
// need to land in. It deliberately does NOT check the total call count: once
// a call completes and Dispatcher.forget releases its claim (the same
// eager-cleanup-on-full-success behavior
// TestDispatcher_Handle_IncidentCreated_ForgetsAfterFullSuccess already pins
// as intentional), a later, independent Handle call legitimately reclaims
// and re-places the call — that is not the race being tested here.
type concurrencyProbeCallSender struct {
	mu        sync.Mutex
	active    int
	maxActive int
	sends     int
}

func (s *concurrencyProbeCallSender) MakeCall(ctx context.Context, to, message string) error {
	s.mu.Lock()
	s.active++
	s.sends++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	return nil
}

// TestDispatcher_Handle_ConcurrentClaimNeverOverlaps is a regression test
// for a real race in the old alreadyDone/markDone pattern: checking "not
// done yet" and marking "done" were two separate lock acquisitions, so two
// Handle calls racing on the same record (e.g. during a Kafka
// consumer-group rebalance transition, which this client's fencing isn't
// guaranteed to make impossible) could both observe an unclaimed channel
// and both attempt the same outbound call before either one recorded it as
// claimed. claim() closes this by checking-and-setting in one lock
// acquisition, so at most one caller is ever inside the send at once —
// concurrencyProbeCallSender's maxActive is what this test actually
// verifies, deliberately not the total send count (see its own doc
// comment for why that's a separate, already-accepted behavior).
func TestDispatcher_Handle_ConcurrentClaimNeverOverlaps(t *testing.T) {
	call := &concurrencyProbeCallSender{}
	d := NewDispatcher(Deps{Email: &mockEmailSender{}, GoogleChat: &mockGoogleChatSender{}, Call: call, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 42, Value: []byte(validIncidentRecord)}

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_ = d.Handle(context.Background(), record)
		}()
	}
	wg.Wait()

	call.mu.Lock()
	defer call.mu.Unlock()
	if call.maxActive > 1 {
		t.Errorf("maxActive = %d, want at most 1 (two Handle calls were inside MakeCall at the same time — claim() failed to serialize them)", call.maxActive)
	}
	if call.sends < 1 {
		t.Error("expected at least one call to have been placed")
	}
}

// blockingCaseAcknowledgedChatSender's SendCaseAcknowledgedAlert blocks on
// proceed until the test closes it, mirroring blockingEmailSender below for
// handleCaseAcknowledged's single-channel claim race.
type blockingCaseAcknowledgedChatSender struct {
	proceed chan struct{}
	calls   int32
}

func (s *blockingCaseAcknowledgedChatSender) SendCaseCreatedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	return nil
}

func (s *blockingCaseAcknowledgedChatSender) SendSecurityReportAnalysisAlert(ctx context.Context, product, caseNumber, wso2CaseID, productName, title, team, caseLink string) error {
	return nil
}

func (s *blockingCaseAcknowledgedChatSender) SendCaseAcknowledgedAlert(ctx context.Context, product, severityLabel, severityColor, caseNumber, wso2CaseID, caseLink, acknowledgerName string) error {
	atomic.AddInt32(&s.calls, 1)
	<-s.proceed
	return nil
}

func (s *blockingCaseAcknowledgedChatSender) SendSeverityChangedAlert(ctx context.Context, product, oldSeverityLabel, oldSeverityColor, newSeverityLabel, newSeverityColor, caseNumber, wso2CaseID, title, team, caseLink string) error {
	return nil
}

// TestDispatcher_Handle_CaseAcknowledged_LosingConcurrentCallDoesNotReleaseWinnersClaim
// is a regression test for the case.acknowledged analogue of
// TestDispatcher_Handle_LosingConcurrentCallDoesNotReleaseWinnersClaim below:
// handleCaseAcknowledged's release condition used to be
// "record.NoMoreRetries || chatOwned", so a losing concurrent call (one that
// never won the claim, chatOwned false) with NoMoreRetries true would force
// -release the key anyway — right out from under a different, still
// in-flight call genuinely blocked inside SendCaseAcknowledgedAlert. A third
// attempt could then reclaim and send a duplicate acknowledgement alert.
func TestDispatcher_Handle_CaseAcknowledged_LosingConcurrentCallDoesNotReleaseWinnersClaim(t *testing.T) {
	chat := &blockingCaseAcknowledgedChatSender{proceed: make(chan struct{})}
	d := newTestDispatcher(&mockEmailSender{}, chat, &mockCallSender{})

	record := eventbus.Record{Value: []byte(`{"type":"case.acknowledged","entityId":"CASE-1","payload":{"caseId":"CASE-1","caseNumber":"CS0001001","severity":"HIGH","product":"api-manager","acknowledgerName":"Jane Doe"}}`)}
	baseKey := recordBaseKey(record)

	winnerDone := make(chan struct{})
	go func() {
		defer close(winnerDone)
		if err := d.Handle(context.Background(), record); err != nil {
			t.Errorf("winner: Handle() error = %v, want nil", err)
		}
	}()

	for atomic.LoadInt32(&chat.calls) == 0 {
		runtime.Gosched()
	}

	// The loser: a second, concurrent Handle call for the exact same
	// record, with NoMoreRetries set — simulating a DLQ-exhausted delivery
	// racing the still in-flight main-topic winner. It must lose the claim
	// and, per the fix, must not force-release chatKey just because
	// NoMoreRetries is true.
	loserRecord := record
	loserRecord.NoMoreRetries = true
	if err := d.Handle(context.Background(), loserRecord); err != nil {
		t.Fatalf("loser: Handle() error = %v, want nil", err)
	}

	d.doneMu.Lock()
	chatStillClaimed := d.done[baseKey+"/chat"]
	d.doneMu.Unlock()
	if !chatStillClaimed {
		t.Fatal("chatKey was released by the losing call while the winner was still mid-SendCaseAcknowledgedAlert")
	}

	close(chat.proceed)
	<-winnerDone

	if atomic.LoadInt32(&chat.calls) != 1 {
		t.Errorf("chat sent %d times total, want exactly 1", chat.calls)
	}
}

// blockingEmailSender's SendEmail blocks on proceed until the test closes
// it, so a test can deterministically pause a Handle call mid-send instead
// of relying on a fixed sleep to widen a race window.
type blockingEmailSender struct {
	proceed chan struct{}
	calls   int32
}

func (s *blockingEmailSender) FromAddress() string { return "noreply@wso2.com" }

func (s *blockingEmailSender) SendEmailFrom(ctx context.Context, from string, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	return s.SendEmail(ctx, to, cc, bcc, replyTo, subject, htmlBody, attachments)
}

func (s *blockingEmailSender) SendEmail(ctx context.Context, to, cc, bcc, replyTo []string, subject, htmlBody string, attachments []notifications.EmailAttachment) error {
	atomic.AddInt32(&s.calls, 1)
	<-s.proceed
	return nil
}

// TestDispatcher_Handle_LosingConcurrentCallDoesNotReleaseWinnersClaim is a
// regression test for a real bug in forgetEmailGroups: it used to release
// every caseLink in the caller's own groups map, not just the ones
// sendPerGroup actually claimed this call. A second, concurrent Handle
// call for the exact same record loses the claim race (claim() correctly
// rejects the already-held key), attempts nothing, and so returns
// sendErr == nil — which used to be read as "the whole call succeeded,
// release every group" and would release the first call's still in-flight
// claim right out from under it. A third attempt (e.g. the next retry)
// could then reclaim and resend an email the first call hadn't even
// finished sending yet — the exact duplicate-email incident this was
// fixed in response to. sendPerGroup now returns which groups THIS call
// actually claimed (owned), and the caller releases only that set on this
// branch, so the losing call here must leave the winner's claim untouched.
func TestDispatcher_Handle_LosingConcurrentCallDoesNotReleaseWinnersClaim(t *testing.T) {
	mock := &blockingEmailSender{proceed: make(chan struct{})}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["agent@wso2.com"]}}`)}
	key := recordBaseKey(record) + "/email/https://csm.example/cases/CASE-1"

	winnerDone := make(chan struct{})
	go func() {
		defer close(winnerDone)
		if err := d.Handle(context.Background(), record); err != nil {
			t.Errorf("winner: Handle() error = %v, want nil", err)
		}
	}()

	// Wait until the winner has actually claimed the key and is blocked
	// inside SendEmail before running the loser.
	for atomic.LoadInt32(&mock.calls) == 0 {
		runtime.Gosched()
	}

	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("loser: Handle() error = %v, want nil (it should see nothing left to do)", err)
	}

	d.doneMu.Lock()
	stillClaimed := d.done[key]
	d.doneMu.Unlock()
	if !stillClaimed {
		t.Fatal("the losing concurrent call released the key while the winning call was still mid-send")
	}

	close(mock.proceed)
	<-winnerDone

	if calls := atomic.LoadInt32(&mock.calls); calls != 1 {
		t.Errorf("SendEmail called %d times, want exactly 1", calls)
	}
	if len(d.done) != 0 {
		t.Errorf("done map should be empty after the winning call finished successfully, has %d entries", len(d.done))
	}
}

// TestDispatcher_Handle_CaseCreated_ConcurrentBlockedEmailDoesNotDuplicateChat
// is a regression test for CodeRabbit's "heavy lift" finding: a call that
// loses the email claim race attempts nothing for email, so its own errs
// stays empty regardless of whether the record is actually done. The old
// release condition (chatOwned && no local errors) read that as "the whole
// record succeeded" and released chatKey while a different, still
// in-flight call was genuinely mid-SendEmail for the very same record —
// letting that call's own eventual success reclaim and resend the Chat
// alert. Here the winner claims email and blocks in SendEmail; the loser
// (a second, concurrent Handle call for the exact same record) only gets
// to attempt Chat. The loser must not release chatKey while the winner is
// still blocked, and the whole record must end up with exactly one Chat
// send once both calls have finished.
func TestDispatcher_Handle_CaseCreated_ConcurrentBlockedEmailDoesNotDuplicateChat(t *testing.T) {
	email := &blockingEmailSender{proceed: make(chan struct{})}
	chat := &mockGoogleChatSender{}
	d := NewDispatcher(Deps{Email: email, GoogleChat: chat, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, CallSendingEnabled: true})

	record := eventbus.Record{Value: []byte(`{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"Something broke","caseType":"Incident","priority":"P3","product":"api-manager","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`)}
	baseKey := recordBaseKey(record)

	winnerDone := make(chan struct{})
	go func() {
		defer close(winnerDone)
		if err := d.Handle(context.Background(), record); err != nil {
			t.Errorf("winner: Handle() error = %v, want nil", err)
		}
	}()

	// Wait until the winner has actually claimed the email group and is
	// blocked inside SendEmail before running the loser.
	for atomic.LoadInt32(&email.calls) == 0 {
		runtime.Gosched()
	}

	// The loser: a second, concurrent Handle call for the exact same
	// record. It loses the email claim (the winner already holds it) and
	// so is the one that gets to attempt the Chat alert.
	if err := d.Handle(context.Background(), record); err != nil {
		t.Fatalf("loser: Handle() error = %v, want nil", err)
	}

	chat.mu.Lock()
	sentSoFar := len(chat.caseCreatedCalls)
	chat.mu.Unlock()
	if sentSoFar != 1 {
		t.Fatalf("chat sent %d times before the winner finished, want exactly 1", sentSoFar)
	}

	// The bug this guards against: the loser releasing chatKey here (having
	// seen a "clean" record with no errors of its own) would let a later
	// attempt reclaim and resend it while the winner is still working.
	d.doneMu.Lock()
	chatStillClaimed := d.done[baseKey+"/chat"]
	d.doneMu.Unlock()
	if !chatStillClaimed {
		t.Fatal("chatKey was released while the winner was still mid-SendEmail")
	}

	close(email.proceed)
	<-winnerDone

	chat.mu.Lock()
	defer chat.mu.Unlock()
	if len(chat.caseCreatedCalls) != 1 {
		t.Errorf("chat sent %d times total, want exactly 1", len(chat.caseCreatedCalls))
	}
}

// TestDispatcher_Handle_SubjectLine_StandardFormat verifies every case.*
// event type's email uses this service's one standard subject format:
// "[WSO2 Support] (<wso2 case id>/<case number>) <title>" — a real,
// explicitly-requested requirement, not a preference.
func TestDispatcher_Handle_SubjectLine_StandardFormat(t *testing.T) {
	tests := []struct {
		name   string
		record string
		want   string
	}{
		{
			name:   "case.created",
			record: `{"type":"case.created","entityId":"CASE-1","payload":{"reporterName":"Reporter","projectName":"Proj","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"desc","recipients":["test-recipient@example.com"]}}`,
			want:   "[WSO2 Support] (WSO2-1000/CS0001001) Something broke",
		},
		{
			name:   "case.comment_added",
			record: `{"type":"case.comment_added","entityId":"CASE-1","payload":{"name":"Commenter","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","caseComment":"fixed it","commentId":"C-1","recipients":["test-recipient@example.com"]}}`,
			want:   "[WSO2 Support] (WSO2-1000/CS0001001) Something broke",
		},
		{
			name:   "case.status_changed",
			record: `{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","newStatus":"Open","recipients":["test-recipient@example.com"]}}`,
			want:   "[WSO2 Support] (WSO2-1000/CS0001001) Something broke",
		},
		{
			name:   "case.assigned",
			record: `{"type":"case.assigned","entityId":"CASE-1","payload":{"assigneeName":"Assignee","assigneeEmail":"assignee@example.com","projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1000","caseTitle":"Something broke","recipients":["test-recipient@example.com"]}}`,
			want:   "[WSO2 Support] (WSO2-1000/CS0001001) Something broke",
		},
		{
			name:   "case.status_changed without wso2CaseId falls back to raw case id",
			record: `{"type":"case.status_changed","entityId":"CASE-1","payload":{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","caseTitle":"Something broke","newStatus":"Open","recipients":["test-recipient@example.com"]}}`,
			want:   "[WSO2 Support] (CASE-1/CS0001001) Something broke",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
			record := eventbus.Record{Value: []byte(tt.record)}
			if err := d.Handle(context.Background(), record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(mock.calls) != 1 {
				t.Fatalf("expected 1 email sent, got %d", len(mock.calls))
			}
			if mock.calls[0].subject != tt.want {
				t.Errorf("subject = %q, want %q", mock.calls[0].subject, tt.want)
			}
		})
	}
}

// crRecord builds a change_request.approval_requested record. audience picks
// which portal the link should point at.
func crRecord(audience, projectID string) eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"change_request.approval_requested","entityId":"CR-1","payload":{` +
		`"changeRequestId":"CR-1","number":"CHG0031234","state":"REVIEW","audience":"` + audience + `",` +
		`"projectId":"` + projectID + `","groupName":"Devops Review","team":"Choreo",` +
		`"subject":"[WSO2 Support] [CR][Choreo] (CHG0031234) Request for approval - Review",` +
		`"recipients":["` + testRecipient + `"]}}`)}
}

// TestDispatcher_Handle_CRApprovalRequested_UsesTheFlowsSubject: the flow
// reproduces the legacy ticketing system's per-branch wording, so this service must send that
// subject verbatim rather than building one of its own.
func TestDispatcher_Handle_CRApprovalRequested_UsesTheFlowsSubject(t *testing.T) {
	mock := &mockEmailSender{}
	d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

	if err := d.Handle(context.Background(), crRecord("internal", "")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(mock.calls))
	}
	sent := mock.calls[0]
	if want := "[WSO2 Support] [CR][Choreo] (CHG0031234) Request for approval - Review"; sent.subject != want {
		t.Errorf("subject = %q, want the flow's own %q", sent.subject, want)
	}
	if len(sent.to) != 1 || sent.to[0] != testRecipient {
		t.Errorf("to = %v, want the payload's resolved recipients verbatim", sent.to)
	}
}

// TestDispatcher_Handle_CRApprovalRequested_LinksByAudience is the regression
// guard for a link that pointed at /cases/<id>: a change request is not a case,
// and the two audiences do not even share a portal.
func TestDispatcher_Handle_CRApprovalRequested_LinksByAudience(t *testing.T) {
	tests := []struct {
		name, audience, projectID, wantLink string
	}{
		{"internal goes to the CSM portal", "internal", "",
			"https://csm.example/operations/change-requests/CR-1"},
		{"customer goes to the customer portal, under the project", "customer", "PROJ-1",
			"https://customer.example/projects/PROJ-1/operations/change-requests/CR-1"},
		{"customer with no project falls back rather than building a broken link", "customer", "",
			"https://csm.example/operations/change-requests/CR-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

			if err := d.Handle(context.Background(), crRecord(tt.audience, tt.projectID)); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(mock.calls) != 1 {
				t.Fatalf("sent %d emails, want 1", len(mock.calls))
			}
			if !strings.Contains(mock.calls[0].htmlBody, tt.wantLink) {
				t.Errorf("body does not link to %q", tt.wantLink)
			}
			if strings.Contains(mock.calls[0].htmlBody, "/cases/CR-1") {
				t.Error("body links to a case URL — a change request is not a case")
			}
		})
	}
}

// TestDispatcher_Handle_CRApprovalRequested_Killswitch: EMAIL_SENDING_ENABLED
// must silence this the same way it silences every other email here.
func TestDispatcher_Handle_CRApprovalRequested_Killswitch(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{CallSendingEnabled: true})

	if err := d.Handle(context.Background(), crRecord("internal", "")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("sent %d emails with sending disabled, want 0", len(mock.calls))
	}
}

// TestDispatcher_Handle_CRApprovalRequested_DebugMode redirects to the test
// mailbox instead of the real approval group.
func TestDispatcher_Handle_CRApprovalRequested_DebugMode(t *testing.T) {
	mock := &mockEmailSender{}
	d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: []string{"debug@wso2.com"}, CallSendingEnabled: true})

	if err := d.Handle(context.Background(), crRecord("internal", "")); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("sent %d emails, want 1", len(mock.calls))
	}
	if got := mock.calls[0].to; len(got) != 1 || got[0] != "debug@wso2.com" {
		t.Errorf("to = %v, want the debug list to replace the real audience", got)
	}
}

// TestDispatcher_Handle_CRPlanDateNotice_DebugMode verifies the debug
// redirect applies here too, for both the internal (visible To) and
// customer (BCC) branches — only the configured debug recipients ever see
// either send, never the real audience, in either branch.
func TestDispatcher_Handle_CRPlanDateNotice_DebugMode(t *testing.T) {
	t.Run("internal audience — To is redirected", func(t *testing.T) {
		mock := &mockEmailSender{}
		d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: []string{"debug@wso2.com"}, CallSendingEnabled: true})

		if err := d.Handle(context.Background(), planDateRecord("customer_proposed", "internal", "")); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		if len(mock.calls) != 1 {
			t.Fatalf("sent %d emails, want 1", len(mock.calls))
		}
		sent := mock.calls[0]
		if len(sent.to) != 1 || sent.to[0] != "debug@wso2.com" {
			t.Errorf("to = %v, want only the debug recipient", sent.to)
		}
		if len(sent.bcc) != 0 {
			t.Errorf("bcc = %v, want none — the real recipient must not appear anywhere in this send", sent.bcc)
		}
	})

	t.Run("customer audience — BCC is redirected, not the real recipient", func(t *testing.T) {
		mock := &mockEmailSender{}
		d := NewDispatcher(Deps{Email: mock, GoogleChat: &mockGoogleChatSender{}, Call: &mockCallSender{}, Links: &mockLinkResolver{}}, Config{EmailSendingEnabled: true, EmailDebugMode: true, EmailDebugRecipients: []string{"debug@wso2.com"}, CallSendingEnabled: true})

		if err := d.Handle(context.Background(), planDateRecord("accepted", "customer", "PROJ-1")); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		if len(mock.calls) != 1 {
			t.Fatalf("sent %d emails, want 1", len(mock.calls))
		}
		sent := mock.calls[0]
		if len(sent.bcc) != 1 || sent.bcc[0] != "debug@wso2.com" {
			t.Errorf("bcc = %v, want only the debug recipient, not the real customer audience", sent.bcc)
		}
		for _, addr := range append([]string{}, sent.to...) {
			if addr == testRecipient {
				t.Errorf("real recipient %q leaked into To during debug mode", testRecipient)
			}
		}
	})
}

// TestDispatcher_Handle_CRApprovalRequested_CustomerAudienceIsBCC guards a real
// exposure. The legacy ticketing system sent one email per recipient, so no customer contact
// ever saw who else was notified; collapsing that into one message must not
// publish a project's contact list to itself. Project contacts routinely span
// several organisations, so a visible To would disclose addresses across
// companies with no relationship to each other.
func TestDispatcher_Handle_CRApprovalRequested_CustomerAudienceIsBCC(t *testing.T) {
	t.Run("customer recipients are hidden from each other", func(t *testing.T) {
		mock := &mockEmailSender{}
		d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

		record := eventbus.Record{Value: []byte(`{"type":"change_request.approval_requested","entityId":"CR-1","payload":{` +
			`"changeRequestId":"CR-1","number":"CHG0031234","state":"CUSTOMER_REVIEW","audience":"customer",` +
			`"projectId":"PROJ-1","subject":"[WSO2 Support] [CR] (CHG0031234) Request for approval - Customer Review",` +
			`"recipients":["a@acme.example","b@globex.example","c@initech.example"]}}`)}

		if err := d.Handle(context.Background(), record); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		if len(mock.calls) != 1 {
			t.Fatalf("sent %d emails, want 1", len(mock.calls))
		}
		sent := mock.calls[0]

		if len(sent.bcc) != 3 {
			t.Errorf("bcc = %v, want all three contacts", sent.bcc)
		}
		if len(sent.to) != 1 || sent.to[0] != "noreply@wso2.com" {
			t.Errorf("to = %v, want only the sender — the service rejects an empty To, and the sender discloses nobody", sent.to)
		}
		for _, addr := range []string{"a@acme.example", "b@globex.example", "c@initech.example"} {
			for _, visible := range sent.to {
				if visible == addr {
					t.Errorf("%s appears in To, where every other recipient can read it", addr)
				}
			}
		}
	})

	t.Run("internal recipients stay visible to each other", func(t *testing.T) {
		mock := &mockEmailSender{}
		d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

		if err := d.Handle(context.Background(), crRecord("internal", "")); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}
		sent := mock.calls[0]
		if len(sent.bcc) != 0 {
			t.Errorf("bcc = %v, want none — one WSO2 approval group should see who else was asked", sent.bcc)
		}
		if len(sent.to) != 1 || sent.to[0] != testRecipient {
			t.Errorf("to = %v, want the approval group itself", sent.to)
		}
	})
}

func planDateRecord(kind, audience, projectID string) eventbus.Record {
	return eventbus.Record{Value: []byte(`{"type":"change_request.plan_date_notice","entityId":"CR-1","payload":{` +
		`"changeRequestId":"CR-1","number":"CHG0031234","kind":"` + kind + `","audience":"` + audience + `",` +
		`"projectId":"` + projectID + `","projectName":"Acme Cloud","actorName":"Perera Nimal",` +
		`"shortDescription":"Upgrade the gateway","description":"Full details",` +
		`"subject":"[WSO2 Support] [CR] (CHG0031234) Customer has updated the plan start date",` +
		`"recipients":["` + testRecipient + `"]}}`)}
}

// TestDispatcher_Handle_CRPlanDateNotice covers all three turns: who sees the
// recipient list, and which portal each is linked into.
func TestDispatcher_Handle_CRPlanDateNotice(t *testing.T) {
	tests := []struct {
		name, kind, audience, projectID string
		wantLink                        string
		wantBCC                         bool
	}{
		{"customer proposed — internal audience, visible To", "customer_proposed", "internal", "",
			"https://csm.example/operations/change-requests/CR-1", false},
		{"WSO2 accepted — customer audience, BCC", "accepted", "customer", "PROJ-1",
			"https://customer.example/projects/PROJ-1/operations/change-requests/CR-1", true},
		{"WSO2 rejected — customer audience, BCC", "rejected", "customer", "PROJ-1",
			"https://customer.example/projects/PROJ-1/operations/change-requests/CR-1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})

			if err := d.Handle(context.Background(), planDateRecord(tt.kind, tt.audience, tt.projectID)); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if len(mock.calls) != 1 {
				t.Fatalf("sent %d emails, want 1", len(mock.calls))
			}
			sent := mock.calls[0]

			if !strings.Contains(sent.htmlBody, tt.wantLink) {
				t.Errorf("body does not link to %q", tt.wantLink)
			}
			if tt.wantBCC {
				if len(sent.bcc) != 1 || len(sent.to) != 1 || sent.to[0] != "noreply@wso2.com" {
					t.Errorf("to=%v bcc=%v, want the audience hidden in BCC", sent.to, sent.bcc)
				}
			} else if len(sent.bcc) != 0 {
				t.Errorf("bcc=%v, want an internal group visible to each other", sent.bcc)
			}
		})
	}
}

// TestDispatcher_Handle_CRPlanDateNotice_WordingPerKind pins the three body
// texts, reproduced from the legacy ticketing system including its own awkward grammar.
func TestDispatcher_Handle_CRPlanDateNotice_WordingPerKind(t *testing.T) {
	tests := []struct{ kind, audience, want string }{
		{"customer_proposed", "internal", "Customer has updated the plan start date. Please review the change."},
		{"accepted", "customer", "The proposed plan start date accepted by the WSO2 Team."},
		{"rejected", "customer", "WSO2 Team request to change the plan start date."},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			mock := &mockEmailSender{}
			d := newTestDispatcher(mock, &mockGoogleChatSender{}, &mockCallSender{})
			if err := d.Handle(context.Background(), planDateRecord(tt.kind, tt.audience, "PROJ-1")); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			body := mock.calls[0].htmlBody
			if !strings.Contains(body, tt.want) {
				t.Errorf("body missing %q", tt.want)
			}
			if !strings.Contains(body, "Perera Nimal") {
				t.Error("body does not name the actor")
			}
			if !strings.Contains(body, "Acme Cloud / CHG0031234") {
				t.Error("body missing the project / number line")
			}
			if strings.Count(body, "<!DOCTYPE") != 1 {
				t.Errorf("rendered %d documents, want 1", strings.Count(body, "<!DOCTYPE"))
			}
		})
	}
}
