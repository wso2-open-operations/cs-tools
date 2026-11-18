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

/**
 * WSO2 staff: the "internal" persona. Mirrors the webapp's useCustomerPermissions `isInternalUser`
 * (apps/customer-portal/webapp/src/hooks/useCustomerPermissions.ts), including the role names
 * ServiceNow sends for the same persona. False until the roles have loaded, so anything meant for
 * staff stays hidden meanwhile.
 */
export function isInternalUserRoles(roles: readonly string[] | undefined): boolean {
  return (roles ?? []).some((role) => ["internal", "snc_internal", "agent", "wso2_agent"].includes(role.trim()));
}

/**
 * The case menu entry that sends a case back to WSO2 ("Waiting On WSO2"), if the user gets one.
 * On a Solution Proposed case it is "Reject Solution" for everyone. On an Awaiting Info case it is
 * "Wait on WSO2", and only staff get it: handing a case back to WSO2 is a staff action, so
 * customers there are left with closing the case alone. Mirrors the webapp's getAvailableCaseActions.
 */
export function getSendBackToWso2Action(
  isSolutionProposed: boolean,
  isAwaitingInfo: boolean,
  isInternalUser: boolean,
): { label: string; hidden: boolean } {
  if (isSolutionProposed) return { label: "Reject Solution", hidden: false };
  if (isAwaitingInfo) return { label: "Wait on WSO2", hidden: !isInternalUser };
  return { label: "Reject Solution", hidden: true };
}
