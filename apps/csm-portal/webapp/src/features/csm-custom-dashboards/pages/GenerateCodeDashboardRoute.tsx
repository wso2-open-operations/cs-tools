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

import { type JSX } from "react";
import { Navigate } from "react-router";
import { useCanGenerateAdvancedDashboards } from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";
import GenerateCodeDashboardPage from "@features/csm-custom-dashboards/pages/GenerateCodeDashboardPage";

/**
 * The `/dashboard/my/new-code` route: the advanced (code-based) generate page
 * when the user may use it (`canGenerateAdvancedDashboards`: the code
 * approach is on and the user is a CS engineer or an admin), else the 404 page.
 * The route itself stays registered either way: without it the path would
 * fall to `dashboard/my/:dashboardId` and My Dashboards would read
 * "new-code" as a dashboard id, instead of the page not existing.
 */
export default function GenerateCodeDashboardRoute(): JSX.Element {
  return useCanGenerateAdvancedDashboards() ? <GenerateCodeDashboardPage /> : <Navigate to="/404" replace />;
}
