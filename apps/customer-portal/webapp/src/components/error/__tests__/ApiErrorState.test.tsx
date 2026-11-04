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
import ApiErrorState from "@components/error/ApiErrorState";
import { ApiError } from "@utils/ApiError";

vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreview: () => ({ data: null, isLoading: false }),
}));

vi.mock("@components/error/ErrorRecoveryAction", () => ({
  default: () => <button type="button">Recovery action</button>,
}));

describe("ApiErrorState", () => {
  it("renders", () => {
    render(<ApiErrorState />);
    expect(screen.getByText(/Something went wrong/)).toBeInTheDocument();
  });

  it.each([
    [403, "Forbidden"],
    [404, "Not Found"],
  ])("omits the recovery action for %i by default (embedded panels)", (status, text) => {
    render(<ApiErrorState error={new ApiError(status, text)} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it.each([
    [403, "Forbidden"],
    [404, "Not Found"],
  ])("renders the recovery action for %i when showRecoveryAction is set", (status, text) => {
    render(<ApiErrorState error={new ApiError(status, text)} showRecoveryAction />);
    expect(screen.getByRole("button", { name: "Recovery action" })).toBeInTheDocument();
  });

  it("never renders the recovery action for 5xx, even when requested", () => {
    render(<ApiErrorState error={new ApiError(500, "Server Error")} showRecoveryAction />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("uses accessMessage for 403 and 404 but not for 500", () => {
    const { unmount } = render(
      <ApiErrorState error={new ApiError(403, "Forbidden")} accessMessage="No access here." />,
    );
    expect(screen.getByText("No access here.")).toBeInTheDocument();
    unmount();

    const { unmount: unmount404 } = render(
      <ApiErrorState
        error={new ApiError(404, "Not Found")}
        fallbackMessage="Load failed."
        accessMessage="No access here."
      />,
    );
    expect(screen.getByText("No access here.")).toBeInTheDocument();
    unmount404();

    render(
      <ApiErrorState
        error={new ApiError(500, "Server Error")}
        fallbackMessage="Load failed."
        accessMessage="No access here."
      />,
    );
    expect(screen.queryByText("No access here.")).not.toBeInTheDocument();
  });
});
