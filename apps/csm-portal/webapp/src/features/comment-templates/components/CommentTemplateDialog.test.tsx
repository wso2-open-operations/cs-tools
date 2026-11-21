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
import CommentTemplateDialog from "./CommentTemplateDialog";

const save = { mutateAsync: vi.fn(), isPending: false, error: null as Error | null };
vi.mock("@features/comment-templates/api/useCommentTemplates", () => ({
  useSaveCommentTemplate: () => save,
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value }: { value?: string }) => <div data-testid="template-editor">{value}</div>,
}));

const draft = { name: "Ack", body: "<p>Thanks.</p>", appliesTo: "COMMENT" as const, shared: false };

describe("CommentTemplateDialog", () => {
  beforeEach(() => {
    save.isPending = false;
    save.mutateAsync = vi.fn().mockResolvedValue({});
  });

  it("cannot be cancelled while its save is in flight, so it never closes a newer draft", () => {
    save.isPending = true;
    const onClose = vi.fn();
    render(<CommentTemplateDialog open initial={draft} canShare={false} onClose={onClose} />);
    const cancel = screen.getByRole("button", { name: "Cancel" });
    expect(cancel).toBeDisabled();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on Cancel when nothing is saving", () => {
    const onClose = vi.fn();
    render(<CommentTemplateDialog open initial={draft} canShare={false} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("refuses a draft with an image, which a template cannot keep", () => {
    render(
      <CommentTemplateDialog
        open
        initial={{ ...draft, body: '<p>See this</p><p><img src="blob:http://localhost/abc"></p>' }}
        canShare={false}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText(/templates can't hold images/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });
});
