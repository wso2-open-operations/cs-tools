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

async function loadFlag(dev: boolean, configFlag: boolean | undefined): Promise<boolean> {
  vi.resetModules();
  vi.stubEnv("DEV", dev);
  (window as unknown as { config?: Record<string, unknown> }).config = {
    CSM_PORTAL_DEV_BYPASS_ACCESS_CHECK: configFlag,
  };
  const mod = await import("./devFlags");
  return mod.devBypassAccessCheck;
}

describe("devBypassAccessCheck", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    delete (window as unknown as { config?: unknown }).config;
  });

  it("is on only in a dev build with the config flag set", async () => {
    expect(await loadFlag(true, true)).toBe(true);
  });

  it("ignores the config flag in a production build", async () => {
    expect(await loadFlag(false, true)).toBe(false);
  });

  it("is off in a dev build when the config flag is not set", async () => {
    expect(await loadFlag(true, undefined)).toBe(false);
  });
});
