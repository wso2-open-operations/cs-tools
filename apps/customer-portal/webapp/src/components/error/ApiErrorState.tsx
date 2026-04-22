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
import Error400Page from "@components/error/Error400Page";
import Error401Page from "@components/error/Error401Page";
import Error403Page from "@components/error/Error403Page";
import Error404Page from "@components/error/Error404Page";
import Error500Page from "@components/error/Error500Page";
import {
  getApiErrorMessage,
  isBadRequestError,
  isForbiddenError,
  isNotFoundError,
  isUnauthorizedError,
} from "@utils/ApiError";

export interface ApiErrorStateProps {
  error?: unknown;
  fallbackMessage?: string;
  containerSx?: SxProps<Theme>;
}

/**
 * Renders a consistent API error state for support pages/panels.
 *
 * @param {ApiErrorStateProps} props - Error payload and optional fallback copy.
 * @returns {JSX.Element} Error400/401/403/404/500 UI based on {@link ApiError} status.
 */
export default function ApiErrorState({
  error,
  fallbackMessage = "Something went wrong while loading data.",
  containerSx,
}: ApiErrorStateProps): JSX.Element {
  const baseContainerSx: SxProps<Theme> = {
    py: 4,
    alignItems: "center",
    justifyContent: "center",
  };
  const mergedContainerSx = (
    containerSx ? [baseContainerSx, containerSx] : baseContainerSx
  ) as SxProps<Theme>;

  if (isUnauthorizedError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error401Page message={getApiErrorMessage(error)} />
      </Stack>
    );
  }

  if (isForbiddenError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error403Page message={getApiErrorMessage(error)} />
      </Stack>
    );
  }

  if (isNotFoundError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error404Page message={getApiErrorMessage(error) ?? fallbackMessage} />
      </Stack>
    );
  }

  if (isBadRequestError(error)) {
    return (
      <Stack spacing={2} sx={mergedContainerSx}>
        <Error400Page message={getApiErrorMessage(error) ?? fallbackMessage} />
      </Stack>
    );
  }

  return (
    <Stack spacing={2} sx={mergedContainerSx}>
      <Error500Page
        message={getApiErrorMessage(error) ?? fallbackMessage}
      />
    </Stack>
  );
}

