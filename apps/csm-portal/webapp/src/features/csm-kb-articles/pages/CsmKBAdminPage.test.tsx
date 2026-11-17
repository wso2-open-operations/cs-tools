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

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const createKbMutate = vi.fn();
const renameMutate = vi.fn();
const addUserMutate = vi.fn();
const postMock = vi.fn();

const mutation = (mutate: ReturnType<typeof vi.fn> = vi.fn()) => ({
  mutate,
  isPending: false,
  isError: false,
  error: null,
});

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@features/csm-kb-articles/api/useListKnowledgeBases", () => ({
  useListKnowledgeBases: () => ({
    data: {
      knowledgeBases: [
        { id: "kb-1", title: "Product KB", active: true, createdOn: "", updatedOn: "" },
      ],
    },
    isLoading: false,
  }),
}));
vi.mock("@features/csm-kb-articles/api/useCreateKnowledgeBase", () => ({
  useCreateKnowledgeBase: () => mutation(createKbMutate),
}));
vi.mock("@features/csm-projects/api/useSearchProducts", () => ({
  useSearchProducts: () => ({ data: [] }),
}));
vi.mock("@features/csm-kb-articles/api/useUpdateKnowledgeBaseName", () => ({
  useUpdateKnowledgeBaseName: () => mutation(renameMutate),
}));
vi.mock("@features/csm-kb-articles/api/useSetKnowledgeBaseActive", () => ({
  useSetKnowledgeBaseActive: () => mutation(),
}));
vi.mock("@features/csm-kb-articles/api/useSearchKBManagerUsers", () => ({
  useSearchKBManagerUsers: () => ({ data: { managers: [] } }),
}));
vi.mock("@features/csm-kb-articles/api/useCreateKBManagerUser", () => ({
  useCreateKBManagerUser: () => mutation(addUserMutate),
}));
vi.mock("@features/csm-kb-articles/api/useDeleteKBManagerUser", () => ({
  useDeleteKBManagerUser: () => mutation(),
}));
vi.mock("@features/csm-kb-articles/api/useSearchKBManagerGroups", () => ({
  useSearchKBManagerGroups: () => ({ data: { managers: [] } }),
}));
vi.mock("@features/csm-kb-articles/api/useCreateKBManagerGroup", () => ({
  useCreateKBManagerGroup: () => mutation(),
}));
vi.mock("@features/csm-kb-articles/api/useDeleteKBManagerGroup", () => ({
  useDeleteKBManagerGroup: () => mutation(),
}));
vi.mock("@features/csm-kb-articles/api/useUsersByIds", () => ({
  useUsersByIds: () => ({ data: new Map() }),
}));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));

import CsmKBAdminPage from "@features/csm-kb-articles/pages/CsmKBAdminPage";

describe("CsmKBAdminPage — required names", () => {
  beforeEach(() => {
    createKbMutate.mockReset();
    renameMutate.mockReset();
    addUserMutate.mockReset();
  });

  it("marks the new knowledge base's name required and keeps Create disabled until it is given", () => {
    render(<CsmKBAdminPage />);
    fireEvent.click(screen.getByRole("button", { name: "Create new" }));

    const name = screen.getByRole("textbox", { name: "Knowledge base name" });
    expect(name).toBeRequired();
    const create = screen.getByRole("button", { name: "Create" });
    expect(create).toBeDisabled();

    fireEvent.change(name, { target: { value: "  " } });
    expect(create).toBeDisabled();
    fireEvent.change(name, { target: { value: "Integration KB" } });
    expect(create).toBeEnabled();
    fireEvent.click(create);
    expect(createKbMutate).toHaveBeenCalledWith({ name: "Integration KB" }, expect.anything());
  });

  it("marks the rename field required and refuses a blank name", () => {
    render(<CsmKBAdminPage />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));

    const name = screen.getByRole("textbox", { name: "Name" });
    expect(name).toBeRequired();
    fireEvent.change(name, { target: { value: "   " } });
    expect(screen.getByRole("button", { name: "Save name" })).toBeDisabled();
  });

  it("keeps the approver-by-email Add disabled until an email is typed, without marking the field", () => {
    render(<CsmKBAdminPage />);
    fireEvent.click(screen.getByRole("button", { name: "Manage approvers" }));

    const email = screen.getByRole("textbox", { name: "Add approver by email" });
    expect(email).not.toBeRequired();
    const [addByEmail] = screen.getAllByRole("button", { name: "Add" });
    expect(addByEmail).toBeDisabled();
    fireEvent.change(email, { target: { value: "lead@example.test" } });
    expect(addByEmail).toBeEnabled();
  });
});
