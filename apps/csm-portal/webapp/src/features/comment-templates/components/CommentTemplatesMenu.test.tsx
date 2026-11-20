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
import type { BeCommentTemplate } from "@api/backend/types";
import CommentTemplatesMenu from "./CommentTemplatesMenu";

const templates: BeCommentTemplate[] = [
  { id: "1", name: "My ack", body: "<p>We are looking into it.</p>", appliesTo: "COMMENT", shared: false, createdOn: "", updatedOn: "" },
  { id: "2", name: "My note", body: "<p>Checked the logs.</p>", appliesTo: "WORK_NOTE", shared: false, createdOn: "", updatedOn: "" },
  { id: "3", name: "Team closing", body: "<p>Closing this case.</p>", appliesTo: "BOTH", shared: true, createdOn: "", updatedOn: "" },
];

const portal = { isAdmin: false };
vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => portal }));
vi.mock("@features/comment-templates/api/useCommentTemplates", () => ({
  useGetCommentTemplates: () => ({ data: templates, isLoading: false, error: null }),
  useSaveCommentTemplate: () => ({ mutateAsync: vi.fn(), isPending: false, error: null }),
  useDeleteCommentTemplate: () => ({ mutate: vi.fn(), isPending: false, error: null }),
}));
// The dialogs' rich-text editor is not what these tests are about.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value }: { value?: string }) => <div data-testid="template-editor">{value}</div>,
}));

function openMenu() {
  fireEvent.click(screen.getByRole("button", { name: "Templates" }));
}

describe("CommentTemplatesMenu", () => {
  beforeEach(() => {
    portal.isAdmin = false;
  });

  it("offers only comment templates for a public reply, mine before shared", () => {
    render(<CommentTemplatesMenu internal={false} draftHtml="" onInsert={vi.fn()} />);
    openMenu();
    expect(screen.getByText("My ack")).toBeInTheDocument();
    expect(screen.getByText("Team closing")).toBeInTheDocument();
    expect(screen.queryByText("My note")).not.toBeInTheDocument();
    expect(screen.getByText("My templates")).toBeInTheDocument();
    expect(screen.getByText("Shared")).toBeInTheDocument();
  });

  it("offers work note templates in work-note mode", () => {
    render(<CommentTemplatesMenu internal draftHtml="" onInsert={vi.fn()} />);
    openMenu();
    expect(screen.getByText("My note")).toBeInTheDocument();
    expect(screen.queryByText("My ack")).not.toBeInTheDocument();
  });

  it("inserts the picked template's body", () => {
    const onInsert = vi.fn();
    render(<CommentTemplatesMenu internal={false} draftHtml="" onInsert={onInsert} />);
    openMenu();
    fireEvent.click(screen.getByText("My ack"));
    expect(onInsert).toHaveBeenCalledWith("<p>We are looking into it.</p>");
  });

  it("filters by the search box", () => {
    render(<CommentTemplatesMenu internal={false} draftHtml="" onInsert={vi.fn()} />);
    openMenu();
    fireEvent.change(screen.getByLabelText("Search templates"), { target: { value: "closing" } });
    expect(screen.getByText("Team closing")).toBeInTheDocument();
    expect(screen.queryByText("My ack")).not.toBeInTheDocument();
  });

  it("cannot save an empty draft as a template", () => {
    render(<CommentTemplatesMenu internal={false} draftHtml="<p><br></p>" onInsert={vi.fn()} />);
    openMenu();
    expect(screen.getByRole("menuitem", { name: /save draft as template/i })).toHaveAttribute("aria-disabled", "true");
  });

  it("offers sharing only to an admin", () => {
    render(<CommentTemplatesMenu internal={false} draftHtml="<p>Draft</p>" onInsert={vi.fn()} />);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /save draft as template/i }));
    expect(screen.getByTestId("template-editor")).toHaveTextContent("<p>Draft</p>");
    expect(screen.queryByLabelText(/share with everyone/i)).not.toBeInTheDocument();
  });

  it("lets an admin share a new template", () => {
    portal.isAdmin = true;
    render(<CommentTemplatesMenu internal={false} draftHtml="<p>Draft</p>" onInsert={vi.fn()} />);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /save draft as template/i }));
    expect(screen.getByLabelText(/share with everyone/i)).toBeInTheDocument();
  });

  it("shows edit and delete for shared templates only to an admin", () => {
    render(<CommentTemplatesMenu internal={false} draftHtml="" onInsert={vi.fn()} />);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /manage templates/i }));
    expect(screen.getByRole("button", { name: "Edit My ack" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit Team closing" })).not.toBeInTheDocument();
  });
});
