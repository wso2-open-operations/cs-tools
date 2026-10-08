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

import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BackendApi } from "@api/backend/client";
import {
  postCountingOnly,
  resetCountOnlySupport,
} from "@api/backend/postCountingOnly";

/** What `BackendApiError` looks like to the helper: an Error with an HTTP status. */
class BackendApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

const post = vi.fn();
const api = { post } as unknown as BackendApi;
const body: { countOnly?: boolean; pagination: { offset: number; limit: number }; filters: { state: string[] } } = {
  pagination: { offset: 0, limit: 1 },
  filters: { state: ["open"] },
};

describe("postCountingOnly", () => {
  beforeEach(() => {
    post.mockReset();
    resetCountOnlySupport();
  });

  it("asks the server to skip the page query and returns its response", async () => {
    post.mockResolvedValue({ cases: [], total: 42 });

    await expect(postCountingOnly(api, "/cases/search", body)).resolves.toEqual({
      cases: [],
      total: 42,
    });
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith("/cases/search", { ...body, countOnly: true }, undefined);
  });

  it("always sets the flag itself, whatever the caller passed", async () => {
    post.mockResolvedValue({});
    await postCountingOnly(api, "/cases/search", { ...body, countOnly: false });
    expect(post).toHaveBeenCalledWith("/cases/search", { ...body, countOnly: true }, undefined);
  });

  it("falls back to the plain search when the entity service rejects the field, and remembers", async () => {
    post.mockImplementation(async (_path: string, sent: { countOnly?: boolean }) => {
      if (sent.countOnly) throw new BackendApiError(400, "Invalid request payload.");
      return { cases: [{ id: "1" }], total: 7 };
    });

    await expect(postCountingOnly(api, "/cases/search", body)).resolves.toEqual({
      cases: [{ id: "1" }],
      total: 7,
    });
    expect(post).toHaveBeenCalledTimes(2);
    expect(post).toHaveBeenLastCalledWith("/cases/search", body, undefined);

    post.mockClear();
    await postCountingOnly(api, "/cases/search", body);
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith("/cases/search", body, undefined); // no flag any more
  });

  it("reports a request that is invalid either way as the error it is, and keeps asking", async () => {
    post.mockRejectedValue(new BackendApiError(400, "Invalid request payload."));

    await expect(postCountingOnly(api, "/cases/search", body)).rejects.toMatchObject({ status: 400 });
    expect(post).toHaveBeenCalledTimes(2);

    // The plain retry failed too, so nothing was learned about the service.
    post.mockReset();
    post.mockResolvedValue({});
    await postCountingOnly(api, "/cases/search", body);
    expect(post).toHaveBeenCalledWith("/cases/search", { ...body, countOnly: true }, undefined);
  });

  it("does not retry any other failure", async () => {
    post.mockRejectedValue(new BackendApiError(500, "boom"));
    await expect(postCountingOnly(api, "/cases/search", body)).rejects.toMatchObject({ status: 500 });
    expect(post).toHaveBeenCalledTimes(1);

    post.mockReset();
    post.mockRejectedValue(new Error("network"));
    await expect(postCountingOnly(api, "/cases/search", body)).rejects.toThrow("network");
    expect(post).toHaveBeenCalledTimes(1);
  });
});
