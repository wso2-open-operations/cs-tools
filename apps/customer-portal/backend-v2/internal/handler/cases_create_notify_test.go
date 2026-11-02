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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

const (
	notifyProjectID = "11111111-1111-1111-1111-111111111111"
	notifyConvID    = "33333333-3333-3333-3333-333333333333"
)

// notifyRecorder records a notification and signals it, because the handler
// sends it on a detached goroutine so the 201 does not wait on it.
type notifyRecorder struct {
	mu        sync.Mutex
	calls     int
	sessionID string
	payload   string
	err       error
	called    chan struct{}
}

func newNotifyRecorder(err error) *notifyRecorder {
	return &notifyRecorder{err: err, called: make(chan struct{}, 1)}
}

func (n *notifyRecorder) NotifyAgent(_ context.Context, sessionID, payload string) error {
	n.mu.Lock()
	n.calls++
	n.sessionID = sessionID
	n.payload = payload
	err := n.err
	n.mu.Unlock()
	select {
	case n.called <- struct{}{}:
	default:
	}
	return err
}

// waitCalled fails the test unless a notification arrives.
func (n *notifyRecorder) waitCalled(t *testing.T) {
	t.Helper()
	select {
	case <-n.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a notification, none arrived")
	}
}

// expectNoCall fails the test if one arrives.
func (n *notifyRecorder) expectNoCall(t *testing.T) {
	t.Helper()
	select {
	case <-n.called:
		t.Fatal("expected no notification, one was sent")
	case <-time.After(250 * time.Millisecond):
	}
}

func (n *notifyRecorder) snapshot() (int, string, string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls, n.sessionID, n.payload
}

// updatingMockClient records the conversation state write so a test can tell the
// two side effects of a chat-raised case apart.
type updatingMockClient struct {
	mockCreateCaseClient
	updatedConversationID string
	updateErr             error
}

func (m *updatingMockClient) UpdateConversation(_ context.Context, id string, _ entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	m.updatedConversationID = id
	if m.updateErr != nil {
		return entity.UpdateConversationResponse{}, m.updateErr
	}
	return entity.UpdateConversationResponse{}, nil
}

func newNotifyMock() *updatingMockClient {
	return &updatingMockClient{
		mockCreateCaseClient: mockCreateCaseClient{
			project: entity.ProjectDetailsView{ID: notifyProjectID},
			createdCase: entity.CreateCaseResponse{
				Case: entity.CreateCaseDetails{
					ID:     "22222222-2222-2222-2222-222222222222",
					Number: "CS12345",
				},
			},
		},
	}
}

func createCaseBody(conversationID string) string {
	body := `{"projectId":"` + notifyProjectID + `","title":"Test Case","description":"Details"`
	if conversationID != "" {
		body += `,"conversationId":"` + conversationID + `"`
	}
	return body + `}`
}

func postCase(t *testing.T, h *CaseHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.CreateCase(rec, authedRequest(http.MethodPost, "/cases", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	return rec
}

// The agent marks a conversation escalated only when it ran a case tool itself,
// so a case raised from the portal left no trace and its effectiveness
// reporting read 0% escalation while cases were being opened.
func TestCreateCase_FromConversation_NotifiesAgent(t *testing.T) {
	mock := newNotifyMock()
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(notifyConvID))
	notifier.waitCalled(t)

	calls, sessionID, payload := notifier.snapshot()
	if calls != 1 {
		t.Fatalf("expected exactly 1 notification, got %d", calls)
	}
	// The agent keys a session on "{project}:{conversation}" — the project id,
	// not the account id.
	if want := notifyProjectID + ":" + notifyConvID; sessionID != want {
		t.Errorf("session id = %q, want %q", sessionID, want)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v (%s)", err, payload)
	}
	for field, want := range map[string]string{
		"type":           "case_created",
		"accountId":      notifyProjectID,
		"conversationId": notifyConvID,
	} {
		if got[field] != want {
			t.Errorf("payload[%q] = %q, want %q", field, got[field], want)
		}
	}
}

// A case raised outside a chat has no conversation to escalate.
func TestCreateCase_WithoutConversation_DoesNotNotify(t *testing.T) {
	mock := newNotifyMock()
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(""))
	notifier.expectNoCall(t)
}

// The case already exists by the time the agent is told, so a notification
// failure must not turn a successful creation into an error.
func TestCreateCase_NotifyFailure_StillSucceeds(t *testing.T) {
	mock := newNotifyMock()
	notifier := newNotifyRecorder(errors.New("agent unreachable"))
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(notifyConvID))
	notifier.waitCalled(t)
	// And the authoritative record was still written.
	if mock.updatedConversationID != notifyConvID {
		t.Errorf("conversation %q marked converted, want %q", mock.updatedConversationID, notifyConvID)
	}
}

// The conversation state is the authoritative record; the notification is only
// what makes it visible to the agent. Neither should depend on the other.
func TestCreateCase_ConversionFailure_StillNotifies(t *testing.T) {
	mock := newNotifyMock()
	mock.updateErr = errors.New("entity unavailable")
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(notifyConvID))
	notifier.waitCalled(t)
}

// A handler built without an agent must behave exactly as before — the two
// dozen existing tests construct one that way.
func TestCreateCase_NoAgentConfigured_DoesNotPanic(t *testing.T) {
	mock := newNotifyMock()
	h := NewCaseHandler(mock)

	postCase(t, h, createCaseBody(notifyConvID))

	if mock.updatedConversationID != notifyConvID {
		t.Errorf("conversation %q marked converted, want %q", mock.updatedConversationID, notifyConvID)
	}
}

// An id that is not a UUID is not forwarded: the agent keys its session on it.
func TestCreateCase_MalformedConversationID_DoesNotNotify(t *testing.T) {
	mock := newNotifyMock()
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody("not-a-uuid"))
	notifier.expectNoCall(t)
}

// The agent identifies a customer by ACCOUNT. The chat path sends
// project.Account.ID, so sending the project id here would file the escalation
// under a different identity than the conversation it belongs to.
func TestCreateCase_SendsAccountIDNotProjectID(t *testing.T) {
	const accountID = "44444444-4444-4444-4444-444444444444"
	mock := newNotifyMock()
	mock.project.Account.ID = accountID
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(notifyConvID))
	notifier.waitCalled(t)

	_, sessionID, payload := notifier.snapshot()
	var got map[string]string
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["accountId"] != accountID {
		t.Errorf("accountId = %q, want the project's account %q", got["accountId"], accountID)
	}
	// The session key stays keyed on the project, matching the chat socket.
	if want := notifyProjectID + ":" + notifyConvID; sessionID != want {
		t.Errorf("session id = %q, want %q", sessionID, want)
	}
}

// Mirrors the chat path's own fallback (`projectDetails?.account?.id ||
// projectId`): the agent rejects an empty accountId outright.
func TestCreateCase_FallsBackToProjectIDWhenAccountEmpty(t *testing.T) {
	mock := newNotifyMock() // Account.ID left empty
	notifier := newNotifyRecorder(nil)
	h := NewCaseHandlerWithAgent(mock, notifier)

	postCase(t, h, createCaseBody(notifyConvID))
	notifier.waitCalled(t)

	_, _, payload := notifier.snapshot()
	var got map[string]string
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got["accountId"] != notifyProjectID {
		t.Errorf("accountId = %q, want the project id as fallback %q", got["accountId"], notifyProjectID)
	}
}

// The case already exists by the time the agent is told, so a silent agent must
// not hold the 201. Before this was detached, a non-answering agent delayed the
// response by up to sideChannelTimeout (30s) and a client could retry a
// creation that had already succeeded.
func TestCreateCase_ResponseDoesNotWaitOnTheNotification(t *testing.T) {
	mock := newNotifyMock()
	release := make(chan struct{})
	notifier := newNotifyRecorder(nil)
	blocking := &blockingNotifier{inner: notifier, release: release}
	h := NewCaseHandlerWithAgent(mock, blocking)

	done := make(chan struct{})
	go func() {
		defer close(done)
		postCase(t, h, createCaseBody(notifyConvID))
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("CreateCase blocked on the notification")
	}
	close(release)
	notifier.waitCalled(t)
}

// blockingNotifier holds NotifyAgent open until released, standing in for an
// agent that accepts the message and never acknowledges it.
type blockingNotifier struct {
	inner   *notifyRecorder
	release chan struct{}
}

func (b *blockingNotifier) NotifyAgent(ctx context.Context, sessionID, payload string) error {
	<-b.release
	return b.inner.NotifyAgent(ctx, sessionID, payload)
}
