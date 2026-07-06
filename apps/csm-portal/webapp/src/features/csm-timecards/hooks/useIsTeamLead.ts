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

import { useTimecardRole } from "@features/csm-timecards/hooks/useTimecardRole";

/**
 * True when the signed-in user may approve time cards. Thin alias over
 * {@link useTimecardRole}'s `isApprover` (admins included), kept for the case
 * "Time tracking" tab. Prefer `useTimecardRole` for new code.
 */
export function useIsTeamLead(): boolean {
  return useTimecardRole().isApprover;
}
