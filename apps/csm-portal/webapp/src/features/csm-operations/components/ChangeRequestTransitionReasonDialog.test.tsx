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

import type { ComponentProps } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ChangeRequestTransitionReasonDialog from "@features/csm-operations/components/ChangeRequestTransitionReasonDialog";

function renderDialog(
  props: Partial<
    ComponentProps<typeof ChangeRequestTransitionReasonDialog>
  > = {},
): {
  onConfirm: ReturnType<typeof vi.fn>;
  onClose: ReturnType<typeof vi.fn>;
} {
  const onConfirm = vi.fn();
  const onClose = vi.fn();
  render(
    <ChangeRequestTransitionReasonDialog
      target="canceled"
      isSubmitting={false}
      onClose={onClose}
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return { onConfirm, onClose };
}

const reasonField = (): HTMLElement => screen.getByLabelText(/reason/i);

describe("ChangeRequestTransitionReasonDialog — reason is required", () => {
  it("disables the confirm action while the reason is empty", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
  });

  it("keeps the confirm action disabled for a whitespace-only reason", () => {
    renderDialog();
    fireEvent.change(reasonField(), { target: { value: "   \n  " } });
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
  });

  it("enables the confirm action once the reason has content", () => {
    renderDialog();
    fireEvent.change(reasonField(), { target: { value: "Superseded by CHG0009999." } });
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeEnabled();
  });

  it("passes the trimmed reason to onConfirm", () => {
    const { onConfirm } = renderDialog();
    fireEvent.change(reasonField(), {
      target: { value: "  Superseded by another change.  " },
    });
    fireEvent.click(screen.getByRole("button", { name: /cancel change/i }));
    expect(onConfirm).toHaveBeenCalledWith("Superseded by another change.");
  });
});

describe("ChangeRequestTransitionReasonDialog — per-target copy", () => {
  it("uses rollback wording and confirm label for the rollback target", () => {
    renderDialog({ target: "rollback" });
    expect(screen.getByRole("heading", { name: /roll back this change/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^roll back$/i })).toBeInTheDocument();
  });

  it("uses cancellation wording and confirm label for the canceled target", () => {
    renderDialog({ target: "canceled" });
    expect(
      screen.getByRole("heading", { name: /cancel this change request/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeInTheDocument();
  });

  it("still renders for a target it has no curated copy for", () => {
    renderDialog({ target: "awaiting_vendor" });
    expect(screen.getByRole("heading", { name: /awaiting vendor/i })).toBeInTheDocument();
    expect(reasonField()).toBeInTheDocument();
  });
});

describe("ChangeRequestTransitionReasonDialog — in-flight and error states", () => {
  it("disables both actions and the field while submitting", () => {
    renderDialog({ isSubmitting: true });
    expect(screen.getByRole("button", { name: /close/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /cancel change/i })).toBeDisabled();
    expect(reasonField()).toBeDisabled();
  });

  it("renders no alert by default", () => {
    renderDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces the caller's error message inline", () => {
    renderDialog({
      error: "Your reason was recorded as a comment, but the state did not change.",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      /recorded as a comment, but the state did not change/i,
    );
  });

  it("locks the reason field once it has already been recorded, so a retry can't post it twice", () => {
    renderDialog({ reasonRecorded: true });
    expect(reasonField()).toBeDisabled();
    expect(
      screen.getByText(/already recorded as a comment/i),
    ).toBeInTheDocument();
  });
});

describe("ChangeRequestTransitionReasonDialog — dismissal", () => {
  it("closes on Escape", () => {
    const { onClose } = renderDialog();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape", code: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("does not close on Escape while submitting", () => {
    const { onClose } = renderDialog({ isSubmitting: true });
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape", code: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});
