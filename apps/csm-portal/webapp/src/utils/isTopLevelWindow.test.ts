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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  isTopLevelWindow,
  silentAuthFrameCanComplete,
} from "./isTopLevelWindow";

function framed(): void {
  vi.spyOn(window, "top", "get").mockReturnValue({} as Window);
}

function search(query: string): void {
  vi.spyOn(window, "location", "get").mockReturnValue({
    ...window.location,
    search: query,
  } as Location);
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("isTopLevelWindow", () => {
  it("is true for the real top-level page", () => {
    expect(isTopLevelWindow()).toBe(true);
  });

  it("is false inside the SDK's hidden auth frame", () => {
    framed();
    expect(isTopLevelWindow()).toBe(false);
  });
});

describe("silentAuthFrameCanComplete", () => {
  it("is true for a frame carrying an authorization code", () => {
    search("?code=abc123&state=instance_0_request_0");
    expect(silentAuthFrameCanComplete()).toBe(true);
  });

  it("is false for a frame carrying only an IdP error", () => {
    // What every browser blocking cross-site cookies returns for prompt=none.
    search("?error_description=Authentication+required&state=s&error=login_required");
    expect(silentAuthFrameCanComplete()).toBe(false);
  });

  it("still allows a frame that carries a code alongside an error", () => {
    search("?code=abc123&error=some_warning");
    expect(silentAuthFrameCanComplete()).toBe(true);
  });

  it("is true when there are no auth params at all", () => {
    search("");
    expect(silentAuthFrameCanComplete()).toBe(true);
  });
});
