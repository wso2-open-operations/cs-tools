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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleAlert() EscalationAlert {
	return EscalationAlert{
		Product: "WSO2 API Manager", Rung: "LEVEL_2", RungRole: "ABT lead",
		Attempt: 1, Priority: "P1", IncidentRef: "INC0012345",
		Title: "Gateway returning 500s in production", RecipientName: "Siluni Perera",
		Instruction: "Update the ticket status to Work In Progress to stop further notifications.",
		Rule:        "R2", PortalURL: "https://csm.example/operations/incidents/abc", Elapsed: "18m",
	}
}

// The header is what a reader sees first in a busy space, so the rung and the
// incident have to be in it.
func TestEscalationCard_Header(t *testing.T) {
	card := buildEscalationCard(sampleAlert()).CardsV2[0].Card
	if card.Header == nil {
		t.Fatal("the card has no header; the rung would not be visible at a glance")
	}
	if !strings.Contains(card.Header.Title, "LEVEL_2") {
		t.Errorf("title = %q, want the rung in it", card.Header.Title)
	}
	if !strings.Contains(card.Header.Subtitle, "INC0012345") {
		t.Errorf("subtitle = %q, want the incident reference", card.Header.Subtitle)
	}
}

// LEVEL_0 is the notification level. Section 3.0 says it is "represented as
// the initial escalation level (but not an actual escalation level)", so a
// card claiming an incident was escalated to it contradicts the document and
// tells a reader it has been through a rung it has not.
func TestEscalationCard_Level0IsNotAnEscalation(t *testing.T) {
	a := sampleAlert()
	a.Rung = "LEVEL_0"
	title := buildEscalationCard(a).CardsV2[0].Card.Header.Title
	if strings.Contains(title, "escalated") {
		t.Errorf("title = %q; LEVEL_0 is a notification, not an escalation", title)
	}
	if !strings.Contains(title, "notifying") {
		t.Errorf("title = %q, want it to say the rotation is being notified", title)
	}

	a.Rung = "LEVEL_3"
	if title := buildEscalationCard(a).CardsV2[0].Card.Header.Title; !strings.Contains(title, "escalated") {
		t.Errorf("title = %q; a real rung is an escalation", title)
	}
}

// Everything a reader needs to act without opening anything.
func TestEscalationCard_Body(t *testing.T) {
	body := buildEscalationCard(sampleAlert()).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	for _, want := range []string{
		"ABT lead",
		"Siluni Perera",
		"Priority P1",
		"unattended 18m",
		"Update the ticket status",
		`<a href="https://csm.example/operations/incidents/abc">View incident</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the card body is missing %q:\n%s", want, body)
		}
	}
}

// A phone number must never reach a room: a space is an audience, and this
// repository keeps numbers out of places people read.
func TestEscalationCard_CarriesNoPhoneNumber(t *testing.T) {
	a := sampleAlert()
	a.RecipientName = "Siluni Perera"
	body := buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if strings.Contains(body, "+94") || strings.Contains(body, "+1") {
		t.Errorf("a phone number reached the card:\n%s", body)
	}
}

// A recipient name is data from a roster, so it cannot be trusted to be free
// of the markup Chat's card text interprets.
func TestEscalationCard_EscapesDynamicValues(t *testing.T) {
	a := sampleAlert()
	a.RecipientName = `<b>evil</b> & co`
	a.Title = `</font><a href="http://evil">x</a>`
	card := buildEscalationCard(a).CardsV2[0].Card
	body := card.Sections[0].Widgets[0].TextParagraph.Text
	if strings.Contains(body, "<b>evil</b>") {
		t.Errorf("a recipient name broke out of its tag:\n%s", body)
	}
	if !strings.Contains(body, "&amp;") {
		t.Errorf("an ampersand was not escaped:\n%s", body)
	}
}

// Optional fields simply do not appear, rather than leaving a dangling label.
func TestEscalationCard_OmitsEmptyFields(t *testing.T) {
	a := sampleAlert()
	a.PortalURL, a.RecipientName, a.Rule = "", "", ""
	body := buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	for _, unwanted := range []string{"Calling", "View incident", "path "} {
		if strings.Contains(body, unwanted) {
			t.Errorf("an absent field still rendered %q:\n%s", unwanted, body)
		}
	}
}

// How long an incident has gone unattended is the one thing a card must
// always carry, because a space flattens time: cards minutes apart sit in a
// list looking the same as cards seconds apart. A zero elapsed is named
// rather than left as "unattended" with nothing after it.
func TestEscalationCard_AlwaysSaysHowLongUnattended(t *testing.T) {
	a := sampleAlert()
	body := buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if !strings.Contains(body, "unattended 18m") {
		t.Errorf("the card does not say how long it has been unattended:\n%s", body)
	}

	a.Elapsed = ""
	body = buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if !strings.Contains(body, "just raised") {
		t.Errorf("a zero elapsed should be named, not blank:\n%s", body)
	}
}

// And where it goes next, which is what turns a card from a notice into a
// deadline.
func TestEscalationCard_SaysWhatHappensNext(t *testing.T) {
	a := sampleAlert()
	a.NextRung, a.NextIn = "LEVEL_3", "7m"
	body := buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if !strings.Contains(body, "Escalates to LEVEL_3") || !strings.Contains(body, "in 7m unless acknowledged") {
		t.Errorf("the card does not say where this goes next:\n%s", body)
	}

	// The final rung has nowhere to climb, and says so rather than going
	// quiet, which would read as the escalation having stopped.
	a.NextRung, a.NextIn = "", ""
	body = buildEscalationCard(a).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text
	if !strings.Contains(body, "final rung") {
		t.Errorf("the last rung does not say it is the last:\n%s", body)
	}
}

// Every rung of one incident belongs in one conversation, or a space scatters
// an unfolding escalation through everything else being posted.
func TestEscalationCard_ThreadsByIncident(t *testing.T) {
	a := sampleAlert()
	a.ThreadKey = "incident-escalation-abc"
	msg := buildEscalationCard(a)
	if msg.Thread == nil || msg.Thread.ThreadKey != "incident-escalation-abc" {
		t.Errorf("thread = %+v, want the incident's own key", msg.Thread)
	}

	a.ThreadKey = ""
	if buildEscalationCard(a).Thread != nil {
		t.Error("a card with no key must post standalone rather than into an empty thread")
	}
}

func TestSendEscalationAlert_Validation(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{})
	a := sampleAlert()
	a.Rung = ""
	if err := c.SendEscalationAlert(context.Background(), a); err == nil {
		t.Error("expected an error with no rung")
	}
	a = sampleAlert()
	a.IncidentRef = ""
	if err := c.SendEscalationAlert(context.Background(), a); err == nil {
		t.Error("expected an error with no incident reference")
	}
}

// The whole thing, over HTTP, to the space its product routes to.
func TestSendEscalationAlert_PostsToTheSpace(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGoogleChatClient(GoogleChatConfig{
		AudienceSpaces: []GoogleChatAudienceSpace{{Audience: "WSO2 API Manager", WebhookURL: srv.URL}},
	})
	if err := c.SendEscalationAlert(context.Background(), sampleAlert()); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("nothing reached the space")
	}
	if _, ok := got["cardsV2"]; !ok {
		t.Errorf("the posted message is not a card: %v", got)
	}
}

// The call cannot always be placed - no telephony account, or a recipient the
// rota knows only by e-mail - so the card carries what it would have said.
func TestBuildEscalationCard_CarriesTheVoiceScript(t *testing.T) {
	script := "WSO2 Support Alert. Priority, P0."
	msg := buildEscalationCard(EscalationAlert{
		Rung: "LEVEL_1", Priority: "P0", IncidentRef: "INC-1", VoiceScript: script,
	})

	sections := msg.CardsV2[0].Card.Sections
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want the body and the voice script", len(sections))
	}
	if sections[1].Header == "" {
		t.Error("the voice section has no header; it reads as more of the card's own prose")
	}
	if got := sections[1].Widgets[0].TextParagraph.Text; !strings.Contains(got, script) {
		t.Errorf("voice section = %q, want it to contain %q", got, script)
	}
}

// Without one, the section is absent rather than empty: an empty labelled
// block reads as "the call says nothing".
func TestBuildEscalationCard_NoVoiceScriptNoSection(t *testing.T) {
	msg := buildEscalationCard(EscalationAlert{Rung: "LEVEL_1", Priority: "P0", IncidentRef: "INC-1"})
	if got := len(msg.CardsV2[0].Card.Sections); got != 1 {
		t.Errorf("sections = %d, want only the body", got)
	}
}

// The script is quoted text from elsewhere. Markup inside it must render as
// characters, not as card formatting.
func TestBuildEscalationCard_VoiceScriptIsEscaped(t *testing.T) {
	msg := buildEscalationCard(EscalationAlert{
		Rung: "LEVEL_1", IncidentRef: "INC-1", VoiceScript: `Account, <b>Acme</b> & Co.`,
	})
	got := msg.CardsV2[0].Card.Sections[1].Widgets[0].TextParagraph.Text
	if strings.Contains(got, "<b>Acme</b>") {
		t.Errorf("voice section = %q, want the markup escaped", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("voice section = %q, want the ampersand escaped", got)
	}
}

// Reading an escalation back afterwards needs more than "unattended 12m":
// which team, which rota window, when it was raised, when this rung went off,
// and who was supposed to have it first.
func TestBuildEscalationCard_CarriesTheContextAReaderNeeds(t *testing.T) {
	body := buildEscalationCard(EscalationAlert{
		Rung: "LEVEL_1", Priority: "P0", IncidentRef: "WSO2-1042",
		Team: "Vega", Rotation: "CRE_EVENING",
		ReportedAt: "2026-09-28 19:42 IST", Firing: "2026-09-28 19:43 IST",
		OnCall: "Ashens",
	}).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text

	for _, want := range []string{"Vega", "CRE_EVENING", "2026-09-28 19:42 IST", "2026-09-28 19:43 IST", "Ashens"} {
		if !strings.Contains(body, want) {
			t.Errorf("card body missing %q:\n%s", want, body)
		}
	}
}

// Every one of them is absent on some rung or some deployment: a configured
// roster resolves no rota window, and only the first rung has an on-call
// name. A card must not carry an empty label for any of them.
func TestBuildEscalationCard_OmitsContextItDoesNotHave(t *testing.T) {
	body := buildEscalationCard(EscalationAlert{
		Rung: "LEVEL_2", Priority: "P0", IncidentRef: "WSO2-1042",
	}).CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text

	for _, absent := range []string{"team ", "rotation ", "Reported ", "This rung ", "On call was"} {
		if strings.Contains(body, absent) {
			t.Errorf("card body has an empty %q label:\n%s", absent, body)
		}
	}
}
