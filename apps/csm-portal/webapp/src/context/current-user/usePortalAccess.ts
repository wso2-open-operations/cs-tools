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
import { useMemo } from "react";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import {
  getPortalAccess,
  type PortalAccess,
} from "@context/current-user/portalAccess";

/**
 * The signed-in user's portal access, derived from `GET /users/me` roles. See
 * {@link getPortalAccess}.
 *
 * Safe to call from chrome that also renders in the pre-sign-in shell (the
 * sidebar, quick-nav), where there is no `CurrentUserProvider` yet: there it
 * reports no access rather than throwing, and picks the real roles up once the
 * provider mounts.
 */
export function usePortalAccess(): PortalAccess {
  let roles: string[] | undefined;
  try {
    // useCurrentUser always runs its `useContext` before it can throw, so the
    // hook order is identical on every render; the lint rule can't see that.
    // eslint-disable-next-line react-hooks/rules-of-hooks
    roles = useCurrentUser().user?.roles;
  } catch {
    roles = undefined;
  }
  return useMemo(() => getPortalAccess(roles), [roles]);
}
