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

import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const organization = {
  id: "org-1",
  organizationName: "Acme Labs",
  createdOn: "2026-01-15T09:00:00Z",
  registeredEmail: "ana@acmelabs.example.test",
  registeredName: null,
  owner: null,
  countryName: null,
  companyNameFromDomain: null,
  moesifCompanyId: null,
  fieldsInherited: false,
  platformCards: [
    {
      orgPlatformId: "op-1",
      product: { id: "p-1", code: "IAM", name: "Identity" },
      lifecycleStage: "ACTIVATED",
      stageName: "Activated",
      stageEnteredOn: "2026-02-01T09:00:00Z",
      registeredOn: "2026-01-15T09:00:00Z",
      subscriptionTier: "PAYG",
      runActive: 0,
      runTotal: 0,
      taskCompleted: 0,
      taskTotal: 0,
      isNew: false,
    },
  ],
};

const page = vi.hoisted(() => ({ current: undefined as Record<string, unknown> | undefined }));

vi.mock("@features/plg/api/hooks", () => ({
  useOrganization: () => ({ data: page.current ?? organization, isPending: false, error: null }),
  useSetOrganizationOwner: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  // The header shows the company id with a reveal; hidden, it fetches nothing.
  useCompanyReveal: () => ({
    identity: undefined,
    notFound: false,
    isFetching: false,
    error: null,
    reveal: () => {},
    hide: () => {},
  }),
}));
// The product tab and the owner picker fetch their own data; neither is
// what these tests are about.
vi.mock("./ProductTab", () => ({ ProductTab: () => <div>Product tab</div> }));
vi.mock("@features/plg/components/CSUserSelect", () => ({ CSUserSelect: () => <div /> }));

import OrganizationDetailPage from "./OrganizationDetailPage";

function Probe() {
  const location = useLocation();
  return (
    <div data-testid="probe">
      {location.pathname} {JSON.stringify(location.state)}
    </div>
  );
}

beforeEach(() => {
  page.current = undefined;
});

function PathProbe() {
  return <div data-testid="path">{useLocation().pathname}</div>;
}

function renderAt(path: string, state: unknown) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: path, state }]}>
      <PathProbe />
      <Routes>
        <Route path="/plg/organizations/:organizationId" element={<OrganizationDetailPage />} />
        <Route path="/plg/organizations/:organizationId/:productCode" element={<OrganizationDetailPage />} />
        <Route path="/plg/organizations" element={<Probe />} />
        <Route path="/dashboard/preview/plg-organizations" element={<Probe />} />
      </Routes>
    </MemoryRouter>,
  );
}

const fromViewMore = {
  from: "/dashboard/preview/plg-organizations?w=w-1",
  parentState: { from: "/dashboard/my/d-1" },
};

describe("OrganizationDetailPage: Back", () => {
  it("goes back to the page it was opened from, with that page's own state", () => {
    renderAt("/plg/organizations/org-1", fromViewMore);
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByTestId("probe").textContent).toBe(
      `/dashboard/preview/plg-organizations ${JSON.stringify({ from: "/dashboard/my/d-1" })}`,
    );
  });

  it("goes to the organisation list, as before, when it was opened from the PLG pages", () => {
    renderAt("/plg/organizations/org-1", null);
    fireEvent.click(screen.getByRole("button", { name: "All organisations" }));
    expect(screen.getByTestId("probe").textContent).toBe("/plg/organizations null");
  });

  it("still goes back to where it was opened from after switching to a product tab", () => {
    renderAt("/plg/organizations/org-1", fromViewMore);
    fireEvent.click(screen.getByRole("tab", { name: "Identity" }));
    expect(screen.getByText("Product tab")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByTestId("probe").textContent).toBe(
      `/dashboard/preview/plg-organizations ${JSON.stringify({ from: "/dashboard/my/d-1" })}`,
    );
  });

  it("keeps the way back when a registered platform card opens its product tab", () => {
    renderAt("/plg/organizations/org-1", fromViewMore);
    fireEvent.click(screen.getAllByText("Identity").at(-1)!);
    expect(screen.getByText("Product tab")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
  });
});

describe("OrganizationDetailPage: Cloud ids", () => {
  const cloud = {
    ...organization,
    organizationName: null,
    moesifCompanyId: "01a1ba70-b841-718f-86f2-56d0de00d195",
    cloudUserId: "01a1ba70-b85b-703a-81f9-85f219c4456f",
    emailClassification: "PERSONAL",
  };

  it("moves a link by record id to the Cloud organisation id, keeping the way back", () => {
    page.current = cloud;
    renderAt("/plg/organizations/org-1", fromViewMore);
    expect(screen.getByTestId("path").textContent).toBe("/plg/organizations/01a1ba70-b841-718f-86f2-56d0de00d195");
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
  });

  it("stays on the record id when there is no Cloud id", () => {
    renderAt("/plg/organizations/org-1", null);
    expect(screen.getByTestId("path").textContent).toBe("/plg/organizations/org-1");
  });

  it("shows the Cloud ids and the email classification", () => {
    page.current = cloud;
    renderAt("/plg/organizations/org-1", null);
    expect(screen.getByText("01a1ba70-b85b-703a-81f9-85f219c4456f")).toBeInTheDocument();
    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByText("Cloud organisation id")).toBeInTheDocument();
  });
});

describe("OrganizationDetailPage: registration metadata", () => {
  const withMeta = {
    ...organization,
    registrationMetadata: {
      utmSource: "google",
      utmMedium: "cpc",
      utmCampaign: "spring-launch",
      initialUtmSource: "newsletter",
      initialUtmMedium: null,
      initialUtmCampaign: null,
      referringDomain: "example.com",
      initialReferringDomain: null,
      signInMethod: "github",
      preferredLanguage: "en-GB",
    },
  };

  it("shows the campaign, source, medium, referring domain and language of the signup's visit", () => {
    page.current = withMeta;
    renderAt("/plg/organizations/org-1", null);
    expect(screen.getByText("UTM campaign")).toBeInTheDocument();
    expect(screen.getByText("spring-launch")).toBeInTheDocument();
    expect(screen.getByText("google")).toBeInTheDocument();
    expect(screen.getByText("cpc")).toBeInTheDocument();
    expect(screen.getByText("example.com")).toBeInTheDocument();
    expect(screen.getByText("en-GB")).toBeInTheDocument();
  });

  it("shows dashes when the signup carried no metadata", () => {
    renderAt("/plg/organizations/org-1", null);
    expect(screen.getByText("UTM campaign")).toBeInTheDocument();
    expect(screen.queryByText("spring-launch")).not.toBeInTheDocument();
  });
});
