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

import type { ReactElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type {
  CaseAlertSettings,
  PagingChainResponse,
  PagingReadinessResponse,
} from "../types";
import {
  pagingChain,
  pagingMember,
  readinessChain,
  readinessGap,
} from "../test/fixtures";
import { ADD_TO_PROFILE_HINT } from "../utils/pagingPhone";
// vi.mock below is hoisted above this import, so the panel gets the mocks.
import CasePagingTab from "./CasePagingTab";

const mutateAsync = vi.fn();
const putMutate = vi.fn();
const delMutate = vi.fn();
const testMutate = vi.fn();
const readiness: { data?: PagingReadinessResponse } = {};
const chain: { data?: PagingChainResponse } = {};

function render(ui: ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status = 0;
  },
  useBackendApi: () => ({}),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
const alertSettings: { data?: CaseAlertSettings } = {};
const noopMutation = {
  mutate: vi.fn(),
  mutateAsync: vi.fn(),
  isPending: false,
};
vi.mock("../api/useCaseAlertSettings", () => ({
  useCaseAlertSettings: () => ({
    data: alertSettings.data,
    isLoading: false,
    isError: false,
  }),
  canViewCaseAlertSettings: (s?: CaseAlertSettings) =>
    Boolean(s?.canManage.view),
  useSaveCaseAlertRoom: () => noopMutation,
  useSetCaseAlertWebhook: () => noopMutation,
  useTestCaseAlertRoom: () => noopMutation,
  useDeleteCaseAlertRoom: () => noopMutation,
  useSaveCaseAlertRoute: () => noopMutation,
  useDeleteCaseAlertRoute: () => noopMutation,
  useSaveCaseAlertValues: () => noopMutation,
  useDeleteCaseAlertValue: () => noopMutation,
}));

/** Settings a CSM admin may see and change. */
function settingsFor(view: boolean): CaseAlertSettings {
  return {
    canManage: {
      view,
      roomLadders: view ? ["SRE", "SME"] : [],
      routeLadders: view ? ["CRE", "SRE"] : [],
    },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 1 },
    rooms: [],
    routes: [],
    values: [],
  };
}

const VIEW_KEY = "csm-ts.alert-settings.view";

vi.mock("../api/usePagingChain", () => ({
  useGetPagingChain: () => ({
    data: chain.data,
    isLoading: false,
    isError: false,
    dataUpdatedAt: 0,
  }),
  usePatchPagingMember: () => ({ mutateAsync }),
  useGetPagingReadiness: () => ({
    data: readiness.data,
    isLoading: false,
    isError: false,
  }),
  usePutPagingContact: () => ({ mutate: putMutate, isPending: false }),
  useDeletePagingContact: () => ({ mutate: delMutate, isPending: false }),
  useTestPagingContact: () => ({ mutate: testMutate, isPending: false }),
}));

beforeEach(() => {
  vi.restoreAllMocks();
  window.localStorage.clear();
  alertSettings.data = undefined;
  mutateAsync.mockReset();
  putMutate.mockReset();
  delMutate.mockReset();
  testMutate.mockReset();
  chain.data = pagingChain();
  readiness.data = {
    generatedAt: "2026-10-08T03:00:00Z",
    from: "2026-10-08",
    to: "2026-10-14",
    chains: [
      readinessChain({ chain: "CRE", label: "CRE" }),
      readinessChain({
        chain: "SRE_SAAS",
        label: "SaaS SRE",
        ready: false,
        gaps: [
          readinessGap({
            code: "NO_L1",
            message: "No L1 in TZ2",
            fix: "rota",
            zoneCode: "TZ2",
            date: "2026-10-10",
          }),
        ],
      }),
      readinessChain({ chain: "SRE_IAAS", label: "IaaS SRE" }),
      readinessChain({ chain: "SME", label: "SME" }),
    ],
  };
});

/** Settings rights for the given ladders. */
function rights(...ladders: ("CRE" | "SRE" | "SME")[]): CaseAlertSettings {
  return {
    ...settingsFor(true),
    canManage: {
      view: true,
      roomLadders: ladders,
      routeLadders: ladders.filter((l) => l !== "SME"),
    },
  };
}

/** The headings of the chains under "Who gets alerts". */
function chainHeadings(): string[] {
  return screen
    .queryAllByRole("heading", { level: 2 })
    .map((h) => h.textContent ?? "");
}

describe("CasePagingTab", () => {
  it("lists every SRE and SME chain under SRE, each with its readiness and a way to its rota", () => {
    const onOpenRota = vi.fn();
    render(<CasePagingTab onOpenRota={onOpenRota} visibleChains={["SRE_SAAS", "SRE_IAAS", "PAAS", "SME"]} />);
    // No settings rights: one family, no family tabs, no sub-tabs.
    expect(screen.queryByRole("tablist", { name: "Family" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist", { name: "Show" })).not.toBeInTheDocument();
    expect(chainHeadings()).toEqual([
      "SaaS SRE alert chain",
      "IaaS SRE alert chain",
      "PaaS SRE alert chain",
      "SME alert chain",
    ]);
    expect(
      screen.getByText("PaaS SRE has no rota yet, so there is no alert chain to show."),
    ).toBeInTheDocument();
    expect(screen.getByText(/⚠ 1 gap/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Open the SaaS SRE rota" }));
    expect(onOpenRota).toHaveBeenLastCalledWith({ family: "SRE", rotaCode: "SRE_SAAS" });
    // A rota gap opens the rota on its zone and day.
    fireEvent.click(screen.getByRole("button", { name: /Open the rota on/ }));
    expect(onOpenRota).toHaveBeenLastCalledWith({
      family: "SRE",
      rotaCode: "SRE_SAAS",
      teamKey: undefined,
      zoneCode: "TZ2",
      date: "2026-10-10",
    });
    fireEvent.click(screen.getByRole("button", { name: "Open the SME rota" }));
    expect(onOpenRota).toHaveBeenLastCalledWith({ family: "SME", rotaCode: undefined });
  });

  it("shows the U2-SRE and B-Central-SRE chains only once their teams are SRE teams", () => {
    const visible = ["SRE_SAAS", "SRE_U2", "SRE_B_CENTRAL"] as const;
    // Before entity-service migration 0242 both are SME teams: no chain of their own.
    const { unmount } = render(
      <CasePagingTab
        onOpenRota={vi.fn()}
        visibleChains={visible}
        teams={[
          { key: "u2", name: "U2", family: "SME" },
          { key: "b-central", name: "B-Central", family: "SME" },
        ]}
      />,
    );
    expect(chainHeadings()).toEqual(["SaaS SRE alert chain"]);
    unmount();

    const onOpenRota = vi.fn();
    render(
      <CasePagingTab
        onOpenRota={onOpenRota}
        visibleChains={visible}
        teams={[
          { key: "u2", name: "U2", family: "SRE" },
          { key: "b-central", name: "B-Central", family: "SRE" },
        ]}
      />,
    );
    expect(chainHeadings()).toEqual([
      "SaaS SRE alert chain",
      "U2-SRE alert chain",
      "B-Central-SRE alert chain",
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Open the U2-SRE rota" }));
    expect(onOpenRota).toHaveBeenLastCalledWith({ family: "SRE", rotaCode: "SME_U2" });
  });

  it("keeps a reader to their own chains", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} visibleChains={["SRE_SAAS", "SRE_IAAS"]} />);
    expect(chainHeadings()).toEqual(["SaaS SRE alert chain", "IaaS SRE alert chain"]);
  });

  it("tells a reader with no chain of theirs so", () => {
    render(<CasePagingTab onOpenRota={vi.fn()} visibleChains={[]} />);
    expect(screen.getByText("There is no alert chain of yours to show.")).toBeInTheDocument();
  });

  it("offers CRE and SRE family tabs to a CSM admin, opening on CRE's alert chain", () => {
    alertSettings.data = rights("CRE", "SRE", "SME");
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="SME" />);
    const families = screen.getByRole("tablist", { name: "Family" });
    expect(within(families).getAllByRole("tab").map((t) => t.textContent)).toEqual(["CRE", "SRE"]);
    expect(within(families).getByRole("tab", { name: "CRE" })).toHaveAttribute("aria-selected", "true");
    // CRE has its "Who gets alerts" -- the chain where first responders are
    // marked -- and its settings.
    const show = screen.getByRole("tablist", { name: "Show" });
    expect(within(show).getByRole("tab", { name: "Who gets alerts" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("heading", { name: /^Alert chain/ })).toBeInTheDocument();

    fireEvent.click(within(show).getByRole("tab", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "CRE settings" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /^Alert chain/ })).not.toBeInTheDocument();

    fireEvent.click(within(families).getByRole("tab", { name: "SRE" }));
    expect(screen.getByRole("tablist", { name: "Show" })).toBeInTheDocument();
  });

  it("shows a CRE Team lead CRE's alert chain alone: where they mark their team's first responders", () => {
    // No settings rights: one family, no family tabs, no sub-tabs, no SRE chains.
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="CRE" visibleChains={["CRE"]} />);
    expect(screen.queryByRole("tablist", { name: "Family" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist", { name: "Show" })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /^Alert chain/ })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "SaaS SRE alert chain" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "CRE settings" })).not.toBeInTheDocument();
  });

  it("offers CRE to a reader given its chain or its settings, and not to an SRE rota admin with neither", () => {
    alertSettings.data = rights("SRE", "SME");
    const { unmount } = render(
      <CasePagingTab onOpenRota={vi.fn()} initialChain="CRE" visibleChains={["SRE_SAAS", "SRE_IAAS", "SME"]} />,
    );
    expect(screen.queryByRole("tab", { name: "CRE" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "CRE settings" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /^Alert chain/ })).not.toBeInTheDocument();
    unmount();

    // The same SRE rota admin who also leads a CRE team: CRE's chain, no CRE settings.
    render(
      <CasePagingTab onOpenRota={vi.fn()} initialChain="CRE" visibleChains={["CRE", "SRE_SAAS", "SRE_IAAS", "SME"]} />,
    );
    const families = screen.getByRole("tablist", { name: "Family" });
    expect(within(families).getByRole("tab", { name: "CRE" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("heading", { name: /^Alert chain/ })).toBeInTheDocument();
    expect(screen.queryByRole("tablist", { name: "Show" })).not.toBeInTheDocument();
  });

  it("shows CRE's settings alone to a CRE rota admin: no tabs, no chain", () => {
    alertSettings.data = rights("CRE");
    render(<CasePagingTab onOpenRota={vi.fn()} initialChain="CRE" visibleChains={[]} />);
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "CRE settings" })).toBeInTheDocument();
    expect(chainHeadings()).toEqual([]);
  });

  it("switches SRE between who gets alerts and the settings, and remembers the choice", () => {
    alertSettings.data = rights("SRE", "SME");
    const { unmount } = render(<CasePagingTab onOpenRota={vi.fn()} />);
    const show = screen.getByRole("tablist", { name: "Show" });
    expect(within(show).getByRole("tab", { name: "Who gets alerts" })).toHaveAttribute("aria-selected", "true");
    expect(chainHeadings()).toContain("SaaS SRE alert chain");

    fireEvent.click(within(show).getByRole("tab", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "SRE settings" })).toBeInTheDocument();
    expect(screen.getByText("SaaS, IaaS and SME")).toBeInTheDocument();
    expect(chainHeadings()).toEqual([]);
    expect(window.localStorage.getItem(VIEW_KEY)).toBe("settings");

    unmount();
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("heading", { name: "SRE settings" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Who gets alerts" }));
    expect(window.localStorage.getItem(VIEW_KEY)).toBe("chain");
  });

  it("lists the SME teams after escalation, never a move-only team, and keeps the team spaces off the tab", () => {
    alertSettings.data = {
      ...rights("SRE", "SME"),
      routeOptions: {
        groups: [],
        rotas: [],
        teams: [
          { key: "apollo", name: "Apollo", family: "SRE" },
          { key: "saas-night-cover", name: "SaaS Night cover", family: "SRE" },
        ],
      },
    };
    window.localStorage.setItem(VIEW_KEY, "settings");
    window.localStorage.setItem("csm-ts.alert-settings.open", JSON.stringify(["SRE.where", "SRE.advanced"]));
    render(
      <CasePagingTab
        onOpenRota={vi.fn()}
        teams={[{ key: "asgardeo", name: "Asgardeo", family: "SME" }]}
        moveTargetKeys={["saas-night-cover"]}
      />,
    );
    const after = screen.getByRole("group", { name: "After escalation" });
    expect(within(after).getByText("Asgardeo")).toBeInTheDocument();
    expect(within(after).queryByText("SaaS Night cover")).not.toBeInTheDocument();
    // The SRE teams' own spaces and the redirects are in the full layout only:
    // the tab shows each ladder's switch, the important settings and where
    // alerts go.
    expect(screen.queryByRole("region", { name: "Advanced" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "+ Add chat space" })).not.toBeInTheDocument();
  });

  it("keeps a family's changes while the other family is on screen", () => {
    alertSettings.data = {
      ...rights("CRE", "SRE", "SME"),
      running: {
        reportedAt: "2026-10-01T00:00:00Z",
        mode: "db",
        ladders: { CRE: { enabled: { value: true, source: "file" } } },
      },
    };
    window.localStorage.setItem(VIEW_KEY, "settings");
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    fireEvent.click(screen.getByRole("switch", { name: "Alerts on" }));
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("1 change not saved yet");

    fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
    fireEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.getByRole("region", { name: "SRE changes to submit" })).toHaveTextContent("No changes to submit.");
    fireEvent.click(screen.getByRole("tab", { name: "CRE" }));
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("1 change not saved yet");
  });

  it("points a missing number to the profile, even for a person the reader may edit", () => {
    const base = pagingChain();
    chain.data = {
      ...base,
      members: [
        ...base.members,
        pagingMember({
          membershipId: "a4",
          name: "Sam Lee",
          canEditPhone: true,
          hasProfilePhone: false,
        }),
      ],
    };
    readiness.data!.chains[1] = readinessChain({
      chain: "SRE_SAAS",
      label: "SaaS SRE",
      ready: false,
      gaps: [
        readinessGap({
          code: "NO_PHONE",
          fix: "profile",
          userId: "u-a4",
          message: "Sam Lee (L1) has no number",
        }),
        readinessGap({
          code: "NO_PHONE",
          fix: "profile",
          userId: "u-a3",
          message: "Ann Poe (L1) has no number",
        }),
      ],
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    const strip = screen.getByRole("list", { name: "SaaS SRE gaps" });
    const [sam, ann] = within(strip).getAllByRole("listitem");
    expect(within(sam).queryByRole("button", { name: "Add number" })).not.toBeInTheDocument();
    expect(sam).toHaveTextContent(ADD_TO_PROFILE_HINT);
    expect(ann).toHaveTextContent(ADD_TO_PROFILE_HINT);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("still switches when the browser's storage throws", () => {
    alertSettings.data = rights("SRE", "SME");
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    render(<CasePagingTab onOpenRota={vi.fn()} />);
    expect(screen.getByRole("tab", { name: "Who gets alerts" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("tab", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "SRE settings" })).toBeInTheDocument();
  });
});
