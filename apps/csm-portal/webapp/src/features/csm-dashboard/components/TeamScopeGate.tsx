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

import { Alert, Skeleton } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import {
  resolveTeamScopeState,
  type TeamScopeInput,
} from "@features/csm-dashboard/utils/teamScopeState";

/**
 * Renders the dashboard's widgets only once the selected team's scope is
 * known; otherwise a skeleton (still loading) or an explicit message, never
 * org-wide numbers under a team's label. See `resolveTeamScopeState`.
 */
export default function TeamScopeGate({
  children,
  ...scope
}: TeamScopeInput & { children: ReactNode }): JSX.Element {
  const state = resolveTeamScopeState(scope);
  if (state === "ready") return <>{children}</>;
  if (state === "loading") return <Skeleton variant="rounded" height={200} />;
  return (
    <Alert severity={state === "error" ? "error" : "info"}>
      {state === "error"
        ? "Could not load the team list, so this team's scope is unknown. Reload to try again."
        : "No team scope: this team has no group configured for this dashboard, so team figures can't be shown. Pick another team, or choose All ABTs."}
    </Alert>
  );
}
