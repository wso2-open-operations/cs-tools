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
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Incidents is the part of the incident service the sink uses, called in-process.
type Incidents interface {
	CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)
	GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error)
	UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error)
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error)
}

// Services finds CMDB services by name.
type Services interface {
	SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error)
}

// IncidentSink creates and updates CSM incidents through the incident service.
type IncidentSink struct {
	Incidents Incidents
	Services  Services
	// CallerID is the user every alert incident is raised by.
	CallerID string
	// DefaultServiceID is the service for an alert whose service label matches none.
	DefaultServiceID string
}

// serviceSearchLimit bounds the services read when matching a label; the search ranks an exact name first.
const serviceSearchLimit = 50

// Open creates the alert's incident; the alert id is the incident's correlation id, so a retry finds the incident it already created.
func (s IncidentSink) Open(ctx context.Context, e Event) (string, error) {
	correlationID := "alert:" + e.AlertID
	if id, err := s.byCorrelationID(ctx, correlationID); err != nil || id != "" {
		return id, err
	}
	serviceID, err := s.serviceID(ctx, e.Alert.Service)
	if err != nil {
		return "", err
	}
	impact, urgency := impactUrgency(e.Alert.Severity)
	note := creationNote(e)
	req := domain.CreateIncidentRequest{
		CallerID:      s.CallerID,
		Category:      incidentCategory(e.Alert.Category),
		ServiceID:     serviceID,
		ContactType:   contactType(e.Alert.Source),
		Impact:        impact,
		Urgency:       urgency,
		Subject:       subject(e),
		WorkNotes:     &note,
		CorrelationID: &correlationID,
	}
	if env := strings.TrimSpace(e.Alert.Environment); env != "" {
		req.Environment = &env
	}
	resp, err := s.Incidents.CreateIncident(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}
	if resp.Incident.ID == "" {
		return "", errors.New("create incident: no id returned")
	}
	return resp.Incident.ID, nil
}

// IsOpen is false once the incident is resolved, closed or cancelled, so the next alert opens a new one.
func (s IncidentSink) IsOpen(ctx context.Context, incidentID string) (bool, error) {
	view, err := s.Incidents.GetIncidentByID(ctx, incidentID)
	if err != nil {
		var notFound *apierror.NotFoundError
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("get incident %s: %w", incidentID, err)
	}
	if view.State == nil {
		return true, nil
	}
	switch domain.IncidentState(strings.ToUpper(*view.State)) {
	case domain.IncidentStateResolved, domain.IncidentStateClosed, domain.IncidentStateCancelled:
		return false, nil
	}
	return true, nil
}

// Note adds a work note to the incident.
func (s IncidentSink) Note(ctx context.Context, incidentID, html string) error {
	_, err := s.Incidents.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: incidentID, WorkNotes: &html})
	return err
}

func (s IncidentSink) byCorrelationID(ctx context.Context, correlationID string) (string, error) {
	resp, err := s.Incidents.SearchIncidents(ctx, domain.SearchIncidentsRequest{
		Filters:    domain.SearchIncidentsFilters{CorrelationID: &correlationID},
		Pagination: domain.Pagination{Limit: 1},
		SkipTotal:  true,
	})
	if err != nil {
		return "", fmt.Errorf("find incident by correlation id: %w", err)
	}
	if len(resp.Incidents) == 0 || resp.Incidents[0].ID == nil {
		return "", nil
	}
	return *resp.Incidents[0].ID, nil
}

// serviceID is the service named exactly by the label, or the default service when there is no label or no match.
func (s IncidentSink) serviceID(ctx context.Context, label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return s.DefaultServiceID, nil
	}
	services, _, err := s.Services.SearchITServices(ctx, label, serviceSearchLimit, 0)
	if err != nil {
		return "", fmt.Errorf("find service %q: %w", label, err)
	}
	for _, svc := range services {
		if svc.Name != nil && strings.EqualFold(*svc.Name, label) {
			return svc.ID, nil
		}
	}
	return s.DefaultServiceID, nil
}

// subject is the metric name, or the service when the alert has none.
func subject(e Event) string {
	if m := strings.TrimSpace(e.Alert.MetricName); m != "" {
		return m
	}
	return orNA(e.Alert.Service)
}
