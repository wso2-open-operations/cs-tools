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

// Package incident writes the processor's decisions to CSM incidents, through the incident service in-process.
package incident

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/severity"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Service is the part of the incident service the writer uses, called in-process.
type Service interface {
	CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)
	GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error)
	UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error)
	SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error)
}

// Notes tells whether an incident already carries a work note, which makes adding one safe to retry.
type Notes interface {
	HasNote(ctx context.Context, incidentID, marker string) (bool, error)
}

// Services finds CMDB services by name.
type Services interface {
	SearchITServices(ctx context.Context, searchQuery string, limit, offset int) ([]domain.ITService, int, error)
}

// Writer creates and updates CSM incidents through the incident service; it implements alertprocessor.IncidentWriter.
type Writer struct {
	Service  Service
	Services Services
	Notes    Notes
	// CallerID is the user every alert incident is raised by.
	CallerID string
	// DefaultServiceID is the service for an alert whose service label matches none.
	DefaultServiceID string
}

// serviceSearchLimit bounds the services read when matching a label; the search ranks an exact name first.
const serviceSearchLimit = 50

// Open creates the alert's incident; the alert id is the incident's correlation id, so a retry finds the incident it already created.
func (s Writer) Open(ctx context.Context, e event.Event) (string, error) {
	if id, err := s.Find(ctx, e.AlertID); err != nil || id != "" {
		return id, err
	}
	correlation := correlationID(e.AlertID)
	serviceID, err := s.serviceID(ctx, e.Alert.Service)
	if err != nil {
		return "", err
	}
	impact, urgency := severity.ImpactUrgency(e.Alert.Severity)
	note := creationNote(e)
	req := domain.CreateIncidentRequest{
		CallerID:      s.CallerID,
		Category:      severity.IncidentCategory(e.Alert.Category),
		ServiceID:     serviceID,
		ContactType:   severity.ContactType(e.Alert.Source),
		Impact:        impact,
		Urgency:       urgency,
		Subject:       subject(e),
		WorkNotes:     &note,
		CorrelationID: &correlation,
	}
	if env := strings.TrimSpace(e.Alert.Environment); env != "" {
		req.Environment = &env
	}
	resp, err := s.Service.CreateIncident(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}
	if resp.Incident.ID == "" {
		return "", errors.New("create incident: no id returned")
	}
	return resp.Incident.ID, nil
}

// IsOpen is false once the incident is resolved, closed or cancelled, so the next alert opens a new one.
func (s Writer) IsOpen(ctx context.Context, incidentID string) (bool, error) {
	view, err := s.Service.GetIncidentByID(ctx, incidentID)
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

// Find returns the incident the alert opened, or "" when it opened none.
func (s Writer) Find(ctx context.Context, alertID string) (string, error) {
	return s.byCorrelationID(ctx, correlationID(alertID))
}

// Fold adds the alert to the incident as a Duplicate or OK work note, once: a retry finds the note by the alert id it names.
func (s Writer) Fold(ctx context.Context, incidentID string, outcome strategy.Outcome, e event.Event) error {
	noted, err := s.Notes.HasNote(ctx, incidentID, noteMarker(e.AlertID))
	if err != nil || noted {
		return err
	}
	note := foldNote(outcome, e)
	_, err = s.Service.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: incidentID, WorkNotes: &note})
	return err
}

func (s Writer) byCorrelationID(ctx context.Context, correlation string) (string, error) {
	resp, err := s.Service.SearchIncidents(ctx, domain.SearchIncidentsRequest{
		Filters:    domain.SearchIncidentsFilters{CorrelationID: &correlation},
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
func (s Writer) serviceID(ctx context.Context, label string) (string, error) {
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

// correlationID ties an incident to the alert that opened it.
func correlationID(alertID string) string { return "alert:" + alertID }

// subject is the metric name, or the service when the alert has none.
func subject(e event.Event) string {
	if m := strings.TrimSpace(e.Alert.MetricName); m != "" {
		return m
	}
	return orNA(e.Alert.Service)
}
