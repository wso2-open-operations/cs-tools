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

import { useCallback, useEffect, useRef } from "react";

/**
 * One idempotency key per (dialog opening, request payload). `getKey(payload)`
 * returns the same key while the dialog stays open and the payload's JSON is
 * identical to the last submitted one (a double click, or a retry after an
 * error). A different payload gets a new key, because the server remembers a
 * key even for a refused request and answers 409 `idempotency_key_reused` for
 * a different body under it. Only the LAST submitted payload is remembered:
 * editing back to an earlier payload mints another new key (never reuses the
 * older one). Closing the dialog forgets everything, so each opening starts
 * with a new key.
 */
export function useOpenIdempotencyKey(open: boolean): (payload: unknown) => string {
  const lastRef = useRef<{ json: string; key: string } | null>(null);

  useEffect(() => {
    if (!open) lastRef.current = null;
  }, [open]);

  return useCallback((payload: unknown): string => {
    const json = JSON.stringify(payload);
    if (lastRef.current?.json !== json) {
      lastRef.current = { json, key: crypto.randomUUID() };
    }
    return lastRef.current.key;
  }, []);
}
