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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  type JSX,
  type ReactNode,
} from "react";
import {
  useGetUsersMe,
  type UsersMeResponse,
} from "@features/settings/api/useGetUsersMe";
import { setUserPreferredTimeZone } from "@utils/dateTime";

interface CurrentUserContextType {
  /** The signed-in user's platform profile (`GET /users/me`), once loaded. */
  user: UsersMeResponse | undefined;
  /** True while the initial `/users/me` fetch is in flight. */
  isLoading: boolean;
  /** True if the `/users/me` fetch failed. */
  isError: boolean;
}

const CurrentUserContext = createContext<CurrentUserContextType | undefined>(
  undefined,
);

interface CurrentUserProviderProps {
  children: ReactNode;
}

/**
 * Fetches the signed-in user's platform profile (`GET /users/me`) **once** and
 * makes it available app-wide via {@link useCurrentUser}. Mounted inside the
 * auth boundary (AuthGuard), so the call fires a single time per session after
 * sign-in — the IdP-issued claims (email, name) come from the ID token via
 * `useIdTokenClaims`; this is the platform-owned counterpart (UUID, phone, …).
 *
 * Components must read the current user through `useCurrentUser()` rather than
 * calling `useGetUsersMe()` ad hoc, so `/users/me` is not re-issued per feature.
 */
export function CurrentUserProvider({
  children,
}: CurrentUserProviderProps): JSX.Element {
  const { data, isLoading, isError } = useGetUsersMe();

  // Seed the app-wide preferred-timezone store (used for view-only date
  // formatting) from the profile, so dates render in the user's own zone.
  useEffect(() => {
    setUserPreferredTimeZone(data?.timeZone);
  }, [data?.timeZone]);

  const value = useMemo<CurrentUserContextType>(
    () => ({ user: data, isLoading, isError }),
    [data, isLoading, isError],
  );

  return (
    <CurrentUserContext.Provider value={value}>
      {children}
    </CurrentUserContext.Provider>
  );
}

/**
 * Access the signed-in user's platform profile. Must be used within a
 * {@link CurrentUserProvider} (mounted by AuthGuard).
 */
export function useCurrentUser(): CurrentUserContextType {
  const ctx = useContext(CurrentUserContext);
  if (ctx === undefined) {
    throw new Error("useCurrentUser must be used within a CurrentUserProvider");
  }
  return ctx;
}
