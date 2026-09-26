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
 * The project key an announcement's "Send test" dry run creates its one real
 * test case in — mirrors the ServiceNow flow's own hardcoded
 * `Project Key = DCPSUB` dry-run scoping (see the announcement-enhancement
 * brief's Section 2). Defaults to "DCPSUB" itself, since that's the same
 * project used for this in production; override via
 * CSM_PORTAL_ANNOUNCEMENT_TEST_PROJECT_KEY only if a given environment needs
 * a different one (e.g. a staging-only sandbox project).
 */
export const DRY_RUN_TEST_PROJECT_KEY =
  window.config?.CSM_PORTAL_ANNOUNCEMENT_TEST_PROJECT_KEY ?? "DCPSUB";
