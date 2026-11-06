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
// useTimeSheets.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

import { BackendApiError } from "@api/backend/client";
import { saveErrorMessage } from "@features/csm-custom-dashboards/utils/saveErrorMessage";

describe("saveErrorMessage", () => {
  it("asks for another name when the name is taken", () => {
    expect(saveErrorMessage(new BackendApiError(409, "you already have a dashboard named: My Cases"), "My Cases")).toBe(
      'You already have a dashboard named "My Cases". Ask for a different name in the chat, then save again.',
    );
  });

  it("shows the backend's message for another 4xx, as a sentence", () => {
    expect(saveErrorMessage(new BackendApiError(400, "at most 50 saved dashboards are allowed"), "X")).toBe(
      "At most 50 saved dashboards are allowed.",
    );
    expect(saveErrorMessage(new BackendApiError(400, "The dashboard name is too long: keep it under 255 characters."), "X")).toBe(
      "The dashboard name is too long: keep it under 255 characters.",
    );
  });

  it("is generic for a server error or anything that is not a backend error", () => {
    const generic = "The dashboard couldn't be saved. Try again in a moment.";
    expect(saveErrorMessage(new BackendApiError(500, "The dashboard couldn't be saved. Try again in a moment."), "X")).toBe(generic);
    expect(saveErrorMessage(new BackendApiError(503, "Service Unavailable"), "X")).toBe(generic);
    expect(saveErrorMessage(new Error("network down"), "X")).toBe(generic);
  });
});
