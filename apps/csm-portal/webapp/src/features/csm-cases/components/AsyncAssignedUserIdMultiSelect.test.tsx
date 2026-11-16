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

import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncAssignedUserIdMultiSelect from "@features/csm-cases/components/AsyncAssignedUserIdMultiSelect";

const USER_ID = "d003901b-1b8b-7410-78af-21f1604bcb4b";
const get = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get, post: vi.fn() }),
}));

function renderPicker(values: string[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <AsyncAssignedUserIdMultiSelect values={values} onChange={vi.fn()} />
    </QueryClientProvider>,
  );
}

describe("AsyncAssignedUserIdMultiSelect — a selection restored without a name", () => {
  it("looks the id up and shows the person's name instead of the UUID", async () => {
    get.mockResolvedValue({ id: USER_ID, userName: "jane", firstName: "Jane", lastName: "Doe", email: "jane@wso2.com" });

    renderPicker([USER_ID]);

    expect(await screen.findByText("Jane Doe")).toBeInTheDocument();
    expect(screen.queryByText(USER_ID)).not.toBeInTheDocument();
    expect(get).toHaveBeenCalledWith(`/users/${USER_ID}`);
  });

  it("keeps the raw id when the lookup fails", async () => {
    get.mockReset();
    get.mockRejectedValue(new Error("404"));

    renderPicker([USER_ID]);

    await waitFor(() => expect(get).toHaveBeenCalled());
    expect(screen.getByText(USER_ID)).toBeInTheDocument();
  });
});
