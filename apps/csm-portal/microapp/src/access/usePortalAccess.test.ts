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

import { AxiosError, AxiosHeaders } from "axios";
import { beforeEach, describe, expect, it } from "vitest";
import {
  denialStillStands,
  portalRolesFromQuery,
  resetRememberedPortalDenial,
  rolesForPortalAccess,
} from "./usePortalAccess";

function axiosError(status: number): AxiosError {
  return new AxiosError("failed", "ERR", undefined, undefined, {
    status,
    statusText: "",
    data: {},
    headers: {},
    config: { headers: new AxiosHeaders() },
  });
}

describe("rolesForPortalAccess", () => {
  const profile = { roles: ["admin"] };

  beforeEach(() => {
    resetRememberedPortalDenial();
  });

  it("uses the roles from a successful profile", () => {
    expect(rolesForPortalAccess({ isSuccess: true, isError: false, data: profile, error: null })).toEqual(["admin"]);
  });

  it("drops a cached admin profile when the refetch is a 403", () => {
    expect(
      rolesForPortalAccess({ isSuccess: false, isError: true, data: profile, error: axiosError(403) }),
    ).toBeUndefined();
  });

  it("keeps the cached profile when the refetch is a 503", () => {
    expect(rolesForPortalAccess({ isSuccess: false, isError: true, data: profile, error: axiosError(503) })).toEqual([
      "admin",
    ]);
  });

  it("grants nothing when the first request fails and there is no profile", () => {
    expect(
      rolesForPortalAccess({ isSuccess: false, isError: true, data: undefined, error: axiosError(503) }),
    ).toBeUndefined();
  });

  it("stays closed when a 503 follows a 401, until a profile succeeds", () => {
    const denied = { isSuccess: false, isError: true, data: profile, error: axiosError(401) };
    const later = { isSuccess: false, isError: true, data: profile, error: axiosError(503) };
    const restored = { isSuccess: true, isError: false, data: profile, error: null };

    expect(denialStillStands(false, denied)).toBe(true);
    expect(portalRolesFromQuery(denied)).toBeUndefined();
    expect(denialStillStands(true, later)).toBe(true);
    expect(portalRolesFromQuery(later)).toBeUndefined();
    expect(denialStillStands(true, restored)).toBe(false);
    expect(portalRolesFromQuery(restored)).toEqual(["admin"]);
  });

  it("keeps a cached profile when a 503 never followed a refusal", () => {
    const later = { isSuccess: false, isError: true, data: profile, error: axiosError(503) };
    expect(denialStillStands(false, later)).toBe(false);
    expect(portalRolesFromQuery(later)).toEqual(["admin"]);
  });
});
