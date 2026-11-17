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

interface Page {
  total: number;
  hasMore: boolean;
}

/**
 * Reads every page of a products API search (`POST .../search`, whose answer
 * holds the rows under `key`), `limit` rows at a time. The catalogue is a few
 * hundred rows, so this is one or two calls.
 */
export async function fetchAllPages<T>(
  api: BackendApi,
  path: string,
  body: Record<string, unknown>,
  key: string,
  limit: number,
): Promise<T[]> {
  const out: T[] = [];
  for (let offset = 0; ; offset += limit) {
    const page = await api.post<Record<string, unknown>, (Page & Record<string, unknown>) | null>(path, {
      ...body,
      pagination: { limit, offset },
    });
    const rows = (page?.[key] as T[] | undefined) ?? [];
    out.push(...rows);
    if (!page?.hasMore || rows.length === 0) return out;
  }
}
