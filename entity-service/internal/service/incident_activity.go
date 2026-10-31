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
	"context"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordIncidentFieldChanges writes the incident's activity-feed entries for an update that has
// already committed: one work_item_activity row per field the request changed. Best-effort, like
// caseService.recordFieldChangeActivity: a failure is logged and never fails the update. Skipped
// when the incident could not be read before the change (nothing to compare against) or there is
// no actor to attribute it to.
func (s *incidentService) recordIncidentFieldChanges(ctx context.Context, req domain.UpdateIncidentRequest, before, after domain.IncidentView, actorEmail string) {
	if before.ID == nil {
		return
	}
	s.writeIncidentFieldChanges(ctx, req.ID, incidentFieldChanges(req, before, after), actorEmail)
}

// afterChildIncidentsResolved does for each child incident the parent's update resolved
// (IncidentRepository.UpdateIncidentLifecycle's cascade) what UpdateIncident does for the parent
// after its commit: the state and resolution code activity rows, attributed to the same actor,
// and incident.acknowledged for a child that left NEW, so its call escalation stops. Best-effort,
// like the parent's.
func (s *incidentService) afterChildIncidentsResolved(ctx context.Context, children []repository.ResolvedChildIncident, actorEmail string) {
	resolved := string(domain.IncidentStateResolved)
	for _, c := range children {
		changes := []repository.IncidentFieldChange{
			{Field: "state", OldValue: incidentEnumLabel(&c.PreviousState), NewValue: incidentEnumLabel(&resolved)},
		}
		if derefString(c.PreviousResolutionCode) != derefString(c.ResolutionCode) {
			changes = append(changes, repository.IncidentFieldChange{
				Field: "resolution_code", OldValue: incidentEnumLabel(c.PreviousResolutionCode), NewValue: incidentEnumLabel(c.ResolutionCode),
			})
		}
		s.writeIncidentFieldChanges(ctx, c.ID, changes, actorEmail)
		if c.PreviousState == string(domain.IncidentStateNew) {
			publishIncidentAcknowledgedEvent(ctx, s.eventPublisher, c.ID, c.PreviousState, resolved)
		}
	}
}

// recordHandoffFieldChanges writes the activity-feed entries for a specialist handoff that has
// already committed: the assignment group it moved and the assignee it cleared. Best-effort and
// skipped without a baseline, as recordIncidentFieldChanges.
func (s *incidentService) recordHandoffFieldChanges(ctx context.Context, incidentID string, before, after domain.IncidentView, actorEmail string) {
	if before.ID == nil {
		return
	}
	s.writeIncidentFieldChanges(ctx, incidentID, handoffFieldChanges(before, after), actorEmail)
}

// writeIncidentFieldChanges records changes, attributed to actorEmail; nothing to record, or no
// actor to attribute it to, writes nothing. A failure is logged, never returned.
func (s *incidentService) writeIncidentFieldChanges(ctx context.Context, incidentID string, changes []repository.IncidentFieldChange, actorEmail string) {
	if len(changes) == 0 || actorEmail == "" {
		return
	}
	if err := s.repo.RecordIncidentFieldChanges(ctx, incidentID, changes, actorEmail); err != nil {
		slog.ErrorContext(ctx, "incident: record field change activity failed",
			"incidentId", incidentID, "fields", len(changes), "error", err)
	}
}

// handoffFieldChanges lists what a specialist handoff changed, in the feed's field order: it
// clears the assignee and moves the incident to the specialist group (ApplySpecialistHandoff).
func handoffFieldChanges(before, after domain.IncidentView) []repository.IncidentFieldChange {
	var out []repository.IncidentFieldChange
	for _, f := range []struct {
		field string
		b, a  *domain.EntityRef
	}{
		{"assigned_to_id", before.AssignedTo, after.AssignedTo},
		{"assignment_group_id", before.AssignmentGroup, after.AssignmentGroup},
	} {
		if incidentRefID(f.b) != incidentRefID(f.a) {
			out = append(out, repository.IncidentFieldChange{Field: f.field, OldValue: incidentRefLabel(f.b), NewValue: incidentRefLabel(f.a)})
		}
	}
	return out
}

// incidentFieldChanges lists the fields an update changed, comparing the incident before and after
// it. Only fields the request named are compared, so a change someone else committed in between is
// never attributed to this caller; priority also follows impact and urgency (it is recomputed from
// them), subcategory follows category (a new category can clear it), and the service offering
// follows the service. A field sent with the value it already had is not a change.
//
// Priority, impact and urgency stay separate rows, one plain value each; the incident feed groups a
// save's rows and shows impact/urgency under the priority they moved (webapp incidentActivity.ts).
//
// The field names key caseActivityFieldChangeLabel's labels. Values are what a person reads:
// enum values title-cased ("IN_PROGRESS" -> "In Progress"), references by name (their id when
// they have none), "" for none.
func incidentFieldChanges(req domain.UpdateIncidentRequest, before, after domain.IncidentView) []repository.IncidentFieldChange {
	var out []repository.IncidentFieldChange
	enumField := func(field string, sent bool, b, a *string) {
		if sent && derefString(b) != derefString(a) {
			out = append(out, repository.IncidentFieldChange{Field: field, OldValue: incidentEnumLabel(b), NewValue: incidentEnumLabel(a)})
		}
	}
	textField := func(field string, sent bool, b, a *string) {
		if sent && derefString(b) != derefString(a) {
			out = append(out, repository.IncidentFieldChange{Field: field, OldValue: derefString(b), NewValue: derefString(a)})
		}
	}
	refField := func(field string, sent bool, b, a *domain.EntityRef) {
		if sent && incidentRefID(b) != incidentRefID(a) {
			out = append(out, repository.IncidentFieldChange{Field: field, OldValue: incidentRefLabel(b), NewValue: incidentRefLabel(a)})
		}
	}

	enumField("state", req.State != nil, before.State, after.State)
	enumField("priority", req.Priority != nil || req.Impact != nil || req.Urgency != nil, before.Priority, after.Priority)
	enumField("impact", req.Impact != nil, before.Impact, after.Impact)
	enumField("urgency", req.Urgency != nil, before.Urgency, after.Urgency)
	enumField("category", req.Category != nil, before.Category, after.Category)
	enumField("subcategory", req.Subcategory != nil || req.Category != nil, before.Subcategory, after.Subcategory)
	enumField("contact_type", req.ContactType != nil, before.ContactType, after.ContactType)
	enumField("resolution_code", req.ResolutionCode != nil, before.ResolutionCode, after.ResolutionCode)
	textField("subject", req.Subject != nil, before.Subject, after.Subject)
	textField("environment", req.Environment != nil, before.Environment, after.Environment)
	refField("assigned_to_id", req.AssignedEngineerID != nil, before.AssignedTo, after.AssignedTo)
	refField("assignment_group_id", req.AssignmentGroupID != nil, before.AssignmentGroup, after.AssignmentGroup)
	refField("service_id", req.ServiceID != nil, before.Service, after.Service)
	refField("service_offering_id", req.ServiceOfferingID != nil || req.ServiceID != nil, before.ServiceOffering, after.ServiceOffering)
	refField("configuration_item_id", req.ConfigurationItemID != nil, before.ConfigurationItem, after.ConfigurationItem)
	refField("parent", req.ParentID != nil, before.Parent, after.Parent)
	refField("parent_incident_id", req.ParentIncidentID != nil, before.ParentIncident, after.ParentIncident)
	refField("change_request_id", req.ChangeRequestID != nil, before.ChangeRequest, after.ChangeRequest)
	refField("problem_id", req.ProblemID != nil, before.Problem, after.Problem)
	refField("caused_by_id", req.CausedByID != nil, before.CausedBy, after.CausedBy)
	return out
}

// incidentEnumLabel renders a stored enum value for a person: "IN_PROGRESS" -> "In Progress".
func incidentEnumLabel(v *string) string {
	s := strings.TrimSpace(derefString(v))
	if s == "" {
		return ""
	}
	return humanizeSnakeCase(strings.ToLower(s))
}

func incidentRefID(r *domain.EntityRef) string {
	if r == nil {
		return ""
	}
	return r.ID
}

func incidentRefLabel(r *domain.EntityRef) string {
	if r == nil {
		return ""
	}
	if name := strings.TrimSpace(r.Name); name != "" {
		return name
	}
	return r.ID
}
