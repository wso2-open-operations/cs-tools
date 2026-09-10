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

import { ROLE_ADMIN } from "@hooks/usePermissions";

/**
 * True when the given `GET /users/me` roles include the platform admin
 * role. Frontend-only gate for the dashboard builder (see
 * `csmNavItems.ts`'s own comment on `admin.dashboards`): unlike every other
 * `/admin` tab, this one has no privileged backend action to fall back on
 * for enforcement — everything the builder does is local to the browser
 * (`localStorage` only), so hiding it here IS the whole gate.
 */
export function hasDashboardBuilderAccess(roles: string[] | undefined): boolean {
  return (roles ?? []).some((r) => r.trim().toLowerCase() === ROLE_ADMIN);
}
