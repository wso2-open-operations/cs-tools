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

import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  SIGNING_OUT_EVENT,
  clearAppOwnedStorage,
  registerSignOutCleanup,
} from "./sessionCleanup";

describe("sessionCleanup", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  afterEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("removes csm*, spl.* and dashboard_* keys from both storages", () => {
    localStorage.setItem("csm.recentViews.v1.user-1", "[]");
    localStorage.setItem("csm:user-1:cases:columns", "{}");
    localStorage.setItem("csm-portal:time-cards:recent-approvers:v1:e1", "[]");
    localStorage.setItem("spl.stateValues", "[]");
    sessionStorage.setItem("csm.caseTabs.v1", "[]");
    sessionStorage.setItem("csm.createChangeRequest.draft.x", "{}");
    sessionStorage.setItem("dashboard_filters", "{}");
    clearAppOwnedStorage();
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("keeps the theme preference and keys it does not own", () => {
    localStorage.setItem("csm.theme", "dark");
    localStorage.setItem("unrelated", "1");
    sessionStorage.setItem("post_login_redirect", "/cases");
    clearAppOwnedStorage();
    expect(localStorage.getItem("csm.theme")).toBe("dark");
    expect(localStorage.getItem("unrelated")).toBe("1");
    expect(sessionStorage.getItem("post_login_redirect")).toBe("/cases");
  });

  it("clears storage and the query cache when the signing-out event fires", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(["users-me"], { id: "u1" });
    localStorage.setItem("csm.recentViews.v1.user-1", "[]");
    const unregister = registerSignOutCleanup(queryClient);

    window.dispatchEvent(new CustomEvent(SIGNING_OUT_EVENT));
    expect(queryClient.getQueryData(["users-me"])).toBeUndefined();
    expect(localStorage.getItem("csm.recentViews.v1.user-1")).toBeNull();

    unregister();
    localStorage.setItem("csm.recentViews.v1.user-1", "[]");
    window.dispatchEvent(new CustomEvent(SIGNING_OUT_EVENT));
    expect(localStorage.getItem("csm.recentViews.v1.user-1")).toBe("[]");
  });
});
