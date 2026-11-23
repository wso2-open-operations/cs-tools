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
  /**
   * True until the first profile result, including while that request is paused
   * offline. `isLoading` alone is false in that pause, because nothing is fetching.
   */
  isLoading: boolean;
  isError: boolean;
  /** A profile is already in hand, including one kept after a failed refetch. */
  hasProfile: boolean;
  error: Error | null;
  refetch: () => void;
}

export interface ProfileQuerySnapshot {
  isSuccess: boolean;
  isError: boolean;
  data: { roles?: string[] } | undefined;
  error: unknown;
}

/**
 * Whether a 401/403 is still in force. A later 5xx replaces the query error
 * but must not reopen a session the portal already refused. Only a successful
 * profile clears it. A 5xx that never followed a refusal leaves the flag as
 * it was, so a profile that was allowed stays allowed.
 */
export function denialStillStands(remembered: boolean, query: ProfileQuerySnapshot): boolean {
  if (query.isSuccess) return false;
  if (query.isError && isNotAuthorizedError(query.error)) return true;
  return remembered;
}

/**
 * Roles the gate may trust. A successful profile, or a failed refetch that is
 * not a 401/403 and still has the last profile, contribute their roles. A
 * 401/403 drops them even when the cache still holds an admin profile, and
 * `deniedUntilSuccess` keeps them dropped when a later failure is not itself
 * a 401/403.
 */
export function rolesForPortalAccess(query: ProfileQuerySnapshot, deniedUntilSuccess = false): string[] | undefined {
  if (deniedUntilSuccess || (query.isError && isNotAuthorizedError(query.error))) {
    return undefined;
  }
  if (query.isSuccess || (query.isError && query.data !== undefined)) {
    return query.data?.roles;
  }
  return undefined;
}

/**
 * The query cache keeps the last profile across errors, and it outlives a
 * remount of this hook. The refusal has to outlive that remount too, or a
 * 503 after a 403 would hand the cached admin roles back.
 */
let deniedUntilSuccess = false;

/** Isolates the remembered refusal between tests. */
export function resetRememberedPortalDenial(): void {
  deniedUntilSuccess = false;
}

/** Roles for this profile snapshot, remembering a refusal until a success. */
export function portalRolesFromQuery(query: ProfileQuerySnapshot): string[] | undefined {
  deniedUntilSuccess = denialStillStands(deniedUntilSuccess, query);
  return rolesForPortalAccess(query, deniedUntilSuccess);
}

/**
 * The signed-in user's portal access, from `GET /users/me` roles. It reads the same query the
 * profile and time-card pages use, so the profile is fetched once, not once per use. A user whose
 * roles have not loaded, or whose profile comes back 401/403, is granted nothing. A later
 * retryable failure keeps the last profile, so a page already open is not treated as a role change,
 * unless that profile was already refused: a 401/403 stays closed until a profile succeeds.
 * See {@link getPortalAccess}.
 */
export function usePortalAccess(): PortalAccessState {
  const query = useQuery(users.me());
  return {
    access: getPortalAccess(portalRolesFromQuery(query)),
    isLoading: query.isPending,
    isError: query.isError,
    hasProfile: query.data !== undefined,
    error: query.error ?? null,
    refetch: () => void query.refetch(),
  };
}
