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

function selectType(typeLabel: string): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: /^type/i }));
  fireEvent.click(screen.getByRole("option", { name: typeLabel }));
}

/** Fills every field required for the simplest type (Query — no Severity or
 * Hotfix Required involved) so tests unrelated to the per-type rules don't
 * need to know about them. */
function fillRequiredFields(): void {
  selectType("Query");
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Token issuance is slow" },
  });
  fireEvent.change(screen.getByLabelText(/description/i), {
    target: { value: "Latency spiked after the last deploy." },
  });
}

describe("CreateGithubIssueDialog — required fields gate submission", () => {
  it("disables Create issue until Type, Subject, and Description are all filled", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
    selectType("Query");
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Token issuance is slow" },
    });
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Latency spiked after the last deploy." },
    });
    expect(screen.getByRole("button", { name: /create issue/i })).toBeEnabled();
  });
});

describe("CreateGithubIssueDialog — per-type field rules", () => {
  it("Query hides Severity and Hotfix Required", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    selectType("Query");
    expect(screen.queryByRole("combobox", { name: /severity/i })).not.toBeInTheDocument();
    expect(screen.queryByText("Hotfix Required")).not.toBeInTheDocument();
  });

  it("Incident requires Severity and hides Hotfix Required", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    selectType("Incident");
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Token issuance is slow" },
    });
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Latency spiked after the last deploy." },
    });
    expect(screen.queryByText("Hotfix Required")).not.toBeInTheDocument();
    // Severity is required for Incident — Create issue stays disabled without it.
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    fireEvent.click(screen.getByRole("option", { name: /p1/i }));
    expect(screen.getByRole("button", { name: /create issue/i })).toBeEnabled();
  });

  it("Patch hides Severity and requires Update Level + Public Git Issue", () => {
    render(
      <CreateGithubIssueDialog
        open
        submitting={false}
        error={null}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    selectType("Patch");
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Token issuance is slow" },
    });
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Latency spiked after the last deploy." },
    });
    expect(screen.queryByRole("combobox", { name: /severity/i })).not.toBeInTheDocument();
    expect(screen.getByText("Hotfix Required")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/update level/i), {
      target: { value: "7.1.0" },
    });
    expect(screen.getByRole("button", { name: /create issue/i })).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/public git issue/i), {
      target: { value: "https://github.com/wso2/example/issues/1" },
    });
    expect(screen.getByRole("button", { name: /create issue/i })).toBeEnabled();
  });
});

describe("CreateGithubIssueDialog — stale per-type fields don't leak into the payload", () => {
  it("drops hotFixRequired once Type is switched away from Patch after toggling it on", () => {
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
    selectType("Patch");
    fireEvent.click(screen.getByRole("switch", { name: /hotfix required/i }));
    // Switching away from Patch hides the control, but the toggled-on state
    // must not still ride along in the submitted payload.
    selectType("Query");
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Token issuance is slow" },
    });
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Latency spiked after the last deploy." },
    });
    fireEvent.click(screen.getByRole("button", { name: /create issue/i }));
    fireEvent.click(screen.getByRole("button", { name: /file issue/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.not.objectContaining({ hotFixRequired: true }),
    );
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
        issueTypeLabel: "Type/Query",
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
