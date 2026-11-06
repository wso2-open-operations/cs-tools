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

import { useCallback } from "react";
import { useBackendApi } from "@api/backend/client";
import type { BeDashboard } from "@api/backend/types";

/** What one chat turn produces: the dashboard to preview and the reply to show. */
export interface GenerateDashboardResult {
  dashboard: BeDashboard;
  reply: string;
}

/** Body of POST /dashboards/generate (backend `generateDashboardRequest`). */
interface GenerateDashboardRequest {
  prompt: string;
  /** The dashboard being edited; left out for the first message. */
  current?: BeDashboard;
}

/**
 * Returns `generate(prompt, current)`, which turns one chat message (and the
 * dashboard being edited, if any) into a dashboard to preview, by calling
 * `POST /dashboards/generate`. The response is `{ dashboard, reply }`, the
 * dashboard in the same shape as `GET /dashboards/{id}`.
 *
 * The backend endpoint is still a stub that returns a sample dashboard; this
 * hook does not change when it starts generating for real. Errors (including
 * the stub's 503 when its sample is not loaded) are thrown, and the generate
 * page shows them as a chat reply.
 */
export function useGenerateDashboard(): (
  prompt: string,
  current: BeDashboard | null,
) => Promise<GenerateDashboardResult> {
  const api = useBackendApi();

  return useCallback(
    (prompt: string, current: BeDashboard | null): Promise<GenerateDashboardResult> =>
      api.post<GenerateDashboardRequest, GenerateDashboardResult>("/dashboards/generate", {
        prompt,
        ...(current ? { current } : {}),
      }),
    [api],
  );
}
