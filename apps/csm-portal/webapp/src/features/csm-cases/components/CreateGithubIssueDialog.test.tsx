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

import type React from "react";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { CreateGithubIssueDialog } from "@features/csm-cases/components/CreateGithubIssueDialog";
import { useGetProductRepoMapping } from "@features/csm-cases/api/useGetProductRepoMapping";

// CreateGithubIssueDialog consumes this hook directly; mock the hook module
// itself (per this app's testing convention — mock the hook when testing a
// component that just consumes an already-built hook) rather than the
// backend client it wraps. Product-name matching lives in the entity service.
vi.mock("@features/csm-cases/api/useGetProductRepoMapping", () => ({
  useGetProductRepoMapping: vi.fn(),
}));

const mockUseGetProductRepoMapping = vi.mocked(useGetProductRepoMapping);

const MAPPING_FIXTURE = {
  productName: "Alpha",
  owner: "example-org",
  repository: "example-repo",
  githubLabel: "Alpha",
};

beforeEach(() => {
  mockUseGetProductRepoMapping.mockReturnValue({
    data: MAPPING_FIXTURE,
    isLoading: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any);
});

/** Opens the dropdown named `comboboxName` and chooses `option`. */
function pick(comboboxName: RegExp, option: RegExp | string): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: comboboxName }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

/** Fills the two required text fields. */
function fillSubjectAndDescription(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Token issuance is slow" },
  });
  fireEvent.change(screen.getByLabelText(/^description/i), {
    target: { value: "Latency spiked after the last deploy." },
  });
}

const JUSTIFICATION = "Customer go-live is blocked on Friday";

/** Types the P0/P1 justification and ticks Approved by CS Leadership. */
function justifyAndApprove(): void {
  fireEvent.change(screen.getByRole("textbox", { name: /priority justification/i }), {
    target: { value: JUSTIFICATION },
  });
  fireEvent.click(approvalBox());
}

/** The Approved by CS Leadership tick box. */
function approvalBox(): HTMLElement {
  return screen.getByRole("checkbox", { name: /approved by cs leadership/i });
}

/** Fills Subject, Description, Priority P1, its justification and the leadership approval. */
function fillRequiredFields(): void {
  fillSubjectAndDescription();
  pick(/^priority$/i, /P1 – High Priority/);
  justifyAndApprove();
}

/** Renders the dialog open with a resolved repository mapping; `props` override the defaults. */
function renderDialog(
  props: Partial<React.ComponentProps<typeof CreateGithubIssueDialog>> = {},
) {
  return render(
    <CreateGithubIssueDialog
      open
      productName="Alpha"
      submitting={false}
      error={null}
      onClose={() => {}}
      onSubmit={() => {}}
      {...props}
    />,
  );
}

/** The form's "Raise issue…" button. */
function createButton(): HTMLElement {
  return screen.getByRole("button", { name: /^raise issue…$/i });
}

/** Clicks "Raise issue…", then "Raise issue" on the confirm step. */
function submit(): void {
  fireEvent.click(createButton());
  fireEvent.click(screen.getByRole("button", { name: /^raise issue$/i }));
}

describe("CreateGithubIssueDialog — fields", () => {
  it("has no Type field and starts with an empty Subject and Description", () => {
    renderDialog();
    expect(screen.queryByRole("combobox", { name: /^type/i })).not.toBeInTheDocument();
    expect(screen.getByLabelText(/subject/i)).toHaveValue("");
    expect(screen.getByLabelText(/^description/i)).toHaveValue("");
  });

  it("shows the fields in the agreed order", () => {
    renderDialog();
    pick(/^priority$/i, /P1 – High Priority/);
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    const labels = [
      "Subject",
      "Description",
      "Reproducing Steps",
      "Priority",
      "Priority Justification",
      "Approved by CS Leadership",
      "Update Level",
      "Public Git Issue or Security Internal JIRA",
      "Related Internal Issue(s)",
      "Hotfix Required",
      "Hotfix Reason",
      "Regression",
      "Migration",
    ];
    const text = document.body.textContent ?? "";
    const positions = labels.map((l) => text.indexOf(l));
    positions.forEach((p, i) => expect(p, labels[i]).toBeGreaterThanOrEqual(0));
    expect([...positions].sort((a, b) => a - b)).toEqual(positions);
  });

  it("leaves Reproducing Steps, Update Level and Related Internal Issue(s) optional", () => {
    renderDialog();
    expect(screen.getByLabelText(/reproducing steps/i)).not.toBeRequired();
    expect(screen.getByLabelText(/update level/i)).not.toBeRequired();
    expect(screen.getByLabelText(/public git issue/i)).not.toBeRequired();
    expect(screen.getByLabelText(/related internal issue/i)).not.toBeRequired();
  });
});

describe("CreateGithubIssueDialog — required fields gate submission", () => {
  it("disables Raise issue until Subject, Description and Priority are filled", () => {
    renderDialog();
    expect(createButton()).toBeDisabled();
    fillSubjectAndDescription();
    expect(createButton()).toBeDisabled();
    pick(/^priority$/i, /P3 – Low Priority/);
    expect(createButton()).toBeEnabled();
  });
});

describe("CreateGithubIssueDialog — priorities (Internal Issue Handling Policy)", () => {
  it("offers P0 to P3 in order", () => {
    renderDialog();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^priority$/i }));
    const names = screen.getAllByRole("option").map((o) => o.textContent);
    expect(names).toEqual([
      "-- Select --",
      "P0 – Immediate Attention",
      "P1 – High Priority",
      "P2 – Medium Priority",
      "P3 – Low Priority",
    ]);
  });

  it("shows the chosen priority's SLA under the field", () => {
    renderDialog();
    expect(screen.queryByText(/SLA:/)).not.toBeInTheDocument();
    pick(/^priority$/i, /P0 – Immediate Attention/);
    expect(screen.getByText(/SLA: 1-hour response, 24x7/)).toBeInTheDocument();
    pick(/^priority$/i, /P3 – Low Priority/);
    expect(screen.getByText(/1-day acknowledgement and 5-day resolution, 12x5 IST/)).toBeInTheDocument();
  });

  it("files each priority with the policy's GitHub label", () => {
    for (const [option, label] of [
      [/P0 – Immediate Attention/, "Priority/Immediate(P0)"],
      [/P1 – High Priority/, "Priority/High(P1)"],
      [/P2 – Medium Priority/, "Priority/Medium(P2)"],
      [/P3 – Low Priority/, "Priority/Low(P3)"],
    ] as const) {
      const onSubmit = vi.fn();
      const { unmount } = renderDialog({ onSubmit });
      fillSubjectAndDescription();
      pick(/^priority$/i, option);
      if (label.endsWith("(P0)") || label.endsWith("(P1)")) justifyAndApprove();
      submit();
      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ priorityLevel: label }));
      unmount();
    }
  });
});

describe("CreateGithubIssueDialog — P0/P1 justification and CS leadership approval", () => {
  it("are hidden for P2 and P3 and until a priority is chosen", () => {
    renderDialog();
    const absent = () => {
      expect(screen.queryByRole("textbox", { name: /priority justification/i })).not.toBeInTheDocument();
      expect(screen.queryByRole("checkbox", { name: /approved by cs leadership/i })).not.toBeInTheDocument();
    };
    absent();
    pick(/^priority$/i, /P2 – Medium Priority/);
    absent();
    pick(/^priority$/i, /P3 – Low Priority/);
    absent();
  });

  it.each([[/P0 – Immediate Attention/], [/P1 – High Priority/]])(
    "for %s, need a free-text justification and the approval tick before Create is enabled",
    (option) => {
      renderDialog();
      fillSubjectAndDescription();
      pick(/^priority$/i, option);
      const justification = screen.getByRole("textbox", { name: /priority justification/i });
      expect(justification).toBeRequired();
      expect(approvalBox()).not.toBeChecked();
      expect(createButton()).toBeDisabled();

      fireEvent.change(justification, { target: { value: "   " } });
      fireEvent.click(approvalBox());
      // A blank justification is not one.
      expect(createButton()).toBeDisabled();

      fireEvent.change(justification, { target: { value: JUSTIFICATION } });
      expect(createButton()).toBeEnabled();

      fireEvent.click(approvalBox());
      expect(createButton()).toBeDisabled();
    },
  );

  it("says the approval is required by CS leadership review", () => {
    renderDialog();
    pick(/^priority$/i, /P0 – Immediate Attention/);
    expect(approvalBox()).toHaveAccessibleDescription(
      "CS leadership review is required before raising a P0 or P1 internal issue.",
    );
  });

  it("clears the approval when the priority changes, but keeps the justification text", () => {
    renderDialog();
    fillRequiredFields();
    expect(createButton()).toBeEnabled();
    pick(/^priority$/i, /P0 – Immediate Attention/);
    expect(approvalBox()).not.toBeChecked();
    expect(screen.getByRole("textbox", { name: /priority justification/i })).toHaveValue(JUSTIFICATION);
    expect(createButton()).toBeDisabled();
    fireEvent.click(approvalBox());
    expect(createButton()).toBeEnabled();
  });

  it("does not send the justification or the approval once the priority is P2 or P3", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillRequiredFields();
    pick(/^priority$/i, /P2 – Medium Priority/);
    submit();
    const payload = onSubmit.mock.calls[0][0];
    expect(payload).not.toHaveProperty("priorityJustification");
    expect(Object.keys(payload).some((k) => /approv|leadership/i.test(k))).toBe(false);
  });

  it("sends the trimmed justification, and never the approval tick", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillSubjectAndDescription();
    pick(/^priority$/i, /P0 – Immediate Attention/);
    fireEvent.change(screen.getByRole("textbox", { name: /priority justification/i }), {
      target: { value: `  ${JUSTIFICATION}  ` },
    });
    fireEvent.click(approvalBox());
    submit();
    const payload = onSubmit.mock.calls[0][0];
    expect(payload.priorityJustification).toBe(JUSTIFICATION);
    expect(Object.keys(payload).some((k) => /approv|leadership/i.test(k))).toBe(false);
  });
});

describe("CreateGithubIssueDialog — Hotfix Reason", () => {
  it("is shown and required only while Hotfix Required is on", () => {
    renderDialog();
    fillRequiredFields();
    expect(screen.queryByRole("combobox", { name: /hotfix reason/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    expect(createButton()).toBeDisabled();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /hotfix reason/i }));
    for (const o of [
      "Customer-Requested Hotfix",
      "Critical Production Bug / Downtime",
      "Defect Blocking Operations / Development",
      "Unable to Upgrade to Latest U2",
    ]) {
      expect(screen.getByRole("option", { name: o })).toBeInTheDocument();
    }
    fireEvent.click(screen.getByRole("option", { name: "Customer-Requested Hotfix" }));
    expect(createButton()).toBeEnabled();
  });

  it("drops the reason when Hotfix Required is switched off again", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillRequiredFields();
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    pick(/hotfix reason/i, "Customer-Requested Hotfix");
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    submit();
    const payload = onSubmit.mock.calls[0][0];
    expect(payload).not.toHaveProperty("hotFixRequired");
    expect(payload).not.toHaveProperty("hotFixReason");
  });
});

describe("CreateGithubIssueDialog — Migration", () => {
  it("is off by default", () => {
    renderDialog();
    expect(screen.getByRole("switch", { name: /migration/i })).not.toBeChecked();
  });

  it("is on by default for a migration engagement and sends reason migration", () => {
    const onSubmit = vi.fn();
    renderDialog({ defaultMigration: true, onSubmit });
    expect(screen.getByRole("switch", { name: /migration/i })).toBeChecked();
    fillRequiredFields();
    submit();
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ reason: "migration" }));
  });
});

describe("CreateGithubIssueDialog — payload", () => {
  it("files a Discussion with every field set", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit, onboardingInProgress: true });
    fillRequiredFields();
    fireEvent.change(screen.getByLabelText(/reproducing steps/i), {
      target: { value: "1. Log in\n2. Request a token" },
    });
    fireEvent.change(screen.getByLabelText(/update level/i), { target: { value: "7.1.0.42" } });
    fireEvent.change(screen.getByLabelText(/public git issue/i), {
      target: { value: "https://github.com/example/example/issues/1" },
    });
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    pick(/hotfix reason/i, "Unable to Upgrade to Latest U2");
    fireEvent.click(screen.getByRole("switch", { name: /regression/i }));
    fireEvent.change(screen.getByLabelText(/related internal issue/i), {
      target: { value: "example/internal#12" },
    });
    submit();
    expect(onSubmit).toHaveBeenCalledWith({
      reason: "default",
      title: "Token issuance is slow",
      description: "Latency spiked after the last deploy.",
      issueTypeLabel: "Type/Discussion",
      priorityLevel: "Priority/High(P1)",
      priorityJustification: JUSTIFICATION,
      reproducingSteps: "1. Log in\n2. Request a token",
      updateLevel: "7.1.0.42",
      publicIssueUrl: "https://github.com/example/example/issues/1",
      onboardingInProgress: true,
      hotFixRequired: true,
      hotFixReason: "Unable to Upgrade to Latest U2",
      regression: true,
      relatedInternalIssues: "example/internal#12",
      repoOverride: { owner: "example-org", repo: "example-repo" },
    });
  });

  it("omits the optional fields when they are left empty", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillSubjectAndDescription();
    pick(/^priority$/i, /P3 – Low Priority/);
    submit();
    expect(onSubmit).toHaveBeenCalledWith({
      reason: "default",
      title: "Token issuance is slow",
      description: "Latency spiked after the last deploy.",
      issueTypeLabel: "Type/Discussion",
      priorityLevel: "Priority/Low(P3)",
      repoOverride: { owner: "example-org", repo: "example-repo" },
    });
  });

  it("hides Update Level and Public Git Issue on a cloud case, but still sends the prefilled update level", () => {
    const onSubmit = vi.fn();
    renderDialog({ showRepoField: true, defaultUpdateLevel: "7.1.0.42", onSubmit });
    expect(screen.queryByLabelText(/update level/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/public git issue/i)).not.toBeInTheDocument();
    fillRequiredFields();
    submit();
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ updateLevel: "7.1.0.42" }));
  });
});

describe("CreateGithubIssueDialog — repo options (showRepoField)", () => {
  it("resolves the selected option's real owner/repo into repoOverride, not a hardcoded owner", () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^raise issue$/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        repoOverride: { owner: "example-org", repo: "example-repo" },
      }),
    );
  });

  it("shows the resolved owner/repo (not a hardcoded string) on the confirm step", () => {
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    const confirmDialog = screen.getByRole("dialog", {
      name: /raise this git issue/i,
    });
    expect(
      within(confirmDialog).getByText(/example-org\/example-repo \(Alpha\)/),
    ).toBeInTheDocument();
  });

  it("disables the repo select while options are loading, instead of rendering broken values", () => {
    mockUseGetProductRepoMapping.mockReturnValue({
      data: undefined,
      isLoading: true,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.queryByRole("combobox", { name: /choose repository/i })).not.toBeInTheDocument();
    expect(screen.getByText(/looking up the github repository/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
  });

  it("keeps Raise issue disabled while repo options are still loading, even with every other field filled", () => {
    mockUseGetProductRepoMapping.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    // No repo can be selected yet — the select itself is disabled — so
    // submitting now would silently omit repoOverride for a cloud case.
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
  });

  it("keeps Raise issue disabled when the repo options fetch has failed", () => {
    mockUseGetProductRepoMapping.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
  });

  it("keeps Raise issue disabled when the lookup finds no mapping", () => {
    mockUseGetProductRepoMapping.mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        productName="Totally Unknown Product"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    expect(screen.getByText(/no github repository is mapped/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
  });

  it("shows the repository the lookup returned", () => {
    mockUseGetProductRepoMapping.mockReturnValue({
      data: {
        productName: "Beta",
        owner: "example-org",
        repository: "other-repo",
        githubLabel: "Beta",
      },
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        productName="Beta"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(
      screen.getByText(/example-org\/other-repo \(Beta\)/),
    ).toBeInTheDocument();
  });

  it("keeps Raise issue disabled while a linked project's status is still loading", () => {
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        projectStatusPending
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    expect(screen.getByText(/waiting for this case's project status/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
  });

  it("keeps Raise issue disabled when the project lookup failed, until retry", () => {
    const onRetryProjectStatus = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        projectStatusFailed
        onRetryProjectStatus={onRetryProjectStatus}
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    expect(screen.getByText(/could not load this case's project status/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(onRetryProjectStatus).toHaveBeenCalledOnce();
  });
});

describe("CreateGithubIssueDialog — confirm step before filing a real issue", () => {
  // Filing an issue is a real write to an external repo with no delete on
  // either side, so it must never fire on the first click.
  it("does not call onSubmit on the first click — opens a confirm step instead", () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: /raise this git issue/i }),
    ).toBeInTheDocument();
  });

  it("calls onSubmit with the built payload only after confirming", () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^raise issue$/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        reason: "default",
        title: "Token issuance is slow",
        description: "Latency spiked after the last deploy.",
        issueTypeLabel: "Type/Discussion",
        priorityLevel: "Priority/High(P1)",
        repoOverride: { owner: "example-org", repo: "example-repo" },
      }),
    );
  });

  it("backing out of the confirm step does not submit", async () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(onSubmit).not.toHaveBeenCalled();
    // MUI's Dialog exit is animated, so the node lingers briefly after Back
    // is clicked — wait for the unmount rather than asserting synchronously.
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: /raise this git issue/i }),
      ).not.toBeInTheDocument(),
    );
    // The form itself is still open/usable, not reset or closed.
    expect(screen.getByRole("button", { name: /^raise issue…$/i })).toBeEnabled();
  });

  it("surfaces the error inside the confirm step, not just the form behind it", () => {
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error="Something went wrong filing the issue."
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    const confirmDialog = screen.getByRole("dialog", {
      name: /raise this git issue/i,
    });
    expect(
      within(confirmDialog).getByText("Something went wrong filing the issue."),
    ).toBeInTheDocument();
  });

  it("disables Back and Raise issue while a submit is in flight", () => {
    const onSubmit = vi.fn();
    const { rerender } = render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));

    rerender(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    const confirmDialog = screen.getByRole("dialog", {
      name: /raise this git issue/i,
    });
    expect(within(confirmDialog).getByRole("button", { name: /^back$/i })).toBeDisabled();
    expect(within(confirmDialog).getByRole("button", { name: /^raise issue$/i })).toBeDisabled();
  });
});

describe("CreateGithubIssueDialog — success view", () => {
  it("shows a clickable link to the created issue instead of closing", () => {
    const onClose = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        createdIssue={{
          message: "Issue created.",
          issue: { url: "https://github.com/example-org/example/issues/42", number: 42, repo: "example" },
        }}
        onClose={onClose}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));

    const link = screen.getByRole("link", { name: /example#42/i });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/example-org/example/issues/42",
    );
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /^done$/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("falls back to the response message when no issue URL is present", () => {
    render(
      <CreateGithubIssueDialog
        open
        productName="Alpha"
        submitting={false}
        error={null}
        createdIssue={{ message: "Filed, awaiting SN sync." }}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /^raise issue…$/i }));
    expect(screen.getByText("Filed, awaiting SN sync.")).toBeInTheDocument();
  });
});
