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

import { describe, expect, it } from "vitest";
import { normalizeStage, toCaseSla } from "./caseSlaMapping";
import type { TaskSlaView } from "@features/csm-cases/types/csmCases";

describe("normalizeStage", () => {
  it("lowercases and turns spaces into underscores", () => {
    expect(normalizeStage("In Progress")).toEqual({
      stage: "in_progress",
      label: "In progress",
    });
  });

  it("maps 'achieved' onto 'completed'", () => {
    expect(normalizeStage("Achieved")).toEqual({
      stage: "completed",
      label: "Completed",
    });
  });

  it("falls back to the raw trimmed string for an unrecognized stage", () => {
    expect(normalizeStage(" Escalated ")).toEqual({
      stage: "escalated",
      label: "Escalated",
    });
  });

  it("defaults to in_progress when the stage is null", () => {
    expect(normalizeStage(null)).toEqual({
      stage: "in_progress",
      label: "In progress",
    });
  });
});

describe("toCaseSla", () => {
  const baseView: TaskSlaView = {
    id: "sla-1",
    slaDefinition: { name: "S1 - Response", target: "1 Business Hour" },
    stage: "In Progress",
    task: { id: "case-1" },
    businessTimeLeft: "30 minutes",
    businessElapsedTime: "30 minutes",
    businessElapsedPercentage: 50,
    startTime: "2026-07-01T10:00:00Z",
    endTime: null,
  };

  it("maps a wire-shape record onto the CaseSla row model", () => {
    expect(toCaseSla(baseView)).toEqual({
      id: "sla-1",
      definition: "S1 - Response",
      target: "1 Business Hour",
      stage: "in_progress",
      stageLabel: "In progress",
      hasBreached: false,
      businessTimeLeftLabel: "30 minutes",
      businessElapsedLabel: "30 minutes",
      businessElapsedPercent: 50,
      startTime: "2026-07-01T10:00:00Z",
      stopTime: null,
    });
  });

  it("derives hasBreached from the elapsed percentage reaching 100", () => {
    const breached = toCaseSla({
      ...baseView,
      businessElapsedPercentage: 100,
    });
    expect(breached.hasBreached).toBe(true);

    const over = toCaseSla({ ...baseView, businessElapsedPercentage: 142 });
    expect(over.hasBreached).toBe(true);
  });

  it("defaults missing definition, target, and elapsed percentage safely", () => {
    const sparse = toCaseSla({
      id: "sla-2",
      slaDefinition: null,
      stage: null,
      task: null,
      businessTimeLeft: null,
      businessElapsedTime: null,
      businessElapsedPercentage: null,
      startTime: null,
      endTime: null,
    });
    expect(sparse).toEqual({
      id: "sla-2",
      definition: "",
      target: null,
      stage: "in_progress",
      stageLabel: "In progress",
      hasBreached: false,
      businessTimeLeftLabel: "",
      businessElapsedLabel: "",
      businessElapsedPercent: 0,
      startTime: null,
      stopTime: null,
    });
  });
});
