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

import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RequireWriteAccess from "@components/RequireWriteAccess";

let mockCanWrite = true;
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({
    hasAnyRole: true,
    canEscalate: true,
    canDownloadAttachment: true,
    canUseOperations: true,
    canUseTimeCardsAndUpdates: true,
    canWrite: mockCanWrite,
  }),
}));

function renderAt(path: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/list" element={<div>List page</div>} />
        <Route
          path="/list/new"
          element={
            <RequireWriteAccess to="/list">
              <div>Create form</div>
            </RequireWriteAccess>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("RequireWriteAccess", () => {
  it("renders its children for a caller with canWrite", () => {
    mockCanWrite = true;
    renderAt("/list/new");
    expect(screen.getByText("Create form")).toBeInTheDocument();
  });

  it("redirects to the given path for a caller without canWrite, never rendering the form", () => {
    mockCanWrite = false;
    renderAt("/list/new");
    expect(screen.queryByText("Create form")).not.toBeInTheDocument();
    expect(screen.getByText("List page")).toBeInTheDocument();
  });
});
