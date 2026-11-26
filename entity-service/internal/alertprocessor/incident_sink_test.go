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

package alertprocessor

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeIncidents struct {
	created  []domain.CreateIncidentRequest
	existing string
	state    *string
	missing  bool
}

func (f *fakeIncidents) CreateIncident(_ context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	f.created = append(f.created, req)
	var resp domain.CreateIncidentResponse
	resp.Incident.ID = "inc-new"
	return resp, nil
}

func (f *fakeIncidents) GetIncidentByID(context.Context, string) (domain.IncidentView, error) {
	if f.missing {
		return domain.IncidentView{}, &apierror.NotFoundError{}
	}
	return domain.IncidentView{State: f.state}, nil
}

func (f *fakeIncidents) UpdateIncident(context.Context, domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	return domain.UpdateIncidentResponse{}, nil
}

func (f *fakeIncidents) SearchIncidents(context.Context, domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if f.existing == "" {
		return domain.SearchIncidentsResponse{}, nil
	}
	return domain.SearchIncidentsResponse{Incidents: []domain.SearchIncidentView{{ID: &f.existing}}}, nil
}

type fakeServices map[string]string

func (f fakeServices) SearchITServices(_ context.Context, query string, _, _ int) ([]domain.ITService, int, error) {
	var out []domain.ITService
	for name, id := range f {
		if strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			out = append(out, domain.ITService{ID: id, Name: &name})
		}
	}
	return out, len(out), nil
}

func sink(incidents *fakeIncidents) IncidentSink {
	return IncidentSink{
		Incidents: incidents, CallerID: "caller", DefaultServiceID: "svc-default",
		Services: fakeServices{"Client-Prod-Alert-Integration": "svc-1", "client-prod-alert-integration-eu": "svc-2"},
	}
}

func TestIncidentSinkOpen(t *testing.T) {
	incidents := &fakeIncidents{}
	e := withID(event("major", 0), "A1")
	e.Alert.Source = "Azure Monitor"
	id, err := sink(incidents).Open(context.Background(), e)
	if err != nil || id != "inc-new" {
		t.Fatalf("Open = %q, %v", id, err)
	}
	req := incidents.created[0]
	if req.ServiceID != "svc-1" || req.CallerID != "caller" || req.Subject != "TEST" ||
		req.Impact != domain.IncidentImpactMedium || req.Urgency != domain.IncidentUrgencyHigh ||
		req.Category != domain.IncidentCategoryServiceInterruption || req.ContactType == nil || *req.ContactType != domain.IncidentContactTypeAzure ||
		req.CorrelationID == nil || *req.CorrelationID != "alert:A1" || req.Environment == nil || *req.Environment != "prod" {
		t.Fatalf("request = %+v", req)
	}
	if req.WorkNotes == nil || !strings.Contains(*req.WorkNotes, "Incident auto-created from Alert: A1") {
		t.Fatalf("work notes = %v, want the creation note", req.WorkNotes)
	}
}

func TestIncidentSinkOpenReusesTheIncidentAlreadyCreated(t *testing.T) {
	incidents := &fakeIncidents{existing: "inc-old"}
	id, err := sink(incidents).Open(context.Background(), withID(event("critical", 0), "A1"))
	if err != nil || id != "inc-old" || len(incidents.created) != 0 {
		t.Fatalf("Open = %q, %v, created %d; want the incident found by correlation id", id, err, len(incidents.created))
	}
}

func TestIncidentSinkFallsBackToTheDefaultService(t *testing.T) {
	incidents := &fakeIncidents{}
	e := withID(event("critical", 0), "A1")
	e.Alert.Service = "unknown-service"
	if _, err := sink(incidents).Open(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if incidents.created[0].ServiceID != "svc-default" {
		t.Fatalf("service = %q, want the default", incidents.created[0].ServiceID)
	}
}

func TestIncidentSinkIsOpen(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := map[string]struct {
		incidents *fakeIncidents
		want      bool
	}{
		"new":       {&fakeIncidents{state: str("NEW")}, true},
		"resolved":  {&fakeIncidents{state: str("Resolved")}, false},
		"closed":    {&fakeIncidents{state: str("CLOSED")}, false},
		"cancelled": {&fakeIncidents{state: str("CANCELLED")}, false},
		"deleted":   {&fakeIncidents{missing: true}, false},
	}
	for name, tt := range tests {
		got, err := sink(tt.incidents).IsOpen(context.Background(), "inc-1")
		if err != nil || got != tt.want {
			t.Errorf("%s: IsOpen = %v, %v; want %v", name, got, err, tt.want)
		}
	}
}
