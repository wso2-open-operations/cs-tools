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
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	defaultSLAStatusLimit = 500
	// maxSLAStatusLimit is far above the generic 50-row cap other search
	// endpoints use. An earlier design had integrations/csm-notification-service
	// poll this endpoint continuously for currently-active clocks (around
	// 5,500 rows checked live) — abandoned in favor of a Redis-based engine
	// that tracks and alerts without polling at all (see that repo's own
	// CLAUDE.md). It now reaches this endpoint only for a one-shot,
	// source=csm-scoped reconciliation pass at startup (see the source
	// parameter's own doc comment on SearchActiveSLAStatuses), a much
	// smaller read than the old poll ever was. The cap stays generous
	// regardless: a human paging through the CSM portal's own SLA tab is
	// the other real caller, and a low cap would only turn one intended
	// round trip into many for no benefit to anyone.
	maxSLAStatusLimit = 2000
)

// normalizeSLAStatusPagination applies this endpoint's own defaults/cap —
// see maxSLAStatusLimit's own doc comment for why they differ from
// normalizePagination's generic ones.
func normalizeSLAStatusPagination(p *domain.Pagination) error {
	if p.Limit <= 0 {
		p.Limit = defaultSLAStatusLimit
	}
	if p.Limit > maxSLAStatusLimit {
		return &apierror.ValidationError{Msg: fmt.Sprintf("limit cannot exceed %d", maxSLAStatusLimit)}
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}

type slaStatusService struct {
	repo                     repository.SLAStatusRepository
	access                   AccessService
	csmPortalBackendClientID string
	publisher                EventPublisherService
}

// NewSLAStatusService constructs an SLAStatusService backed by the given
// repository. access gates every read to internal callers (an Unrestricted
// AccessScope) -- see requireInternalCaller's own doc comment for why:
// unlike every other Postgres-backed read, this endpoint has no per-project/
// per-case filtering of its own to scope by (it returns every currently-
// active clock across every case in one bulk list), so there is no scope
// short of "internal service" that would be safe to hand this out under.
// csmPortalBackendClientID is CSM_PORTAL_BACKEND_CLIENT_ID, gating
// SearchActiveSLAStatuses' own admin=true request and RequestRecalculation
// -- the CSM portal's "Recalculate SLA" admin action -- the same way
// portalBannerService.requireCSMPortalBackendCaller gates banner mutations:
// trusting the caller's client id instead of re-checking AUTH_ADMIN_ROLES
// here. This used to check AUTH_ADMIN_ROLES directly (requireAdminRole), which
// requires that list to be configured identically in both csm-portal-backend
// (which already gates this route on its own PermAdmin check) and here --
// nothing enforces the two copies agree, and a real admin whose role name
// only matched one of the two copies got a 403 from entity-service despite
// passing csm-portal-backend's own gate. publisher may be nil (Event Hub not
// configured for this deployment); RequestRecalculation reports that as a
// ServiceUnavailableError rather than silently doing nothing.
func NewSLAStatusService(repo repository.SLAStatusRepository, access AccessService, csmPortalBackendClientID string, publisher EventPublisherService) SLAStatusService {
	return &slaStatusService{repo: repo, access: access, csmPortalBackendClientID: csmPortalBackendClientID, publisher: publisher}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- delegates to the shared RequireInternalCaller (require_internal.go).
func (s *slaStatusService) requireInternalCaller(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "sla status is only available to internal services")
}

// requireCSMPortalBackendCaller gates the SLA engine's admin actions
// (SearchActiveSLAStatuses' own admin=true request, RequestRecalculation) on
// the caller being apps/csm-portal/backend itself -- mirrors
// portalBannerService.requireCSMPortalBackendCaller exactly; see that
// method's own doc comment for the full reasoning (one admin check, in
// csm-portal-backend's own PermAdmin gate, not two independently-configured
// AUTH_ADMIN_ROLES lists that can silently drift apart).
func (s *slaStatusService) requireCSMPortalBackendCaller(ctx context.Context) error {
	if err := RequireInternalCaller(ctx, s.access, "the SLA engine admin actions are only available through the CSM portal"); err != nil {
		return err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "the SLA engine admin actions can only be used by a signed-in person"}
	}
	if s.csmPortalBackendClientID == "" || id.ClientID != s.csmPortalBackendClientID {
		return &apierror.ForbiddenError{Msg: "the SLA engine admin actions can only be used through the CSM Portal"}
	}
	return nil
}

// slaClockTargetFilters maps the lowercase, caller-facing target values
// GetClockState accepts to sla_policy_target_enum's real labels -- same
// explicit-allow-list reasoning as slaStatusSourceFilters above. Unlike
// that map, there is no "" entry: target is required for GetClockState (a
// single clock lookup needs to know which of a work item's up-to-three
// clocks it's asking about), so an empty value is rejected the same way an
// unrecognized one is.
var slaClockTargetFilters = map[string]string{
	"response":   "RESPONSE",
	"workaround": "WORKAROUND",
	"resolution": "RESOLUTION",
}

// SearchActiveSLAStatuses implements SLAStatusService. admin=true swaps the
// ordinary internal-caller check for requireCSMPortalBackendCaller -- it
// only ever narrows who may call this with admin=true, never widens the
// plain (admin=false) call every existing internal caller already makes
// (csm-notification-service's own startup reconciliation included), so this
// is purely additive: nothing about the non-admin path's behavior changes
// for a caller that never sends it.
func (s *slaStatusService) SearchActiveSLAStatuses(ctx context.Context, req domain.Pagination, admin bool, filters domain.SLAStatusFilters) (domain.SearchSLAStatusResponse, error) {
	if admin {
		// requireCSMPortalBackendCaller already includes the internal-caller
		// check, so this is the admin=true request's only gate -- no
		// redundant second call.
		if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
			return domain.SearchSLAStatusResponse{}, err
		}
	} else if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	return s.searchActiveSLAStatuses(ctx, req, filters)
}

// searchActiveSLAStatuses is SearchActiveSLAStatuses' shared body, once the
// caller's own access check (above) has passed.
func (s *slaStatusService) searchActiveSLAStatuses(ctx context.Context, req domain.Pagination, filters domain.SLAStatusFilters) (domain.SearchSLAStatusResponse, error) {
	if err := normalizeSLAStatusPagination(&req); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	statuses, total, err := s.repo.SearchActiveSLAStatuses(ctx, req, filters)
	if err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	return domain.SearchSLAStatusResponse{
		Statuses: statuses,
		Total:    total,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}, nil
}

// slaStatusCaseTypeFilters maps the lowercase, caller-facing caseType query
// value GET /sla-status accepts to work_item_type_enum's real uppercase
// labels -- same explicit-allow-list shape as slaClockTargetFilters below,
// restricted to the five case-like types activeSLAStatusFromJoins' own WHERE
// clause already scopes this endpoint to (caseLikeWorkItemTypes).
var slaStatusCaseTypeFilters = map[string]string{
	"case":                     "CASE",
	"engagement":               "ENGAGEMENT",
	"service_request":          "SERVICE_REQUEST",
	"security_report_analysis": "SECURITY_REPORT_ANALYSIS",
	"announcement":             "ANNOUNCEMENT",
}

// slaStatusSeverityFilters maps the lowercase, caller-facing severity query
// value to domain.CaseSeverity -- the same vocabulary the rest of this
// codebase's severity filters/writes already use (see case_repo.go's own
// caseSeverityToEnum, which the repository applies to turn this into the
// real "S0".."S4" column value).
var slaStatusSeverityFilters = map[string]domain.CaseSeverity{
	"catastrophic": domain.CaseSeverityCatastrophic,
	"critical":     domain.CaseSeverityCritical,
	"high":         domain.CaseSeverityHigh,
	"medium":       domain.CaseSeverityMedium,
	"low":          domain.CaseSeverityLow,
}

// slaStatusElapsedBucketFilters is the caller-facing spelling of
// domain.SLAStatusFilters.ElapsedBucket's four accepted values -- identical
// to the constants themselves today, kept as its own allow-list (rather than
// comparing the raw query value directly) so the wire vocabulary and the
// internal constant are free to diverge later without a breaking API change.
var slaStatusElapsedBucketFilters = map[string]string{
	"under50": domain.SLAElapsedBucketUnder50,
	"50to75":  domain.SLAElapsedBucket50To75,
	"75to100": domain.SLAElapsedBucket75To100,
	"over100": domain.SLAElapsedBucketOver100,
}

// ParseSLAStatusFilters validates GET /sla-status's optional severity/
// caseType/search/elapsedBucket query params into a domain.SLAStatusFilters.
// Every param is optional; an absent (empty string) one leaves that field at
// its zero value, applying no filtering for it. search is passed through
// verbatim (trimmed) -- see domain.SLAStatusFilters.Search's own doc comment
// for what it matches against.
func ParseSLAStatusFilters(severity, caseType, search, elapsedBucket string) (domain.SLAStatusFilters, error) {
	var f domain.SLAStatusFilters
	if severity != "" {
		sev, ok := slaStatusSeverityFilters[strings.ToLower(severity)]
		if !ok {
			return domain.SLAStatusFilters{}, &apierror.ValidationError{Msg: "severity must be one of: catastrophic, critical, high, medium, low"}
		}
		f.Severity = sev
	}
	if caseType != "" {
		ct, ok := slaStatusCaseTypeFilters[strings.ToLower(caseType)]
		if !ok {
			return domain.SLAStatusFilters{}, &apierror.ValidationError{Msg: "caseType must be one of: case, engagement, service_request, security_report_analysis, announcement"}
		}
		f.CaseType = ct
	}
	f.Search = strings.TrimSpace(search)
	if elapsedBucket != "" {
		b, ok := slaStatusElapsedBucketFilters[strings.ToLower(elapsedBucket)]
		if !ok {
			return domain.SLAStatusFilters{}, &apierror.ValidationError{Msg: "elapsedBucket must be one of: under50, 50to75, 75to100, over100"}
		}
		f.ElapsedBucket = b
	}
	return f, nil
}

// slaRecalculationEntityID is the fixed Event Hub partition key
// TypeSLARecalculationRequested publishes under -- there is no single
// case/incident this request is "about", unlike every other event this
// service publishes.
const slaRecalculationEntityID = "sla-engine"

// RequestRecalculation implements SLAStatusService.
func (s *slaStatusService) RequestRecalculation(ctx context.Context) error {
	if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
		return err
	}
	if s.publisher == nil {
		return &apierror.ServiceUnavailableError{Msg: "SLA recalculation is not available in this deployment"}
	}
	requestedBy := auth.IdentityFromContext(ctx).UserEmail
	payload, err := json.Marshal(events.SLARecalculationRequestedPayload{
		RequestedByEmail: requestedBy,
		RequestedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("encode sla.recalculation_requested payload: %w", err)
	}
	if err := s.publisher.Publish(ctx, events.TypeSLARecalculationRequested, slaRecalculationEntityID, payload); err != nil {
		return fmt.Errorf("publish sla.recalculation_requested: %w", err)
	}
	// requestedBy (an email address) is deliberately not logged here -- see
	// this repo's own "No PII in keys or logs" convention. It's carried in
	// the published payload itself for any downstream consumer that
	// genuinely needs it for audit.
	slog.InfoContext(ctx, "sla engine: recalculation requested")
	return nil
}

// GetClockState implements SLAStatusService.
func (s *slaStatusService) GetClockState(ctx context.Context, workItemID, target string) (domain.SLAClockState, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SLAClockState{}, err
	}
	if !validate.IsUUID(workItemID) {
		return domain.SLAClockState{}, &apierror.ValidationError{Msg: "workItemId must be a valid UUID"}
	}
	targetEnum, ok := slaClockTargetFilters[strings.ToLower(target)]
	if !ok {
		return domain.SLAClockState{}, &apierror.ValidationError{Msg: "target must be one of: response, workaround, resolution"}
	}
	return s.repo.GetClockState(ctx, workItemID, targetEnum)
}
