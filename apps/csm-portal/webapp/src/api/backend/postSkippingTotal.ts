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

import type { BackendApi } from "@api/backend/client";

/**
 * Whether the entity service behind this backend still accepts `skipTotal`.
 * It rejects request fields it does not declare (a 400), so a portal that is
 * deployed before the entity service it talks to must not strand every
 * palette search. Cleared the first time such a rejection is seen, then the
 * flag is simply not sent for the rest of the session.
 */
let skipTotalAccepted = true;

/** A `BackendApiError` carries the HTTP status; matched by shape rather than
 * `instanceof` so this module needs nothing from the client at runtime. */
function isBadRequest(err: unknown): boolean {
  return err instanceof Error && (err as { status?: unknown }).status === 400;
}

/** For tests: forget what an earlier request learned about the entity service. */
export function resetSkipTotalSupport(): void {
  skipTotalAccepted = true;
}

/**
 * `api.post` for a search whose caller never shows a total (the quick-nav
 * palette lists a handful of hits): asks the server to skip counting every
 * match (`skipTotal: true`, which also frees the second pool connection the
 * count would hold), and falls back to the plain search if the entity service
 * predates the field. The fallback only happens on a 400, and only counts as
 * "unsupported" if the plain retry succeeds, so a request that is simply
 * invalid is reported as the error it is and does not switch the flag off.
 */
export async function postSkippingTotal<
  TBody extends { skipTotal?: boolean },
  TResponse,
>(api: BackendApi, path: string, body: TBody): Promise<TResponse> {
  const { skipTotal: _ignored, ...plain } = body;
  void _ignored;
  if (!skipTotalAccepted) {
    return api.post<TBody, TResponse>(path, plain as TBody);
  }
  try {
    return await api.post<TBody, TResponse>(path, { ...plain, skipTotal: true } as TBody);
  } catch (err) {
    if (!isBadRequest(err)) throw err;
    const response = await api.post<TBody, TResponse>(path, plain as TBody);
    skipTotalAccepted = false;
    return response;
  }
}
