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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureIncidentCard(t *testing.T) (*GoogleChatClient, func() (chatCardMessage, string, int)) {
	t.Helper()
	var msg chatCardMessage
	var replyOption string
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		replyOption = r.URL.Query().Get("messageReplyOption")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: IncidentAudience("Artemis SRE Group"), WebhookURL: srv.URL}}})
	return c, func() (chatCardMessage, string, int) { return msg, replyOption, posts }
}

func TestIncidentAudience_IgnoresCaseAndSurroundingSpace(t *testing.T) {
	if IncidentAudience("  Artemis SRE Group ") != IncidentAudience("ARTEMIS SRE GROUP") {
		t.Errorf("IncidentAudience should match an assignment group whatever its case")
	}
}

func TestSendIncidentCreatedAlert_ReproducesServiceNowCard(t *testing.T) {
	c, got := captureIncidentCard(t)
	err := c.SendIncidentCreatedAlert(context.Background(), IncidentAudience("artemis sre group"), IncidentCreatedAlert{
		IncidentID: "inc-1", Number: "INC0088718", Team: "Artemis SRE Group", Priority: "CRITICAL",
		ShortDescription: "ContainerRecentlyOOMKilled", Service: "wso2-cloud", Environment: "central",
		Category: "SERVICE_INTERRUPTION", State: "NEW", IncidentLink: "https://csm.example/operations/incidents/inc-1",
	})
	if err != nil {
		t.Fatalf("SendIncidentCreatedAlert: %v", err)
	}
	msg, replyOption, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Title != "ARTEMIS SRE GROUP | Critical Priority Incident Reported" {
		t.Errorf("title = %q", card.Header.Title)
	}
	if card.Header.Subtitle != "#INC0088718 | wso2-cloud | central" {
		t.Errorf("subtitle = %q", card.Header.Subtitle)
	}
	summary := card.Sections[0].Widgets
	if !strings.Contains(summary[0].TextParagraph.Text, "ContainerRecentlyOOMKilled") {
		t.Errorf("short description missing: %q", summary[0].TextParagraph.Text)
	}
	if b := summary[1].ButtonList.Buttons[0]; b.Text != "View Incident" || b.OnClick.OpenLink.URL != "https://csm.example/operations/incidents/inc-1" {
		t.Errorf("button = %+v", b)
	}
	details := card.Sections[1]
	if details.Header != "Incident Details" || !details.Collapsible {
		t.Errorf("details section = %+v", details)
	}
	text := sectionText(details)
	for _, want := range []string{"service_interruption", "P1 - Critical", "<b>State:</b> New"} {
		if !strings.Contains(text, want) {
			t.Errorf("details %q missing %q", text, want)
		}
	}
	if msg.Thread == nil || msg.Thread.ThreadKey != "incident-inc-1" || replyOption != chatThreadReplyOption {
		t.Errorf("thread = %+v, replyOption = %q", msg.Thread, replyOption)
	}
}

func TestSendIncidentCreatedAlert_OmitsWhatIsUnknown(t *testing.T) {
	c, got := captureIncidentCard(t)
	err := c.SendIncidentCreatedAlert(context.Background(), IncidentAudience("Artemis SRE Group"), minimalIncidentAlert())
	if err != nil {
		t.Fatalf("SendIncidentCreatedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Title != "Priority Incident Reported" || card.Header.Subtitle != "#INC1" {
		t.Errorf("header = %+v", card.Header)
	}
	if len(card.Sections[0].Widgets) != 1 {
		t.Errorf("no link should mean no button, got %d widgets", len(card.Sections[0].Widgets))
	}
}

func minimalIncidentAlert() IncidentCreatedAlert {
	return IncidentCreatedAlert{IncidentID: "inc-1", Number: "INC1"}
}

func TestSendIncidentAssignedAlert_RepliesInTheIncidentThread(t *testing.T) {
	c, got := captureIncidentCard(t)
	err := c.SendIncidentAssignedAlert(context.Background(), IncidentAudience("Artemis SRE Group"), IncidentAssignedAlert{
		IncidentID: "inc-1", Number: "INC0088718", AssigneeName: "Shan Anjana", UpdatedBy: "shana@wso2.com",
	})
	if err != nil {
		t.Fatalf("SendIncidentAssignedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Title != "Incident Acknowledged." || card.Header.Subtitle != "INC0088718" {
		t.Errorf("header = %+v", card.Header)
	}
	text := sectionText(card.Sections[0])
	if text != "Incident acknowledged by Shan Anjana.<br>Incident updated by shana@wso2.com." {
		t.Errorf("body = %q", text)
	}
	if msg.Thread == nil || msg.Thread.ThreadKey != "incident-inc-1" {
		t.Errorf("thread = %+v", msg.Thread)
	}
}

func TestSendIncidentCreatedAlert_UnconfiguredGroupPostsNothing(t *testing.T) {
	c, got := captureIncidentCard(t)
	if err := c.SendIncidentCreatedAlert(context.Background(), IncidentAudience("Some Other Group"), minimalIncidentAlert()); err != nil {
		t.Fatalf("an unconfigured audience is a no-op, got %v", err)
	}
	if _, _, posts := got(); posts != 0 {
		t.Errorf("posts = %d, want 0", posts)
	}
}

func TestSendIncidentCreatedAlert_PNotationAndNoNumber(t *testing.T) {
	c, got := captureIncidentCard(t)
	if err := c.SendIncidentCreatedAlert(context.Background(), IncidentAudience("Artemis SRE Group"), IncidentCreatedAlert{IncidentID: "inc-9", Priority: "p2"}); err != nil {
		t.Fatalf("SendIncidentCreatedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Title != "High Priority Incident Reported" || card.Header.Subtitle != "#inc-9" {
		t.Errorf("header = %+v, want the P2 word and the id standing in for the number", card.Header)
	}
	if !strings.Contains(sectionText(card.Sections[1]), "P2 - High") {
		t.Errorf("details = %q, want P2 - High", sectionText(card.Sections[1]))
	}
}

func TestSendIncidentAssignedAlert_CarriesTheLinkAndFallsBackToTheID(t *testing.T) {
	c, got := captureIncidentCard(t)
	err := c.SendIncidentAssignedAlert(context.Background(), IncidentAudience("Artemis SRE Group"), IncidentAssignedAlert{
		IncidentID: "inc-9", AssigneeName: "Shan Anjana", IncidentLink: "https://csm.example/operations/incidents/inc-9",
	})
	if err != nil {
		t.Fatalf("SendIncidentAssignedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Subtitle != "inc-9" {
		t.Errorf("subtitle = %q, want the id when no number was sent", card.Header.Subtitle)
	}
	if !strings.Contains(sectionText(card.Sections[0]), `<a href="https://csm.example/operations/incidents/inc-9">View incident</a>`) {
		t.Errorf("body = %q, want a View incident link", sectionText(card.Sections[0]))
	}
}

// A group's incident messages go to its INCIDENT_CHAT_FANOUT groups, matched ignoring case, else to the group itself.
func TestIncidentGroups(t *testing.T) {
	fanout := map[string][]string{"Default SRE Group": {"Apollo SRE Group", "Artemis SRE Group"}}
	if got := IncidentGroups(fanout, " default sre group "); len(got) != 2 || got[0] != "Apollo SRE Group" {
		t.Errorf("fanned-out group = %v; want both SaaS groups", got)
	}
	if got := IncidentGroups(fanout, "MS/PC SRE Group"); len(got) != 1 || got[0] != "MS/PC SRE Group" {
		t.Errorf("own group = %v; want just itself", got)
	}
	if got := IncidentGroups(fanout, "  "); got != nil {
		t.Errorf("no group = %v; want none", got)
	}
}

// The SME card mirrors the "Incident Reported" layout, and names who escalated it and the L1 on call.
func TestBuildSMEIncidentEscalatedCard(t *testing.T) {
	msg := buildSMEIncidentEscalatedCard(SMEIncidentEscalatedAlert{
		IncidentID: "inc-1", Number: "INC0089973", SMETeam: "Choreo Runtime", Priority: "P1",
		ShortDescription: "OOM on opensearch-data-cold-5", Service: "Moseif", State: "IN_PROGRESS",
		EscalatedBy: "Kasun Perera", EscalatedFrom: "Apollo SRE Group", OnCall: "Nimal <Silva>",
		EscalatedAt: "22 Oct 2026, 20:31 IST", IncidentLink: "https://portal/incidents/inc-1",
	})
	card := msg.CardsV2[0].Card
	if card.Header.Title != "CHOREO RUNTIME | Critical Priority Incident Escalated" || card.Header.Subtitle != "#INC0089973 | Moseif" {
		t.Errorf("header = %+v", card.Header)
	}
	if msg.Thread == nil || msg.Thread.ThreadKey != IncidentThreadKey("inc-1") {
		t.Errorf("thread = %+v; want the incident's thread", msg.Thread)
	}
	w := card.Sections[0].Widgets
	if w[1].DecoratedText == nil || w[1].DecoratedText.TopLabel != "Escalated by" || w[1].DecoratedText.Text != "<b>Kasun Perera</b> from Apollo SRE Group" {
		t.Errorf("escalated by widget = %+v", w[1].DecoratedText)
	}
	if w[2].DecoratedText == nil || w[2].DecoratedText.TopLabel != "SME on call" || w[2].DecoratedText.Text != "<b>Nimal &lt;Silva&gt;</b>" {
		t.Errorf("on call widget = %+v", w[2].DecoratedText)
	}
	if w[3].ButtonList == nil || w[3].ButtonList.Buttons[0].OnClick.OpenLink.URL != "https://portal/incidents/inc-1" {
		t.Errorf("view incident button = %+v", w[3].ButtonList)
	}
	details := card.Sections[1]
	if !details.Collapsible || !strings.Contains(details.Widgets[0].TextParagraph.Text, "<b>Escalated from:</b> Apollo SRE Group") ||
		strings.Contains(details.Widgets[0].TextParagraph.Text, "Paging:") {
		t.Errorf("details = %+v", details)
	}
}

// With nobody rostered the card says so instead of naming an empty on call.
func TestBuildSMEIncidentEscalatedCard_NobodyOnCall(t *testing.T) {
	msg := buildSMEIncidentEscalatedCard(SMEIncidentEscalatedAlert{IncidentID: "inc-1", Number: "INC1"})
	w := msg.CardsV2[0].Card.Sections[0].Widgets
	if w[1].DecoratedText == nil || !strings.Contains(w[1].DecoratedText.Text, "Nobody rostered") {
		t.Errorf("on call widget = %+v; want Nobody rostered", w[1].DecoratedText)
	}
	if got := msg.CardsV2[0].Card.Header.Title; got != "Priority Incident Escalated" {
		t.Errorf("title = %q", got)
	}
}

// With L1 empty the card names the first rostered tier and when it is called, never plain "Nobody rostered".
func TestBuildSMEIncidentEscalatedCard_FirstRostered(t *testing.T) {
	msg := buildSMEIncidentEscalatedCard(SMEIncidentEscalatedAlert{IncidentID: "inc-1", Number: "INC1",
		OnCall: "Nimal Fernando", OnCallRung: "L2", OnCallAt: "2026-10-23 14:10 IST"})
	got := msg.CardsV2[0].Card.Sections[0].Widgets[1].DecoratedText
	if got == nil {
		t.Fatal("no on call widget")
	}
	for _, want := range []string{"Nobody rostered before L2", "<b>Nimal Fernando</b> (L2)", "called at 2026-10-23 14:10 IST"} {
		if !strings.Contains(got.Text, want) {
			t.Errorf("on call = %q; want it to contain %q", got.Text, want)
		}
	}
}
