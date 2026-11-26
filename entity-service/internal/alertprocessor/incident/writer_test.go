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

package incident

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
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

func alert(id, severity string) event.Event {
	return event.Event{AlertID: id, ReceivedOn: time.Now(), Alert: model.Alert{
		Service: "client-prod-alert-integration", MetricName: "TEST", Severity: severity,
		Category: "service_interruption", Environment: "prod", Source: "elasticsearch", UniqueIdentifier: "r2",
	}}
}

type fakeNotes struct{ has bool }

func (f fakeNotes) HasNote(context.Context, string, string) (bool, error) { return f.has, nil }

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

func newWriter(incidents *fakeIncidents) Writer {
	return Writer{
		Service: incidents, Notes: fakeNotes{}, CallerID: "caller", DefaultServiceID: "svc-default",
		Services: fakeServices{"Client-Prod-Alert-Integration": "svc-1", "client-prod-alert-integration-eu": "svc-2"},
	}
}

func TestWriterOpen(t *testing.T) {
	incidents := &fakeIncidents{}
	e := alert("A1", "major")
	e.Alert.Source = "Azure Monitor"
	id, err := newWriter(incidents).Open(context.Background(), e)
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

func TestWriterOpenReusesTheIncidentAlreadyCreated(t *testing.T) {
	incidents := &fakeIncidents{existing: "inc-old"}
	id, err := newWriter(incidents).Open(context.Background(), alert("A1", "critical"))
	if err != nil || id != "inc-old" || len(incidents.created) != 0 {
		t.Fatalf("Open = %q, %v, created %d; want the incident found by correlation id", id, err, len(incidents.created))
	}
}

func TestWriterFallsBackToTheDefaultService(t *testing.T) {
	incidents := &fakeIncidents{}
	e := alert("A1", "critical")
	e.Alert.Service = "unknown-service"
	if _, err := newWriter(incidents).Open(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if incidents.created[0].ServiceID != "svc-default" {
		t.Fatalf("service = %q, want the default", incidents.created[0].ServiceID)
	}
}

func TestWriterIsOpen(t *testing.T) {
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
		got, err := newWriter(tt.incidents).IsOpen(context.Background(), "inc-1")
		if err != nil || got != tt.want {
			t.Errorf("%s: IsOpen = %v, %v; want %v", name, got, err, tt.want)
		}
	}
}

// recordingIncidents counts work notes, to show a note already on the incident is not added again.
type recordingIncidents struct {
	fakeIncidents
	notes int
}

func (r *recordingIncidents) UpdateIncident(context.Context, domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	r.notes++
	return domain.UpdateIncidentResponse{}, nil
}

func TestWriterFoldIsIdempotent(t *testing.T) {
	for name, tt := range map[string]struct {
		already bool
		want    int
	}{"first time": {false, 1}, "retry after the note was added": {true, 0}} {
		incidents := &recordingIncidents{}
		s := Writer{Service: incidents, Notes: fakeNotes{has: tt.already}}
		if err := s.Fold(context.Background(), "inc-1", strategy.Duplicate, alert("A2", "critical")); err != nil {
			t.Fatal(err)
		}
		if incidents.notes != tt.want {
			t.Errorf("%s: %d notes added, want %d", name, incidents.notes, tt.want)
		}
	}
}
