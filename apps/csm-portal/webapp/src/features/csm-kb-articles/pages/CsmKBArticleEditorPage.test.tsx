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
import { MemoryRouter, Route, Routes } from "react-router";
import type { KBArticle } from "@features/csm-kb-articles/types/csmKbArticles";

const createMutate = vi.fn();
const patchContentMutate = vi.fn();
const patchStateMutate = vi.fn();
let article: KBArticle | undefined;
const PRODUCT_KB = { id: "kb-1", title: "Product KB", active: true, createdOn: "", updatedOn: "" };
// The knowledge base list as the hook returns it; one active knowledge base unless a test says otherwise.
let kbList: { data: unknown; isSuccess: boolean; isError: boolean };

const mutation = (mutate: ReturnType<typeof vi.fn>) => ({ mutate, isPending: false, error: null });

// Stubbed to a plain textarea (same technique as LogTimeCardDialog.test.tsx),
// forwarding the one prop under test here.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
    ariaRequired,
  }: {
    value: string;
    onChange: (v: string) => void;
    ariaRequired?: boolean;
  }) => (
    <textarea
      aria-label="body-editor"
      aria-required={ariaRequired || undefined}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => vi.fn() }));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" } }),
}));
vi.mock("@features/csm-kb-articles/api/useGetKBArticle", () => ({
  useGetKBArticle: (id?: string) => ({
    data: id ? article : undefined,
    isLoading: false,
    isError: false,
    error: null,
  }),
}));
vi.mock("@features/csm-kb-articles/api/useGetKBArticleHistory", () => ({
  useGetKBArticleHistory: () => ({ data: { history: [] } }),
}));
vi.mock("@features/csm-kb-articles/api/useListKnowledgeBases", () => ({
  useListKnowledgeBases: () => kbList,
}));
vi.mock("@features/csm-kb-articles/api/useSearchTeams", () => ({
  useSearchTeams: () => ({ data: [] }),
}));
vi.mock("@features/csm-kb-articles/api/useCreateKBArticle", () => ({
  useCreateKBArticle: () => mutation(createMutate),
}));
vi.mock("@features/csm-kb-articles/api/usePatchKBArticleContent", () => ({
  usePatchKBArticleContent: () => mutation(patchContentMutate),
}));
vi.mock("@features/csm-kb-articles/api/usePatchKBArticleState", () => ({
  usePatchKBArticleState: () => mutation(patchStateMutate),
}));
vi.mock("@features/csm-kb-articles/api/useDeleteKBArticle", () => ({
  useDeleteKBArticle: () => mutation(vi.fn()),
}));
vi.mock("@features/csm-kb-articles/components/KBArticleHistoryPanel", () => ({
  default: () => null,
}));

import CsmKBArticleEditorPage from "@features/csm-kb-articles/pages/CsmKBArticleEditorPage";

const DRAFT: KBArticle = {
  id: "art-1",
  knowledgeBaseId: "kb-1",
  title: "How to rotate keys",
  body: "<p>Steps</p>",
  state: "draft",
  authorId: "user-1",
  reviewerId: null,
  sourceCaseId: null,
  rejectionComment: null,
  updatedBy: null,
  teamKey: null,
  createdOn: "2026-10-01T00:00:00Z",
  updatedOn: "2026-10-01T00:00:00Z",
  publishedOn: null,
  retiredOn: null,
};

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/knowledge/my-articles/new" element={<CsmKBArticleEditorPage />} />
        <Route path="/knowledge/my-articles/:id" element={<CsmKBArticleEditorPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

const bodyCaptionMark = (): Element | null =>
  document.querySelector("#kb-article-body-label .MuiFormLabel-asterisk");

describe("CsmKBArticleEditorPage — required fields", () => {
  beforeEach(() => {
    createMutate.mockReset();
    patchContentMutate.mockReset();
    patchStateMutate.mockReset();
    article = undefined;
    kbList = { data: { knowledgeBases: [PRODUCT_KB] }, isSuccess: true, isError: false };
  });

  it("marks Knowledge base, Title and Body required when creating", () => {
    renderAt("/knowledge/my-articles/new");

    expect(document.querySelector("#kb-select-label .MuiFormLabel-asterisk")).not.toBeNull();
    expect(screen.getByRole("textbox", { name: "Title" })).toBeRequired();
    expect(bodyCaptionMark()).not.toBeNull();
    expect(screen.getByLabelText("body-editor")).toHaveAttribute("aria-required", "true");
  });

  it("does not mark the knowledge base when there is none to pick, says why, and holds Save draft back", () => {
    kbList = { data: { knowledgeBases: [{ ...PRODUCT_KB, active: false }] }, isSuccess: true, isError: false };
    renderAt("/knowledge/my-articles/new");

    expect(screen.getByRole("combobox", { name: "Knowledge base" })).toHaveAttribute("aria-disabled", "true");
    expect(document.querySelector("#kb-select-label .MuiFormLabel-asterisk")).toBeNull();
    expect(screen.getByText("No knowledge base is taking new articles, so the article can't be saved.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
  });

  it("says so when the knowledge bases couldn't be loaded, without marking the empty select", () => {
    kbList = { data: undefined, isSuccess: false, isError: true };
    renderAt("/knowledge/my-articles/new");

    expect(document.querySelector("#kb-select-label .MuiFormLabel-asterisk")).toBeNull();
    expect(
      screen.getByText("Couldn't load the knowledge bases, so the article can't be saved yet. Try again later."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
  });

  it("says nothing about the knowledge bases while their list has not arrived (a query paused offline)", () => {
    // Paused: neither loading nor failed, and no data yet.
    kbList = { data: undefined, isSuccess: false, isError: false };
    renderAt("/knowledge/my-articles/new");

    expect(screen.queryByText(/No knowledge base is taking new articles/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Couldn't load the knowledge bases/)).not.toBeInTheDocument();
    expect(document.querySelector("#kb-select-label .MuiFormLabel-asterisk")).toBeNull();
    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
  });

  it("does not create an article with an empty body, and says why", () => {
    renderAt("/knowledge/my-articles/new");
    fireEvent.change(screen.getByRole("textbox", { name: "Title" }), {
      target: { value: "Rotating keys" },
    });

    // An editor typed in and emptied again still emits markup.
    fireEvent.change(screen.getByLabelText("body-editor"), { target: { value: "<p><br></p>" } });
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));
    expect(createMutate).not.toHaveBeenCalled();
    expect(screen.getByText("Body is required.")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("body-editor"), { target: { value: "<p>Steps</p>" } });
    expect(screen.queryByText("Body is required.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));
    expect(createMutate).toHaveBeenCalledWith(
      { knowledgeBaseId: "kb-1", title: "Rotating keys", body: "<p>Steps</p>", authorId: "user-1" },
      expect.anything(),
    );
  });

  it("creates an article whose body is an inline image only", () => {
    renderAt("/knowledge/my-articles/new");
    fireEvent.change(screen.getByRole("textbox", { name: "Title" }), {
      target: { value: "Key rotation diagram" },
    });
    const imageOnly = '<p><img src="data:image/png;base64,AAAA" alt=""></p>';
    fireEvent.change(screen.getByLabelText("body-editor"), { target: { value: imageOnly } });
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));

    expect(screen.queryByText("Body is required.")).not.toBeInTheDocument();
    expect(createMutate).toHaveBeenCalledWith(
      { knowledgeBaseId: "kb-1", title: "Key rotation diagram", body: imageOnly, authorId: "user-1" },
      expect.anything(),
    );
  });

  it("only needs a title when saving an existing draft: the body is not required there", () => {
    article = { ...DRAFT };
    renderAt("/knowledge/my-articles/art-1");

    expect(screen.getByRole("textbox", { name: "Title" })).toBeRequired();
    expect(bodyCaptionMark()).toBeNull();
    expect(screen.getByLabelText("body-editor")).not.toHaveAttribute("aria-required");

    fireEvent.change(screen.getByLabelText("body-editor"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(patchContentMutate).toHaveBeenCalledWith({ title: "How to rotate keys", body: "" });
  });

  it("does not mark the title of an article it cannot edit (pending review)", () => {
    article = { ...DRAFT, state: "pending_review" };
    renderAt("/knowledge/my-articles/art-1");

    expect(screen.getByRole("textbox", { name: "Title" })).not.toBeRequired();
  });
});
