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
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeploymentUsageUploadDialog from "@features/usage-metrics/components/DeploymentUsageUploadDialog";

const mutateMock = vi.fn();
const resetMock = vi.fn();
const showSuccessMock = vi.fn();

vi.mock("@features/usage-metrics/api/usePostDeploymentUsagesImport", () => ({
  usePostDeploymentUsagesImport: () => ({
    mutate: mutateMock,
    isPending: false,
    error: null,
    reset: resetMock,
  }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({
    showSuccess: showSuccessMock,
  }),
}));

describe("DeploymentUsageUploadDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows validation error for non-zip file selections", () => {
    render(<DeploymentUsageUploadDialog open={true} onClose={vi.fn()} />);

    const input = document.querySelector("input[type='file']") as HTMLInputElement;
    const txtFile = new File(["hello"], "note.txt", { type: "text/plain" });
    fireEvent.change(input, { target: { files: [txtFile] } });

    expect(screen.getAllByText("Only ZIP files are accepted.").length).toBeGreaterThan(0);
    expect(mutateMock).not.toHaveBeenCalled();
  });
});

