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

import { renderToStaticMarkup } from "react-dom/server";
import { AxiosError, AxiosHeaders } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { getPortalAccess } from "@src/access/portalAccess";
import { errorStatus } from "@src/access/usePortalAccess";
import { ApiError } from "@utils/ApiError";

const state = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock("@src/access/usePortalAccess", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@src/access/usePortalAccess")>();
  return { ...actual, usePortalAccess: () => state.current };
});

// The UI packages pull in CSS that a node-side test cannot load, and only the gate's branching is
// under test, so they are stood in for with plain elements.
vi.mock("@wso2/oxygen-ui", () => {
  const box = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  return {
    Stack: box,
    Typography: box,
    CircularProgress: () => <span>Loading</span>,
    Button: ({ children }: { children?: React.ReactNode }) => <button>{children}</button>,
  };
});
vi.mock("@utils/logger", () => ({ Logger: { warn: vi.fn(), info: vi.fn(), error: vi.fn() } }));
vi.mock("@wso2/oxygen-ui-icons-react", () => ({ Ban: () => <i />, TriangleAlert: () => <i /> }));

import { AccessGate } from "./AccessGate";

function axiosError(status: number): AxiosError {
  return new AxiosError("failed", "ERR", undefined, undefined, {
    status,
    statusText: "",
    data: {},
    headers: {},
    config: { headers: new AxiosHeaders() },
  });
}

function render(over: Record<string, unknown>): string {
  state.current = {
    access: getPortalAccess(undefined),
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    ...over,
  };
  return renderToStaticMarkup(
    <AccessGate>
      <p>the pages</p>
    </AccessGate>,
  );
}

describe("AccessGate", () => {
  beforeEach(() => {
    state.current = {};
  });

  it("shows no pages while the roles are loading", () => {
    const html = render({ isLoading: true });
    expect(html).not.toContain("the pages");
    expect(html).toContain("Loading");
  });

  it("shows the pages to a user who holds a portal role", () => {
    expect(render({ access: getPortalAccess(["viewer"]) })).toContain("the pages");
  });

  it("shows the no-access screen to a user who holds no portal role", () => {
    const html = render({ access: getPortalAccess([]) });
    expect(html).not.toContain("the pages");
    expect(html).toContain("don&#x27;t have access");
  });

  it("does not count a role that grants nothing on its own", () => {
    const html = render({ access: getPortalAccess(["announcement_creator"]) });
    expect(html).not.toContain("the pages");
    expect(html).toContain("don&#x27;t have access");
  });

  it("shows the no-access screen when the profile request is a 401 or 403", () => {
    for (const error of [axiosError(401), axiosError(403), new ApiError(403, "Forbidden")]) {
      const html = render({ isError: true, error });
      expect(html).not.toContain("the pages");
      expect(html).toContain("don&#x27;t have access");
      expect(html).toContain(`answered ${errorStatus(error)}`);
    }
  });

  it("offers a retry, not the no-access screen, for any other failure", () => {
    for (const error of [axiosError(500), axiosError(503), new Error("Network Error")]) {
      const html = render({ isError: true, error });
      expect(html).not.toContain("the pages");
      expect(html).not.toContain("don&#x27;t have access");
      expect(html).toContain("Retry");
    }
  });
});
