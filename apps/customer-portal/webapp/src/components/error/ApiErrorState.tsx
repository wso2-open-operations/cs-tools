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

import { Stack, type SxProps, type Theme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ErrorRecoveryAction from "@components/error/ErrorRecoveryAction";
import Error400Page from "@components/error/Error400Page";
import Error401Page from "@components/error/Error401Page";
import Error403Page from "@components/error/Error403Page";
import Error404Page from "@components/error/Error404Page";
import Error500Page from "@components/error/Error500Page";
import {
  isBadRequestError,
  isForbiddenError,
  isNotFoundError,
  isUnauthorizedError,
} from "@utils/ApiError";

export interface ApiErrorStateProps {
  error?: unknown;
  fallbackMessage?: string;
  containerSx?: SxProps<Theme>;
  /** Adds a button off the page for 403/404 (see {@link ErrorRecoveryAction}).
   *  Opt in for full-page errors only; leave off for errors embedded in a
   *  panel or tab, where the surrounding page already offers navigation. */
  showRecoveryAction?: boolean;
  /** Caller-chosen copy for 403 and 404, which otherwise show the page's own
   *  default (403) or `fallbackMessage` (404). */
  accessMessage?: string;
}

/**
 * Renders a consistent API error state for support pages/panels.
 *
 * Deliberately does not surface the backend's own error reason text (via
 * `getApiErrorMessage`) — each Error400/401/403/404/500Page shows its own
 * generic, user-friendly copy instead. `fallbackMessage` is the one exception:
 * it's caller-chosen UI copy for this specific loading context (e.g. "Could
 * not load cases."), not backend-returned detail, so it's still passed
 * through for 404/400. `error` is still forwarded to every page so its
 * "Tracking ID" copy affordance can render when the error carries a
 * correlation ID.
 *
 * @param {ApiErrorStateProps} props - Error payload, optional fallback copy, and recovery opt-in.
 * @returns {JSX.Element} Error400/401/403/404/500 UI based on {@link ApiError} status.
 */
export default function ApiErrorState({
  error,
  fallbackMessage = "Something went wrong while loading data.",
  containerSx,
  showRecoveryAction = false,
  accessMessage,
}: ApiErrorStateProps): JSX.Element {
  const baseContainerSx: SxProps<Theme> = {
    py: 4,
    alignItems: "center",
    justifyContent: "center",
  };
  const mergedContainerSx = (
    containerSx ? [baseContainerSx, containerSx] : baseContainerSx
  ) as SxProps<Theme>;

  const recoveryAction = showRecoveryAction ? <ErrorRecoveryAction /> : undefined;

  if (isUnauthorizedError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error401Page error={error} />
      </Stack>
    );
  }

  if (isForbiddenError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error403Page
          message={accessMessage}
          error={error}
          actions={recoveryAction}
        />
      </Stack>
    );
  }

  if (isNotFoundError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error404Page
          message={accessMessage ?? fallbackMessage}
          error={error}
          actions={recoveryAction}
        />
      </Stack>
    );
  }

  if (isBadRequestError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error400Page message={fallbackMessage} error={error} />
      </Stack>
    );
  }

  return (
    <Stack spacing={2} sx={mergedContainerSx}>
      <Error500Page message={fallbackMessage} error={error} />
    </Stack>
  );
}

