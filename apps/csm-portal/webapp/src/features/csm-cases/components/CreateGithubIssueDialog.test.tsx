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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { CreateGithubIssueDialog } from "@features/csm-cases/components/CreateGithubIssueDialog";
import { useGetGithubIssueRepoOptions } from "@features/csm-cases/api/useGetGithubIssueRepoOptions";

// CreateGithubIssueDialog consumes this hook directly; mock the hook module
// itself (per this app's testing convention — mock the hook when testing a
// component that just consumes an already-built hook) rather than the
// backend client it wraps.
vi.mock("@features/csm-cases/api/useGetGithubIssueRepoOptions", () => ({
  useGetGithubIssueRepoOptions: vi.fn(),
}));

const mockUseGetGithubIssueRepoOptions = vi.mocked(useGetGithubIssueRepoOptions);

const REPO_OPTIONS_FIXTURE = [
  { value: "asgardeo", displayLabel: "Asgardeo", owner: "wso2-enterprise", repo: "wso2-iam-internal" },
  {
    value: "choreo",
    displayLabel: "WSO2 Developer Platform (Choreo)",
    owner: "wso2-enterprise",
    repo: "choreo",
  },
];

beforeEach(() => {
  mockUseGetGithubIssueRepoOptions.mockReturnValue({
    data: REPO_OPTIONS_FIXTURE,
    isLoading: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } as any);
});

function selectOption(comboName: RegExp, optionName: string | RegExp): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: comboName }));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

function fillSubjectAndDescription(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Token issuance is slow" },
  });
  fireEvent.change(screen.getByLabelText(/description/i), {
    target: { value: "Latency spiked after the last deploy." },
  });
}

/** Fills every always-required field, using P3 so no Priority Justification
 * or Hotfix Reason is involved. */
function fillRequiredFields(): void {
  fillSubjectAndDescription();
  selectOption(/^priority/i, "P3");
}

function renderDialog(
  props: Partial<React.ComponentProps<typeof CreateGithubIssueDialog>> = {},
) {
  return render(
    <CreateGithubIssueDialog
      open
      submitting={false}
      error={null}
      onClose={() => {}}
      onSubmit={() => {}}
      {...props}
    />,
  );
}

const createButton = () => screen.getByRole("button", { name: /create issue/i });

describe("CreateGithubIssueDialog — required fields gate submission", () => {
  it("starts with Subject and Description empty, ignoring any case content", () => {
    renderDialog();
    expect(screen.getByLabelText(/subject/i)).toHaveValue("");
    expect(screen.getByLabelText(/description/i)).toHaveValue("");
  });

  it("disables Create issue until Subject, Description, and Priority are all filled", () => {
    renderDialog();
    expect(createButton()).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Token issuance is slow" },
    });
    expect(createButton()).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Latency spiked after the last deploy." },
    });
    expect(createButton()).toBeDisabled();
    selectOption(/^priority/i, "P3");
    expect(createButton()).toBeEnabled();
  });

  it("offers only P1, P2 and P3 as priorities", () => {
    renderDialog();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^priority/i }));
    const names = screen
      .getAllByRole("option")
      .map((o) => o.textContent)
      .filter((t) => t !== "-- Select --");
    expect(names).toEqual(["P1", "P2", "P3"]);
  });

  it("keeps Reproducing Steps, Update Level and Public Git Issue optional", () => {
    renderDialog();
    fillRequiredFields();
    expect(screen.getByLabelText(/reproducing steps/i)).not.toBeRequired();
    expect(screen.getByLabelText(/update level/i)).not.toBeRequired();
    expect(screen.getByLabelText(/public git issue/i)).not.toBeRequired();
    expect(createButton()).toBeEnabled();
  });
});

describe("CreateGithubIssueDialog — Priority Justification", () => {
  it.each(["P1", "P2"])("is shown and required for %s", (priority) => {
    renderDialog();
    fillSubjectAndDescription();
    expect(screen.queryByLabelText(/priority justification/i)).not.toBeInTheDocument();
    selectOption(/^priority/i, priority);
    expect(screen.getByLabelText(/priority justification/i)).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/priority justification/i), {
      target: { value: "Production outage" },
    });
    expect(createButton()).toBeEnabled();
  });

  it("is hidden for P3", () => {
    renderDialog();
    selectOption(/^priority/i, "P3");
    expect(screen.queryByLabelText(/priority justification/i)).not.toBeInTheDocument();
  });

  it("does not send a stale justification after switching to P3", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillSubjectAndDescription();
    selectOption(/^priority/i, "P1");
    fireEvent.change(screen.getByLabelText(/priority justification/i), {
      target: { value: "Production outage" },
    });
    selectOption(/^priority/i, "P3");
    fireEvent.click(createButton());
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.not.objectContaining({ priorityJustification: expect.anything() }),
    );
  });
});

describe("CreateGithubIssueDialog — Hotfix Reason", () => {
  it("is hidden until Hotfix Required is on, then required", () => {
    renderDialog();
    fillRequiredFields();
    expect(screen.queryByRole("combobox", { name: /hotfix reason/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    expect(screen.getByRole("combobox", { name: /hotfix reason/i })).toBeInTheDocument();
    expect(createButton()).toBeDisabled();
    selectOption(/hotfix reason/i, "Customer-requested hotfix");
    expect(createButton()).toBeEnabled();
  });

  it("sends hotFixRequired with its reason", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillRequiredFields();
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    selectOption(/hotfix reason/i, "Customer-requested hotfix");
    fireEvent.click(createButton());
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        hotFixRequired: true,
        hotFixReason: "Customer-requested hotfix",
      }),
    );
  });

  it("drops the hotfix fields when the toggle is switched back off", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });
    fillRequiredFields();
    const toggle = screen.getByRole("switch", { name: /hotfix required/i });
    fireEvent.click(toggle);
    selectOption(/hotfix reason/i, "Customer-requested hotfix");
    fireEvent.click(toggle);
    fireEvent.click(createButton());
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    const payload = onSubmit.mock.calls[0][0];
    expect(payload).not.toHaveProperty("hotFixRequired");
    expect(payload).not.toHaveProperty("hotFixReason");
  });
});

describe("CreateGithubIssueDialog — payload", () => {
  it("includes optional fields only when filled", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit, defaultUpdateLevel: "7.1.0" });
    fillRequiredFields();
    fireEvent.change(screen.getByLabelText(/reproducing steps/i), {
      target: { value: "1. Call the token endpoint" },
    });
    fireEvent.change(screen.getByLabelText(/public git issue/i), {
      target: { value: "https://github.com/example/example/issues/1" },
    });
    fireEvent.click(createButton());
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith({
      reason: "default",
      title: "Token issuance is slow",
      description: "Latency spiked after the last deploy.",
      priorityLevel: "P3",
      reproducingSteps: "1. Call the token endpoint",
      updateLevel: "7.1.0",
      publicIssueUrl: "https://github.com/example/example/issues/1",
    });
  });
});

describe("CreateGithubIssueDialog — repo options (showRepoField)", () => {
  it("resolves the selected option's real owner/repo into repoOverride, not a hardcoded owner", () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /choose repository/i }));
    fireEvent.click(screen.getByRole("option", { name: "Asgardeo" }));
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        repoOverride: { owner: "wso2-enterprise", repo: "wso2-iam-internal" },
      }),
    );
  });

  it("shows the resolved owner/repo (not a hardcoded string) on the confirm step", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /choose repository/i }));
    fireEvent.click(screen.getByRole("option", { name: "Asgardeo" }));
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    const confirmDialog = screen.getByRole("dialog", {
      name: /file this github issue/i,
    });
    expect(
      within(confirmDialog).getByText(/wso2-enterprise\/wso2-iam-internal \(Asgardeo\)/),
    ).toBeInTheDocument();
  });

  it("disables the repo select while options are loading, instead of rendering broken values", () => {
    mockUseGetGithubIssueRepoOptions.mockReturnValue({
      data: undefined,
      isLoading: true,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByRole("combobox", { name: /choose repository/i })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("keeps Create issue disabled while repo options are still loading, even with every other field filled", () => {
    mockUseGetGithubIssueRepoOptions.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
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
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
  });

  it("keeps Create issue disabled when the repo options fetch has failed", () => {
    mockUseGetGithubIssueRepoOptions.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        showRepoField
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
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
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: /file this github issue/i }),
    ).toBeInTheDocument();
  });

  it("calls onSubmit with the built payload only after confirming", () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        reason: "default",
        title: "Token issuance is slow",
        description: "Latency spiked after the last deploy.",
        priorityLevel: "P3",
      }),
    );
  });

  it("backing out of the confirm step does not submit", async () => {
    const onSubmit = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    fireEvent.click(screen.getByRole("button", { name: /^back$/i }));
    expect(onSubmit).not.toHaveBeenCalled();
    // MUI's Dialog exit is animated, so the node lingers briefly after Back
    // is clicked — wait for the unmount rather than asserting synchronously.
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: /file this github issue/i }),
      ).not.toBeInTheDocument(),
    );
    // The form itself is still open/usable, not reset or closed.
    expect(screen.getByRole("button", { name: /create issue/i })).toBeEnabled();
  });

  it("surfaces the error inside the confirm step, not just the form behind it", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error="Something went wrong filing the issue."
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    const confirmDialog = screen.getByRole("dialog", {
      name: /file this github issue/i,
    });
    expect(
      within(confirmDialog).getByText("Something went wrong filing the issue."),
    ).toBeInTheDocument();
  });

  it("disables Back and File issue while a submit is in flight", () => {
    const onSubmit = vi.fn();
    const { rerender } = render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));

    rerender(
      <CreateGithubIssueDialog
        open
        submitting
        error={null}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    const confirmDialog = screen.getByRole("dialog", {
      name: /file this github issue/i,
    });
    expect(within(confirmDialog).getByRole("button", { name: /^back$/i })).toBeDisabled();
    expect(within(confirmDialog).getByRole("button", { name: /file issue/i })).toBeDisabled();
  });
});

describe("CreateGithubIssueDialog — success view", () => {
  it("shows a clickable link to the created issue instead of closing", () => {
    const onClose = vi.fn();
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        createdIssue={{
          message: "Issue created.",
          issue: { url: "https://github.com/wso2-enterprise/example/issues/42", number: 42, repo: "example" },
        }}
        onClose={onClose}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));

    const link = screen.getByRole("link", { name: /example#42/i });
    expect(link).toHaveAttribute(
      "href",
      "https://github.com/wso2-enterprise/example/issues/42",
    );
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /^done$/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("falls back to the response message when no issue URL is present", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        createdIssue={{ message: "Filed, awaiting SN sync." }}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    expect(screen.getByText("Filed, awaiting SN sync.")).toBeInTheDocument();
  });
});
