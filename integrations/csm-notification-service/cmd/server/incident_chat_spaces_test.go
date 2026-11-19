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

package main

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

func TestParseIncidentChatSpaces(t *testing.T) {
	spaces := parseIncidentChatSpaces(`{"Artemis SRE Group":"https://chat.example/a","":"https://chat.example/x","Empty":""}`)
	if len(spaces) != 1 {
		t.Fatalf("got %d spaces, want only the complete entry: %+v", len(spaces), spaces)
	}
	if spaces[0].Audience != notifications.IncidentAudience("artemis sre group") || spaces[0].WebhookURL != "https://chat.example/a" {
		t.Errorf("space = %+v", spaces[0])
	}
	for _, raw := range []string{"", "   ", "not json", `["a"]`} {
		if got := parseIncidentChatSpaces(raw); got != nil {
			t.Errorf("parseIncidentChatSpaces(%q) = %+v, want nil", raw, got)
		}
	}
}

func TestParseIncidentChatFanout(t *testing.T) {
	spaces := parseIncidentChatSpaces(`{"Apollo SRE Group":"https://chat.example/a","Artemis SRE Group":"https://chat.example/b"}`)
	got := parseIncidentChatFanout(`{"Default SRE Group":["Apollo SRE Group","Artemis SRE Group"]}`, spaces)
	if targets := got["Default SRE Group"]; len(got) != 1 || len(targets) != 2 || targets[0] != "Apollo SRE Group" || targets[1] != "Artemis SRE Group" {
		t.Errorf("fanout = %+v", got)
	}
	dup := parseIncidentChatFanout(`{"Default SRE Group":["Apollo SRE Group"]," default sre group ":["Artemis SRE Group"],"MS/PC SRE Group":["Apollo SRE Group"]}`, spaces)
	if _, ok := dup["MS/PC SRE Group"]; len(dup) != 1 || !ok {
		t.Errorf("fanout with a group spelled twice = %+v; want only MS/PC SRE Group, the ambiguous group dropped", dup)
	}
	for _, raw := range []string{"", "   ", "not json", `["a"]`, `{"Default SRE Group":"Apollo SRE Group"}`} {
		if got := parseIncidentChatFanout(raw, spaces); got != nil {
			t.Errorf("parseIncidentChatFanout(%q) = %+v, want nil (no fan-out)", raw, got)
		}
	}
}
