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

/** The route that opens a dashboard's link (App.tsx), without the link id. */
export const DASHBOARD_LINK_PATH = "/dashboard/shared/link";

/** The full link the owner copies: this portal's origin + `/dashboard/shared/link/<linkId>`. */
export function dashboardLinkUrl(linkId: string): string {
  return `${window.location.origin}${DASHBOARD_LINK_PATH}/${encodeURIComponent(linkId)}`;
}
