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

import type { BackendApi, BackendApiPostOptions } from "@api/backend/client";

/**
 * Whether the entity service behind this backend still accepts `countOnly`.
 * It rejects request fields it does not declare (a 400), so a portal that is
 * deployed before the entity service it talks to must not strand every count
 * or pie/bar dashboard widget. Cleared the first time such a rejection is
 * seen, then the flag is simply not sent for the rest of the session --
 * mirrors `postSkippingTotal.ts`'s own `skipTotalAccepted` latch.
 */
let countOnlyAccepted = true;

/** A `BackendApiError` carries the HTTP status; matched by shape rather than
 * `instanceof` so this module needs nothing from the client at runtime. */
function isBadRequest(err: unknown): boolean {
  return err instanceof Error && (err as { status?: unknown }).status === 400;
}

/** For tests: forget what an earlier request learned about the entity service. */
export function resetCountOnlySupport(): void {
  countOnlyAccepted = true;
}

/**
 * `api.post` for a search whose caller only ever reads `total` off the
 * response (a count tile, one pie/bar slice): asks the server to skip the
 * page query entirely (`countOnly: true`, the mirror image of
 * `postSkippingTotal`'s `skipTotal`), and falls back to the plain search if
 * the entity service predates the field. The fallback only happens on a 400,
 * and only counts as "unsupported" if the plain retry succeeds, so a request
 * that is simply invalid is reported as the error it is and does not switch
 * the flag off.
 */
export async function postCountingOnly<
  TBody extends { countOnly?: boolean },
  TResponse,
>(api: BackendApi, path: string, body: TBody, options?: BackendApiPostOptions): Promise<TResponse> {
  const { countOnly: _ignored, ...plain } = body;
  void _ignored;
  if (!countOnlyAccepted) {
    return api.post<TBody, TResponse>(path, plain as TBody, options);
  }
  try {
    return await api.post<TBody, TResponse>(path, { ...plain, countOnly: true } as TBody, options);
  } catch (err) {
    if (!isBadRequest(err)) throw err;
    const response = await api.post<TBody, TResponse>(path, plain as TBody, options);
    countOnlyAccepted = false;
    return response;
  }
}
