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
import {
  buildOnboardingPatch,
  initialFormValues,
  matchCurrentStatus,
  validateOnboardingDates,
  type OnboardingFormValues,
} from "@features/csm-projects/utils/onboardingEdit";

// The status list lives beside the PATCH hook, whose backend client reads
// runtime config at module load (absent under vitest).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

const STORED = {
  onboardingStartDate: "2026-03-01T00:00:00Z",
  goLivePlanDate: "2026-04-01T00:00:00Z",
  goLiveDate: null,
  onboardingExpiryDate: "2026-12-31T00:00:00Z",
  productionReadinessCompletionDate: "2026-05-01T00:00:00Z",
};
const OWNER = "00000000-0000-0000-0000-000000000000";
const baseline = (): OnboardingFormValues => initialFormValues("In-Progress", OWNER, STORED);
const withDates = (d: Partial<OnboardingFormValues["dates"]>): OnboardingFormValues => ({
  ...baseline(),
  dates: { ...baseline().dates, ...d },
});

describe("matchCurrentStatus", () => {
  it.each([
    ["OnHold", "On-Hold"],
    ["in progress", "In-Progress"],
    ["NOT_STARTED", "Not-Started"],
  ])("normalizes %s to %s", (raw, want) => expect(matchCurrentStatus(raw)).toBe(want));

  it.each([null, undefined, "", "nonsense"])("gives an empty selection for %s", (raw) =>
    expect(matchCurrentStatus(raw)).toBe(""),
  );
});

describe("initialFormValues", () => {
  it("reads the UTC calendar day of every date, including the readiness date", () => {
    expect(baseline().dates).toEqual({
      start: "2026-03-01",
      plan: "2026-04-01",
      goLive: "",
      expiry: "2026-12-31",
      readiness: "2026-05-01",
    });
  });
});

describe("buildOnboardingPatch", () => {
  it("is empty when nothing changed", () => {
    expect(buildOnboardingPatch(baseline(), baseline())).toEqual({});
  });

  it("maps each changed date to its request key, readiness included", () => {
    const values = withDates({
      start: "2026-02-01",
      plan: "2026-04-02",
      goLive: "2026-06-01",
      expiry: "2027-01-01",
      readiness: "2026-05-02",
    });
    expect(buildOnboardingPatch(values, baseline())).toEqual({
      onboardingStartDate: "2026-02-01",
      onboardingGoLivePlanDate: "2026-04-02",
      onboardingGoLiveDate: "2026-06-01",
      onboardingExpiryDate: "2027-01-01",
      onboardingProductionReadinessCompletionDate: "2026-05-02",
    });
  });

  it("never sends an emptied date, status or owner (nothing can be cleared)", () => {
    const values: OnboardingFormValues = {
      status: "",
      ownerId: "",
      dates: { start: "", plan: "", goLive: "", expiry: "", readiness: "" },
    };
    expect(buildOnboardingPatch(values, baseline())).toEqual({});
  });

  it("sends a changed status and owner", () => {
    const values: OnboardingFormValues = {
      ...baseline(),
      status: "Completed",
      ownerId: "11111111-1111-1111-1111-111111111111",
    };
    expect(buildOnboardingPatch(values, baseline())).toEqual({
      onboardingStatus: "Completed",
      onboardingOwner: "11111111-1111-1111-1111-111111111111",
    });
  });
});

describe("validateOnboardingDates", () => {
  it("accepts unchanged values", () => {
    expect(validateOnboardingDates(baseline(), baseline())).toEqual({});
  });

  it.each(["plan", "goLive", "expiry", "readiness"] as const)(
    "rejects a changed %s date earlier than the stored start",
    (k) => {
      const errors = validateOnboardingDates(withDates({ [k]: "2026-02-01" }), baseline());
      expect(errors[k]).toBe("Can't be earlier than the onboarding start date.");
    },
  );

  it("catches a start moved past an unchanged stored date, readiness included", () => {
    const errors = validateOnboardingDates(withDates({ start: "2026-05-15" }), baseline());
    expect(errors.plan).toContain("The stored date (2026-04-01)");
    expect(errors.readiness).toContain("The stored date (2026-05-01)");
    expect(errors.expiry).toBeUndefined();
  });

  it("allows the start to move earlier", () => {
    expect(validateOnboardingDates(withDates({ start: "2026-01-15" }), baseline())).toEqual({});
  });

  it("does not block an unrelated edit on stored data that already breaks the order", () => {
    const stale = initialFormValues("In-Progress", OWNER, { ...STORED, goLivePlanDate: "2026-02-01T00:00:00Z" });
    const values = { ...stale, dates: { ...stale.dates, goLive: "2026-06-01" } };
    expect(validateOnboardingDates(values, stale)).toEqual({});
  });
});
