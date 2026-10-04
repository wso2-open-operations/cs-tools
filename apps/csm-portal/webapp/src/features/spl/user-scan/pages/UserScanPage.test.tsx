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
import "@testing-library/jest-dom/vitest";

const mutateMock = vi.fn();
let isPending = false;
vi.mock("@features/spl/user-scan/api/useScanUser", () => ({
  useScanUser: () => ({ mutate: mutateMock, isPending }),
}));

import UserScanPage from "@features/spl/user-scan/pages/UserScanPage";

function fill(): void {
  fireEvent.change(screen.getByLabelText(/^Email/), { target: { value: "jane.doe@example.com" } });
  fireEvent.change(screen.getByLabelText(/^Subscription Key/), { target: { value: "key-1" } });
}

describe("UserScanPage", () => {
  beforeEach(() => {
    mutateMock.mockReset();
    isPending = false;
  });

  it("keeps Analyze disabled until both fields are filled", () => {
    render(<UserScanPage />);
    expect(screen.getByRole("button", { name: "Analyze" })).toBeDisabled();
    fill();
    expect(screen.getByRole("button", { name: "Analyze" })).toBeEnabled();
  });

  it("disables Analyze while a scan is running, so a double click sends one scan", () => {
    const { rerender } = render(<UserScanPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Analyze" }));
    expect(mutateMock).toHaveBeenCalledTimes(1);

    isPending = true;
    rerender(<UserScanPage />);
    expect(screen.getByRole("button", { name: "Analyze" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Analyze" }));
    expect(mutateMock).toHaveBeenCalledTimes(1);
  });
});
