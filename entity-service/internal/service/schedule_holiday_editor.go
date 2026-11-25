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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
)

// errNotHolidayEditor refuses a public holiday change from anyone but a portal
// admin or a rota admin.
var errNotHolidayEditor = &apierror.ForbiddenError{Msg: "only a portal admin or a rota admin can change the public holidays"}

// requireHolidayEditor admits a portal admin (an admin role on the token or
// the database admin role) or a rota admin of any family. A public holiday is
// per country and shared by every family, so being a lead or a Rota Manager
// of one team is not enough.
func (s *scheduleService) requireHolidayEditor(ctx context.Context) error {
	if err := s.requireInternalCaller(ctx); err != nil {
		return err
	}
	if auth.IdentityFromContext(ctx).UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "changing the public holidays needs a user token, not a service credential"}
	}
	admin, err := s.isPortalAdmin(ctx)
	if err != nil {
		return err
	}
	if admin {
		return nil
	}
	families, err := s.rotaAdminFamilies(ctx)
	if err != nil {
		return err
	}
	if len(families) == 0 {
		return errNotHolidayEditor
	}
	return nil
}

// HolidayEditorCheck is the schedule service's check for adding and removing
// public holidays, for PublicHolidayService (WithHolidayEditorCheck); nil when
// svc is not the schedule service.
func HolidayEditorCheck(svc ScheduleService) HolidayEditorAuthorizer {
	s, ok := svc.(*scheduleService)
	if !ok {
		return nil
	}
	return s.requireHolidayEditor
}
