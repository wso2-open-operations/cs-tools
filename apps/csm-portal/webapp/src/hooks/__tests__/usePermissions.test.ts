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

import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { usePermissions } from "@hooks/usePermissions";
import { useCurrentUserOptional } from "@context/current-user/CurrentUserContext";

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: vi.fn(),
  useCurrentUserOptional: vi.fn(),
}));

describe("usePermissions", () => {
  it("defaults to false and empty roles when rendered outside provider (context undefined)", () => {
    vi.mocked(useCurrentUserOptional).mockReturnValue(undefined);

    const { result } = renderHook(() => usePermissions());

    expect(result.current.roles).toEqual([]);
    expect(result.current.isAdmin).toBe(false);
    expect(result.current.isAgent).toBe(false);
    expect(result.current.isTimecardApprover).toBe(false);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.hasRole("admin")).toBe(false);
    expect(result.current.hasAnyRole("admin", "agent")).toBe(false);
  });

  it("defaults to false and empty roles when user is undefined or loading", () => {
    vi.mocked(useCurrentUserOptional).mockReturnValue({
      user: undefined,
      isLoading: true,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => usePermissions());

    expect(result.current.roles).toEqual([]);
    expect(result.current.isAdmin).toBe(false);
    expect(result.current.isAgent).toBe(false);
    expect(result.current.isTimecardApprover).toBe(false);
    expect(result.current.isLoading).toBe(true);
    expect(result.current.hasRole("admin")).toBe(false);
    expect(result.current.hasAnyRole("admin", "agent")).toBe(false);
  });

  it("evaluates admin role correctly (case-insensitive)", () => {
    vi.mocked(useCurrentUserOptional).mockReturnValue({
      user: {
        id: "u-1",
        email: "admin@wso2.com",
        roles: ["ADMIN"],
      },
      isLoading: false,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => usePermissions());

    expect(result.current.roles).toEqual(["admin"]);
    expect(result.current.isAdmin).toBe(true);
    expect(result.current.isAgent).toBe(false);
    expect(result.current.isTimecardApprover).toBe(false);
    expect(result.current.hasRole("admin")).toBe(true);
    expect(result.current.hasRole("ADMIN")).toBe(true);
    expect(result.current.hasAnyRole("agent", "admin")).toBe(true);
  });

  it("evaluates agent and timecard_approver roles correctly", () => {
    vi.mocked(useCurrentUserOptional).mockReturnValue({
      user: {
        id: "u-2",
        email: "agent@wso2.com",
        roles: ["agent", "timecard_approver"],
      },
      isLoading: false,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => usePermissions());

    expect(result.current.roles).toEqual(["agent", "timecard_approver"]);
    expect(result.current.isAdmin).toBe(false);
    expect(result.current.isAgent).toBe(true);
    expect(result.current.isTimecardApprover).toBe(true);
    expect(result.current.hasRole("agent")).toBe(true);
    expect(result.current.hasRole("timecard_approver")).toBe(true);
    expect(result.current.hasRole("admin")).toBe(false);
  });

  it("handles empty or whitespace roles safely", () => {
    vi.mocked(useCurrentUserOptional).mockReturnValue({
      user: {
        id: "u-3",
        email: "user@wso2.com",
        roles: ["  ", " agent "],
      },
      isLoading: false,
      isError: false,
      error: null,
    });

    const { result } = renderHook(() => usePermissions());

    expect(result.current.roles).toEqual(["agent"]);
    expect(result.current.isAgent).toBe(true);
    expect(result.current.hasRole("")).toBe(false);
  });
});
