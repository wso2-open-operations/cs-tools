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
import { useNavigate, useLocation, resolvePath } from "react-router";
import type { NavigateOptions } from "react-router";

/**
 * Returns a navigation function that behaves like react-router's `navigate` on a
 * normal click, but opens the target URL in a new browser tab when the user holds
 * Ctrl (Windows/Linux) or Cmd/Meta (macOS) at the time of the click.
 *
 * Because many card click handlers do not receive the original MouseEvent, this
 * hook tracks the modifier-key state via window-level keydown/keyup listeners
 * and exposes it through a ref that is checked at call time.
 */
export function useModifierAwareNavigate(): (
  path: string,
  options?: NavigateOptions,
) => void {
  const navigate = useNavigate();
  const location = useLocation();
  const modifierKeyRef = useRef(false);

  useEffect(() => {
    const onKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Control" || e.key === "Meta")
        modifierKeyRef.current = true;
    };
    const onKeyUp = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Control" || e.key === "Meta")
        modifierKeyRef.current = false;
    };
    const onBlur = () => {
      modifierKeyRef.current = false;
    };
    const onVisibilityChange = () => {
      if (document.hidden) modifierKeyRef.current = false;
    };
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("keyup", onKeyUp);
    window.addEventListener("blur", onBlur);
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("keyup", onKeyUp);
      window.removeEventListener("blur", onBlur);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, []);

  return useCallback(
    (path: string, options?: NavigateOptions) => {
      if (modifierKeyRef.current) {
        const resolved = resolvePath(path, location.pathname);
        const url =
          window.location.origin +
          resolved.pathname +
          (resolved.search ?? "") +
          (resolved.hash ?? "");
        window.open(url, "_blank", "noopener,noreferrer");
      } else {
        navigate(path, options);
      }
    },
    [navigate, location.pathname],
  );
}
