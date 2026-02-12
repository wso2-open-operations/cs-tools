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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import CaseDetailsTabs from "@case-details/CaseDetailsTabs";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

describe("CaseDetailsTabs", () => {
  it("should render Activity, Details, Attachments, Calls, Knowledge Base tabs", () => {
    const onChange = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsTabs value={0} onChange={onChange} />
      </ThemeProvider>,
    );
    expect(screen.getByRole("tab", { name: /activity/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /details/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /attachments/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /calls/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /knowledge base/i })).toBeInTheDocument();
  });

  it("should call onChange when a tab is clicked", () => {
    const onChange = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsTabs value={0} onChange={onChange} />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("tab", { name: /details/i }));
    expect(onChange).toHaveBeenCalled();
  });

  it("should show Focus Mode button when onFocusModeToggle is provided", () => {
    const onFocusModeToggle = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsTabs
          value={0}
          onChange={vi.fn()}
          onFocusModeToggle={onFocusModeToggle}
        />
      </ThemeProvider>,
    );
    expect(screen.getByRole("button", { name: /focus mode/i })).toBeInTheDocument();
  });

  it("should show Exit Focus Mode when focusMode is true", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsTabs
          value={0}
          onChange={vi.fn()}
          focusMode={true}
          onFocusModeToggle={vi.fn()}
        />
      </ThemeProvider>,
    );
    expect(screen.getByRole("button", { name: /exit focus mode/i })).toBeInTheDocument();
  });
});
