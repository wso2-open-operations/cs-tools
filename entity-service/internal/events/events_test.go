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

package events

import (
	"encoding/json"
	"strings"
	"testing"
)

// csm-notification-service decodes the resend marker as "isResend"; a
// different name here silently turns every portal resend into an ordinary
// invitation that its duplicate guard then skips.
func TestProjectContactInvitedPayload_ResendWireName(t *testing.T) {
	raw, err := json.Marshal(ProjectContactInvitedPayload{Resend: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"isResend":true`) {
		t.Errorf("payload = %s, want the resend marker as \"isResend\"", raw)
	}

	raw, err = json.Marshal(ProjectContactInvitedPayload{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "esend") {
		t.Errorf("payload = %s, an ordinary invitation must omit the resend marker", raw)
	}
}

// csm-notification-service decodes this payload with DisallowUnknownFields,
// so the wire names are pinned here.
func TestProjectContactRegisteredPayload_WireNames(t *testing.T) {
	raw, err := json.Marshal(ProjectContactRegisteredPayload{IsIntegrationUser: true, EventModifiedOn: "2026-09-18T06:37:07Z"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"membershipSfId", "contactSfId", "email", "givenName", "familyName", "projectName", "projectKey", "isIntegrationUser", "eventModifiedOn"}
	if len(got) != len(want) {
		t.Errorf("payload = %s, want exactly %v", raw, want)
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("payload = %s, missing %q", raw, k)
		}
	}
}

// severityChangedWire is a fully filled SeverityChangedPayload as published.
// csm-notification-service decodes it with DisallowUnknownFields, and its
// events.TestValidate_EntityServiceSeverityChanged validates this exact
// string: change both together.
const severityChangedWire = `{"projectId":"PROJ-1","caseId":"CASE-1","caseNumber":"CS0001001","wso2CaseId":"WSO2-1001","caseTitle":"Gateway down","oldSeverity":"HIGH","newSeverity":"CATASTROPHIC","product":"WSO2 API Manager","team":"Phoenix","sreTeamId":"SRE-1","sreTeamName":"Apollo SRE Group","recipients":["watcher@example.com"],"projectName":"Customer 3 Project","changedAt":"2026-10-27T06:07:15.123456789Z"}`

func TestSeverityChangedPayload_WireNames(t *testing.T) {
	raw, err := json.Marshal(SeverityChangedPayload{
		ProjectID: "PROJ-1", CaseID: "CASE-1", CaseNumber: "CS0001001", WSO2CaseID: "WSO2-1001", CaseTitle: "Gateway down",
		OldSeverity: "HIGH", NewSeverity: "CATASTROPHIC", Product: "WSO2 API Manager", Team: "Phoenix",
		SRETeamID: "SRE-1", SRETeamName: "Apollo SRE Group", Recipients: []string{"watcher@example.com"},
		ProjectName: "Customer 3 Project", ChangedAt: "2026-10-27T06:07:15.123456789Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != severityChangedWire {
		t.Errorf("payload =\n%s\nwant\n%s\n(csm-notification-service validates the latter; update both)", raw, severityChangedWire)
	}
}
