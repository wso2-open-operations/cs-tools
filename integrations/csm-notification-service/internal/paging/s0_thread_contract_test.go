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

package paging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/chataudience"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/dispatch"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/recipientlinks"
)

// The dispatcher's fakes: no email, call or link is part of this contract.
type noEmail struct{}

func (noEmail) SendEmail(context.Context, []string, []string, []string, []string, string, string, []notifications.EmailAttachment) error {
	return nil
}
func (noEmail) SendEmailFrom(context.Context, string, []string, []string, []string, []string, string, string, []notifications.EmailAttachment) error {
	return nil
}
func (noEmail) FromAddress() string { return "noreply@example.com" }

type noCall struct{}

func (noCall) MakeCall(context.Context, string, string) (notifications.Call, error) {
	return notifications.Call{}, nil
}

type noLinks struct{}

func (noLinks) ResolveLinks(context.Context, []string, string, string) ([]recipientlinks.RecipientLink, error) {
	return nil, nil
}
func (noLinks) CSMLink(string) string                            { return "" }
func (noLinks) ChangeRequestLink(string, string, string) string  { return "" }
func (noLinks) OutageLink(string) string                         { return "" }
func (noLinks) ServiceRequestLink(string) string                 { return "" }
func (noLinks) IncidentLink(string) string                       { return "" }
func (noLinks) IsCustomer(context.Context, string) (bool, error) { return false, nil }

// roomCardThreads runs the record through the real dispatcher and returns
// the threadKey of each card it posted to the SRE room, as sent to Chat.
func roomCardThreads(t *testing.T, rec eventbus.Record) []string {
	t.Helper()
	var mu sync.Mutex
	var threads []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sre-room" {
			var body struct {
				Thread struct {
					ThreadKey string `json:"threadKey"`
				} `json:"thread"`
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &body); err != nil {
				t.Errorf("decode Chat request: %v", err)
			}
			mu.Lock()
			threads = append(threads, body.Thread.ThreadKey)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	chat := notifications.NewGoogleChatClient(notifications.GoogleChatConfig{AudienceSpaces: []notifications.GoogleChatAudienceSpace{
		{Audience: chataudience.IncidentMonitor, WebhookURL: srv.URL + "/monitor"},
		{Audience: notifications.IncidentAudience("apollo"), WebhookURL: srv.URL + "/sre-room"},
	}})
	routes := &notifications.IncidentRoutes{}
	routes.SetTeamless([]string{"apollo"})
	d := dispatch.NewDispatcher(noEmail{}, chat, noCall{}, noLinks{}, false, false, nil, false, "", nil).WithIncidentRoutes(routes)
	if err := d.Handle(context.Background(), rec); err != nil {
		t.Fatalf("dispatcher Handle: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return threads
}

// The S0 room card (dispatcher) and the SRE ladder's replies (engine) are
// two consumers of one record, and they meet in one thread only if both
// derive its key the same way. Feed the same record to both and compare
// the thread the card was posted to with the one the chain replies in.
func TestS0Thread_CardAndRepliesShareOneThread(t *testing.T) {
	for _, tc := range []struct {
		name string
		rec  func(*testing.T) eventbus.Record
	}{
		{"created at S0", func(t *testing.T) eventbus.Record { return caseCreated(t, "Atlas", "CATASTROPHIC") }},
		{"raised to S0", func(t *testing.T) eventbus.Record { return severityChanged(t, "CRITICAL", "CATASTROPHIC") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := tc.rec(t)
			threads := roomCardThreads(t, rec)
			if len(threads) != 1 {
				t.Fatalf("SRE room cards = %d; want 1", len(threads))
			}

			store := newMemStore()
			if err := ladderEngine(LadderSRE, &fakeChat{}, store).Handle(context.Background(), rec); err != nil {
				t.Fatalf("engine Handle: %v", err)
			}
			st, found, _ := store.Get(context.Background(), testIncidentID)
			if !found {
				t.Fatal("no SRE chain for the S0 record")
			}
			if got := st.Plan.Trigger.chatThreadKey(); got != threads[0] {
				t.Errorf("the SRE ladder replies in thread %q, but the card is in %q: its rungs would land in a thread with no card", got, threads[0])
			}
		})
	}
}
