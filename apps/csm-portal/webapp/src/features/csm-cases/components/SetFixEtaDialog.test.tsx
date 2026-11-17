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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import SetFixEtaDialog from "@features/csm-cases/components/SetFixEtaDialog";

// The Product field is a dropdown backed by the product catalogue, so every
// test that reveals it needs a real product list to pick from -- mocked
// directly rather than pulling in the real backend client (which requires
// window.config at module load, per this app's own testing convention for
// @api/backend/client-backed hooks).
const DEFAULT_PRODUCTS = {
  data: [
    { id: "prod-apim", name: "API Manager" },
    { id: "prod-is", name: "Identity Server" },
  ],
  isLoading: false,
};
const useSearchProductsMock = vi.fn(() => DEFAULT_PRODUCTS);
vi.mock("@features/csm-projects/api/useSearchProducts", () => ({
  useSearchProducts: () => useSearchProductsMock(),
}));

afterEach(() => {
  useSearchProductsMock.mockReturnValue(DEFAULT_PRODUCTS);
});

type DialogProps = Parameters<typeof SetFixEtaDialog>[0];

function makeProps(overrides: Partial<DialogProps> = {}): DialogProps {
  return {
    isSaving: false,
    onClose: vi.fn(),
    onSaveDates: vi.fn().mockResolvedValue(undefined),
    onPostComment: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

function renderDialog(overrides: Partial<DialogProps> = {}) {
  const props = makeProps(overrides);
  const utils = render(<SetFixEtaDialog {...props} />);
  return { props, ...utils };
}

const saveButton = (): HTMLElement =>
  screen.getByRole("button", { name: /^(save|post comment)$/i });
const commentField = (): HTMLTextAreaElement =>
  screen.getByLabelText(/comment to customer/i) as HTMLTextAreaElement;

function toggleShare(): void {
  fireEvent.click(
    screen.getByRole("switch", { name: /share fix eta with customer/i }),
  );
}

async function pickProduct(name = "API Manager"): Promise<void> {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: /product/i }));
  fireEvent.click(await screen.findByRole("option", { name }));
}

const TICKET = "https://github.com/wso2/product-apim/issues/1";

describe("SetFixEtaDialog — single combined save", () => {
  it("renders one Save button, disabled until at least one field is set", () => {
    renderDialog();
    expect(screen.getAllByRole("button", { name: /^save$/i })).toHaveLength(1);
    expect(saveButton()).toBeDisabled();
  });

  it("enables Save once seeded with existing estimates", () => {
    renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-18",
    });
    expect(saveButton()).toBeEnabled();
  });

  it("saves all three seeded estimates together in one dates payload, then closes", async () => {
    const { props } = renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-18",
    });
    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalledTimes(1));
    expect(props.onSaveDates).toHaveBeenCalledTimes(1);
    expect(props.onSaveDates).toHaveBeenCalledWith({
      bestCaseFixEta: "2099-06-16",
      mostLikelyFixEta: "2099-06-17",
      worstCaseFixEta: "2099-06-18",
    });
    // Share is off: no comment step at all.
    expect(props.onPostComment).not.toHaveBeenCalled();
  });

  it("saves just one seeded estimate, independent of the other two being empty", async () => {
    const { props } = renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.onSaveDates).toHaveBeenCalledWith({
      bestCaseFixEta: "2099-06-16",
    });
  });

  it("stays open and does not post when saving the estimates fails", async () => {
    const onSaveDates = vi.fn().mockRejectedValue(new Error("boom"));
    const { props } = renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      onSaveDates,
    });
    toggleShare();
    await pickProduct();
    fireEvent.click(saveButton());
    await waitFor(() => expect(onSaveDates).toHaveBeenCalledTimes(1));
    // Back to idle (Save enabled again), still the plain Save label.
    await waitFor(() => expect(saveButton()).toBeEnabled());
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
    expect(props.onPostComment).not.toHaveBeenCalled();
    expect(props.onClose).not.toHaveBeenCalled();
    // Nothing was saved, so the dates are still editable and no failure banner.
    expect(
      screen.queryByText(/estimates were saved, but the comment/i),
    ).not.toBeInTheDocument();
  });

  it("calls onClose on Close without saving or posting anything", () => {
    const { props } = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: /^close$/i }));
    expect(props.onClose).toHaveBeenCalled();
    expect(props.onSaveDates).not.toHaveBeenCalled();
    expect(props.onPostComment).not.toHaveBeenCalled();
  });

  it("blocks a past fix ETA date and shows an inline error", () => {
    const { props } = renderDialog({ currentBestCaseFixEta: "2020-01-01" });
    expect(saveButton()).toBeDisabled();
    expect(screen.getByText(/this date is in the past/i)).toBeInTheDocument();
    fireEvent.click(saveButton());
    expect(props.onSaveDates).not.toHaveBeenCalled();
  });

  it("blocks Save when the three estimates are out of best <= most likely <= worst case order", () => {
    const { props } = renderDialog({
      currentBestCaseFixEta: "2099-06-18",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-16",
    });
    expect(saveButton()).toBeDisabled();
    expect(
      screen.getByText(/best case must not be later than most likely/i),
    ).toBeInTheDocument();
    fireEvent.click(saveButton());
    expect(props.onSaveDates).not.toHaveBeenCalled();
  });

  it("allows the estimates in correct best <= most likely <= worst case order", () => {
    renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-18",
    });
    expect(saveButton()).toBeEnabled();
  });
});

describe("SetFixEtaDialog — sharing with the customer", () => {
  it("reveals product/ticket/comment fields and enables Save once a product is picked", async () => {
    renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    expect(screen.getByLabelText(/public ticket \(optional\)/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/public ticket/i)).not.toBeRequired();
    expect(commentField()).toBeInTheDocument();
    // Product is still required while the text is template-driven.
    expect(saveButton()).toBeDisabled();
    expect(
      screen.getByText(/product is required to share with the customer/i),
    ).toBeInTheDocument();

    await pickProduct();
    expect(saveButton()).toBeEnabled();
  });

  it("saves the dates (no comment fields), then posts the pre-filled text, then closes", async () => {
    const order: string[] = [];
    const onSaveDates = vi.fn().mockImplementation(async () => {
      order.push("dates");
    });
    const onPostComment = vi.fn().mockImplementation(async () => {
      order.push("comment");
    });
    const { props } = renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-18",
      onSaveDates,
      onPostComment,
    });
    toggleShare();
    await pickProduct();
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: `  ${TICKET}  ` },
    });
    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalledTimes(1));

    expect(order).toEqual(["dates", "comment"]);
    // The PATCH carries only the estimates: no addPublicComment/product/ticket.
    expect(onSaveDates).toHaveBeenCalledWith({
      bestCaseFixEta: "2099-06-16",
      mostLikelyFixEta: "2099-06-17",
      worstCaseFixEta: "2099-06-18",
    });
    expect(onPostComment).toHaveBeenCalledTimes(1);
    expect(onPostComment).toHaveBeenCalledWith(
      "ETA information for the fix will be as follows.\n\n" +
        "Product: API Manager\n" +
        `Public git issue: ${TICKET}\n` +
        "Best Case Estimate: 2099-06-16\n" +
        "Most Likely Estimate: 2099-06-17\n" +
        "Worst Case Estimate: 2099-06-18",
    );
  });

  it("omits the ticket line when the ticket is blank or whitespace", async () => {
    const { props } = renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    await pickProduct();
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: "   " },
    });
    expect(commentField().value).not.toMatch(/Public git issue/);
    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.onPostComment).toHaveBeenCalledWith(
      "ETA information for the fix will be as follows.\n\nProduct: API Manager\nBest Case Estimate: 2099-06-16",
    );
  });

  it("blocks sharing with customer when no fix ETA is set, even with product/ticket filled in", async () => {
    renderDialog();
    toggleShare();
    await pickProduct();
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: TICKET },
    });
    expect(saveButton()).toBeDisabled();
    expect(
      screen.getByText(/pick at least one fix eta to share with the customer/i),
    ).toBeInTheDocument();
  });

  it("rejects a public ticket that isn't a GitHub issue/PR URL", async () => {
    renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    await pickProduct();
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: "not-a-url" },
    });
    expect(saveButton()).toBeDisabled();
    expect(
      screen.getAllByText(/must be a github issue or pull request url/i).length,
    ).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: "https://github.com/wso2/product-apim/issues/42" },
    });
    expect(saveButton()).toBeEnabled();
  });

  // CodeRabbit catch: a product selected before a catalogue refetch removed
  // its entry used to still read as "set" (productId alone), so Save stayed
  // enabled and the raw product UUID fell through into the customer-visible
  // comment instead of a real name. hasProduct is derived from a live match in
  // the fetched list, so a mid-session refetch that drops the selected product
  // blocks Save instead.
  it("blocks Save, rather than falling back to the raw product id, when the selected product disappears from a refetched catalogue", async () => {
    const props = makeProps({ currentBestCaseFixEta: "2099-06-16" });
    const { rerender } = render(<SetFixEtaDialog {...props} />);
    toggleShare();
    await pickProduct();
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: TICKET },
    });
    expect(saveButton()).toBeEnabled();

    // Simulate a refetch, in the same mounted dialog, that drops the
    // previously-selected product ("prod-apim") from the catalogue.
    useSearchProductsMock.mockReturnValue({
      data: [{ id: "prod-is", name: "Identity Server" }],
      isLoading: false,
    });
    rerender(<SetFixEtaDialog {...props} />);

    expect(saveButton()).toBeDisabled();
    expect(
      screen.getByText(/product is required to share with the customer/i),
    ).toBeInTheDocument();
    fireEvent.click(saveButton());
    expect(props.onSaveDates).not.toHaveBeenCalled();
  });

  // Sharing posts through the same comment endpoint as the case's own
  // public-reply composer, gated by the identical work_in_progress + ongoing +
  // assignee rule -- a share attempt that doesn't satisfy it is guaranteed to
  // fail after the ETA has already saved, so it is caught here instead.
  describe("gated by publicCommentDisabledReason", () => {
    const GATE_REASON =
      "Replies are locked until the case is resumed and assigned to you.";

    it("disables the share toggle and shows the reason when sharing is unavailable", () => {
      renderDialog({
        publicCommentDisabledReason: GATE_REASON,
        currentBestCaseFixEta: "2099-06-16",
      });
      expect(
        screen.getByRole("switch", { name: /share fix eta with customer/i }),
      ).toBeDisabled();
      expect(screen.getByText(GATE_REASON)).toBeInTheDocument();
      // The ETA itself is unaffected -- Save stays enabled on the seeded estimate.
      expect(saveButton()).toBeEnabled();
    });

    it("leaves the share toggle enabled and shows no reason when sharing is available", () => {
      renderDialog({
        publicCommentDisabledReason: null,
        currentBestCaseFixEta: "2099-06-16",
      });
      expect(
        screen.getByRole("switch", { name: /share fix eta with customer/i }),
      ).toBeEnabled();
      expect(screen.queryByText(GATE_REASON)).not.toBeInTheDocument();
    });
  });
});

describe("SetFixEtaDialog — editable comment", () => {
  it("regenerates the text from Product / ticket / the three dates until it is edited", async () => {
    renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      currentMostLikelyFixEta: "2099-06-17",
      currentWorstCaseFixEta: "2099-06-18",
    });
    toggleShare();
    expect(commentField().value).toBe(
      "ETA information for the fix will be as follows.\n\n" +
        "Best Case Estimate: 2099-06-16\n" +
        "Most Likely Estimate: 2099-06-17\n" +
        "Worst Case Estimate: 2099-06-18",
    );

    await pickProduct("Identity Server");
    expect(commentField().value).toContain("Product: Identity Server");

    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: TICKET },
    });
    expect(commentField().value).toContain(`Public git issue: ${TICKET}`);

    // Switching product re-renders the line.
    await pickProduct("API Manager");
    expect(commentField().value).toContain("Product: API Manager");
    expect(commentField().value).not.toContain("Identity Server");
    expect(
      screen.queryByRole("button", { name: /reset to template/i }),
    ).not.toBeInTheDocument();
  });

  it("stops regenerating once edited by hand, and posts the edited text", async () => {
    const { props } = renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    await pickProduct();
    fireEvent.change(commentField(), {
      target: { value: "Our analysis will be done by:\n2099-06-16" },
    });

    // Later input changes no longer overwrite the engineer's text.
    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: TICKET },
    });
    await pickProduct("Identity Server");
    expect(commentField().value).toBe("Our analysis will be done by:\n2099-06-16");
    expect(
      screen.getByRole("button", { name: /reset to template/i }),
    ).toBeInTheDocument();

    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.onPostComment).toHaveBeenCalledWith(
      "Our analysis will be done by:\n2099-06-16",
    );
  });

  it("Reset to template restores the generated text and resumes regeneration", async () => {
    renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    await pickProduct();
    fireEvent.change(commentField(), { target: { value: "custom" } });
    fireEvent.click(screen.getByRole("button", { name: /reset to template/i }));

    expect(commentField().value).toBe(
      "ETA information for the fix will be as follows.\n\nProduct: API Manager\nBest Case Estimate: 2099-06-16",
    );
    expect(
      screen.queryByRole("button", { name: /reset to template/i }),
    ).not.toBeInTheDocument();
    // Regeneration is live again.
    await pickProduct("Identity Server");
    expect(commentField().value).toContain("Product: Identity Server");
  });

  it("uses the report-analysis opening line when Report analysis is chosen, keeping Fix as the default", async () => {
    const { props } = renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    expect(screen.getByRole("radio", { name: "Fix" })).toBeChecked();
    expect(commentField().value).toMatch(
      /^ETA information for the fix will be as follows\./,
    );

    fireEvent.click(screen.getByRole("radio", { name: "Report analysis" }));
    expect(commentField().value).toMatch(
      /^ETA information for the report analysis will be as follows\./,
    );
    expect(commentField().value).not.toMatch(/for the fix/);

    await pickProduct();
    fireEvent.click(saveButton());
    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.onPostComment).toHaveBeenCalledWith(
      "ETA information for the report analysis will be as follows.\n\nProduct: API Manager\nBest Case Estimate: 2099-06-16",
    );
  });

  it("does not let the case-type choice overwrite a hand-edited text", () => {
    renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    fireEvent.change(commentField(), { target: { value: "custom" } });
    fireEvent.click(screen.getByRole("radio", { name: "Report analysis" }));
    expect(commentField().value).toBe("custom");
  });

  it("no longer requires Product or a valid ticket once the text is hand-edited, but requires non-blank text", () => {
    renderDialog({ currentBestCaseFixEta: "2099-06-16" });
    toggleShare();
    expect(saveButton()).toBeDisabled(); // template-driven, no product yet

    fireEvent.change(commentField(), { target: { value: "custom" } });
    expect(saveButton()).toBeEnabled();

    fireEvent.change(screen.getByLabelText(/public ticket/i), {
      target: { value: "not-a-url" },
    });
    expect(saveButton()).toBeEnabled();

    fireEvent.change(commentField(), { target: { value: "  \n " } });
    expect(saveButton()).toBeDisabled();
    expect(
      screen.getByText(/write the comment to share with the customer/i),
    ).toBeInTheDocument();
  });
});

describe("SetFixEtaDialog — comment post fails after the estimates saved", () => {
  async function setupFailure() {
    const onPostComment = vi
      .fn()
      .mockRejectedValueOnce(new Error("network error"))
      .mockResolvedValue(undefined);
    const utils = renderDialog({
      currentBestCaseFixEta: "2099-06-16",
      onPostComment,
    });
    toggleShare();
    await pickProduct();
    fireEvent.change(commentField(), { target: { value: "My edited text" } });
    fireEvent.click(saveButton());
    await screen.findByText(/estimates were saved, but the comment/i);
    return { ...utils, onPostComment };
  }

  it("stays open, says the ETAs were saved, and keeps the text", async () => {
    const { props } = await setupFailure();
    expect(props.onClose).not.toHaveBeenCalled();
    expect(props.onSaveDates).toHaveBeenCalledTimes(1);
    expect(commentField().value).toBe("My edited text");
    // Saved estimates and the share choice are locked; Save became a retry.
    expect(
      screen.getByRole("switch", { name: /share fix eta with customer/i }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: /^post comment$/i })).toBeEnabled();
    expect(screen.queryByRole("button", { name: /^save$/i })).not.toBeInTheDocument();
  });

  it("retries only the comment (never re-saves the dates) and closes on success", async () => {
    const { props, onPostComment } = await setupFailure();
    fireEvent.change(commentField(), { target: { value: "My fixed text" } });
    fireEvent.click(screen.getByRole("button", { name: /^post comment$/i }));
    await waitFor(() => expect(props.onClose).toHaveBeenCalledTimes(1));
    expect(props.onSaveDates).toHaveBeenCalledTimes(1);
    expect(onPostComment).toHaveBeenCalledTimes(2);
    expect(onPostComment).toHaveBeenLastCalledWith("My fixed text");
  });

  it("Close leaves without posting again", async () => {
    const { props, onPostComment } = await setupFailure();
    fireEvent.click(screen.getByRole("button", { name: /^close$/i }));
    expect(props.onClose).toHaveBeenCalledTimes(1);
    expect(onPostComment).toHaveBeenCalledTimes(1);
  });
});
