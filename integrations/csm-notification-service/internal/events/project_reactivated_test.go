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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package events

import "testing"

const reactivatedProjectID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"

// reactivatedPayload is every field entity-service's ACP reopen check sends,
// spelled as its events.ProjectReactivatedPayload marshals them: the two
// copies must decode each other strictly.
const reactivatedPayload = `{"projectId":"e3e87599-1bc7-6650-182c-0dc5604bcb68","projectSfId":"a0dE200000CZ9ZNIA1",` +
	`"projectName":"ACP Test Project","projectKey":"APPSUB","accountOwnerName":"Alex Owner","accountOwnerEmail":"owner@example.com",` +
	`"startDate":"2026-10-28","endDate":"2027-10-27","reasons":["renewal","invoice_paid"],` +
	`"recipients":["owner@example.com","tech.owner@example.com"],"reactivatedOn":"2026-10-29T09:30:00Z"}`

func TestValidate_ProjectReactivated(t *testing.T) {
	if !TypeProjectReactivated.IsKnown() {
		t.Fatal("project.reactivated is not a known type")
	}
	if err := Validate(reactivatedProjectID, TypeProjectReactivated, rawJSON(t, reactivatedPayload)); err != nil {
		t.Fatalf("the full payload: %v", err)
	}
	// Only the required fields; no recipients is valid (dispatch logs and drops it).
	minimal := `{"projectId":"e3e87599-1bc7-6650-182c-0dc5604bcb68","projectSfId":"","projectName":"P","projectKey":"",` +
		`"reasons":[],"recipients":[],"reactivatedOn":"2026-10-29T09:30:00Z"}`
	if err := Validate(reactivatedProjectID, TypeProjectReactivated, rawJSON(t, minimal)); err != nil {
		t.Errorf("the minimal payload: %v", err)
	}
}

func TestValidate_ProjectReactivatedRefusals(t *testing.T) {
	base := func(field, value string) string {
		fields := map[string]string{
			"projectId":     `"e3e87599-1bc7-6650-182c-0dc5604bcb68"`,
			"projectName":   `"P"`,
			"reasons":       `["renewal"]`,
			"recipients":    `["owner@example.com"]`,
			"reactivatedOn": `"2026-10-29T09:30:00Z"`,
		}
		if value == "" {
			delete(fields, field)
		} else {
			fields[field] = value
		}
		out := "{"
		first := true
		for k, v := range fields {
			if !first {
				out += ","
			}
			first = false
			out += `"` + k + `":` + v
		}
		return out + "}"
	}
	cases := map[string]string{
		"another project":    base("projectId", `"11111111-1111-1111-1111-111111111111"`),
		"no project name":    base("projectName", `"  "`),
		"a bad recipient":    base("recipients", `["not-an-email"]`),
		"an unknown reason":  base("reasons", `["refund"]`),
		"no reactivatedOn":   base("reactivatedOn", ""),
		"a bad reactivation": base("reactivatedOn", `"yesterday"`),
		"an unknown field":   base("surprise", `1`),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(reactivatedProjectID, TypeProjectReactivated, rawJSON(t, payload)); err == nil {
				t.Errorf("Validate(%s) = nil, want an error", payload)
			}
		})
	}
}
