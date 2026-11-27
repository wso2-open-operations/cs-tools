// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import type { AlertMuteRuleMutedAlertsResponse } from "@features/csm-operations/types/alertMuteRules";

// The backend client reads runtime config at module load; the helper under test never calls it.
vi.mock("@api/backend/client", () => ({ BackendApiError: class extends Error {}, useBackendApi: () => ({}) }));

const { mutedAlertsUnsettled } = await import("@features/csm-operations/api/useAlertMuteRules");

const page = (summary: AlertMuteRuleMutedAlertsResponse["summary"]): AlertMuteRuleMutedAlertsResponse => ({
  items: [],
  total: 0,
  limit: 20,
  offset: 0,
  summary,
});

describe("mutedAlertsUnsettled", () => {
  it("keeps polling while an alert is still muted or being delivered after the rule ended", () => {
    expect(mutedAlertsUnsettled(page({ muted: 1, incident: 2 }))).toBe(true);
    expect(mutedAlertsUnsettled(page({ delivering: 1 }))).toBe(true);
  });

  it("stops once every alert has its final outcome", () => {
    expect(mutedAlertsUnsettled(page({ incident: 2, closed_quietly: 1 }))).toBe(false);
    expect(mutedAlertsUnsettled(page({}))).toBe(false);
    expect(mutedAlertsUnsettled(undefined)).toBe(false);
  });
});
