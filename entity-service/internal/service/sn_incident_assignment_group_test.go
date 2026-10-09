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

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// snServicesStub serves ServiceNow's POST /services/search from services,
// paged by the request's offset/limit exactly as ServiceNow pages it.
func snServicesStub(services []snServiceFixture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body snITServiceSearchPayload
		_ = json.NewDecoder(r.Body).Decode(&body)
		page := []map[string]any{}
		for i := body.Pagination.Offset; i < len(services) && i < body.Pagination.Offset+body.Pagination.Limit; i++ {
			name := services[i].name
			if name == "" {
				name = "svc"
			}
			svc := map[string]any{"id": services[i].sysid, "name": name}
			if services[i].group != "" {
				label := services[i].groupName
				if label == "" {
					label = "Group"
				}
				svc["supportGroup"] = map[string]any{"id": services[i].group, "label": label}
			}
			page = append(page, svc)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"services": page, "totalRecords": len(services)})
	}
}

// snServiceFixture is one ServiceNow service: its sys_id, its support group's
// sys_id ("" for none) and, optionally, names.
type snServiceFixture struct{ sysid, group, name, groupName string }

// snCreateCapturingClient serves the lookup and POST /incidents, recording
// the create body and how many lookups were made.
func snCreateCapturingClient(t *testing.T, services []snServiceFixture, gotBody *map[string]any, lookups *int32) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	stub := snServicesStub(services)
	mux.HandleFunc("/services/search", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(lookups, 1)
		stub(w, r)
	})
	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Incident created successfully.","incident":{"id":"` + testIncidentSysid + `","number":"INC0001","createdOn":"2026-01-01 00:00:00","createdBy":"engineer@example.com"}}`))
	})
	return mux
}

// filler returns n services that are not the one looked for, each with a group.
func filler(n int) []snServiceFixture {
	out := make([]snServiceFixture, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, snServiceFixture{sysid: fmt.Sprintf("%032x", i+1), group: "0123456789abcdef0123456789abcdef"})
	}
	return out
}

// snService builds a ServiceNow incident service over a fake ServiceNow that serves services, with defaultService as
// its Default service; the fake records the create body in body and counts service lookups in lookups.
func snService(t *testing.T, services []snServiceFixture, defaultService string, body *map[string]any, lookups *int32) IncidentService {
	t.Helper()
	return WithIncidentDefaultService(NewServiceNowIncidentService(newTestSNClient(t, snCreateCapturingClient(t, services, body, lookups)), nil), defaultService)
}

// DATA_SOURCE=servicenow: the group comes from ServiceNow's own service
// record, found by id even when it is not on the first page, and the work
// note names the service.
func TestSNCreateIncident_GroupFromTheServiceOnALaterPage(t *testing.T) {
	req := validCreateIncidentRequest()
	const group = "fedcba9876543210fedcba9876543210"
	services := append(filler(maxLimit+4), snServiceFixture{sysid: uuidToSysid(req.ServiceID), group: group, name: "Choreo"})

	var body map[string]any
	var lookups int32
	if _, err := snService(t, services, "", &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if body["assignmentGroupId"] != group {
		t.Errorf("assignmentGroupId sent to ServiceNow = %v, want the service's support group %s", body["assignmentGroupId"], group)
	}
	if body["workNotes"] != "Assignment group set from service Choreo's support group" {
		t.Errorf("workNotes = %v", body["workNotes"])
	}
	if lookups != 2 {
		t.Errorf("lookups = %d, want 2 (stop at the page holding the service)", lookups)
	}
}

// A service with no support group, or one ServiceNow does not list, goes to
// the default service's support group.
func TestSNCreateIncident_NoGroupGoesToTheDefaultTeam(t *testing.T) {
	req := validCreateIncidentRequest()
	defaultSysid := uuidToSysid(testDefaultService)
	const defaultGroup = "abcdefabcdefabcdefabcdefabcdefab"
	for name, own := range map[string][]snServiceFixture{
		"service has no support group": {{sysid: uuidToSysid(req.ServiceID), name: "Billing"}},
		"service not listed":           nil,
	} {
		t.Run(name, func(t *testing.T) {
			logs := captureSlog(t)
			services := append(append([]snServiceFixture{}, own...), snServiceFixture{sysid: defaultSysid, group: defaultGroup, groupName: "Default Team"})
			var body map[string]any
			var lookups int32
			if _, err := snService(t, services, testDefaultService, &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req); err != nil {
				t.Fatalf("CreateIncident: %v", err)
			}
			if body["assignmentGroupId"] != defaultGroup {
				t.Errorf("assignmentGroupId = %v, want the default team %s", body["assignmentGroupId"], defaultGroup)
			}
			note, _ := body["workNotes"].(string)
			if !strings.HasSuffix(note, "has no support group; assigned to the default team (Default Team)") {
				t.Errorf("workNotes = %q", note)
			}
			if !strings.Contains(logs.String(), "level=WARN") {
				t.Errorf("want a warning, got: %s", logs.String())
			}
		})
	}
}

// No default team: created unassigned, with an error log.
func TestSNCreateIncident_NoDefaultTeamLeavesItUnassigned(t *testing.T) {
	req := validCreateIncidentRequest()
	for name, services := range map[string][]snServiceFixture{
		"service has no support group": {{sysid: uuidToSysid(req.ServiceID)}},
		"service not listed":           {{sysid: "0123456789abcdef0123456789abcdef", group: "fedcba9876543210fedcba9876543210"}},
	} {
		t.Run(name, func(t *testing.T) {
			logs := captureSlog(t)
			var body map[string]any
			var lookups int32
			if _, err := snService(t, services, "", &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req); err != nil {
				t.Fatalf("CreateIncident: %v", err)
			}
			if v, ok := body["assignmentGroupId"]; ok {
				t.Errorf("assignmentGroupId = %v, want none", v)
			}
			if !strings.Contains(logs.String(), "level=ERROR") {
				t.Errorf("want an error log, got: %s", logs.String())
			}
		})
	}
}

// An explicit group is accepted when it supports some service in ServiceNow --
// found by scanning, stopping at the first service it supports.
func TestSNCreateIncident_AnAllowedGroupIsUsed(t *testing.T) {
	req := validCreateIncidentRequest()
	req.AssignmentGroupID = strPtrGroup(testOtherGroup)
	services := append(filler(maxLimit+1), snServiceFixture{sysid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", group: uuidToSysid(testOtherGroup)})

	var body map[string]any
	var lookups int32
	if _, err := snService(t, services, "", &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if body["assignmentGroupId"] != uuidToSysid(testOtherGroup) {
		t.Errorf("assignmentGroupId = %v, want %s", body["assignmentGroupId"], uuidToSysid(testOtherGroup))
	}
	if note, _ := body["workNotes"].(string); !strings.HasPrefix(note, "Assignment group chosen by ") {
		t.Errorf("workNotes = %q", note)
	}
	if lookups != 2 {
		t.Errorf("lookups = %d, want 2", lookups)
	}
}

// A group that supports no ServiceNow service, or is not a UUID, is a 400 and
// no incident is created.
func TestSNCreateIncident_AGroupOutsideTheSetIsRefused(t *testing.T) {
	for name, sent := range map[string]string{"supports no service": testOtherGroup, "not a UUID": "nope"} {
		t.Run(name, func(t *testing.T) {
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = strPtrGroup(sent)
			var body map[string]any
			var lookups int32
			_, err := snService(t, filler(3), "", &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want a ValidationError", err)
			}
			if wantCode := map[string]string{"supports no service": apierror.CodeIncidentAssignmentGroupNotAllowed}[name]; ve.Code != wantCode {
				t.Errorf("code = %q, want %q", ve.Code, wantCode)
			}
			if body != nil {
				t.Errorf("an incident was created: %v", body)
			}
		})
	}
}

// The dual-write mirror sends the group incidentService chose from Postgres
// and never looks one up in ServiceNow, so the two sides cannot disagree.
func TestSNIncidentMirror_SendsTheGivenGroupWithoutALookup(t *testing.T) {
	req := validCreateIncidentRequest()
	group := testSupportGroup
	req.AssignmentGroupID = &group
	services := []snServiceFixture{{sysid: uuidToSysid(req.ServiceID), group: "fedcba9876543210fedcba9876543210"}}

	var body map[string]any
	var lookups int32
	svc := NewServiceNowIncidentMirrorService(newTestSNClient(t, snCreateCapturingClient(t, services, &body, &lookups)))
	if _, err := svc.CreateIncident(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if lookups != 0 {
		t.Errorf("lookups = %d, want 0", lookups)
	}
	if body["assignmentGroupId"] != uuidToSysid(testSupportGroup) {
		t.Errorf("assignmentGroupId = %v, want %s", body["assignmentGroupId"], uuidToSysid(testSupportGroup))
	}
}

// Running out of pages is not proof -- the service, or a service the group
// supports, may be further on -- so the create fails rather than going out
// unassigned or being refused.
func TestSNCreateIncident_ExhaustedScanIsAnError(t *testing.T) {
	for name, sent := range map[string]*string{"no group sent": nil, "group sent": strPtrGroup(testOtherGroup)} {
		t.Run(name, func(t *testing.T) {
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = sent
			services := append(filler(snServiceScanMaxPages*maxLimit), snServiceFixture{sysid: uuidToSysid(req.ServiceID), group: uuidToSysid(testOtherGroup)})

			var body map[string]any
			var lookups int32
			_, err := snService(t, services, "", &body, &lookups).CreateIncident(contextWithUserIDToken("token"), req)
			if err == nil {
				t.Fatal("CreateIncident succeeded; an inconclusive scan must not decide the group")
			}
			var ve *apierror.ValidationError
			if errors.As(err, &ve) {
				t.Errorf("err = %v, want an error that is not a 400", err)
			}
			if body != nil {
				t.Errorf("an incident was created: %v", body)
			}
			if lookups != snServiceScanMaxPages {
				t.Errorf("lookups = %d, want %d", lookups, snServiceScanMaxPages)
			}
		})
	}
}

// GET /incidents/create-defaults on DATA_SOURCE=servicenow reads the default
// service from ServiceNow.
func TestSNGetIncidentCreateDefaults(t *testing.T) {
	const defaultGroup = "abcdefabcdefabcdefabcdefabcdefab"
	services := append(filler(2), snServiceFixture{sysid: uuidToSysid(testDefaultService), group: defaultGroup, groupName: "Default Team"})
	var body map[string]any
	var lookups int32
	got, err := snService(t, services, testDefaultService, &body, &lookups).GetIncidentCreateDefaults(contextWithUserIDToken("token"))
	if err != nil {
		t.Fatalf("GetIncidentCreateDefaults: %v", err)
	}
	want := domain.IncidentCreateDefaults{DefaultServiceID: strPtrGroup(testDefaultService), DefaultGroup: &domain.EntityRef{ID: sysidToUUID(defaultGroup), Name: "Default Team"}}
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if string(gb) != string(wb) {
		t.Errorf("got %s, want %s", gb, wb)
	}
}
