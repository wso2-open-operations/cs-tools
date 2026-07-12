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

import { startTransition, useCallback } from "react";
import { useNavigate, type NavigateOptions, type To } from "react-router";

/**
 * Wraps useNavigate so every navigation is scheduled as a React transition.
 * This prevents the Suspense fallback from flashing during lazy-route navigation
 * — React keeps the current page visible until the new chunk is ready.
 * The returned function is stable (memoized via useCallback) so it is safe
 * to use as a useCallback/useEffect dependency.
 */
export function useNavTransition() {
  const navigate = useNavigate();

  return useCallback(
    (to: To | number, options?: NavigateOptions): void => {
      startTransition(() => {
        if (typeof to === "number") {
          navigate(to);
        } else {
          navigate(to, options);
        }
      });
    },
    [navigate],
  );
}
