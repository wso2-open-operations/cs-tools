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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RegistrationItem, RegistrationPlatform } from "@features/plg/api/types";
import NewRegistrationsPage from "./NewRegistrationsPage";

const platform = (id: string, code: string, name: string, isNew = true): RegistrationPlatform => ({
  orgPlatformId: id,
  product: { id: `prod-${code}`, code, name },
  registeredOn: "2026-11-12T10:00:00Z",
  lifecycleStage: "REGISTRATION",
  stageName: "Registration",
  isNew,
});

const item = (orgId: string, companyId: string, platforms: RegistrationPlatform[]): RegistrationItem => ({
  ...platforms[0],
  organizationId: orgId,
  organizationName: null,
  moesifCompanyId: companyId,
  registeredEmail: null,
  registeredName: null,
  owner: null,
  platforms,
});

const page = vi.hoisted(() => ({
  navigate: vi.fn(),
  acknowledge: vi.fn(),
  registrations: [] as RegistrationItem[],
}));

vi.mock("react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-router")>()),
  useNavigate: () => page.navigate,
}));

vi.mock("@features/plg/api/hooks", () => ({
  useProducts: () => ({ data: [] }),
  useRegistrations: () => ({
    data: { registrations: page.registrations, total: page.registrations.length, limit: 25, offset: 0 },
    isPending: false,
    error: null,
  }),
  useAcknowledge: () => ({ mutateAsync: page.acknowledge, isPending: false, error: null }),
  useCompanyReveal: () => ({
    identity: undefined,
    notFound: false,
    isFetching: false,
    error: null,
    reveal: () => {},
    hide: () => {},
  }),
}));

beforeEach(() => {
  page.navigate.mockReset();
  page.acknowledge.mockReset().mockResolvedValue({});
  page.registrations = [
    item("o1", "cloud-1", [
      platform("p1", "AGENT_PLATFORM", "Agent Platform"),
      platform("p2", "IAM", "Identity & Access Management"),
      platform("p3", "API_PLATFORM", "API Platform", false),
    ]),
    item("o2", "cloud-2", [platform("p4", "IAM", "Identity & Access Management", false)]),
  ];
});

describe("NewRegistrationsPage", () => {
  it("shows each organisation once, with a chip per platform", () => {
    render(<NewRegistrationsPage />);
    expect(screen.getAllByText("cloud-1")).toHaveLength(1);
    expect(screen.getByText("Agent Platform")).toBeInTheDocument();
    expect(screen.getByText("API Platform")).toBeInTheDocument();
    expect(screen.getAllByText("Identity & Access Management")).toHaveLength(2);
  });

  it("acknowledges each of the organisation's unacknowledged registrations", async () => {
    render(<NewRegistrationsPage />);
    fireEvent.click(screen.getAllByRole("button", { name: "Acknowledge" })[0]);
    await waitFor(() => expect(page.acknowledge).toHaveBeenCalledTimes(2));
    expect(page.acknowledge.mock.calls.map(([arg]) => arg.orgPlatformId)).toEqual(["p1", "p2"]);
  });

  it("disables Acknowledge when nothing is left to acknowledge", () => {
    render(<NewRegistrationsPage />);
    expect(screen.getAllByRole("button", { name: "Acknowledge" })[1]).toBeDisabled();
  });

  it("opens the organisation by its Cloud id", () => {
    render(<NewRegistrationsPage />);
    fireEvent.click(screen.getAllByRole("button", { name: "Open" })[0]);
    expect(page.navigate).toHaveBeenCalledWith("/plg/organizations/cloud-1");
  });
});
