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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { qs, request } from "./client";

describe("qs", () => {
  it("emits an array value as one repeated key per element", () => {
    expect(qs({ status: ["WOC", "Pending Patch Queue"] })).toBe(
      "?status=WOC&status=Pending+Patch+Queue",
    );
  });

  it("drops an empty array entirely, without emitting the bare key", () => {
    expect(qs({ status: [], repo: "org/alpha" })).toBe("?repo=org%2Falpha");
  });

  it("mixes scalar and array params in one query string", () => {
    const s = qs({ q: "42", status: ["WOC", "Open"], limit: 20 });
    expect(s).toBe("?q=42&status=WOC&status=Open&limit=20");
  });

  it("drops undefined and empty-string entries, scalar or array element", () => {
    expect(qs({ repo: undefined, priority: ["", "High(P2)"] })).toBe("?priority=High%28P2%29");
  });
});

describe("request", () => {
  beforeEach(() => {
    // @ts-expect-error -- partial config is fine for these tests
    window.config = { GID_BACKEND_BASE_URL: "https://backend.example.test" };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("does not set Content-Type on a bodyless GET", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await request("/taxonomy");

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(new Headers(init.headers).has("Content-Type")).toBe(false);
  });

  it("sets Content-Type: application/json on a POST with a body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    // Body content is irrelevant here; this only checks the Content-Type header.
    await request("/sync/runs", { method: "POST", body: JSON.stringify({}) });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(new Headers(init.headers).get("Content-Type")).toBe("application/json");
  });
});
