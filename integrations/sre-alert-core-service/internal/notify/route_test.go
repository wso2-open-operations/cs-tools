// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package notify

import (
	"encoding/json"
	"testing"

	"alert-core-service/internal/model"
)

// The create carries the contact type (a monitoring source), which the SRE escalation ladder routes by, but
// never an assignment group, even when the service has one: entity-service assigns the service's support
// group itself and rejects a create that names a group.
func TestCreateRequest_CarriesContactTypeButNoAssignmentGroup(t *testing.T) {
	n := &Notifier{callerID: "caller"}
	inc := model.Incident{Fingerprint: "fp", Service: "svc", Source: "Azure", Category: "availability", Impact: "HIGH", Urgency: "HIGH"}

	req := n.createRequest(inc, resolvedService{id: "svc-id", groupID: "grp-apollo"}, "[fp:tag]", "Incident auto-created from Alert: ALT1")

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if v, present := got["assignmentGroupId"]; present {
		t.Errorf("assignmentGroupId = %v; want it omitted, entity-service rejects it", v)
	}
	if got["contactType"] != "AZURE" {
		t.Errorf("contactType = %v, want AZURE", got["contactType"])
	}
	if got["serviceId"] != "svc-id" {
		t.Errorf("serviceId = %v, want svc-id", got["serviceId"])
	}
}

// AWS (and any source the contact-type enum has no value for) sends no contactType rather than a wrong one.
func TestCreateRequest_OmitsWhatIsUnknown(t *testing.T) {
	n := &Notifier{callerID: "caller"}
	req := n.createRequest(model.Incident{Fingerprint: "fp", Source: "AWS"}, resolvedService{id: "svc-id"}, "[fp:tag]", "")

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"assignmentGroupId", "contactType"} {
		if v, present := got[key]; present {
			t.Errorf("%s = %v; want it omitted", key, v)
		}
	}
}

func TestContactTypeForSource(t *testing.T) {
	for source, want := range map[string]string{
		"Azure":              "AZURE",
		"azure-monitor":      "AZURE",
		"Site24x7":           "SITE_247",
		"Site 24x7":          "SITE_247",
		"Microsoft Sentinel": "SENTINEL",
		"AWS":                "",
		"Grafana":            "",
		"":                   "",
	} {
		if got := contactTypeForSource(source); got != want {
			t.Errorf("contactTypeForSource(%q) = %q, want %q", source, got, want)
		}
	}
}

// The group comes from the most specific signal the incident has; each case removes the one above it.
func TestAssignmentGroup_TakesTheMostSpecificSignal(t *testing.T) {
	n := New(testLogger(), nil, Config{
		DefaultAssignmentGroupID: "grp-default",
		AssignmentGroupRoutes: map[string]string{
			"group:SRE - Apollo":                             "grp-apollo",
			"Topic:arn:aws:sns:us-east-1:111:artemis-alerts": "grp-artemis",
			"account:222":                                    "grp-account",
		},
	})
	full := model.Incident{
		AssignmentGroup: "sre - apollo", // names match ignoring case
		SourceTopic:     "arn:aws:sns:us-east-1:111:artemis-alerts",
		SourceAccount:   "222",
	}
	cases := []struct {
		name         string
		inc          model.Incident
		serviceGroup string
		wantID       string
		wantBy       string
	}{
		{"alert names its group", full, "grp-service", "grp-apollo", "alert"},
		{"alert names a group id directly", model.Incident{AssignmentGroup: "01234567-89ab-cdef-0123-456789abcdef"}, "grp-service", "01234567-89ab-cdef-0123-456789abcdef", "alert"},
		// entity-service rejects a bare sys_id with a 400, which would stop the incident being created.
		{"a bare sys_id is not used as an id", model.Incident{AssignmentGroup: "0123456789abcdef0123456789abcdef"}, "grp-service", "grp-service", "service"},
		{"unrouted name falls through to the service", model.Incident{AssignmentGroup: "Nobody Knows"}, "grp-service", "grp-service", "service"},
		{"service's support group", model.Incident{SourceTopic: full.SourceTopic}, "grp-service", "grp-service", "service"},
		{"topic it was sent from", model.Incident{SourceTopic: full.SourceTopic, SourceAccount: "222"}, "", "grp-artemis", "topic"},
		{"account it was sent from", model.Incident{SourceTopic: "arn:aws:sns:us-east-1:222:other", SourceAccount: "222"}, "", "grp-account", "account"},
		{"nothing matches: the default", model.Incident{SourceAccount: "999"}, "", "grp-default", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, by := n.assignmentGroup(tc.inc, tc.serviceGroup)
			if id != tc.wantID || by != tc.wantBy {
				t.Errorf("assignmentGroup = (%q, %q), want (%q, %q)", id, by, tc.wantID, tc.wantBy)
			}
		})
	}
}

func TestAssignmentGroup_NothingConfiguredLeavesItUnassigned(t *testing.T) {
	n := New(testLogger(), nil, Config{})
	if id, by := n.assignmentGroup(model.Incident{SourceAccount: "222"}, ""); id != "" || by != "none" {
		t.Errorf("assignmentGroup = (%q, %q), want unassigned", id, by)
	}
}

func TestLooksLikeGroupID(t *testing.T) {
	for v, want := range map[string]bool{
		"0123456789abcdef0123456789abcdef":     false, // a bare ServiceNow sys_id: entity-service would reject it
		"01234567-89ab-cdef-0123-456789abcdef": true,  // UUID
		"SRE - Apollo":                         false, // a name
		"0123456789abcdef":                     false,
	} {
		if got := looksLikeGroupID(v); got != want {
			t.Errorf("looksLikeGroupID(%q) = %v, want %v", v, got, want)
		}
	}
}

// A route or default that is not a UUID would make every incident it routes fail to create, so it stops startup.
func TestValidateGroupIDs(t *testing.T) {
	ok := Config{
		DefaultAssignmentGroupID: "01234567-89ab-cdef-0123-456789abcdef",
		AssignmentGroupRoutes:    map[string]string{"account:487629103847": "11111111-2222-3333-4444-555555555555"},
	}
	if err := ValidateGroupIDs(ok); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	for name, bad := range map[string]Config{
		"sys_id default":   {DefaultAssignmentGroupID: "0123456789abcdef0123456789abcdef"},
		"name as a value":  {AssignmentGroupRoutes: map[string]string{"account:487629103847": "SRE - Apollo"}},
		"key with no kind": {AssignmentGroupRoutes: map[string]string{"Apollo": "11111111-2222-3333-4444-555555555555"}},
		"misspelled kind":  {AssignmentGroupRoutes: map[string]string{"grup:SRE": "11111111-2222-3333-4444-555555555555"}},
		"same route, two groups": {AssignmentGroupRoutes: map[string]string{
			"Group:SRE": "11111111-2222-3333-4444-555555555555",
			"group:sre": "66666666-7777-8888-9999-000000000000",
		}},
	} {
		if err := ValidateGroupIDs(bad); err == nil {
			t.Errorf("%s: accepted, want refused", name)
		}
	}

	// The same route spelled twice is harmless when both name the same group.
	same := Config{AssignmentGroupRoutes: map[string]string{
		"Group:SRE": "11111111-2222-3333-4444-555555555555",
		"group:sre": "11111111-2222-3333-4444-555555555555",
	}}
	if err := ValidateGroupIDs(same); err != nil {
		t.Errorf("same route, same group: refused: %v", err)
	}
}
