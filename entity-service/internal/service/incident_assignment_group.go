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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// assignmentGroupLookups is how resolveAssignmentGroup reads one data source:
// Postgres (incidentService) or ServiceNow (snIncidentService). The decision
// itself is the same code for both.
type assignmentGroupLookups struct {
	// supportGroupOf returns the service's name and support group; Found is
	// false when no service has the id.
	supportGroupOf func(ctx context.Context, serviceID string) (repository.ServiceSupportGroup, error)
	// isSupportGroup reports whether groupID is in the set an explicit
	// assignmentGroupId must belong to: active groups that are the support
	// group of at least one service.
	isSupportGroup func(ctx context.Context, groupID string) (bool, error)
}

// assignmentGroupDecision is what resolveAssignmentGroup chose: the group (nil
// = unassigned) and the creation work note saying why ("" = none).
type assignmentGroupDecision struct {
	groupID *string
	note    string
}

// errAssignmentGroupNotAllowed is the 400 for an explicit assignmentGroupId
// outside the support-group set.
const errAssignmentGroupNotAllowed = "assignmentGroupId must be the active support group of a service"

// resolveAssignmentGroup decides an incident's assignment group on create.
//
// *** THE ONLY PLACE THE GROUP IS CHOSEN. *** Every caller -- the portal, the
// microapp, alert-born incidents from sre-alert-core-service, any M2M client --
// and every DATA_SOURCE goes through it, once, before any write:
//
//  1. assignmentGroupId sent: used if it is in the support-group set, else a
//     400 (a value that is not a UUID is a 400 too). A blank value is not sent.
//  2. not sent, the service has a support group: that group.
//  3. not sent, the service has none (or does not exist): the support group of
//     the default service, with a warning naming the service.
//  4. no default service, or it does not exist or has no support group:
//     unassigned, logged as an error (a misconfiguration, not a bad request).
//  5. a lookup that fails returns its error: the incident is not created,
//     rather than created unassigned.
//
// In dual-write mode it runs on Postgres before the ServiceNow create, so both
// stores get the same group and a refused group never reaches ServiceNow.
//
// An invalid serviceId is left for request validation to reject.
func resolveAssignmentGroup(ctx context.Context, req domain.CreateIncidentRequest, defaultServiceID string, l assignmentGroupLookups) (assignmentGroupDecision, error) {
	if req.AssignmentGroupID != nil {
		if sent := strings.TrimSpace(*req.AssignmentGroupID); sent != "" {
			if err := validateUUIDs("assignmentGroupId", []string{sent}); err != nil {
				return assignmentGroupDecision{}, err
			}
			ok, err := l.isSupportGroup(ctx, sent)
			if err != nil {
				return assignmentGroupDecision{}, err
			}
			if !ok {
				return assignmentGroupDecision{}, &apierror.ValidationError{Msg: errAssignmentGroupNotAllowed, Code: apierror.CodeIncidentAssignmentGroupNotAllowed}
			}
			return assignmentGroupDecision{groupID: &sent, note: "Assignment group chosen by " + actorOf(ctx)}, nil
		}
	}

	serviceID := strings.TrimSpace(req.ServiceID)
	if serviceID == "" || validateUUIDs("serviceId", []string{serviceID}) != nil {
		return assignmentGroupDecision{}, nil
	}
	svc, err := l.supportGroupOf(ctx, serviceID)
	if err != nil {
		return assignmentGroupDecision{}, err
	}
	serviceLabel := nameOrID(svc.ServiceName, serviceID)
	if svc.GroupID != "" {
		group := svc.GroupID
		return assignmentGroupDecision{groupID: &group, note: "Assignment group set from service " + serviceLabel + "'s support group"}, nil
	}

	defaultServiceID = strings.TrimSpace(defaultServiceID)
	if defaultServiceID == "" {
		slog.ErrorContext(ctx, "incident create: the service has no support group and INCIDENT_DEFAULT_SERVICE_ID is not set; creating the incident unassigned",
			"serviceId", serviceID, "serviceName", svc.ServiceName)
		return assignmentGroupDecision{}, nil
	}
	if strings.EqualFold(defaultServiceID, serviceID) {
		slog.ErrorContext(ctx, "incident create: the default service has no support group; creating the incident unassigned",
			"serviceId", serviceID, "serviceName", svc.ServiceName)
		return assignmentGroupDecision{}, nil
	}
	def, err := l.supportGroupOf(ctx, defaultServiceID)
	if err != nil {
		return assignmentGroupDecision{}, err
	}
	if def.GroupID == "" {
		reason := "the default service has no support group"
		if !def.Found {
			reason = "the default service does not exist"
		}
		slog.ErrorContext(ctx, "incident create: the service has no support group and "+reason+"; creating the incident unassigned",
			"serviceId", serviceID, "serviceName", svc.ServiceName, "defaultServiceId", defaultServiceID)
		return assignmentGroupDecision{}, nil
	}
	slog.WarnContext(ctx, "incident create: the service has no support group; assigning the default service's support group",
		"serviceId", serviceID, "serviceName", svc.ServiceName, "defaultServiceId", defaultServiceID, "groupId", def.GroupID)
	group := def.GroupID
	return assignmentGroupDecision{
		groupID: &group,
		note:    "Service " + serviceLabel + " has no support group; assigned to the default team (" + nameOrID(def.GroupName, def.GroupID) + ")",
	}, nil
}

// withAssignmentGroup applies d to req: the group, and d's note as a work
// note -- after the caller's own work notes, in a paragraph of its own, so the
// caller's text is kept as sent.
func withAssignmentGroup(req domain.CreateIncidentRequest, d assignmentGroupDecision) domain.CreateIncidentRequest {
	req.AssignmentGroupID = d.groupID
	if d.note == "" {
		return req
	}
	note := d.note
	if req.WorkNotes != nil && strings.TrimSpace(*req.WorkNotes) != "" {
		note = *req.WorkNotes + "\n\n" + d.note
	}
	req.WorkNotes = &note
	return req
}

// incidentCreateDefaults answers GET /incidents/create-defaults from l: the
// default service and its support group. A lookup failure is an error; a
// default that is unset, missing or groupless is a null defaultGroup.
func incidentCreateDefaults(ctx context.Context, defaultServiceID string, l assignmentGroupLookups) (domain.IncidentCreateDefaults, error) {
	defaultServiceID = strings.TrimSpace(defaultServiceID)
	if defaultServiceID == "" {
		return domain.IncidentCreateDefaults{}, nil
	}
	out := domain.IncidentCreateDefaults{DefaultServiceID: &defaultServiceID}
	def, err := l.supportGroupOf(ctx, defaultServiceID)
	if err != nil {
		return domain.IncidentCreateDefaults{}, err
	}
	if def.GroupID != "" {
		out.DefaultGroup = &domain.EntityRef{ID: def.GroupID, Name: nameOrID(def.GroupName, def.GroupID)}
	}
	return out, nil
}

// nameOrID is name, or id when the name is blank.
func nameOrID(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return id
}

// WithIncidentDefaultService sets the default service (INCIDENT_DEFAULT_SERVICE_ID)
// whose support group an incident gets when its own service has none. Applies
// to the Postgres and the ServiceNow incident services; the dual-write
// ServiceNow mirror never decides a group, so it needs none.
func WithIncidentDefaultService(svc IncidentService, serviceID string) IncidentService {
	switch s := svc.(type) {
	case *incidentService:
		s.defaultServiceID = strings.TrimSpace(serviceID)
	case *snIncidentService:
		s.defaultServiceID = strings.TrimSpace(serviceID)
	}
	return svc
}

// CheckIncidentDefaultService logs, once at startup, whether the default
// service is usable: an error when it does not exist or has no support group
// (every incident on a groupless service would then be created unassigned), a
// warning when it could not be checked. Best effort: it never stops the
// service, and does nothing when no default service is configured.
func CheckIncidentDefaultService(ctx context.Context, svc IncidentService, serviceID string) {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return
	}
	// A startup check has no caller; it reads only service and "group",
	// neither row-level secured, as the system.
	ctx, cancel := context.WithTimeout(repository.WithSystemIdentity(ctx), 30*time.Second)
	defer cancel()
	defaults, err := svc.GetIncidentCreateDefaults(ctx)
	switch {
	case err != nil:
		slog.WarnContext(ctx, "INCIDENT_DEFAULT_SERVICE_ID could not be checked", "serviceId", serviceID, "error", err)
	case defaults.DefaultGroup == nil:
		slog.ErrorContext(ctx, "INCIDENT_DEFAULT_SERVICE_ID names a service that does not exist or has no support group; incidents on a service with no support group will be created unassigned",
			"serviceId", serviceID)
	default:
		slog.InfoContext(ctx, "incident default team", "serviceId", serviceID, "groupId", defaults.DefaultGroup.ID, "groupName", defaults.DefaultGroup.Name)
	}
}
