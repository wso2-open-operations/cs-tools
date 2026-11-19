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

  it("disables Go back and ignores Escape while the cancellation is in flight", () => {
    const onClose = vi.fn();
    render(<CancelIncidentDialog isSubmitting onClose={onClose} onConfirm={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on Escape when nothing is in flight", () => {
    const onClose = vi.fn();
    render(<CancelIncidentDialog isSubmitting={false} onClose={onClose} onConfirm={vi.fn()} />);
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("says final without implying it can be reopened elsewhere", () => {
    render(<CancelIncidentDialog incidentNumber="INC0012345" isSubmitting={false} onClose={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByRole("dialog")).toHaveTextContent("a cancelled incident can't be reopened or commented on.");
    expect(screen.getByRole("dialog")).not.toHaveTextContent(/from here/);
  });

  it("speaks of every incident when it covers more than one (bulk Edit)", () => {
    render(<CancelIncidentDialog incidentCount={3} isSubmitting={false} onClose={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByRole("dialog", { name: "Cancel 3 incidents?" })).toHaveTextContent("3 incidents will be moved to Cancelled.");
    expect(screen.getByRole("button", { name: "Cancel incidents" })).toBeInTheDocument();
  });
});
