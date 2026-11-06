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

vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
  useLocation: () => ({ state: undefined }),
  useSearchParams: () => [new URLSearchParams()],
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: vi.fn() }),
}));
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({ data: [], isLoading: false, isError: false, refetch: vi.fn() }),
}));
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: () => ({ data: [], isLoading: false, isError: false, refetch: vi.fn() }),
}));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: () => <input aria-label="Project" />,
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ ariaRequired }: { ariaRequired?: boolean }) => (
    <textarea aria-label="editor" aria-required={ariaRequired ? "true" : undefined} />
  ),
}));

// Imported after the mocks above so the module picks them up.
import CsmEngagementCreatePage from "@features/csm-engagements/pages/CsmEngagementCreatePage";

describe("CsmEngagementCreatePage — required Description", () => {
  it("marks the rich-text Description required: an asterisk on its caption and aria-required on the editor", () => {
    render(<CsmEngagementCreatePage />);
    const caption = screen.getByText("Description");
    expect(caption.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
    expect(screen.getByLabelText("editor")).toHaveAttribute("aria-required", "true");
    expect(screen.getByRole("button", { name: /^create engagement$/i })).toBeDisabled();
  });
});
