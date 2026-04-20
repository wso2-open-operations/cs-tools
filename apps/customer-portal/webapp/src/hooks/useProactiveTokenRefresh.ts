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

import { useEffect, useRef } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLogger } from "@hooks/useLogger";
import { recoverViaSilentSignIn } from "@hooks/authRecovery";
import { HIDDEN_REFRESH_THRESHOLD_MS } from "@constants/authConstants";

export function useProactiveTokenRefresh(): void {
  const { isSignedIn, signInSilently } = useAsgardeo();
  const logger = useLogger();
  const hiddenAtRef = useRef<number | null>(null);

  useEffect(() => {
    if (!isSignedIn) {
      hiddenAtRef.current = null;
      return;
    }

    if (document.visibilityState === "hidden" && hiddenAtRef.current === null) {
      hiddenAtRef.current = Date.now();
    }

    const handleVisibilityChange = (): void => {
      if (document.visibilityState === "hidden") {
        hiddenAtRef.current = Date.now();
        return;
      }

      if (document.visibilityState !== "visible") {
        return;
      }

      const hiddenAt = hiddenAtRef.current;
      hiddenAtRef.current = null;
      if (hiddenAt === null) {
        return;
      }

      const hiddenDuration = Date.now() - hiddenAt;
      if (hiddenDuration < HIDDEN_REFRESH_THRESHOLD_MS) {
        return;
      }

      logger.info("[proactiveRefresh] tab-returned-after-idle", {
        hiddenDurationMs: hiddenDuration,
      });
      void recoverViaSilentSignIn(signInSilently, logger);
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [isSignedIn, signInSilently, logger]);
}
