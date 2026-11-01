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

import { isClientError } from "@api/backend/client";

const MAX_RETRIES = 2;

/**
 * React Query retry policy: a 4xx will fail the same way again (a missing
 * article stays missing, a rejected request body stays rejected), so it is
 * never retried. Transient failures (5xx, network) are retried a couple of
 * times before the error state shows.
 */
export function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (isClientError(error)) return false;
  return failureCount < MAX_RETRIES;
}
