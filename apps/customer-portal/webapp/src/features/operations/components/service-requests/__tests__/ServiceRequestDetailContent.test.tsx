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
import ServiceRequestDetailContent from "@features/operations/components/service-requests/ServiceRequestDetailContent";

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@features/support/api/usePatchCase", () => ({
  usePatchCase: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => ({ data: { email: "user@test.dev" }, isLoading: false }),
}));

vi.mock("@features/support/api/useGetCaseCommentsInfinite", () => ({
  default: () => ({
    data: { pages: [] },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

vi.mock("@features/support/api/usePostComment", () => ({
  usePostComment: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@case-details-attachments/CaseDetailsAttachmentsPanel", () => ({
  default: () => <div data-testid="attachments-panel" />,
}));

vi.mock("@components/rich-text-editor/Editor", () => ({
  default: () => <div data-testid="comment-editor" />,
}));

const caseDetails = {
  id: "case-1",
  number: "SR100",
  title: "Renew certificate",
  description: "Details",
  status: { id: "1", label: "Open" },
  severity: { id: "3", label: "S3" },
  createdOn: "2026-01-01",
} as never;

describe("ServiceRequestDetailContent", () => {
  it("renders service request title", () => {
    render(
      <ServiceRequestDetailContent
        data={caseDetails}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.getByText("Renew certificate")).toBeInTheDocument();
  });
});
