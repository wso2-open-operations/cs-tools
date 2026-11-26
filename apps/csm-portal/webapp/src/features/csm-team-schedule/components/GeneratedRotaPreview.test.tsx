/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import GeneratedRotaPreview from "./GeneratedRotaPreview";
import type { GenerateRotaResult } from "../api/useGenerateRota";

function result(turns: GenerateRotaResult["turns"]): GenerateRotaResult {
  return {
    rotaCode: "SRE_SAAS",
    month: "2026-11",
    from: "2026-11-01",
    dryRun: true,
    alreadyGenerated: false,
    summary: { planned: turns.length, keptByHand: 0, written: 0, replaced: 0, lieuPlanned: 0, lieuWritten: 0 },
    turns,
    lieuLeave: [],
    warnings: [],
  };
}

const turn = (name: string, shiftCode: string, tier: string, rotaDate = "2026-11-07") => ({
  userId: name.toLowerCase(), name, teamKey: "apollo", shiftCode, tier, rotaDate,
});

describe("GeneratedRotaPreview", () => {
  it("shows a TZ1+2 / TZ3 weekend as the weekend day and TZ3", () => {
    render(<GeneratedRotaPreview result={result([turn("Ann", "SRE_WE_TZ1", "L1"), turn("Max", "SRE_TZ3", "L1")])} />);
    expect(screen.getByText(/Weekend day: L1 Ann/)).toBeInTheDocument();
    expect(screen.getByText("L1 Max")).toBeInTheDocument();
  });

  it("shows a Day/Night weekend under Day and Night, in the same columns", () => {
    render(<GeneratedRotaPreview result={result([turn("Ann", "SRE_WE_DAY", "L1"), turn("Max", "SRE_WE_NIGHT", "L1")])} />);
    expect(screen.getByText(/^Day: L1 Ann/)).toBeInTheDocument();
    expect(screen.getByText(/Night: L1 Max/)).toBeInTheDocument();
    expect(screen.queryByText(/Weekend day/)).not.toBeInTheDocument();
  });
});
