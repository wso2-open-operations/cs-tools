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
	"testing"
)

func rawJSON(t *testing.T, s string) json.RawMessage {
	t.Helper()
	return json.RawMessage(s)
}

func TestValidate_Valid(t *testing.T) {
	cases := map[string]struct {
		entityID string
		typ      Type
		payload  string
	}{
		"case.created": {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created omits priority (e.g. security_report_analysis, which has no severity)": {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"SECURITY_REPORT_ANALYSIS","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.comment_added":                    {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"case.status_changed":                   {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["r@x.com"]}`},
		"case.assigned":                         {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"case.acknowledged":                     {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-1","acknowledgerName":"n"}`},
		"case.severity_changed":                 {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"case.workaround_provided":              {"CASE-1", TypeWorkaroundProvided, `{"caseId":"CASE-1"}`},
		"incident.created":                      {"INC-1", TypeIncidentCreated, `{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","callTo":"+15551234567"}`},
		"incident.created omits product/callTo": {"INC-1", TypeIncidentCreated, `{"title":"P1 outage","shortDescription":"Everything is down"}`},
		"incident.acknowledged":                 {"INC-1", TypeIncidentAcknowledged, `{"previousState":"NEW","newState":"IN_PROGRESS"}`},
		"incident.priority_elevated":            {"INC-1", TypeIncidentPriorityElevated, `{"oldPriority":"MODERATE","newPriority":"HIGH","title":"Gateway 500s"}`},
		// entity-service builds title from a nilable ServiceNow field, so it
		// can genuinely publish this. It must not be rejected: an invalid
		// payload is retried, dead-lettered and dropped.
		"incident.priority_elevated without a title": {"INC-1", TypeIncidentPriorityElevated, `{"oldPriority":"MODERATE","newPriority":"HIGH"}`},
		"incident.comment_added (public)":            {"INC-1", TypeIncidentCommentAdded, `{"commentId":"c-1","isPublic":true}`},
		"incident.comment_added (work note)":         {"INC-1", TypeIncidentCommentAdded, `{"commentId":"c-1","isPublic":false}`},
		"incident.assigned":                          {"INC-1", TypeIncidentAssigned, `{"assigneeId":"u-1","assigneeName":"Ana"}`},
		"incident.assigned without a name":           {"INC-1", TypeIncidentAssigned, `{"assigneeId":"u-1"}`},
		"sla.tier_reached":                           {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response","tier":"50"}`},
		"project_contact.invited":                    {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin","Portal user"],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.invited resend":             {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin"],"isIntegrationUser":false,"type":"OWN CONTACT","isResend":true}`},
		"project_contact.registered":                 {"a0e000000000001AAA", TypeProjectContactRegistered, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","eventModifiedOn":"2026-09-18T06:37:07Z"}`},
		"project_contact.invited without names":      {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"","email":"svc@acme.com","givenName":"","familyName":"","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":null,"isIntegrationUser":true,"type":"OWN CONTACT"}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(c.entityID, c.typ, rawJSON(t, c.payload)); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidate_RequiresFields(t *testing.T) {
	cases := map[string]struct {
		entityID string
		typ      Type
		payload  string
	}{
		"case.created missing caseTitle":                           {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created missing projectId":                           {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created missing recipients":                          {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d"}`},
		"case.created empty recipients":                            {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":[]}`},
		"case.created blank recipient":                             {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":[""]}`},
		"case.created malformed recipient":                         {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["not-an-email"]}`},
		"case.created caseId/entityId mismatch":                    {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-2","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created unknown field":                               {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"],"extra":true}`},
		"case.created rejects legacy caseLink":                     {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","recipients":["r@x.com"]}`},
		"comment_added missing caseComment":                        {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","commentId":"C-1","recipients":["r@x.com"]}`},
		"comment_added missing commentId":                          {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","recipients":["r@x.com"]}`},
		"comment_added missing recipients":                         {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","commentId":"C-1"}`},
		"comment_added missing caseId":                             {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"comment_added caseId/entityId mismatch":                   {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-2","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"status_changed missing newStatus":                         {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"status_changed missing projectId":                         {"CASE-1", TypeStatusChanged, `{"caseId":"CASE-1","newStatus":"Open","recipients":["r@x.com"]}`},
		"status_changed missing recipients":                        {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open"}`},
		"status_changed caseId/entityId mismatch":                  {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-2","newStatus":"Open","recipients":["r@x.com"]}`},
		"assigned missing assigneeEmail":                           {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"assigned missing projectId":                               {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"assigned missing recipients":                              {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-1"}`},
		"assigned caseId/entityId mismatch":                        {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-2","recipients":["r@x.com"]}`},
		"acknowledged missing acknowledgerName":                    {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-1"}`},
		"acknowledged missing caseId":                              {"CASE-1", TypeCaseAcknowledged, `{"acknowledgerName":"n"}`},
		"acknowledged caseId/entityId mismatch":                    {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-2","acknowledgerName":"n"}`},
		"workaround_provided missing caseId":                       {"CASE-1", TypeWorkaroundProvided, `{}`},
		"workaround_provided caseId/entityId mismatch":             {"CASE-1", TypeWorkaroundProvided, `{"caseId":"CASE-2"}`},
		"severity_changed missing oldSeverity":                     {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing newSeverity":                     {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","recipients":["r@x.com"]}`},
		"severity_changed missing projectId":                       {"CASE-1", TypeSeverityChanged, `{"caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing recipients":                      {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW"}`},
		"severity_changed caseId/entityId mismatch":                {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-2","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed no-op transition":                        {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"HIGH","recipients":["r@x.com"]}`},
		"severity_changed whitespace-only oldSeverity":             {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"   ","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed whitespace-only newSeverity":             {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"   ","recipients":["r@x.com"]}`},
		"incident missing title":                                   {"INC-1", TypeIncidentCreated, `{"product":"api-manager","shortDescription":"d","callTo":"+15551234567"}`},
		"incident malformed callTo":                                {"INC-1", TypeIncidentCreated, `{"product":"api-manager","title":"t","shortDescription":"d","callTo":"555-1234"}`},
		"incident missing entityId":                                {"", TypeIncidentCreated, `{"title":"t","shortDescription":"d"}`},
		"unknown type":                                             {"CASE-1", Type("case.deleted"), `{}`},
		"sla.tier_reached missing clockType":                       {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","tier":"50"}`},
		"sla.tier_reached missing tier":                            {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response"}`},
		"sla.tier_reached invalid tier":                            {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response","tier":"60"}`},
		"sla.tier_reached caseId/entityId mismatch":                {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-2","clockType":"response","tier":"50"}`},
		"project_contact.invited missing membershipSfId":           {"a0e000000000001AAA", TypeProjectContactInvited, `{"contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":[],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.invited missing email":                    {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":[],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.invited malformed email":                  {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"not-an-email","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":[],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.invited membershipSfId/entityId mismatch": {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000002AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":[],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.registered missing email":                 {"a0e000000000001AAA", TypeProjectContactRegistered, `{"membershipSfId":"a0e000000000001AAA","projectName":"Acme Cloud"}`},
		"project_contact.registered entityId mismatch":             {"a0e000000000001AAA", TypeProjectContactRegistered, `{"membershipSfId":"a0e000000000002AAA","email":"jane@acme.com"}`},
		"project_contact.registered unknown field":                 {"a0e000000000001AAA", TypeProjectContactRegistered, `{"membershipSfId":"a0e000000000001AAA","email":"jane@acme.com","roles":[]}`},
		"project_contact.registered bad eventModifiedOn":           {"a0e000000000001AAA", TypeProjectContactRegistered, `{"membershipSfId":"a0e000000000001AAA","email":"jane@acme.com","eventModifiedOn":"yesterday"}`},
		"project_contact.invited unknown field":                    {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","email":"jane@acme.com","password":"x"}`},
		"incident.acknowledged without entityId":                   {"", TypeIncidentAcknowledged, `{"previousState":"NEW","newState":"IN_PROGRESS"}`},
		"incident.acknowledged without newState":                   {"INC-1", TypeIncidentAcknowledged, `{"previousState":"NEW"}`},
		"incident.priority_elevated without newP":                  {"INC-1", TypeIncidentPriorityElevated, `{"oldPriority":"MODERATE","title":"t"}`},
		"incident.priority_elevated without oldP":                  {"INC-1", TypeIncidentPriorityElevated, `{"newPriority":"HIGH","title":"t"}`},
		"incident.comment_added without commentId":                 {"INC-1", TypeIncidentCommentAdded, `{"isPublic":true}`},
		"incident.assigned without assigneeId":                     {"INC-1", TypeIncidentAssigned, `{"assigneeName":"Ana"}`},
		"incident.assigned without an incident":                    {"", TypeIncidentAssigned, `{"assigneeId":"u-1"}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(c.entityID, c.typ, rawJSON(t, c.payload)); err == nil {
				t.Error("Validate() = nil, want an error")
			}
		})
	}
}

// The change-request notices carry an already-resolved audience, so the
// validator is the only boundary between a malformed publisher and a real
// mailbox. These are the rules it has to hold.
func TestValidate_ChangeRequestRules(t *testing.T) {
	const id = "11111111-2222-3333-4444-555555555555"
	plan := func(kind, audience, number string) string {
		return `{"changeRequestId":"` + id + `","number":"` + number +
			`","kind":"` + kind + `","audience":"` + audience +
			`","subject":"s","recipients":["r@x.com"]}`
	}
	approval := func(number string) string {
		return `{"changeRequestId":"` + id + `","number":"` + number +
			`","state":"ASSESS","audience":"internal","subject":"s","recipients":["r@x.com"]}`
	}

	rejected := map[string]struct {
		typ     Type
		payload string
	}{
		// Number is documented as required and is what a reader identifies the
		// change request by; a notice without it names nothing.
		"plan date missing number": {TypeCRPlanDateNotice, plan("accepted", "customer", "")},
		"approval missing number":  {TypeCRApprovalRequested, approval("")},

		// A mismatched kind/audience pair sends customer wording to an internal
		// group, or puts an internal audience in BCC where the customer link is
		// used. Each kind goes with exactly one audience.
		"customer_proposed with customer": {TypeCRPlanDateNotice, plan("customer_proposed", "customer", "CHG1")},
		"accepted with internal":          {TypeCRPlanDateNotice, plan("accepted", "internal", "CHG1")},
		"rejected with internal":          {TypeCRPlanDateNotice, plan("rejected", "internal", "CHG1")},
	}
	for name, tc := range rejected {
		t.Run(name, func(t *testing.T) {
			if err := Validate(id, tc.typ, json.RawMessage(tc.payload)); err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
		})
	}

	accepted := map[string]struct {
		typ     Type
		payload string
	}{
		"customer_proposed with internal": {TypeCRPlanDateNotice, plan("customer_proposed", "internal", "CHG1")},
		"accepted with customer":          {TypeCRPlanDateNotice, plan("accepted", "customer", "CHG1")},
		"rejected with customer":          {TypeCRPlanDateNotice, plan("rejected", "customer", "CHG1")},
		"approval with number":            {TypeCRApprovalRequested, approval("CHG1")},
	}
	for name, tc := range accepted {
		t.Run(name, func(t *testing.T) {
			if err := Validate(id, tc.typ, json.RawMessage(tc.payload)); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestValidate_ServiceRequest covers the three sr.* types: their required
// fields, commentType's closed set, the caseId/entityId match, and that the
// optional parts (sreTeamName, tags, description, ...) may be absent.
func TestValidate_ServiceRequest(t *testing.T) {
	const (
		created     = `{"caseId":"SR-1","number":"SR0001001","wso2CaseId":"WSO2-1","subject":"Open port 443","sreTeamId":"T-1","sreTeamName":"MS/PC SRE Group","assignmentGroupName":"MS/PC SRE Group","description":"<p>Please</p>","state":"Open","projectId":"P-1","projectName":"Acme","createdBy":"Jane","createdOn":"2026-10-07T10:00:00Z"}`
		acked       = `{"caseId":"SR-1","number":"SR0001001","subject":"Open port 443","sreTeamName":"MS/PC SRE Group","commentId":"C-1"}`
		commentBase = `"caseId":"SR-1","number":"SR0001001","subject":"Open port 443","sreTeamName":"MS/PC SRE Group","commentId":"C-2","content":"hi","authorEmail":"jane@acme.com","authorName":"Jane","createdOn":"2026-10-07T10:05:00Z"`
	)
	cases := []struct {
		name     string
		entityID string
		typ      Type
		payload  string
		wantErr  bool
	}{
		{"created", "SR-1", TypeSRCreated, created, false},
		{"created minimal", "SR-1", TypeSRCreated, `{"caseId":"SR-1","number":"SR0001001","subject":"s","state":"","createdOn":"2026-10-07T10:00:00Z"}`, false},
		{"created without subject (catalog form SR)", "SR-1", TypeSRCreated, `{"caseId":"SR-1","number":"SR0001001","subject":"","state":"Open","createdOn":"2026-10-07T10:00:00Z"}`, false},
		{"created missing number", "SR-1", TypeSRCreated, `{"caseId":"SR-1","subject":"s","state":"Open","createdOn":"2026-10-07T10:00:00Z"}`, true},
		{"created missing createdOn", "SR-1", TypeSRCreated, `{"caseId":"SR-1","number":"SR0001001","subject":"s","state":"Open"}`, true},
		{"created caseId/entityId mismatch", "SR-2", TypeSRCreated, created, true},
		{"created unknown field", "SR-1", TypeSRCreated, `{"caseId":"SR-1","number":"SR0001001","subject":"s","state":"Open","createdOn":"2026-10-07T10:00:00Z","extra":1}`, true},

		{"acknowledged", "SR-1", TypeSRAcknowledged, acked, false},
		{"acknowledged without subject", "SR-1", TypeSRAcknowledged, `{"caseId":"SR-1","number":"SR0001001","subject":"","commentId":"C-1"}`, false},
		{"acknowledged missing commentId", "SR-1", TypeSRAcknowledged, `{"caseId":"SR-1","number":"SR0001001","subject":"s"}`, true},
		{"acknowledged missing number", "SR-1", TypeSRAcknowledged, `{"caseId":"SR-1","subject":"s","commentId":"C-1"}`, true},
		{"acknowledged caseId/entityId mismatch", "SR-2", TypeSRAcknowledged, acked, true},

		{"comment_added comment", "SR-1", TypeSRCommentAdded, `{` + commentBase + `,"commentType":"comment","tags":["devops-sm"]}`, false},
		{"comment_added work note, no tags", "SR-1", TypeSRCommentAdded, `{` + commentBase + `,"commentType":"work_note","tags":[]}`, false},
		{"comment_added null tags", "SR-1", TypeSRCommentAdded, `{` + commentBase + `,"commentType":"comment","tags":null}`, false},
		{"comment_added unknown commentType", "SR-1", TypeSRCommentAdded, `{` + commentBase + `,"commentType":"note","tags":[]}`, true},
		{"comment_added missing commentType", "SR-1", TypeSRCommentAdded, `{` + commentBase + `,"tags":[]}`, true},
		{"comment_added missing authorEmail", "SR-1", TypeSRCommentAdded, `{"caseId":"SR-1","number":"SR0001001","subject":"s","commentId":"C-2","commentType":"comment","content":"hi","createdOn":"2026-10-07T10:05:00Z","tags":[]}`, true},
		{"comment_added missing commentId", "SR-1", TypeSRCommentAdded, `{"caseId":"SR-1","number":"SR0001001","subject":"s","commentType":"comment","content":"hi","authorEmail":"jane@acme.com","createdOn":"2026-10-07T10:05:00Z","tags":[]}`, true},
		{"comment_added missing createdOn", "SR-1", TypeSRCommentAdded, `{"caseId":"SR-1","number":"SR0001001","subject":"s","commentId":"C-2","commentType":"comment","content":"hi","authorEmail":"jane@acme.com","tags":[]}`, true},
		{"comment_added caseId/entityId mismatch", "SR-2", TypeSRCommentAdded, `{` + commentBase + `,"commentType":"comment","tags":[]}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.typ.IsKnown() {
				t.Fatalf("%s is not in KnownTypes", c.typ)
			}
			err := Validate(c.entityID, c.typ, rawJSON(t, c.payload))
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

// TestValidate_CaseEscalated covers case.escalated: required fields, the
// levels being an escalation (never a de-escalation, which is not
// published), the caseId/entityId match, and that an escalation with nobody
// to mail is rejected.
func TestValidate_CaseEscalated(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	payload := func(mod func(p map[string]any)) json.RawMessage {
		p := map[string]any{
			"caseId": id, "caseNumber": "CS0012345", "caseTitle": "Gateway down",
			"escalationId": "e1", "previousLevel": 1, "currentLevel": 2,
			"actorEmail": "a@x.com", "escalatedOn": "2026-10-08T09:03:17Z", "recipients": []string{"r@x.com"},
		}
		if mod != nil {
			mod(p)
		}
		raw, _ := json.Marshal(p)
		return raw
	}

	if err := Validate(id, TypeCaseEscalated, payload(nil)); err != nil {
		t.Fatalf("a well-formed escalation: Validate() = %v, want nil", err)
	}

	for name, mod := range map[string]func(p map[string]any){
		"no case number":      func(p map[string]any) { delete(p, "caseNumber") },
		"no escalation id":    func(p map[string]any) { delete(p, "escalationId") },
		"no actor":            func(p map[string]any) { delete(p, "actorEmail") },
		"no time":             func(p map[string]any) { delete(p, "escalatedOn") },
		"de-escalation":       func(p map[string]any) { p["previousLevel"], p["currentLevel"] = 2, 0 },
		"level above EL5":     func(p map[string]any) { p["currentLevel"] = 6 },
		"same level":          func(p map[string]any) { p["previousLevel"] = 2 },
		"no recipients":       func(p map[string]any) { p["recipients"] = []string{} },
		"malformed recipient": func(p map[string]any) { p["recipients"] = []string{"not-an-email"} },
		"other case's id":     func(p map[string]any) { p["caseId"] = "22222222-2222-2222-2222-222222222222" },
		"unknown field":       func(p map[string]any) { p["action"] = "ESCALATE" },
	} {
		t.Run(name, func(t *testing.T) {
			if err := Validate(id, TypeCaseEscalated, payload(mod)); err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
		})
	}
}

func TestValidate_OutageStatusPageDue(t *testing.T) {
	const ok = `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","number":"OUT0010021","cloud":"choreo","event":"outage_begin","timestamp":"2026-10-09T06:54:00.000Z"}`
	cases := []struct {
		name     string
		entityID string
		payload  string
		wantErr  bool
	}{
		{"valid", "O-1", ok, false},
		{"end, agent-manager", "O-1", `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","cloud":"agent-manager","event":"outage_end","timestamp":"2026-10-09T07:00:00.123Z"}`, false},
		{"outageId/entityId mismatch", "O-2", ok, true},
		{"unknown event", "O-1", `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","cloud":"choreo","event":"OUTAGE_BEGIN","timestamp":"2026-10-09T06:54:00.000Z"}`, true},
		{"unknown cloud", "O-1", `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","cloud":"CHOREO","event":"outage_begin","timestamp":"2026-10-09T06:54:00.000Z"}`, true},
		{"timestamp without millis", "O-1", `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","cloud":"choreo","event":"outage_begin","timestamp":"2026-10-09T06:54:00Z"}`, true},
		{"missing claimToken", "O-1", `{"webhookId":"W-1","outageId":"O-1","cloud":"choreo","event":"outage_begin","timestamp":"2026-10-09T06:54:00.000Z"}`, true},
		{"unknown field", "O-1", `{"webhookId":"W-1","claimToken":"T-1","outageId":"O-1","cloud":"choreo","event":"outage_begin","timestamp":"2026-10-09T06:54:00.000Z","x":1}`, true},
	}
	for _, c := range cases {
		err := Validate(c.entityID, TypeOutageStatusPageDue, json.RawMessage(c.payload))
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
