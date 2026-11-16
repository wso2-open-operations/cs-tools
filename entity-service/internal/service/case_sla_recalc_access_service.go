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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
)

// CaseSLARecalcService exposes SLAEngineService.RecalculateCasePauseTotals
// to an HTTP caller, admin-gated -- see NewCaseSLARecalcService's own doc
// comment for why this is a separate, narrow wrapper rather than adding an
// access check directly onto SLAEngineService itself.
type CaseSLARecalcService interface {
	RecalculateCasePauseTotals(ctx context.Context, caseID string) (CaseSLAPauseRecalcReport, error)
}

type caseSLARecalcService struct {
	engine                   SLAEngineService
	access                   AccessService
	csmPortalBackendClientID string
}

// NewCaseSLARecalcService wraps an existing SLAEngineService with the one
// admin-gate check this one action needs, rather than adding access/
// csmPortalBackendClientID fields to SLAEngineService itself -- that
// interface is otherwise purely internal (every other method is called
// from case/incident handling code, never directly from an HTTP request),
// and SLAEngineService's own constructor is already called from several
// existing test files with no access dependency at all; threading one
// through just for this one new, admin-only action would touch every one
// of those call sites for a concern specific to a single method. Same
// trusting-the-caller's-client-id pattern as
// slaStatusService/portalBannerService's own requireCSMPortalBackendCaller
// -- see that method's own doc comment for why this doesn't also
// independently re-check an AUTH_ADMIN_ROLES list: this backend's own
// PermAdmin gate (cmd/server/main.go) is the one real admin check; this
// just confirms the caller really is csm-portal-backend, not a second,
// separately-configured admin check that could silently disagree with the
// first.
func NewCaseSLARecalcService(engine SLAEngineService, access AccessService, csmPortalBackendClientID string) CaseSLARecalcService {
	return &caseSLARecalcService{engine: engine, access: access, csmPortalBackendClientID: csmPortalBackendClientID}
}

func (s *caseSLARecalcService) requireCSMPortalBackendCaller(ctx context.Context) error {
	if err := RequireInternalCaller(ctx, s.access, "the per-case SLA recalculate action is only available through the CSM portal"); err != nil {
		return err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "the per-case SLA recalculate action can only be used by a signed-in person"}
	}
	if s.csmPortalBackendClientID == "" || id.ClientID != s.csmPortalBackendClientID {
		return &apierror.ForbiddenError{Msg: "the per-case SLA recalculate action can only be used through the CSM Portal"}
	}
	return nil
}

// RecalculateCasePauseTotals implements CaseSLARecalcService.
func (s *caseSLARecalcService) RecalculateCasePauseTotals(ctx context.Context, caseID string) (CaseSLAPauseRecalcReport, error) {
	if err := s.requireCSMPortalBackendCaller(ctx); err != nil {
		return CaseSLAPauseRecalcReport{}, err
	}
	return s.engine.RecalculateCasePauseTotals(ctx, caseID)
}
