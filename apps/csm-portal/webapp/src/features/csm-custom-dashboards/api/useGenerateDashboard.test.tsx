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

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BeDashboard } from "@api/backend/types";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useGenerateDashboard } from "@features/csm-custom-dashboards/api/useGenerateDashboard";

const current: BeDashboard = {
  id: "5e636a12-0000-4000-8000-000000000001",
  displayName: "My Cases",
  isDefault: false,
  isTeamBased: false,
  widgets: [],
};

describe("useGenerateDashboard", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("sends the prompt alone for the first message", async () => {
    postMock.mockResolvedValue({ dashboard: current, reply: "Here it is." });
    const { result } = renderHook(() => useGenerateDashboard());

    const reply = await result.current("My cases", null);

    expect(postMock).toHaveBeenCalledWith("/dashboards/generate", { prompt: "My cases" });
    expect(reply).toEqual({ dashboard: current, reply: "Here it is." });
  });

  it("sends the dashboard being edited with a follow-up message", async () => {
    postMock.mockResolvedValue({ dashboard: current, reply: "Changed." });
    const { result } = renderHook(() => useGenerateDashboard());

    await result.current("Only open ones", current);

    expect(postMock).toHaveBeenCalledWith("/dashboards/generate", { prompt: "Only open ones", current });
  });

  it("passes a failure on to the caller", async () => {
    const err = new Error("gateway timeout");
    postMock.mockRejectedValue(err);
    const { result } = renderHook(() => useGenerateDashboard());

    await expect(result.current("My cases", null)).rejects.toBe(err);
  });
});
