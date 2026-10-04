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

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useProfileUnknown } from "./useProfileStatus";

const state: { isLoading: boolean; isError: boolean; throws: boolean } = {
  isLoading: false,
  isError: false,
  throws: false,
};

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => {
    if (state.throws) throw new Error("no provider");
    return { isLoading: state.isLoading, isError: state.isError };
  },
}));

describe("useProfileUnknown", () => {
  beforeEach(() => {
    state.isLoading = false;
    state.isError = false;
    state.throws = false;
  });

  it("is true while the profile is loading", () => {
    state.isLoading = true;
    expect(renderHook(() => useProfileUnknown()).result.current).toBe(true);
  });

  it("is true when the profile failed to load", () => {
    state.isError = true;
    expect(renderHook(() => useProfileUnknown()).result.current).toBe(true);
  });

  it("is false once the profile loaded", () => {
    expect(renderHook(() => useProfileUnknown()).result.current).toBe(false);
  });

  it("is false outside a CurrentUserProvider", () => {
    state.throws = true;
    expect(renderHook(() => useProfileUnknown()).result.current).toBe(false);
  });
});
