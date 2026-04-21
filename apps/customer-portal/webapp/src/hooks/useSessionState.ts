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

import { type Dispatch, type SetStateAction, useState, useEffect } from "react";

/**
 * Drop-in replacement for `useState` that persists the value to `sessionStorage`.
 * On mount the stored value is restored so filters survive navigation back from detail pages.
 *
 * @param key - Unique storage key (include projectId to avoid cross-project leakage).
 * @param defaultValue - Initial value when nothing is stored yet.
 * @param validate - Optional type guard; if provided, stored values that fail the
 *   check are discarded and `defaultValue` is used instead (protects against stale
 *   or migrated enum values in storage).
 */
export function useSessionState<T>(
  key: string,
  defaultValue: T,
  validate?: (value: unknown) => value is T,
): [T, Dispatch<SetStateAction<T>>] {
  const [state, setState] = useState<T>(() => {
    try {
      const stored = sessionStorage.getItem(key);
      if (stored !== null) {
        const parsed: unknown = JSON.parse(stored);
        if (!validate || validate(parsed)) {
          return parsed as T;
        }
      }
    } catch {
      // ignore parse / storage errors
    }
    return defaultValue;
  });

  useEffect(() => {
    try {
      sessionStorage.setItem(key, JSON.stringify(state));
    } catch {
      // ignore quota / private-browsing errors
    }
  }, [key, state]);

  return [state, setState];
}
