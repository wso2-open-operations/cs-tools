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
import SetAutocloseHoldDialog from "@features/csm-cases/components/SetAutocloseHoldDialog";

describe("SetAutocloseHoldDialog — submission gating", () => {
  it("disables Hold until a date is picked", () => {
    render(
      <SetAutocloseHoldDialog
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /^hold$/i })).toBeDisabled();
  });

  it("calls onSave with a UTC ISO string once a pre-seeded future date is submitted", () => {
    const onSave = vi.fn();
    render(
      <SetAutocloseHoldDialog
        currentHoldUntil="2099-06-15T23:59:00.000Z"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^hold$/i }));
    expect(onSave).toHaveBeenCalledTimes(1);
    const arg = onSave.mock.calls[0][0] as string;
    expect(() => new Date(arg).toISOString()).not.toThrow();
    expect(new Date(arg).getUTCFullYear()).toBe(2099);
  });

  it("calls onClose on Cancel without calling onSave", () => {
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <SetAutocloseHoldDialog
        isSaving={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("seeds the picker from currentHoldUntil when provided", () => {
    render(
      <SetAutocloseHoldDialog
        currentHoldUntil="2099-06-15T23:59:00.000Z"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    // A pre-filled valid future date should leave Hold enabled immediately.
    expect(screen.getByRole("button", { name: /^hold$/i })).toBeEnabled();
  });
});
