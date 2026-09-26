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
		"incident.created":                      {"INC-1", TypeIncidentCreated, `{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","callTo":"+15551234567"}`},
		"incident.created omits product/callTo": {"INC-1", TypeIncidentCreated, `{"title":"P1 outage","shortDescription":"Everything is down"}`},
		"sla.tier_reached":                      {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response","tier":"50"}`},
		"project_contact.invited":               {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin","Portal user"],"isIntegrationUser":false,"type":"OWN CONTACT"}`},
		"project_contact.invited resend":        {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"003000000000001AAA","email":"jane@acme.com","givenName":"Jane","familyName":"Doe","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":["Admin"],"isIntegrationUser":false,"type":"OWN CONTACT","isResend":true}`},
		"project_contact.invited without names": {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","contactSfId":"","email":"svc@acme.com","givenName":"","familyName":"","projectName":"Acme Cloud","projectKey":"ACMECLOUD","roles":null,"isIntegrationUser":true,"type":"OWN CONTACT"}`},
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
		"severity_changed missing oldSeverity":                     {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing newSeverity":                     {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","recipients":["r@x.com"]}`},
		"severity_changed missing projectId":                       {"CASE-1", TypeSeverityChanged, `{"caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing recipients":                      {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW"}`},
		"severity_changed caseId/entityId mismatch":                {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-2","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed no-op transition":                        {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"HIGH","recipients":["r@x.com"]}`},
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
		"project_contact.invited unknown field":                    {"a0e000000000001AAA", TypeProjectContactInvited, `{"membershipSfId":"a0e000000000001AAA","email":"jane@acme.com","password":"x"}`},
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
