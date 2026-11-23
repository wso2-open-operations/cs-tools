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

import { useQuery } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { users } from "@src/services/users";
import { ApiError } from "@utils/ApiError";
import { getPortalAccess, type PortalAccess } from "./portalAccess";

/** The HTTP status behind a failed request, whichever way the client surfaced it. */
export function errorStatus(error: unknown): number | undefined {
  if (error instanceof ApiError) return error.status;
  if (isAxiosError(error)) return error.response?.status;
  return undefined;
}

/** True for a 401 or 403, which means "not allowed here", unlike a 5xx or a network failure. */
export function isNotAuthorizedError(error: unknown): boolean {
  const status = errorStatus(error);
  return status === 401 || status === 403;
}

export interface PortalAccessState {
  /** What the signed-in user's roles allow. Nothing is allowed until the roles have loaded. */
  access: PortalAccess;
  /** True while the first `GET /users/me` is in flight. */
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  refetch: () => void;
}

/**
 * The signed-in user's portal access, from `GET /users/me` roles. It reads the same query the
 * profile and time-card pages use, so the profile is fetched once, not once per use. A user whose
 * roles have not loaded (or whose request failed) is granted nothing, so controls fail closed.
 * See {@link getPortalAccess}.
 */
export function usePortalAccess(): PortalAccessState {
  const { data, isLoading, isError, error, refetch } = useQuery(users.me());
  return {
    access: getPortalAccess(data?.roles),
    isLoading,
    isError,
    error: error ?? null,
    refetch: () => void refetch(),
  };
}
