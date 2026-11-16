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

import { afterEach, describe, expect, it } from "vitest";
import { isCodeDashboardGenerationEnabled } from "./codeDashboardConfig";

describe("isCodeDashboardGenerationEnabled", () => {
  const original = window.config;
  afterEach(() => {
    window.config = original;
  });

  it("is off when the key is absent", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: undefined };
    expect(isCodeDashboardGenerationEnabled()).toBe(false);
  });

  it.each([false, "false", "", "1", "TRUE", "True", "yes", "on"])("is off for %j", (raw) => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: raw };
    expect(isCodeDashboardGenerationEnabled()).toBe(false);
  });

  it.each([true, "true", " true "])("is on for %j", (raw) => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: raw };
    expect(isCodeDashboardGenerationEnabled()).toBe(true);
  });
});
