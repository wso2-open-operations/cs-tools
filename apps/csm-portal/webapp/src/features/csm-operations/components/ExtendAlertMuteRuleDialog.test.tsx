// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { AlertMuteRule } from "@features/csm-operations/types/alertMuteRules";
import ExtendAlertMuteRuleDialog from "@features/csm-operations/components/ExtendAlertMuteRuleDialog";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";

afterEach(() => {
  cleanup();
  clearUserPreferredTimeZone();
  vi.unstubAllEnvs();
});

const rule = {
  id: "r1",
  number: "AMR0000001",
  name: "Gateway upgrade",
  description: "",
  timeZone: "Asia/Colombo",
  startOn: "2030-01-01T10:00:00.000Z",
  endOn: "2030-01-01T12:30:00.000Z", // 18:00 in Colombo
  conditions: [],
  createdBy: "a@example.test",
  createdOn: "2029-12-31T00:00:00.000Z",
  updatedBy: "a@example.test",
  updatedOn: "2029-12-31T00:00:00.000Z",
  endedBy: null,
  endedOn: null,
} as AlertMuteRule;

describe("ExtendAlertMuteRuleDialog", () => {
  it("shows the new end in the display zone, not the browser's, and submits the instant it means", () => {
    vi.stubEnv("TZ", "UTC");
    setUserPreferredTimeZone("Asia/Colombo");
    const onSubmit = vi.fn();
    render(<ExtendAlertMuteRuleDialog rule={rule} isSubmitting={false} error={null} onClose={vi.fn()} onSubmit={onSubmit} />);

    fireEvent.click(screen.getByRole("button", { name: "+30 min" }));
    // 18:30 in Colombo, as the "Currently ends" text reads; the browser (UTC) would show 13:00.
    const field = screen.getByRole("group", { name: "New end" });
    expect(field.textContent).toMatch(/06:30\s*PM|18:30/);
    fireEvent.click(screen.getByRole("button", { name: "Extend" }));
    expect(onSubmit).toHaveBeenCalledWith("2030-01-01T13:00:00.000Z");
  });
});
