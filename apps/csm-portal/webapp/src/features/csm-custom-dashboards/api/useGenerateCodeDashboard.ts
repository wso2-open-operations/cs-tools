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

/** What one chat turn produces: the dashboard's source code, the reply to show and the name the model suggests. */
export interface GenerateCodeDashboardResult {
  source: string;
  reply: string;
  /** The name to start Save's dialog from (the dashboard's title); absent when the model gave none. */
  name?: string;
}

/** Body of POST /dashboards/generate-code (backend `generateCodeDashboardRequest`). */
interface GenerateCodeDashboardRequest {
  prompt: string;
  /** The source being edited; left out for the first message. */
  current?: string;
}

/**
 * The code approach (phase 2): returns `generate(prompt, current)`, which
 * turns one chat message (and the source being edited, if any) into dashboard
 * source code by calling `POST /dashboards/generate-code`. The code-based
 * counterpart of `useGenerateDashboard`, which it does not touch.
 *
 * Without the AI integration service the backend answers a fixed sample.
 * Errors are thrown (a `BackendApiError` for a non-2xx answer), and the page
 * shows them as a chat reply.
 */
export function useGenerateCodeDashboard(): (
  prompt: string,
  current: string | null,
) => Promise<GenerateCodeDashboardResult> {
  const api = useBackendApi();

  return useCallback(
    (prompt: string, current: string | null): Promise<GenerateCodeDashboardResult> =>
      api.post<GenerateCodeDashboardRequest, GenerateCodeDashboardResult>("/dashboards/generate-code", {
        prompt,
        ...(current ? { current } : {}),
      }),
    [api],
  );
}
