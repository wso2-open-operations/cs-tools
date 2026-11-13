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
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OrganizationDetail, UserRef } from "@features/plg/api/types";
import OrganizationDetailPage from "./OrganizationDetailPage";

const ana: UserRef = { id: "u1", name: "Ana Perera", email: "ana@example.test" };
const ben: UserRef = { id: "u2", name: "Ben Silva", email: "ben@example.test" };
const engineers = [ana, ben];

const org: OrganizationDetail = {
  id: "o1",
  organizationName: "Acme",
  createdOn: "2026-09-01T00:00:00Z",
  registeredName: "Acme admin",
  registeredEmail: "admin@acme.test",
  companyNameFromDomain: null,
  countryName: null,
  moesifCompanyId: null,
  fieldsInherited: false,
  owner: ana,
  platformCards: [],
};

const setOwner = { mutate: vi.fn(), isPending: false, error: null };

const page = vi.hoisted(() => ({
  navigate: vi.fn(),
  current: undefined as OrganizationDetail | undefined,
}));

vi.mock("react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-router")>()),
  useParams: () => ({ organizationId: "o1" }),
  useNavigate: () => page.navigate,
}));

vi.mock("@features/plg/api/hooks", () => ({
  useOrganization: () => ({ data: page.current ?? org, isPending: false, error: null }),
  useSetOrganizationOwner: () => setOwner,
  useCSUsers: () => ({ data: engineers, isFetching: false }),
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

// The product tabs are not on the Overview.
vi.mock("./ProductTab", () => ({ ProductTab: () => null }));

beforeEach(() => {
  setOwner.mutate.mockReset();
  page.navigate.mockReset();
  page.current = undefined;
});

describe("OrganizationDetailPage: the owner", () => {
  it("marks the PLG CS owner required", () => {
    render(<OrganizationDetailPage />);
    const owner = screen.getByRole("combobox", { name: "PLG CS owner" });
    expect(owner).toBeRequired();
    expect(document.querySelector(`label[for="${owner.id}"] .MuiFormLabel-asterisk`)).toBeInTheDocument();
  });

  it("will not save a cleared owner, which the service refuses", () => {
    render(<OrganizationDetailPage />);
    const save = screen.getByRole("button", { name: "Save owner" });
    expect(save).toBeDisabled();
    fireEvent.click(screen.getByTitle("Clear"));
    expect(screen.getByRole("combobox", { name: "PLG CS owner" })).toHaveValue("");
    expect(save).toBeDisabled();
  });

  it("saves another engineer", () => {
    render(<OrganizationDetailPage />);
    const owner = screen.getByRole("combobox", { name: "PLG CS owner" });
    fireEvent.mouseDown(owner);
    fireEvent.click(screen.getByRole("option", { name: "Ben Silva" }));
    const save = screen.getByRole("button", { name: "Save owner" });
    expect(save).toBeEnabled();
    fireEvent.click(save);
    expect(setOwner.mutate).toHaveBeenCalledWith("u2");
  });
});

describe("OrganizationDetailPage: Cloud ids", () => {
  const cloud: OrganizationDetail = {
    ...org,
    organizationName: null,
    moesifCompanyId: "01a1ba70-b841-718f-86f2-56d0de00d195",
    cloudUserId: "01a1ba70-b85b-703a-81f9-85f219c4456f",
    emailClassification: "PERSONAL",
  };

  it("moves a link by record id to the Cloud organisation id", () => {
    page.current = cloud;
    render(<OrganizationDetailPage />);
    expect(page.navigate).toHaveBeenCalledWith("/plg/organizations/01a1ba70-b841-718f-86f2-56d0de00d195", {
      replace: true,
    });
  });

  it("stays on the record id when there is no Cloud id", () => {
    render(<OrganizationDetailPage />);
    expect(page.navigate).not.toHaveBeenCalled();
  });

  it("shows the Cloud ids and the email classification", () => {
    page.current = cloud;
    render(<OrganizationDetailPage />);
    expect(screen.getByText("01a1ba70-b85b-703a-81f9-85f219c4456f")).toBeInTheDocument();
    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByText("Cloud organisation id")).toBeInTheDocument();
  });
});
