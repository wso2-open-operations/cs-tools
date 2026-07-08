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
import ChangeSeverityDialog from "@features/csm-cases/components/ChangeSeverityDialog";

describe("ChangeSeverityDialog — submission gating", () => {
  it("disables Change severity until a different value is picked", () => {
    render(
      <ChangeSeverityDialog
        currentSeverity="S4"
        isManagedCloud
        isChanging={false}
        onClose={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /change severity/i })).toBeDisabled();
    fireEvent.click(screen.getByRole("radio", { name: /s3/i }));
    expect(screen.getByRole("button", { name: /change severity/i })).toBeEnabled();
  });

  it("calls onChange with the newly picked severity", () => {
    const onChange = vi.fn();
    render(
      <ChangeSeverityDialog
        currentSeverity="S4"
        isManagedCloud
        isChanging={false}
        onClose={() => {}}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByRole("radio", { name: /s2/i }));
    fireEvent.click(screen.getByRole("button", { name: /change severity/i }));
    expect(onChange).toHaveBeenCalledWith("S2");
  });

  it("calls onClose on Cancel without calling onChange", () => {
    const onChange = vi.fn();
    const onClose = vi.fn();
    render(
      <ChangeSeverityDialog
        currentSeverity="S4"
        isManagedCloud
        isChanging={false}
        onClose={onClose}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("ChangeSeverityDialog — S0 is reserved for Managed Cloud", () => {
  it("disables the S0 option for a non-Managed-Cloud case", () => {
    render(
      <ChangeSeverityDialog
        currentSeverity="S4"
        isManagedCloud={false}
        isChanging={false}
        onClose={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.getByRole("radio", { name: /s0/i })).toBeDisabled();
  });

  it("allows picking S0 for a Managed Cloud case", () => {
    const onChange = vi.fn();
    render(
      <ChangeSeverityDialog
        currentSeverity="S4"
        isManagedCloud
        isChanging={false}
        onClose={() => {}}
        onChange={onChange}
      />,
    );
    const s0 = screen.getByRole("radio", { name: /s0/i });
    expect(s0).toBeEnabled();
    fireEvent.click(s0);
    fireEvent.click(screen.getByRole("button", { name: /change severity/i }));
    expect(onChange).toHaveBeenCalledWith("S0");
  });
});
