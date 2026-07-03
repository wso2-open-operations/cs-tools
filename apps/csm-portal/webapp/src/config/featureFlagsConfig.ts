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
 * Runtime feature flags read once from `window.config`. Follows the same
 * module-scope accessor pattern as the other `config/*` modules: read
 * `window.config?.KEY`, apply a safe default, export a typed constant.
 */

/**
 * When true, every page/menu item flagged `wip` (see `csmNavItems.ts`) is shown
 * in the sidebar but disabled: greyed out, non-clickable, with a
 * "work in progress" tooltip. Its routes render the shared "coming soon" page
 * instead of the unfinished feature (see `App.tsx`'s `WipRouteGuard`). Lets a
 * deployment advertise upcoming sections without exposing unfinished pages.
 * Defaults to false (everything enabled) when the key is absent.
 */
export const DISABLE_WIP_FEATURES: boolean =
  window.config?.CSM_PORTAL_DISABLE_WIP_FEATURES ?? false;
