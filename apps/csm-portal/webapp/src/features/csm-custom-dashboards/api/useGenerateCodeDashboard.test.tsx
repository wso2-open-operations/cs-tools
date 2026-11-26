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

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useGenerateCodeDashboard } from "@features/csm-custom-dashboards/api/useGenerateCodeDashboard";

describe("useGenerateCodeDashboard", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ source: "async function render() {}\n", reply: "Done." });
  });

  it("sends the prompt alone on a first message", async () => {
    const { result } = renderHook(() => useGenerateCodeDashboard());
    await expect(result.current("My cases", null)).resolves.toEqual({ source: "async function render() {}\n", reply: "Done." });
    expect(postMock).toHaveBeenCalledWith("/dashboards/generate-code", { prompt: "My cases" });
  });

  it("sends the source being edited with a follow-up", async () => {
    const { result } = renderHook(() => useGenerateCodeDashboard());
    await result.current("Add a list", "function render() {}");
    expect(postMock).toHaveBeenCalledWith("/dashboards/generate-code", { prompt: "Add a list", current: "function render() {}" });
  });
});
