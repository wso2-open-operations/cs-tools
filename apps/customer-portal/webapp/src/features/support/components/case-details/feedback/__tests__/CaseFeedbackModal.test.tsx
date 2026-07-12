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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CaseFeedbackModal from "../CaseFeedbackModal";

const EMOJIS = [
  {
    id: "e1",
    name: "Poor",
    value: "1",
    unselectedImage: "u1.png",
    selectedImage: "s1.png",
    chips: [{ id: "c1a", name: "Slow", value: "slow" }],
  },
  {
    id: "e2",
    name: "Great",
    value: "5",
    unselectedImage: "u2.png",
    selectedImage: "s2.png",
    chips: [
      { id: "c2a", name: "Fast", value: "fast" },
      { id: "c2b", name: "Helpful", value: "helpful" },
    ],
  },
];

vi.mock("@api/useGetMetadata", () => ({
  default: () => ({ data: { feedbackEmojies: EMOJIS }, isLoading: false }),
}));

const mutate = vi.fn();
vi.mock("@features/support/api/usePostCaseFeedback", () => ({
  usePostCaseFeedback: () => ({ isPending: false, mutate }),
}));

function renderModal(props: Partial<React.ComponentProps<typeof CaseFeedbackModal>> = {}) {
  const onClose = vi.fn();
  const onSubmitted = vi.fn();
  const onError = vi.fn();
  render(
    <ThemeProvider theme={createTheme()}>
      <CaseFeedbackModal
        open
        caseId="c1"
        onClose={onClose}
        onSubmitted={onSubmitted}
        onError={onError}
        {...props}
      />
    </ThemeProvider>,
  );
  return { onClose, onSubmitted, onError };
}

describe("CaseFeedbackModal", () => {
  beforeEach(() => {
    mutate.mockReset();
  });

  it("renders the feedback prompt with emoji options", () => {
    renderModal();
    expect(screen.getByText(/how was your support experience/i)).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Poor" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Great" })).toBeInTheDocument();
  });

  it("disables submit until an emoji is selected", () => {
    renderModal();
    const submit = screen.getByRole("button", { name: /submit feedback/i });
    expect(submit).toBeDisabled();
    fireEvent.click(screen.getByRole("radio", { name: "Great" }));
    expect(submit).toBeEnabled();
  });

  it("shows chips for the selected emoji and swaps them when the emoji changes", () => {
    renderModal();
    expect(screen.queryByText("Fast")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "Great" }));
    expect(screen.getByText("Fast")).toBeInTheDocument();
    expect(screen.getByText("Helpful")).toBeInTheDocument();
    expect(screen.queryByText("Slow")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "Poor" }));
    expect(screen.getByText("Slow")).toBeInTheDocument();
    expect(screen.queryByText("Fast")).not.toBeInTheDocument();
  });

  it("submits emojiId, chipIds and comment", () => {
    renderModal();
    fireEvent.click(screen.getByRole("radio", { name: "Great" }));
    fireEvent.click(screen.getByText("Fast"));
    fireEvent.change(screen.getByLabelText(/additional comments/i), {
      target: { value: "  nice support  " },
    });
    fireEvent.click(screen.getByRole("button", { name: /submit feedback/i }));

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0][0]).toEqual({
      emojiId: "e2",
      chipIds: ["c2a"],
      additionalComment: "nice support",
    });
  });

  it("skips without submitting", () => {
    const { onClose } = renderModal();
    fireEvent.click(screen.getByRole("button", { name: /skip/i }));
    expect(onClose).toHaveBeenCalled();
    expect(mutate).not.toHaveBeenCalled();
  });

  it("shows an inline error on 4xx responses", () => {
    mutate.mockImplementation((_payload, { onError }) => {
      const err = Object.assign(new Error("Feedback already submitted"), { status: 409 });
      onError(err);
    });
    renderModal();
    fireEvent.click(screen.getByRole("radio", { name: "Great" }));
    fireEvent.click(screen.getByRole("button", { name: /submit feedback/i }));
    expect(screen.getByText(/feedback already submitted/i)).toBeInTheDocument();
  });
});
