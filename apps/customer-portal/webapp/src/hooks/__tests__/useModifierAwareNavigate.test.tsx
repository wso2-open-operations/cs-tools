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

import { renderHook, act } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";

const navigateMock = vi.fn();
const openMock = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

const wrapper = ({ children }: { children: ReactNode }) => (
  <MemoryRouter initialEntries={["/projects/1/dashboard"]}>{children}</MemoryRouter>
);

describe("useModifierAwareNavigate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.open = openMock;
  });

  it("navigates normally without modifier key", () => {
    const { result } = renderHook(() => useModifierAwareNavigate(), { wrapper });
    act(() => {
      result.current("/support");
    });
    expect(navigateMock).toHaveBeenCalledWith("/support", undefined);
    expect(openMock).not.toHaveBeenCalled();
  });

  it("opens a new tab when modifier key is held", () => {
    const { result } = renderHook(() => useModifierAwareNavigate(), { wrapper });

    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key: "Meta" }));
    });
    act(() => {
      result.current("/support/cases");
    });

    expect(openMock).toHaveBeenCalled();
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
