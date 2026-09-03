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
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import EscalationLevelChip from "@components/EscalationLevelChip";

describe("EscalationLevelChip", () => {
  it("renders the full label for a real level, outlined", () => {
    render(<EscalationLevelChip level="2" />);
    const chip = screen.getByText("EL2 — Technology Unit Head");
    expect(chip).toBeInTheDocument();
    expect(chip.closest(".MuiChip-root")).toHaveClass("MuiChip-outlined");
  });

  it("renders the compact label in short mode", () => {
    render(<EscalationLevelChip level="3" short />);
    expect(screen.getByText("EL3")).toBeInTheDocument();
    expect(screen.queryByText(/CRE Head/)).not.toBeInTheDocument();
  });

  it("renders 'Not escalated' for level 0", () => {
    render(<EscalationLevelChip level="0" />);
    expect(screen.getByText("Not escalated")).toBeInTheDocument();
  });

  it("falls back to a raw 'ELn' label for an unrecognised level instead of hiding it", () => {
    render(<EscalationLevelChip level="9" />);
    expect(screen.getByText("EL9")).toBeInTheDocument();
  });
});
