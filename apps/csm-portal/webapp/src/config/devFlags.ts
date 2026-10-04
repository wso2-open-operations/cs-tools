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

// TEMPORARY / LOCAL DEV ONLY flags — see the CSM_PORTAL_DEV_BYPASS_ACCESS_CHECK
// and CSM_PORTAL_DEV_VIEW_OVERRIDE declarations on Window.config in
// authConfig.ts for the full explanation of each. Deliberately its OWN
// module, separate from authConfig.ts: that file's `authConfig` export calls
// getAuthConfig(), which throws synchronously (at module-evaluation time,
// for EVERY importer) when window.config lacks the real OIDC fields — true
// in every unit test, which stub only the handful of config keys each test
// actually needs. These two flags must stay importable from a plain
// component/hook test with no OIDC config stubbed at all, so they read
// window.config directly here rather than living behind that throw.
//
// The access-check bypass additionally requires a dev build
// (`import.meta.env.DEV`): a production bundle ignores the runtime flag, so a
// config edit alone can never switch the role checks off.
export const devBypassAccessCheck: boolean =
  import.meta.env.DEV && window.config?.CSM_PORTAL_DEV_BYPASS_ACCESS_CHECK === true;

export const devViewOverride: "cs-abt" | "sales-sa" | undefined =
  window.config?.CSM_PORTAL_DEV_VIEW_OVERRIDE;
