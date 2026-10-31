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
	repo       repository.SLAStatusRepository
	access     AccessService
	adminRoles []string
	publisher  EventPublisherService
}

// NewSLAStatusService constructs an SLAStatusService backed by the given
// repository. access gates every read to internal callers (an Unrestricted
// AccessScope) -- see requireInternalCaller's own doc comment for why:
// unlike every other Postgres-backed read, this endpoint has no per-project/
// per-case filtering of its own to scope by (it returns every currently-
// active clock across every case in one bulk list), so there is no scope
// short of "internal service" that would be safe to hand this out under.
// adminRoles is AUTH_ADMIN_ROLES, additionally gating SearchActiveSLAStatuses'
// own admin=true request and RequestRecalculation -- the CSM portal's
// "Recalculate SLA" admin action. publisher may be nil (Event Hub not
// configured for this deployment); RequestRecalculation reports that as a
// ServiceUnavailableError rather than silently doing nothing.
func NewSLAStatusService(repo repository.SLAStatusRepository, access AccessService, adminRoles []string, publisher EventPublisherService) SLAStatusService {
	return &slaStatusService{repo: repo, access: access, adminRoles: adminRoles, publisher: publisher}
}

// requireInternalCaller rejects anyone whose AccessScope is not Unrestricted
// -- delegates to the shared RequireInternalCaller (require_internal.go).
func (s *slaStatusService) requireInternalCaller(ctx context.Context) error {
	return RequireInternalCaller(ctx, s.access, "sla status is only available to internal services")
}

// requireAdminRole rejects anyone who isn't a holder of one of adminRoles --
// the role-only half of the admin gate. Callers that also need the internal-
// caller check (every one of them, today) call requireInternalCaller first;
// kept separate rather than folded into one function so
// SearchActiveSLAStatuses can skip a redundant second requireInternalCaller
// call when admin=true is also requested on an already-internal-caller request.
func (s *slaStatusService) requireAdminRole(ctx context.Context) error {
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" || !holdsAnyRole(id.Roles, s.adminRoles) {
		return &apierror.ForbiddenError{Msg: "only a CSM admin can manage the SLA engine"}
	}
	return nil
}

// requireAdmin rejects anyone who isn't both an internal caller (reached
// through the portal BFF, not a bare end-user token) and a holder of one of
// adminRoles -- same two-part gate as portalBannerService.requireAdmin.
func (s *slaStatusService) requireAdmin(ctx context.Context) error {
	if err := RequireInternalCaller(ctx, s.access, "the SLA engine admin actions are only available through the CSM portal"); err != nil {
		return err
	}
	return s.requireAdminRole(ctx)
}

// slaStatusSourceFilters maps the lowercase, caller-facing sourceFilter
// values this endpoint accepts to sla_source_enum's real labels -- kept as
// an explicit allow-list (rather than just upper-casing whatever the caller
// sends) so an unrecognized value is a 400 naming the problem, not a query
// bound to a string that can never match any row.
var slaStatusSourceFilters = map[string]string{
	"":           "",
	"csm":        "CSM",
	"servicenow": "SERVICENOW",
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

// SearchActiveSLAStatuses implements SLAStatusService. admin=true adds the
// CSM-admin role check on top of the ordinary internal-caller check -- it
// only ever narrows who may call this with admin=true, never widens the
// plain (admin=false) call every existing internal caller already makes
// (csm-notification-service's own startup reconciliation included), so this
// is purely additive: nothing about the non-admin path's behavior changes
// for a caller that never sends it.
func (s *slaStatusService) SearchActiveSLAStatuses(ctx context.Context, req domain.Pagination, sourceFilter string, admin bool) (domain.SearchSLAStatusResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	if admin {
		if err := s.requireAdminRole(ctx); err != nil {
			return domain.SearchSLAStatusResponse{}, err
		}
	}
	return s.searchActiveSLAStatuses(ctx, req, sourceFilter)
}

// searchActiveSLAStatuses is SearchActiveSLAStatuses' shared body, once the
// caller's own access check (above) has passed.
func (s *slaStatusService) searchActiveSLAStatuses(ctx context.Context, req domain.Pagination, sourceFilter string) (domain.SearchSLAStatusResponse, error) {
	if err := normalizeSLAStatusPagination(&req); err != nil {
		return domain.SearchSLAStatusResponse{}, err
	}
	source, ok := slaStatusSourceFilters[strings.ToLower(sourceFilter)]
	if !ok {
		return domain.SearchSLAStatusResponse{}, &apierror.ValidationError{Msg: "source must be one of: csm, servicenow"}
	}
	statuses, total, err := s.repo.SearchActiveSLAStatuses(ctx, req, source)
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

// slaRecalculationEntityID is the fixed Event Hub partition key
// TypeSLARecalculationRequested publishes under -- there is no single
// case/incident this request is "about", unlike every other event this
// service publishes.
const slaRecalculationEntityID = "sla-engine"

// RequestRecalculation implements SLAStatusService.
func (s *slaStatusService) RequestRecalculation(ctx context.Context) error {
	if err := s.requireAdmin(ctx); err != nil {
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
func (s *slaStatusService) GetClockState(ctx context.Context, workItemID, target, sourceFilter string) (domain.SLAClockState, error) {
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
	source, ok := slaStatusSourceFilters[strings.ToLower(sourceFilter)]
	if !ok {
		return domain.SLAClockState{}, &apierror.ValidationError{Msg: "source must be one of: csm, servicenow"}
	}
	return s.repo.GetClockState(ctx, workItemID, targetEnum, source)
}
