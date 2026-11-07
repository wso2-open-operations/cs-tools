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

import { describe, expect, it, vi } from "vitest";

// A stand-in for the real class, so the module under test and this test
// share it without loading the real client (same approach as
// saveErrorMessage.test.ts).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    payload?: { message?: string };
    constructor(status: number, message: string, payload?: { message?: string }) {
      super(message);
      this.status = status;
      this.payload = payload;
    }
  },
}));

import { BackendApiError } from "@api/backend/client";
import { generateErrorMessage } from "@features/csm-custom-dashboards/utils/generateErrorMessage";

/** A failure as the backend answers it: its message in the JSON body. */
function backend(status: number, message: string): BackendApiError {
  return new BackendApiError(status, message, { message });
}

describe("generateErrorMessage", () => {
  it.each([
    [400, "The description is too long: keep it under 2,000 characters."],
    [413, "The dashboard being edited is too large to change in the chat. Start a new one."],
    [429, "You've sent a lot of messages in a short time. Try again in 42 seconds."],
    [502, "The AI assistant can't be reached right now. Try again in a few minutes."],
    [502, "The dashboard was too large to create in one go. Ask for fewer widgets, or build it in steps."],
    [503, "The AI assistant has reached its usage limit. Try again later."],
    [504, "The AI assistant took too long to answer. Try again."],
  ])("shows the backend's own message for a %d", (status, message) => {
    expect(generateErrorMessage(backend(status, message), false)).toBe(message);
    expect(generateErrorMessage(backend(status, message), true)).toBe(message);
  });

  it.each([
    ["the model declining", "A cases-per-month line chart can cover at most 24 months."],
    ["a refusal", "This request can't be turned into a dashboard. Try rephrasing it."],
    ["a team it cannot filter by", "Team \"Orion\" is not one of the teams a dashboard can filter by."],
  ])("says the input did not work for a 422 (%s), not the backend's sentence", (_name, message) => {
    expect(generateErrorMessage(backend(422, message), false)).toBe("The dashboard cannot be generated from the input.");
    expect(generateErrorMessage(backend(422, message), true)).toBe("This dashboard cannot be modified with your input.");
  });

  it("says too slow for a proxy's own 504 (its error page, no JSON message)", () => {
    expect(generateErrorMessage(new BackendApiError(504, "Gateway Timeout"), false)).toBe(
      "The AI assistant took too long to answer. Try again.",
    );
  });

  it.each([502, 503])("says unreachable for a proxy's own %d", (status) => {
    expect(generateErrorMessage(new BackendApiError(status, "Bad Gateway"), false)).toBe(
      "The AI assistant can't be reached right now. Try again in a few minutes.",
    );
  });

  it.each([
    ["offline", new TypeError("Failed to fetch")],
    ["a dropped connection", new TypeError("NetworkError when attempting to fetch resource.")],
    ["an unexpected error", new Error("boom")],
  ])("says the portal got no answer when there was none (%s)", (_name, err) => {
    expect(generateErrorMessage(err, false)).toBe(
      "The portal didn't get an answer. Check your connection and try again.",
    );
  });

  it("says something went wrong for a 500, creating or changing the dashboard", () => {
    const err = backend(500, "Internal error");
    expect(generateErrorMessage(err, false)).toBe("Something went wrong while creating the dashboard. Try again.");
    expect(generateErrorMessage(err, true)).toBe("Something went wrong while changing the dashboard. Try again.");
  });

  it("never shows a message the backend sent with a status it does not word for users", () => {
    expect(generateErrorMessage(backend(401, "token expired"), false)).toBe(
      "Something went wrong while creating the dashboard. Try again.",
    );
  });

  it("never says the input did not work when the failure was elsewhere", () => {
    for (const err of [new TypeError("Failed to fetch"), new BackendApiError(504, "Gateway Timeout"), backend(500, "x")]) {
      expect(generateErrorMessage(err, false)).not.toMatch(/input/);
    }
  });
});
