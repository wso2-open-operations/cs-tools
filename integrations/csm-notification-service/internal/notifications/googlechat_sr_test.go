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

// captureSRCard serves one webhook, returning the client and a func that
// yields what was posted: the decoded message, the raw body and the
// messageReplyOption query value.
func captureSRCard(t *testing.T) (*GoogleChatClient, func() (chatCardMessage, string, string)) {
	t.Helper()
	var msg chatCardMessage
	var raw, replyOption string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		if err := json.Unmarshal(b, &msg); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		replyOption = r.URL.Query().Get("messageReplyOption")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "MS/PC SRE Group", WebhookURL: srv.URL}}})
	return c, func() (chatCardMessage, string, string) { return msg, raw, replyOption }
}

func sectionText(s chatCardSection) string {
	if len(s.Widgets) != 1 || s.Widgets[0].TextParagraph == nil {
		return ""
	}
	return s.Widgets[0].TextParagraph.Text
}

func assertSRThread(t *testing.T, msg chatCardMessage, replyOption string) {
	t.Helper()
	if msg.Thread == nil || msg.Thread.ThreadKey != "case-3e6e2582-3149-4c0a-b9bf-cd0defbab51d" {
		t.Errorf("Thread = %+v, want threadKey case-<SR id>", msg.Thread)
	}
	if replyOption != chatThreadReplyOption {
		t.Errorf("messageReplyOption = %q, want %q", replyOption, chatThreadReplyOption)
	}
}

func TestSendSRCreatedAlert_ReproducesServiceNowCard(t *testing.T) {
	c, got := captureSRCard(t)
	err := c.SendSRCreatedAlert(context.Background(), "MS/PC SRE Group", SRCreatedAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", WSO2CaseID: "WSO2-77", Subject: "Open port 443 & 80",
		AssignmentGroupName: "MS/PC SRE Group", State: "Open",
		Description: "<p>Please open <b>443</b> &amp; 80</p><script>x()</script>",
		CaseLink:    "https://csm.example/cases/SR-1",
	})
	if err != nil {
		t.Fatalf("SendSRCreatedAlert: %v", err)
	}
	msg, raw, replyOption := got()
	assertSRThread(t, msg, replyOption)
	card := msg.CardsV2[0].Card
	if card.Header == nil || card.Header.Title != "New Service Request | SR0001001" || card.Header.Subtitle != "Assigned to MS/PC SRE Group" {
		t.Errorf("Header = %+v", card.Header)
	}
	if len(card.Sections) != 4 {
		t.Fatalf("sections = %d, want 4 (short description, details, description, button): %+v", len(card.Sections), card.Sections)
	}
	want := []struct{ header, text string }{
		{"Short Description", "Open port 443 &amp; 80"},
		{"Details", "<b>WSO2 Case ID:</b> WSO2-77<br><b>State:</b> Open"},
		{"Description", "Please open 443 &amp; 80"},
	}
	for i, w := range want {
		if card.Sections[i].Header != w.header || sectionText(card.Sections[i]) != w.text {
			t.Errorf("section %d = %q / %q, want %q / %q", i, card.Sections[i].Header, sectionText(card.Sections[i]), w.header, w.text)
		}
	}
	if card.Sections[0].Collapsible || card.Sections[1].Collapsible || !card.Sections[2].Collapsible {
		t.Error("only the Description section should be collapsible")
	}
	if !strings.Contains(raw, `"collapsible":true`) || strings.Count(raw, `"collapsible"`) != 1 {
		t.Errorf("raw body should carry collapsible exactly once: %s", raw)
	}
	btns := card.Sections[3].Widgets[0].ButtonList
	if btns == nil || len(btns.Buttons) != 1 || btns.Buttons[0].Text != "View Case" || btns.Buttons[0].OnClick.OpenLink.URL != "https://csm.example/cases/SR-1" {
		t.Errorf("button section = %+v", card.Sections[3])
	}
}

func TestSendSRCreatedAlert_OmitsEmptyParts(t *testing.T) {
	c, got := captureSRCard(t)
	if err := c.SendSRCreatedAlert(context.Background(), "MS/PC SRE Group", SRCreatedAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", Subject: "s", Description: "<p> </p>", CaseLink: "https://csm.example/cases/SR-1",
	}); err != nil {
		t.Fatalf("SendSRCreatedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Subtitle != "" {
		t.Errorf("Subtitle = %q, want empty with no assignment group", card.Header.Subtitle)
	}
	if len(card.Sections) != 2 || card.Sections[0].Header != "Short Description" || card.Sections[1].Widgets[0].ButtonList == nil {
		t.Errorf("sections = %+v, want short description + button only", card.Sections)
	}
}

// TestSendSRCreatedAlert_EmptySubjectShowsEmDash: an SR raised from the
// customer portal's catalog form has no subject. ServiceNow's card shows "—"
// for an empty short description; so does this one, rather than a blank line.
func TestSendSRCreatedAlert_EmptySubjectShowsEmDash(t *testing.T) {
	c, got := captureSRCard(t)
	if err := c.SendSRCreatedAlert(context.Background(), "MS/PC SRE Group", SRCreatedAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", CaseLink: "https://csm.example/operations/service-requests/SR-1",
	}); err != nil {
		t.Fatalf("SendSRCreatedAlert: %v", err)
	}
	msg, _, _ := got()
	if text := msg.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text; text != "\u2014" {
		t.Errorf("Short Description = %q, want an em dash", text)
	}
}

func TestSendSRAcknowledgedAlert_ReproducesServiceNowCard(t *testing.T) {
	c, got := captureSRCard(t)
	if err := c.SendSRAcknowledgedAlert(context.Background(), "MS/PC SRE Group", SRAcknowledgedAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", WSO2CaseID: "WSO2-77", AssignmentGroupName: "Infra <Ops>", SRETeamName: "MS/PC SRE Group",
		CaseLink: "https://csm.example/cases/SR-1",
	}); err != nil {
		t.Fatalf("SendSRAcknowledgedAlert: %v", err)
	}
	msg, _, replyOption := got()
	assertSRThread(t, msg, replyOption)
	card := msg.CardsV2[0].Card
	if card.Header == nil || card.Header.Title != "Acknowledged" || card.Header.Subtitle != "SR0001001 / WSO2-77" {
		t.Errorf("Header = %+v", card.Header)
	}
	if len(card.Sections) != 3 {
		t.Fatalf("sections = %+v, want 3", card.Sections)
	}
	if sectionText(card.Sections[0]) != "An automated acknowledgement has been posted to this case." {
		t.Errorf("section 0 = %q", sectionText(card.Sections[0]))
	}
	if card.Sections[1].Header != "Status" || sectionText(card.Sections[1]) != "Awaiting review by Infra &lt;Ops&gt;" {
		t.Errorf("section 1 = %q / %q", card.Sections[1].Header, sectionText(card.Sections[1]))
	}
	if sectionText(card.Sections[2]) != `<a href="https://csm.example/cases/SR-1">View case</a>` {
		t.Errorf("section 2 = %q", sectionText(card.Sections[2]))
	}
}

func TestSendSRAcknowledgedAlert_FallsBackToSRETeam(t *testing.T) {
	c, got := captureSRCard(t)
	if err := c.SendSRAcknowledgedAlert(context.Background(), "MS/PC SRE Group", SRAcknowledgedAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", SRETeamName: "MS/PC SRE Group", CaseLink: "https://csm.example/cases/SR-1",
	}); err != nil {
		t.Fatalf("SendSRAcknowledgedAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Subtitle != "SR0001001" {
		t.Errorf("Subtitle = %q, want the number alone with no wso2CaseId", card.Header.Subtitle)
	}
	if sectionText(card.Sections[1]) != "Awaiting review by MS/PC SRE Group" {
		t.Errorf("status = %q", sectionText(card.Sections[1]))
	}
}

func TestSendSRCustomerCommentAlert_SendsExpectedCard(t *testing.T) {
	long := strings.Repeat("a", 310)
	c, got := captureSRCard(t)
	if err := c.SendSRCustomerCommentAlert(context.Background(), "MS/PC SRE Group", SRCustomerCommentAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", WSO2CaseID: "WSO2-77", Subject: "Open port 443",
		AuthorName: "Jane <Doe>", AuthorEmail: "jane@acme.com", Content: "[code]<p>" + long + "</p>[/code]",
		CommentLink: "https://csm.example/cases/SR-1#C-2",
	}); err != nil {
		t.Fatalf("SendSRCustomerCommentAlert: %v", err)
	}
	msg, _, replyOption := got()
	assertSRThread(t, msg, replyOption)
	card := msg.CardsV2[0].Card
	if card.Header == nil || card.Header.Title != "💬 Customer comment · SR0001001 · WSO2-77" || card.Header.Subtitle != "Open port 443" {
		t.Errorf("Header = %+v", card.Header)
	}
	want := "<b>Jane &lt;Doe&gt;</b><br>" + strings.Repeat("a", 300) + `...<br><a href="https://csm.example/cases/SR-1#C-2">View comment</a>`
	if got := sectionText(card.Sections[0]); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestSendSRCustomerCommentAlert_EmailWhenNoName(t *testing.T) {
	c, got := captureSRCard(t)
	if err := c.SendSRCustomerCommentAlert(context.Background(), "MS/PC SRE Group", SRCustomerCommentAlert{
		CaseID: "3e6e2582-3149-4c0a-b9bf-cd0defbab51d",
		Number: "SR0001001", AuthorEmail: "jane@acme.com", Content: "hi", CommentLink: "https://csm.example/cases/SR-1#C-2",
	}); err != nil {
		t.Fatalf("SendSRCustomerCommentAlert: %v", err)
	}
	msg, _, _ := got()
	card := msg.CardsV2[0].Card
	if card.Header.Title != "💬 Customer comment · SR0001001" {
		t.Errorf("Title = %q", card.Header.Title)
	}
	if want := `<b>jane@acme.com</b><br>hi<br><a href="https://csm.example/cases/SR-1#C-2">View comment</a>`; sectionText(card.Sections[0]) != want {
		t.Errorf("text = %q, want %q", sectionText(card.Sections[0]), want)
	}
}

func TestSendSRAlerts_RejectEmptyNumber(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "MS/PC SRE Group", WebhookURL: "https://example.com"}}})
	ctx := context.Background()
	if err := c.SendSRCreatedAlert(ctx, "MS/PC SRE Group", SRCreatedAlert{}); err == nil {
		t.Error("SendSRCreatedAlert: want error for empty number")
	}
	if err := c.SendSRAcknowledgedAlert(ctx, "MS/PC SRE Group", SRAcknowledgedAlert{}); err == nil {
		t.Error("SendSRAcknowledgedAlert: want error for empty number")
	}
	if err := c.SendSRCustomerCommentAlert(ctx, "MS/PC SRE Group", SRCustomerCommentAlert{}); err == nil {
		t.Error("SendSRCustomerCommentAlert: want error for empty number")
	}
}

func TestSRChatPlainText(t *testing.T) {
	cases := []struct {
		name, in, want string
		max            int
	}{
		{"plain", "hello world", "hello world", 300},
		{"blocks become spaces", "<p>one</p><p>two<br>three</p>", "one two three", 300},
		{"inline tags vanish", "a <b>bold</b> <i>word</i>", "a bold word", 300},
		{"entities decode", "Tom &amp; Jerry &lt;3", "Tom & Jerry <3", 300},
		{"whitespace collapses", "  a \n\n\t b  ", "a b", 300},
		{"script and style dropped", "<style>p{}</style>keep<script>alert(1)</script>", "keep", 300},
		{"SN code markers dropped", "[code]<p>hi</p>[/code]", "hi", 300},
		{"truncated by rune", "héllo wörld", "héllo...", 5},
		{"exactly max not cut", "abcde", "abcde", 5},
		{"empty", "", "", 300},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := srChatPlainText(c.in, c.max); got != c.want {
				t.Errorf("srChatPlainText(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
			}
		})
	}
}

// TestSRThreadKey: an SR's thread follows its id, so two SRs that share a
// number (two environments, or a reseeded database) never share a thread.
func TestSRThreadKey(t *testing.T) {
	if a, b := srThreadKey("id-1", "CS-PORTAL-000003"), srThreadKey("id-2", "CS-PORTAL-000003"); a == b {
		t.Errorf("two SRs with one number share thread %q", a)
	}
	if got := srThreadKey("id-1", "CS-PORTAL-000003"); got != "case-id-1" {
		t.Errorf("srThreadKey = %q, want case-id-1", got)
	}
	if got := srThreadKey("", "CS-PORTAL-000003"); got != chatThreadKey("CS-PORTAL-000003") {
		t.Errorf("srThreadKey without an id = %q, want the number-based key", got)
	}
}
