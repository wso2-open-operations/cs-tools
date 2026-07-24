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
import SetFixEtaDialog from "@features/csm-cases/components/SetFixEtaDialog";

describe("SetFixEtaDialog — four independent fields", () => {
  it("renders one Save button per field, each disabled until a date/time is picked", () => {
    render(
      <SetFixEtaDialog isSaving={false} onClose={() => {}} onSave={() => {}} />,
    );
    const saveButtons = screen.getAllByRole("button", { name: /^save$/i });
    expect(saveButtons).toHaveLength(4);
    saveButtons.forEach((btn) => expect(btn).toBeDisabled());
  });

  it("seeds each field from its current value and enables that field's Save immediately", () => {
    render(
      <SetFixEtaDialog
        currentFixEta="2099-06-15T10:30:00.000Z"
        currentBestCaseFixEta="2099-06-16T10:30:00.000Z"
        currentMostLikelyFixEta="2099-06-17T10:30:00.000Z"
        currentWorstCaseFixEta="2099-06-18T10:30:00.000Z"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    const saveButtons = screen.getAllByRole("button", { name: /^save$/i });
    expect(saveButtons).toHaveLength(4);
    saveButtons.forEach((btn) => expect(btn).toBeEnabled());
  });

  it("saves only the fixEta field with a UTC ISO string, independent of the other three", () => {
    const onSave = vi.fn();
    render(
      <SetFixEtaDialog
        currentFixEta="2099-06-15T10:30:00.000Z"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    const saveButtons = screen.getAllByRole("button", { name: /^save$/i });
    // The fixEta row is the only one seeded, so it's the only enabled Save.
    const enabled = saveButtons.filter((btn) => !btn.hasAttribute("disabled"));
    expect(enabled).toHaveLength(1);
    fireEvent.click(enabled[0]);
    expect(onSave).toHaveBeenCalledTimes(1);
    const [field, arg] = onSave.mock.calls[0] as [string, string];
    expect(field).toBe("fixEta");
    expect(() => new Date(arg).toISOString()).not.toThrow();
    expect(new Date(arg).getUTCFullYear()).toBe(2099);
  });

  it("calls onClose on Close without calling onSave", () => {
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <SetFixEtaDialog isSaving={false} onClose={onClose} onSave={onSave} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^close$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });
});
