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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import type { DashboardAnalytics } from "@features/plg/api/types";

// Recharts needs a layout jsdom lacks; the cards and their data are under test.
vi.mock("@features/plg/components/charts", () => ({
  DonutChart: ({ data, solid }: { data: { label: string; count: number }[]; solid?: boolean }) => (
    <div>{solid ? "pie" : "donut"}:{data.map((d) => `${d.label}=${d.count}`).join(",")}</div>
  ),
  MultiLineChart: () => <div>lines</div>,
  CountBarChart: ({ data }: { data: { label: string }[] }) => <div>bars:{data.map((d) => d.label).join(",")}</div>,
  StackedBarChart: ({ rows }: { rows: { label: string }[] }) => <div>stack:{rows.map((r) => r.label).join(",")}</div>,
}));

import { DashboardCharts } from "./DashboardCharts";

const data: DashboardAnalytics = {
  summary: {
    totalOrganizations: 2,
    totalRegistrations: 3,
    newRegistrations: 0,
    pairingsNeedingAttention: 0,
    trialsEndingSoon: 0,
    unresolvedIngestFailures: 0,
  },
  registrationsByProduct: [{ label: "WSO2 Cloud", count: 9 }],
  registrationsByEntryProduct: [{ label: "API Platform", count: 2 }],
  registrationsByLifecycleStage: [],
  subscriptionMix: [],
  registrationsOverTime: { buckets: [], series: [] },
  utmSources: [
    { label: "google", count: 2 },
    { label: "NOT_RECORDED", count: 1 },
    { label: "OTHER", count: 4 },
  ],
  emailClassification: [
    { label: "CORPORATE", count: 3 },
    { label: "UNKNOWN", count: 2 },
  ],
  qualificationOverview: [{ label: "ELIGIBLE", count: 1 }],
  qualifiedByUtm: [{ label: "google", eligible: 1, monitor: 0, excluded: 0, notAssessed: 1, total: 2 }],
};

describe("DashboardCharts", () => {
  it("draws the six overview cards", () => {
    render(<DashboardCharts data={data} />);
    for (const title of [
      "Registration Trends",
      "Registrations by Entry Product",
      "UTM Sources of the Registrations",
      "Email Classification by Registration",
      "Registration Qualification Overview",
      "Qualified Registrations with UTM",
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
    // A registration with no UTM source and the folded remainder read as words, a real source as itself.
    expect(screen.getByText("pie:google=2,Not recorded=1,Other=4")).toBeInTheDocument();
    // The validation's "could not tell" is not a class of email.
    // The four email classes are always listed; Unknown is left out.
    expect(
      screen.getByText("donut:CORPORATE=3,PERSONAL=0,DISPOSABLE=0,INTERNAL=0"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/UNKNOWN/)).not.toBeInTheDocument();
    // The entry product is a bar chart too, not a ring.
    expect(screen.getByText("bars:API Platform")).toBeInTheDocument();
    expect(screen.queryByText(/WSO2 Cloud/)).not.toBeInTheDocument();
    expect(screen.getByText("stack:google")).toBeInTheDocument();
  });
});
