/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { pagingMember } from "../test/fixtures";
import PagingPhoneDialog, { PAGING_PHONE_HELPER } from "./PagingPhoneDialog";

const member = pagingMember({ membershipId: "a", name: "Jane Doe" });

function Host({ onSave = vi.fn(), mode = "edit" as "edit" | "remove", onRemove = vi.fn(), error }: {
  onSave?: (p: string) => void; mode?: "edit" | "remove"; onRemove?: () => void; error?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      {open ? (
        <PagingPhoneDialog
          mode={mode}
          member={{ ...member, pagingPhone: mode === "remove" ? { masked: "+94•••••123", setBy: "x", setAt: "y" } : null }}
          busy={false}
          error={error}
          onSave={onSave}
          onRemove={onRemove}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}

// By role and name: the label also carries the aria-hidden required asterisk.
const phoneInput = () => screen.getByRole("textbox", { name: "Mobile number (with country code)" });

function openDialog() {
  const opener = screen.getByRole("button", { name: "Open" });
  opener.focus();
  fireEvent.click(opener);
  return opener;
}

describe("PagingPhoneDialog", () => {
  it("is a labelled modal that takes focus, explains itself, and gives focus back on Escape", () => {
    render(<Host />);
    const opener = openDialog();
    const dialog = screen.getByRole("dialog", { name: "Add alert number for Jane Doe" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    const input = phoneInput();
    expect(input).toHaveFocus();
    expect(screen.getByText(PAGING_PHONE_HELPER)).toBeInTheDocument();
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("marks the number required, with the asterisk every required field has", () => {
    render(<Host />);
    openDialog();
    const input = phoneInput();
    expect(input).toBeRequired();
    const label = document.querySelector(`label[for="${input.id}"]`);
    expect(label?.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
  });

  it("keeps Tab inside the dialog", () => {
    render(<Host />);
    openDialog();
    const dialog = screen.getByRole("dialog");
    const save = screen.getByRole("button", { name: "Save" });
    save.focus();
    fireEvent.keyDown(dialog, { key: "Tab" });
    expect(phoneInput()).toHaveFocus();
    fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
    expect(save).toHaveFocus();
  });

  it("validates inline, announces the error and does not save a bad number", () => {
    const onSave = vi.fn();
    render(<Host onSave={onSave} />);
    openDialog();
    const input = phoneInput();
    fireEvent.change(input, { target: { value: "0771234567" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(/Start with \+ and the country code/);
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input.getAttribute("aria-describedby")).toContain(screen.getByRole("alert").id);

    fireEvent.change(input, { target: { value: "+94 77 123 4567" } });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith("+94771234567");
  });

  it("shows the server's refusal", () => {
    render(<Host error="That number was not accepted. Check it and try again." />);
    openDialog();
    expect(screen.getByRole("alert")).toHaveTextContent("That number was not accepted");
  });

  it("asks before removing", () => {
    const onRemove = vi.fn();
    render(<Host mode="remove" onRemove={onRemove} />);
    openDialog();
    expect(screen.getByRole("dialog", { name: "Remove alert number for Jane Doe?" })).toBeInTheDocument();
    expect(screen.getByText(/\+94•••••123/)).toBeInTheDocument();
    const remove = screen.getByRole("button", { name: "Remove" });
    expect(remove).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onRemove).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    openDialog();
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(onRemove).toHaveBeenCalledTimes(1);
  });
});
