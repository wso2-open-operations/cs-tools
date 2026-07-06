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

import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import ServiceNowCaseRedirectPage from "@features/project-hub/pages/ServiceNowCaseRedirectPage";

const mockNavigate = vi.fn();
const mockShowError = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useSearchParams: () => [new URLSearchParams("sys_id=case-99")],
  };
});

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: mockShowError }),
}));

vi.mock("@features/support/api/useGetCaseDetails", () => ({
  default: () => ({
    data: { project: { id: "proj-1" } },
    isLoading: false,
    isError: false,
  }),
}));

describe("ServiceNowCaseRedirectPage", () => {
  it("redirects to case details when project resolves", () => {
    mockNavigate.mockClear();
    render(
      <MemoryRouter>
        <ServiceNowCaseRedirectPage />
      </MemoryRouter>,
    );
    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/proj-1/support/cases/case-99",
      { replace: true },
    );
  });
});
