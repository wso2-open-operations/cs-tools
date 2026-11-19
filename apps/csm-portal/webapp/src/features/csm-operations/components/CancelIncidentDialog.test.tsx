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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CancelIncidentDialog from "@features/csm-operations/components/CancelIncidentDialog";

describe("CancelIncidentDialog", () => {
  it("names the incident and confirms only on the Cancel incident button", () => {
    const onConfirm = vi.fn();
    const onClose = vi.fn();
    render(
      <CancelIncidentDialog incidentNumber="INC0012345" isSubmitting={false} onClose={onClose} onConfirm={onConfirm} />,
    );
    expect(screen.getByRole("dialog", { name: "Cancel this incident?" })).toHaveTextContent("INC0012345");
    fireEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Cancel incident" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("disables Go back while the cancellation is in flight", () => {
    render(<CancelIncidentDialog isSubmitting onClose={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
  });
});
