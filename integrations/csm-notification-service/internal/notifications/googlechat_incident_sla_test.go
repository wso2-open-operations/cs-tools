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
		ClockType:        "resolution",
		Tier:             tier,
		Number:           "INC0091536",
		Title:            "Router control plane 5xx",
		AssignmentGroup:  "Artemis SRE Group",
		GroupManagerName: "Artemis Lead",
		Priority:         "CRITICAL",
		Service:          "Choreo",
		OpenedAt:         "2026-11-03 07:19:53 (UTC)",
		IncidentLink:     "https://csm.example/incidents/inc-1",
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
	for _, want := range []string{"Incident ID :</b> INC0091536", "Service :</b> Choreo", "Artemis SRE Group (Artemis Lead)", "P1 - Critical", "100%"} {
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

func TestIncidentSLAAudiences(t *testing.T) {
	has := func(spaces ...string) func(string) bool {
		return func(a string) bool { return slices.Contains(spaces, a) }
	}
	apollo, artemis := IncidentSLAAudience("Apollo SRE Group"), IncidentSLAAudience("Artemis SRE Group")
	fanout := map[string][]string{"Default SRE Group": {"Apollo SRE Group", "Artemis SRE Group"}}

	cases := []struct {
		name   string
		routes *IncidentRoutes
		group  string
		has    func(string) bool
		want   []string
	}{
		{"own group", nil, "Apollo SRE Group", has(apollo, artemis), []string{apollo}},
		{"default group fans out", nil, "default sre group", has(apollo, artemis), []string{apollo, artemis}},
		{"fan-out target without a space is left out", nil, "Default SRE Group", has(artemis), []string{artemis}},
		{"no SLA space, no fallback", nil, "Apollo SRE Group", has(IncidentAudience("Apollo SRE Group"), IncidentFallbackAudience), nil},
		{"empty group", nil, "", has(apollo), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IncidentSLAAudiences(tc.routes, fanout, tc.group, tc.has)
			if !slices.Equal(got, tc.want) {
				t.Errorf("IncidentSLAAudiences(%q) = %v, want %v", tc.group, got, tc.want)
			}
		})
	}

	// The Alert Settings mapping fans a default group out to SRE team keys.
	routes := &IncidentRoutes{}
	routes.Set(map[string][]string{"ms/pc sre group": {"apollo", "artemis"}})
	got := IncidentSLAAudiences(routes, nil, "MS/PC SRE Group", has(IncidentSLAAudience("apollo"), IncidentSLAAudience("artemis")))
	if want := []string{IncidentSLAAudience("apollo"), IncidentSLAAudience("artemis")}; !slices.Equal(got, want) {
		t.Errorf("IncidentSLAAudiences via mapping = %v, want %v", got, want)
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
