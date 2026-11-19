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
	"slices"
	"strings"
	"testing"
)

func incidentSLAAlert(tier string) IncidentSLAAlert {
	return IncidentSLAAlert{
		ClockType:       "resolution",
		Tier:            tier,
		Number:          "INC0091536",
		Title:           "Router control plane 5xx",
		AssignmentGroup: "Artemis SRE Group",
		Priority:        "CRITICAL",
		Service:         "Choreo",
		OpenedAt:        "2026-11-03 07:19:53 (UTC)",
		IncidentLink:    "https://csm.example/incidents/inc-1",
	}
}

func TestSendIncidentSLAAlert_Breach(t *testing.T) {
	c, got := captureSLABreachCard(t)
	if err := c.SendIncidentSLAAlert(context.Background(), "Team Nova", incidentSLAAlert("100")); err != nil {
		t.Fatalf("SendIncidentSLAAlert: %v", err)
	}
	card := got().CardsV2[0].Card
	if want := "[CRITICAL] Resolution SLA Violation - INC0091536"; card.Header.Title != want {
		t.Errorf("title = %q, want %q", card.Header.Title, want)
	}
	body := card.Sections[0].Widgets[0].TextParagraph.Text
	for _, want := range []string{"Incident ID :</b> INC0091536", "Service :</b> Choreo", "Artemis SRE Group", "P1 - Critical", "100%"} {
		if !strings.Contains(body, want) {
			t.Errorf("body = %q, want it to contain %q", body, want)
		}
	}
	if strings.Contains(body, "Case ID") {
		t.Errorf("body = %q, want incident wording, not case", body)
	}
	b := card.Sections[1].Widgets[0].ButtonList.Buttons[0]
	if b.Text != "Open in CSM Portal" || b.OnClick.OpenLink.URL != "https://csm.example/incidents/inc-1" {
		t.Errorf("button = %q -> %q, want Open in CSM Portal -> the incident link", b.Text, b.OnClick.OpenLink.URL)
	}
}

func TestSendIncidentSLAAlert_AtRisk(t *testing.T) {
	c, got := captureSLABreachCard(t)
	if err := c.SendIncidentSLAAlert(context.Background(), "Team Nova", incidentSLAAlert("50")); err != nil {
		t.Fatalf("SendIncidentSLAAlert: %v", err)
	}
	if title := got().CardsV2[0].Card.Header.Title; title != "Resolution SLA at Risk - INC0091536" {
		t.Errorf("title = %q, want the at-risk wording", title)
	}
}

func TestSendIncidentSLAAlert_RequiresNumber(t *testing.T) {
	c, _ := captureSLABreachCard(t)
	a := incidentSLAAlert("100")
	a.Number = ""
	if err := c.SendIncidentSLAAlert(context.Background(), "Team Nova", a); err == nil {
		t.Error("SendIncidentSLAAlert with no number: want an error")
	}
}

func TestIncidentRoutes_SRETeams(t *testing.T) {
	var nilRoutes *IncidentRoutes
	if got := nilRoutes.SRETeams("Apollo SRE Group"); got != nil {
		t.Errorf("nil routes = %v, want none", got)
	}
	routes := &IncidentRoutes{}
	if got := routes.SRETeams("Apollo SRE Group"); got != nil {
		t.Errorf("before SetSRETeams = %v, want none", got)
	}
	routes.SetSRETeams(func(group string) []string {
		return map[string][]string{"Apollo SRE Group": {"apollo"}, "Default SRE Group": {"apollo", "artemis"}}[group]
	})
	if got := routes.SRETeams("Default SRE Group"); !slices.Equal(got, []string{"apollo", "artemis"}) {
		t.Errorf("SRETeams(Default SRE Group) = %v, want both teamless teams", got)
	}
	if got := routes.SRETeams("  "); got != nil {
		t.Errorf("SRETeams(blank) = %v, want none", got)
	}
}

// An SRE team's incident SLA space is its team.chat_webhook_url by team key,
// reachable only under IncidentSLAAudience: never as a case team audience,
// and a static or incident card space never answers for it.
func TestGoogleChatClient_IncidentSLATeamSpaces(t *testing.T) {
	c := NewGoogleChatClient(GoogleChatConfig{AudienceSpaces: []GoogleChatAudienceSpace{
		{Audience: IncidentSLAAudience("artemis"), WebhookURL: "https://chat.example/static"},
	}})
	c.SetIncidentSLATeamSpaces(map[string]string{" Apollo ": "https://chat.example/apollo", "iaas": "", "": "https://chat.example/x"})

	if !c.HasAudienceSpace(IncidentSLAAudience("apollo")) {
		t.Error("apollo's SLA space not found")
	}
	if c.HasAudienceSpace(IncidentSLAAudience("artemis")) {
		t.Error("a static space answered for artemis's SLA space")
	}
	if c.HasAudienceSpace(IncidentSLAAudience("iaas")) {
		t.Error("a team with a blank URL has an SLA space")
	}
	if c.HasAudienceSpace("apollo") || c.HasAudienceSpace(IncidentAudience("apollo")) {
		t.Error("an SRE team's SLA space answered as a case or incident card audience")
	}
	c.SetIncidentSLATeamSpaces(nil)
	if c.HasAudienceSpace(IncidentSLAAudience("apollo")) {
		t.Error("SLA spaces not cleared")
	}
}

func TestSLABreachEmails_IncidentWording(t *testing.T) {
	d := SLABreachEmailData{ClockType: "resolution", Tier: "100", Severity: "CRITICAL", CaseNumber: "INC0091536", CaseLink: "https://csm.example/incidents/inc-1", Team: "Artemis SRE Group"}

	caseTeam, caseAssignee := RenderSLABreachTeamEmail("", d), RenderSLABreachAssigneeEmail("", d)
	if !strings.Contains(caseTeam, "Case ID :") || !strings.Contains(caseAssignee, "View Case") {
		t.Error("case emails: want the Case wording unchanged")
	}

	d.Incident = true
	team, assignee := RenderSLABreachTeamEmail("", d), RenderSLABreachAssigneeEmail("", d)
	if !strings.Contains(team, "Incident ID :") || strings.Contains(team, "Case ID") {
		t.Error("incident team email: want Incident ID, not Case ID")
	}
	if !strings.Contains(assignee, "View Incident") || strings.Contains(assignee, "View Case") {
		t.Error("incident assignee email: want View Incident, not View Case")
	}
	if strings.Contains(team, "[ITEM_NOUN]") || strings.Contains(assignee, "[ITEM_NOUN]") {
		t.Error("emails: placeholder left unreplaced")
	}
}

// An incident's emails call the product its Service, and the 100% email's
// preview says the SLA was violated; a case keeps "Product" and "SLA at risk"
// at every tier.
func TestSLABreachEmail_IncidentServiceLabelAndViolatedPreview(t *testing.T) {
	base := SLABreachEmailData{ClockType: "resolution", Severity: "CRITICAL", CaseNumber: "INC0094500", CaseTitle: "Router 5xx", Product: "Choreo", Team: "Apollo SRE Group"}
	for _, tc := range []struct {
		incident             bool
		tier, title, product string
	}{
		{true, "100", "<title>INC0094500 SLA violated</title>", "<b>Service :</b> Choreo"},
		{true, "75", "<title>INC0094500 SLA at risk</title>", "<b>Service :</b> Choreo"},
		{false, "100", "<title>INC0094500 SLA at risk</title>", "<b>Product :</b> Choreo"},
		{false, "50", "<title>INC0094500 SLA at risk</title>", "<b>Product :</b> Choreo"},
	} {
		d := base
		d.Incident, d.Tier = tc.incident, tc.tier
		team := RenderSLABreachTeamEmail("Lead", d)
		if !strings.Contains(team, tc.title) || !strings.Contains(team, tc.product) {
			t.Errorf("incident=%v tier=%s team email: want %q and %q", tc.incident, tc.tier, tc.title, tc.product)
		}
		if assignee := RenderSLABreachAssigneeEmail("Engineer", d); !strings.Contains(assignee, tc.title) {
			t.Errorf("incident=%v tier=%s assignee email: want %q", tc.incident, tc.tier, tc.title)
		}
	}
}
