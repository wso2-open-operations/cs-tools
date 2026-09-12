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
		"case.created":                          {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.comment_added":                    {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"case.status_changed":                   {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open","recipients":["r@x.com"]}`},
		"case.assigned":                         {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"case.acknowledged":                     {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-1","acknowledgerName":"n"}`},
		"case.severity_changed":                 {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"case.mentioned":                        {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","projectId":"PROJ-1","caseId":"CASE-1","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"incident.created":                      {"INC-1", TypeIncidentCreated, `{"product":"api-manager","title":"P1 outage","shortDescription":"Everything is down","callTo":"+15551234567"}`},
		"incident.created omits product/callTo": {"INC-1", TypeIncidentCreated, `{"title":"P1 outage","shortDescription":"Everything is down"}`},
		"sla.clock.register":                    {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"response":"2h"}}`},
		"sla.tier_reached":                      {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response","tier":"50"}`},
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
		"case.created missing caseTitle":              {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created missing projectId":              {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created missing recipients":             {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d"}`},
		"case.created empty recipients":               {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":[]}`},
		"case.created blank recipient":                {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":[""]}`},
		"case.created malformed recipient":            {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["not-an-email"]}`},
		"case.created caseId/entityId mismatch":       {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-2","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"]}`},
		"case.created unknown field":                  {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","recipients":["r@x.com"],"extra":true}`},
		"case.created rejects legacy caseLink":        {"CASE-1", TypeCaseCreated, `{"reporterName":"n","projectName":"p","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseType":"Incident","priority":"P3","createdAt":"2026-01-01","description":"d","caseLink":"https://x","recipients":["r@x.com"]}`},
		"comment_added missing caseComment":           {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","commentId":"C-1","recipients":["r@x.com"]}`},
		"comment_added missing commentId":             {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","recipients":["r@x.com"]}`},
		"comment_added missing recipients":            {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-1","caseTitle":"t","caseComment":"c","commentId":"C-1"}`},
		"comment_added missing caseId":                {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"comment_added caseId/entityId mismatch":      {"CASE-1", TypeCommentAdded, `{"name":"n","projectId":"PROJ-1","caseId":"CASE-2","caseTitle":"t","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"status_changed missing newStatus":            {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"status_changed missing projectId":            {"CASE-1", TypeStatusChanged, `{"caseId":"CASE-1","newStatus":"Open","recipients":["r@x.com"]}`},
		"status_changed missing recipients":           {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newStatus":"Open"}`},
		"status_changed caseId/entityId mismatch":     {"CASE-1", TypeStatusChanged, `{"projectId":"PROJ-1","caseId":"CASE-2","newStatus":"Open","recipients":["r@x.com"]}`},
		"assigned missing assigneeEmail":              {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","projectId":"PROJ-1","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"assigned missing projectId":                  {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","caseId":"CASE-1","recipients":["r@x.com"]}`},
		"assigned missing recipients":                 {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-1"}`},
		"assigned caseId/entityId mismatch":           {"CASE-1", TypeCaseAssigned, `{"assigneeName":"n","assigneeEmail":"e@x.com","projectId":"PROJ-1","caseId":"CASE-2","recipients":["r@x.com"]}`},
		"acknowledged missing acknowledgerName":       {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-1"}`},
		"acknowledged missing caseId":                 {"CASE-1", TypeCaseAcknowledged, `{"acknowledgerName":"n"}`},
		"acknowledged caseId/entityId mismatch":       {"CASE-1", TypeCaseAcknowledged, `{"caseId":"CASE-2","acknowledgerName":"n"}`},
		"severity_changed missing oldSeverity":        {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing newSeverity":        {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","recipients":["r@x.com"]}`},
		"severity_changed missing projectId":          {"CASE-1", TypeSeverityChanged, `{"caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed missing recipients":         {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"LOW"}`},
		"severity_changed caseId/entityId mismatch":   {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-2","oldSeverity":"HIGH","newSeverity":"LOW","recipients":["r@x.com"]}`},
		"severity_changed no-op transition":           {"CASE-1", TypeSeverityChanged, `{"projectId":"PROJ-1","caseId":"CASE-1","oldSeverity":"HIGH","newSeverity":"HIGH","recipients":["r@x.com"]}`},
		"mentioned missing caseComment":               {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","projectId":"PROJ-1","caseId":"CASE-1","commentId":"C-1","recipients":["r@x.com"]}`},
		"mentioned missing commentId":                 {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","projectId":"PROJ-1","caseId":"CASE-1","caseComment":"c","recipients":["r@x.com"]}`},
		"mentioned missing recipients":                {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","projectId":"PROJ-1","caseId":"CASE-1","caseComment":"c","commentId":"C-1"}`},
		"mentioned missing projectId":                 {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","caseId":"CASE-1","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"mentioned missing mentionerName":             {"CASE-1", TypeCaseMentioned, `{"projectId":"PROJ-1","caseId":"CASE-1","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"mentioned caseId/entityId mismatch":          {"CASE-1", TypeCaseMentioned, `{"mentionerName":"n","projectId":"PROJ-1","caseId":"CASE-2","caseComment":"c","commentId":"C-1","recipients":["r@x.com"]}`},
		"incident missing title":                      {"INC-1", TypeIncidentCreated, `{"product":"api-manager","shortDescription":"d","callTo":"+15551234567"}`},
		"incident malformed callTo":                   {"INC-1", TypeIncidentCreated, `{"product":"api-manager","title":"t","shortDescription":"d","callTo":"555-1234"}`},
		"incident missing entityId":                   {"", TypeIncidentCreated, `{"title":"t","shortDescription":"d"}`},
		"unknown type":                                {"CASE-1", Type("case.deleted"), `{}`},
		"sla.clock.register missing caseId":           {"CASE-1", TypeSLAClockRegister, `{"durations":{"response":"2h"}}`},
		"sla.clock.register missing durations":        {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{}}`},
		"sla.clock.register empty clock type":         {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"":"2h"}}`},
		"sla.clock.register empty duration":           {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"response":""}}`},
		"sla.clock.register caseId/entityId mismatch": {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-2","durations":{"response":"2h"}}`},
		"sla.clock.register unparsable duration":      {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"response":"not-a-duration"}}`},
		"sla.clock.register zero duration":            {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"response":"0s"}}`},
		"sla.clock.register negative duration":        {"CASE-1", TypeSLAClockRegister, `{"caseId":"CASE-1","durations":{"response":"-2h"}}`},
		"sla.tier_reached missing clockType":          {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","tier":"50"}`},
		"sla.tier_reached missing tier":               {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response"}`},
		"sla.tier_reached invalid tier":               {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-1","clockType":"response","tier":"60"}`},
		"sla.tier_reached caseId/entityId mismatch":   {"CASE-1", TypeSLATierReached, `{"caseId":"CASE-2","clockType":"response","tier":"50"}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(c.entityID, c.typ, rawJSON(t, c.payload)); err == nil {
				t.Error("Validate() = nil, want an error")
			}
		})
	}
}
