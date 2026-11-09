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

vi.mock("react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-router")>()),
  useParams: () => ({ organizationId: "o1" }),
  useNavigate: () => vi.fn(),
}));

vi.mock("@features/plg/api/hooks", () => ({
  useOrganization: () => ({ data: org, isPending: false, error: null }),
  useSetOrganizationOwner: () => setOwner,
  useCSUsers: () => ({ data: engineers, isFetching: false }),
}));

// The product tabs are not on the Overview.
vi.mock("./ProductTab", () => ({ ProductTab: () => null }));

beforeEach(() => setOwner.mutate.mockReset());

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
