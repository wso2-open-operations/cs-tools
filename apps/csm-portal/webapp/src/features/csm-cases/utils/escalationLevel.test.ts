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
import {
  canDeescalate,
  canEscalateFurther,
  escalationLevelColor,
  escalationLevelLabel,
  isEscalationLevelUnset,
} from "./escalationLevel";

describe("escalationLevelLabel", () => {
  it("gives every level 0-5 a human label", () => {
    expect(escalationLevelLabel("0")).toBe("Not escalated");
    expect(escalationLevelLabel("1")).toBe("EL1 — Team Lead");
    expect(escalationLevelLabel("5")).toBe("EL5 — CEO");
  });

  it("gives the short form a compact 'ELn' label", () => {
    expect(escalationLevelLabel("3", true)).toBe("EL3");
  });

  it("falls back to a raw 'ELn' label for an id it doesn't recognise", () => {
    expect(escalationLevelLabel("9")).toBe("EL9");
  });
});

describe("escalationLevelColor", () => {
  it("ramps up in severity with the level", () => {
    expect(escalationLevelColor("0")).toBe("default");
    expect(escalationLevelColor("1")).toBe("info");
    expect(escalationLevelColor("4")).toBe("error");
    expect(escalationLevelColor("5")).toBe("error");
  });

  it("falls back to 'default' for an unrecognised level", () => {
    expect(escalationLevelColor("9")).toBe("default");
  });
});

describe("isEscalationLevelUnset / canEscalateFurther / canDeescalate", () => {
  it("treats null, undefined, and '0' as unset", () => {
    expect(isEscalationLevelUnset(null)).toBe(true);
    expect(isEscalationLevelUnset(undefined)).toBe(true);
    expect(isEscalationLevelUnset("0")).toBe(true);
    expect(isEscalationLevelUnset("1")).toBe(false);
  });

  it("can escalate further from 0 through 4, but not from 5", () => {
    expect(canEscalateFurther(null)).toBe(true);
    expect(canEscalateFurther("4")).toBe(true);
    expect(canEscalateFurther("5")).toBe(false);
  });

  it("can de-escalate from any set level, but not from 0/unset", () => {
    expect(canDeescalate(null)).toBe(false);
    expect(canDeescalate("0")).toBe(false);
    expect(canDeescalate("1")).toBe(true);
    expect(canDeescalate("5")).toBe(true);
  });
});
