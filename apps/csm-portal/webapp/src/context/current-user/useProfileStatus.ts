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

import { useCurrentUser } from "@context/current-user/CurrentUserContext";

/**
 * True while the signed-in user's profile (`GET /users/me`) is still loading
 * or failed to load, i.e. the role-derived `can*` flags are "not known yet"
 * rather than genuinely false. Route guards must not redirect on those flags
 * in this state: a transient profile failure would otherwise bounce a deep
 * link to a fallback page. The backend remains the authority on each request.
 *
 * Reports `false` where there is no `CurrentUserProvider` (the pre-sign-in
 * shell), matching `usePortalAccess`.
 */
export function useProfileUnknown(): boolean {
  try {
    // useCurrentUser always runs its `useContext` before it can throw, so the
    // hook order is identical on every render; the lint rule can't see that.
    // eslint-disable-next-line react-hooks/rules-of-hooks
    const { isLoading, isError } = useCurrentUser();
    return isLoading || isError;
  } catch {
    return false;
  }
}
