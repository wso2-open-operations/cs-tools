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

import { describe, expect, it } from "vitest";
import { UsageTimeRange } from "@features/project-details/types/usage";
import {
  buildUsageInnerTabs,
  getActiveUsageDeploymentId,
  getUsagePresetShortLabel,
} from "@features/usage-metrics/utils/usageMetricsTab";

describe("usageMetricsTab", () => {
  it("builds overview plus deployment tabs and resolves active deployment id", () => {
    const tabs = buildUsageInnerTabs([{ id: "dep-1", name: "Production" }]);
    const id = getActiveUsageDeploymentId("um-dep-dep-1");

    expect(tabs).toHaveLength(2);
    expect(id).toBe("dep-1");
    expect(getUsagePresetShortLabel(UsageTimeRange.ONE_MONTH)).toBe("1M");
  });
});

