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
    payload?: { message?: unknown; errorCode?: unknown };
    constructor(status: number, message: string, payload?: { message?: unknown; errorCode?: unknown }) {
      super(message);
      this.status = status;
      this.payload = payload;
    }
  },
}));

import { BackendApiError } from "@api/backend/client";
import type { BeErrorPayload } from "@api/backend/types";
import { generateErrorMessage } from "@features/csm-custom-dashboards/utils/generateErrorMessage";

/** A failure as the backend answers it: its message in the JSON body, marked
 * by its own errorCode. */
function backend(status: number, message: string, errorCode = "generate_call_failed"): BackendApiError {
  return new BackendApiError(status, message, { message, errorCode });
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
    ["the model declining", "generate_declined", "A cases-per-month line chart can cover at most 24 months."],
    ["a refusal", "generate_refused", "This request can't be turned into a dashboard. Try rephrasing it."],
    ["a team it cannot filter by", "generate_team", "Team \"Orion\" is not one of the teams a dashboard can filter by."],
    ["a product that names none", "generate_product", 'No product is named like "Orion".'],
    ["a version its product does not have", "generate_product", 'WSO2 API Manager has no version "3.9" (its versions: 4.3.0).'],
  ])("says the input did not work for a 422 (%s), then why on the next line", (_name, errorCode, message) => {
    expect(generateErrorMessage(backend(422, message, errorCode), false)).toBe(
      `The dashboard cannot be generated from the input.\n${message}`,
    );
    expect(generateErrorMessage(backend(422, message, errorCode), true)).toBe(
      `This dashboard cannot be modified with your input.\n${message}`,
    );
  });

  it("trims the backend's reason", () => {
    expect(generateErrorMessage(backend(422, "  Revenue is not in the portal. \n", "generate_declined"), false)).toBe(
      "The dashboard cannot be generated from the input.\nRevenue is not in the portal.",
    );
  });

  it.each([
    ["no body", new BackendApiError(422, "Unprocessable Entity")],
    ["a proxy's JSON message without a code", new BackendApiError(422, "x", { message: "upstream said no" })],
    ["a message with a code of its own", new BackendApiError(422, "x", { message: "upstream said no", errorCode: "UPSTREAM" })],
    ["the backend's code without a message", new BackendApiError(422, "x", { errorCode: "generate_declined" })],
    ["an empty message", backend(422, "  ", "generate_declined")],
    ["a message that is not text", new BackendApiError(422, "x", { message: { text: "no" }, errorCode: "generate_declined" } as unknown as BeErrorPayload)],
  ])("says only that the input did not work for a 422 without the backend's reason (%s)", (_name, err) => {
    expect(generateErrorMessage(err, false)).toBe("The dashboard cannot be generated from the input.");
    expect(generateErrorMessage(err, true)).toBe("This dashboard cannot be modified with your input.");
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
    ["no body", new BackendApiError(429, "Too Many Requests")],
    ["a JSON body without a message", new BackendApiError(429, "Too Many Requests", {})],
  ])("says too many requests for a proxy's own 429 (%s)", (_name, err) => {
    expect(generateErrorMessage(err, false)).toBe("Too many requests right now. Wait a minute and try again.");
    expect(generateErrorMessage(err, true)).toBe("Too many requests right now. Wait a minute and try again.");
  });

  it("words a proxy's own 413 like the backend's draft-too-large message", () => {
    expect(generateErrorMessage(new BackendApiError(413, "Payload Too Large"), true)).toBe(
      "The dashboard being edited is too large to change in the chat. Start a new one.",
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

  it.each([
    ["a top-level message", { message: "Runtime Error" }],
    ["a nested message", { error: { code: "ServiceUnavailable", message: "upstream connect error" } }],
    ["a message and a code of its own", { message: "no healthy upstream", errorCode: "UPSTREAM_DOWN" }],
    ["a message that is not text", { message: { text: "Runtime Error" }, errorCode: "generate_call_failed" }],
  ])("never shows a gateway's own 503 JSON (%s)", (_name, payload) => {
    // Any JSON at run time, whatever the payload's type says.
    const err = new BackendApiError(503, "Service Unavailable", payload as unknown as BeErrorPayload);
    expect(generateErrorMessage(err, false)).toBe(
      "The AI assistant can't be reached right now. Try again in a few minutes.",
    );
  });

  it("words a message without the backend's errorCode (an older backend) by its status", () => {
    const err = new BackendApiError(503, "x", { message: "The AI assistant has reached its usage limit. Try again later." });
    expect(generateErrorMessage(err, false)).toBe(
      "The AI assistant can't be reached right now. Try again in a few minutes.",
    );
  });

  it("never shows a gateway's own JSON for a 502 or 504 either", () => {
    expect(generateErrorMessage(new BackendApiError(502, "Bad Gateway", { message: "Runtime Error" }), false)).toBe(
      "The AI assistant can't be reached right now. Try again in a few minutes.",
    );
    expect(generateErrorMessage(new BackendApiError(504, "Gateway Timeout", { message: "upstream timed out" }), false)).toBe(
      "The AI assistant took too long to answer. Try again.",
    );
  });

  it("never shows a message the backend sent with a status it does not word for users", () => {
    expect(generateErrorMessage(backend(401, "token expired"), false)).toBe(
      "Something went wrong while creating the dashboard. Try again.",
    );
  });

  it("never says the input did not work when the failure was elsewhere", () => {
    for (const err of [
      new TypeError("Failed to fetch"),
      new BackendApiError(504, "Gateway Timeout"),
      new BackendApiError(429, "Too Many Requests"),
      new BackendApiError(413, "Payload Too Large"),
      backend(500, "x"),
    ]) {
      expect(generateErrorMessage(err, false)).not.toMatch(/input/);
    }
  });
});
