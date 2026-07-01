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

import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { resolveUserInfo } from "@utils/userClaims";
import {
  teamLeadGroup,
  timecardAdminGroup,
} from "@features/csm-timecards/constants/timeCardConstants";

export interface TimecardRole {
  /** May approve/reject/recall time cards (approver group, or admin). */
  isApprover: boolean;
  /** Time-card admin: edit any user's editable cards, approve by exception. */
  isAdmin: boolean;
}

/**
 * The signed-in user's time-card role, from the OIDC `groups` claim. Admins are
 * a superset of approvers. Client-side affordance only — the backend must
 * enforce the same gates (Phase 2).
 */
export function useTimecardRole(): TimecardRole {
  const { groups } = resolveUserInfo(useIdTokenClaims());
  const lower = groups.map((g) => g.toLowerCase());
  const isAdmin = lower.includes(timecardAdminGroup().toLowerCase());
  const isApprover = isAdmin || lower.includes(teamLeadGroup().toLowerCase());
  return { isApprover, isAdmin };
}
