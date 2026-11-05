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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A service request carries an internal severity, S1-S4 (never S0). It is set
// by internal users and integration clients only, never shown to customers, and
// has no effect on SLA clocks or notification events. It is stored in Postgres
// only: nothing here is ever sent to the backing data source.

// validServiceRequestSeverity is validCaseSeverity without catastrophic (S0).
var validServiceRequestSeverity = map[domain.CaseSeverity]bool{
	domain.CaseSeverityCritical: true,
	domain.CaseSeverityHigh:     true,
	domain.CaseSeverityMedium:   true,
	domain.CaseSeverityLow:      true,
}

// serviceRequestSeverityByLabel maps the S1-S4 labels of the configuration to
// the severity values of the API.
var serviceRequestSeverityByLabel = map[string]domain.CaseSeverity{
	"S1": domain.CaseSeverityCritical,
	"S2": domain.CaseSeverityHigh,
	"S3": domain.CaseSeverityMedium,
	"S4": domain.CaseSeverityLow,
}

// WithServiceRequestDefaultSeverity sets the severity a service request gets
// when it is created without one (SERVICE_REQUEST_DEFAULT_SEVERITY). An empty
// label means S4. Startup validation (config.Config.Validate) already refused
// any other value, so an unrecognised label here falls back to S4 and is
// logged, never fatal. Same post-construction wiring shape as
// WithCSEngineerRole.
func WithServiceRequestDefaultSeverity(svc CaseService, label string) CaseService {
	cs, ok := svc.(*caseService)
	if !ok {
		return svc
	}
	norm, err := config.NormalizeServiceRequestSeverity(label)
	if err != nil {
		slog.Error("service request default severity is invalid, using S4", "value", label)
		norm = config.DefaultServiceRequestSeverity
	}
	cs.srDefaultSeverity = serviceRequestSeverityByLabel[norm]
	return svc
}

// serviceRequestDefaultSeverity is the configured default, S4 (low) when none
// was wired.
func (s *caseService) serviceRequestDefaultSeverity() domain.CaseSeverity {
	if s.srDefaultSeverity != "" {
		return s.srDefaultSeverity
	}
	return domain.CaseSeverityLow
}

// callerMaySetInternalSeverity reports whether the caller may set a service
// request's severity: an internal user or a machine-to-machine integration, as
// decided from platform data (the user's type in Postgres, the configured
// client ids), never from a token role claim. A request that arrived through
// the customer portal's backend is excluded even when its user is staff: that
// portal is the customer's view, and the severity is not part of it. An
// identity that cannot be resolved is not trusted.
func (s *caseService) callerMaySetInternalSeverity(ctx context.Context) bool {
	if s.access == nil {
		return false
	}
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return false
	}
	return scope.Unrestricted && !scope.ViaCustomerPortal
}

// applyServiceRequestSeverityOnCreate settles req.Severity for a service
// request being created, so every service request has one:
//
//   - an external caller's severity is dropped, not refused: the field is not
//     theirs, and clients that send a case-shaped body for every type have
//     always had it ignored on a service request;
//   - an internal caller's explicit severity must be S1-S4;
//   - none (or dropped) becomes the configured default.
//
// It does nothing for any other type.
func (s *caseService) applyServiceRequestSeverityOnCreate(ctx context.Context, req *domain.CreateCaseRequest) error {
	if req.Type != "service_request" {
		return nil
	}
	if req.Severity != "" {
		if !s.callerMaySetInternalSeverity(ctx) {
			slog.InfoContext(ctx, "create service request: severity ignored for a caller that may not set it")
			req.Severity = ""
		} else if !validServiceRequestSeverity[req.Severity] {
			return &apierror.ValidationError{Msg: "severity contains invalid value for a service request: " + string(req.Severity) + " (allowed: critical, high, medium, low)"}
		}
	}
	if req.Severity == "" {
		req.Severity = s.serviceRequestDefaultSeverity()
	}
	return nil
}

// updateServiceRequestSeverity changes a service request's severity. Internal
// callers only (403 otherwise); S1-S4 only. It touches nothing but the severity:
// no SLA clock is revised, no severity-changed notification is published and
// nothing is mirrored to the backing data source, because a service request's
// severity exists in Postgres only. The change is recorded in the activity
// feed like a case's.
func (s *caseService) updateServiceRequestSeverity(ctx context.Context, req domain.UpdateCaseRequest) (domain.UpdateCaseResponse, error) {
	if !s.callerMaySetInternalSeverity(ctx) {
		return domain.UpdateCaseResponse{}, &apierror.ForbiddenError{Msg: "only internal users can set a service request's severity"}
	}
	if !validServiceRequestSeverity[*req.Severity] {
		return domain.UpdateCaseResponse{}, &apierror.ValidationError{Msg: "severity contains invalid value for a service request: " + string(*req.Severity) + " (allowed: critical, high, medium, low)"}
	}
	var actorEmail string
	var actorID *string
	if actor, err := s.resolveActor(ctx); err == nil {
		actorEmail = actor.Email
		if actor.ID != "" {
			actorID = &actor.ID
		}
	}
	c, previous, _, err := s.repo.UpdateCase(ctx, req, actorID)
	if err != nil {
		return domain.UpdateCaseResponse{}, err
	}
	if c.Severity != nil && derefSeverity(previous) != *c.Severity {
		s.recordFieldChangeActivity(ctx, req.ID, "severity", humanizeSnakeCase(string(derefSeverity(previous))), humanizeSnakeCase(string(*c.Severity)), actorEmail)
	}
	return domain.UpdateCaseResponse{
		Message: "Case updated successfully",
		Case:    domain.UpdatedCase{ID: c.ID, UpdatedOn: c.UpdatedOn, Severity: c.Severity, State: c.State, WorkState: c.WorkState},
	}, nil
}

// slaSeverity is the severity to hand an SLA clock registration or revision for
// a case-like record: a service request's severity never reaches the SLA
// engine, so its clocks stay severity-agnostic.
func slaSeverity(cv domain.CaseView) *domain.CaseSeverity {
	if cv.Type != nil && *cv.Type == "service_request" {
		return nil
	}
	return cv.Severity
}

// eventSeverity is the severity a case.* event payload reports. A service
// request's severity is internal and is not part of any event.
func eventSeverity(cv domain.CaseView) *domain.CaseSeverity {
	return slaSeverity(cv)
}
