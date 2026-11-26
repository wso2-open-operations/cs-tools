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

import type { ReactNode } from "react";
import { CircularProgress, Stack } from "@wso2/oxygen-ui";
import { errorStatus, isNotAuthorizedError, usePortalAccess } from "@src/access/usePortalAccess";
import { NoPortalAccess } from "@components/common/NoPortalAccess";
import { ErrorState } from "@components/support/ErrorState";
import { Logger } from "@utils/logger";

/**
 * Lets the app's pages render only once the signed-in user's roles are known and include at least
 * one portal role. Mirrors the webapp's AuthGuard: no portal role, or a 401/403 from
 * `GET /users/me`, shows the "no access" screen. The role check still applies when a later refetch
 * fails. Any other failure with no profile yet offers a retry. A retryable failure after an
 * allowed profile has loaded keeps the page mounted and offers a retry beside it.
 */
export function AccessGate({ children }: { children: ReactNode }) {
  const { access, isLoading, isError, error, hasProfile, refetch } = usePortalAccess();

  if (isLoading) {
    return (
      <Stack alignItems="center" justifyContent="center" py={8} role="status" aria-label="Loading">
        <CircularProgress size={28} />
      </Stack>
    );
  }
  if (isError && isNotAuthorizedError(error)) {
    Logger.warn("Portal access check was rejected", { status: errorStatus(error) });
    return <NoPortalAccess status={errorStatus(error)} />;
  }
  // The webapp checks the role whenever the user record is present, including
  // when a later refetch failed. The first check, with nothing loaded yet,
  // still waits on Retry rather than guessing a role.
  if (!access.hasAnyRole) {
    if (isError && !hasProfile) {
      return <ErrorState onRetry={refetch} />;
    }
    Logger.warn("Signed-in user holds no portal role");
    return <NoPortalAccess />;
  }
  // A refetch that fails after an allowed profile loaded must not unmount the
  // page: a New Case draft lives in that page. Retry stays available.
  return (
    <>
      {isError ? <ErrorState onRetry={refetch} /> : null}
      {children}
    </>
  );
}
