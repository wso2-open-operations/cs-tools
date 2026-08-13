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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true }),
}));

vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecentViews: () => [],
}));

vi.mock("@config/featureFlags", () => ({
  navigableNavNodes: () => [],
}));

const quickCaseSearchMock = vi.fn();
vi.mock("@features/csm-cases/api/useQuickCaseSearch", () => ({
  QUICK_CASE_MIN_QUERY_LEN: 2,
  useQuickCaseSearch: (q: string, options?: { forceFreeText?: boolean }) =>
    quickCaseSearchMock(q, options),
  // A same-module-shape stand-in for the real `classifyQuickCaseQuery` —
  // duplicated here (rather than pulled in via `importOriginal`) because the
  // real module transitively imports the backend API client, which reads
  // runtime config unavailable under vitest. Mirrors the two patterns in
  // `useQuickCaseSearch.ts` exactly; if those patterns change, the matching
  // assertions below (and this copy) must change too.
  classifyQuickCaseQuery: (query: string): "number" | "internalId" | "text" => {
    if (/^CS\d{7}$/.test(query)) return "number";
    if (/^[a-zA-Z0-9]+-\d{1,4}$/.test(query)) return "internalId";
    return "text";
  },
}));

// Same "same-module-shape stand-in" reasoning as `classifyQuickCaseQuery`
// above applies to the three classify functions below.
const quickIncidentSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickIncidentSearch", () => ({
  QUICK_INCIDENT_MIN_QUERY_LEN: 2,
  useQuickIncidentSearch: (q: string, options?: { forceFreeText?: boolean }) =>
    quickIncidentSearchMock(q, options),
  classifyQuickIncidentQuery: (query: string): "number" | "text" =>
    /^INC\d{7}$/.test(query) ? "number" : "text",
}));

const quickChangeRequestSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickChangeRequestSearch", () => ({
  QUICK_CHANGE_REQUEST_MIN_QUERY_LEN: 2,
  useQuickChangeRequestSearch: (
    q: string,
    options?: { forceFreeText?: boolean },
  ) => quickChangeRequestSearchMock(q, options),
  classifyQuickChangeRequestQuery: (query: string): "number" | "text" =>
    /^CHG\d{7}$/.test(query) ? "number" : "text",
}));

const quickProblemSearchMock = vi.fn();
vi.mock("@features/csm-operations/api/useQuickProblemSearch", () => ({
  QUICK_PROBLEM_MIN_QUERY_LEN: 2,
  useQuickProblemSearch: (q: string, options?: { forceFreeText?: boolean }) =>
    quickProblemSearchMock(q, options),
  classifyQuickProblemQuery: (query: string): "number" | "text" =>
    /^PRB\d{7}$/.test(query) ? "number" : "text",
}));

const quickConversationSearchMock = vi.fn();
vi.mock("@features/csm-projects/api/useQuickConversationSearch", () => ({
  QUICK_CONVERSATION_MIN_QUERY_LEN: 2,
  useQuickConversationSearch: (
    q: string,
    options?: { forceFreeText?: boolean },
  ) => quickConversationSearchMock(q, options),
  classifyQuickConversationQuery: (query: string): "number" | "text" =>
    /^CHAT\d+$/.test(query) ? "number" : "text",
}));

const navigateMock = vi.fn();
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

const { default: QuickNav } = await import("./QuickNav");

const idleResult = { data: undefined, isFetching: false };

function renderQuickNav(initialEntries?: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <QuickNav />
    </MemoryRouter>,
  );
}

/**
 * Which of the four search hooks a given query actually gets routed to as
 * its real (non-suppressed) query — mirrors `QuickNav`'s own suppression
 * logic (only the entity kind whose exact-match shape the query matches
 * gets the real query; the other three get `""` and never fire). A query
 * matching none of the four shapes routes to all four, so waiting on the
 * case mock is representative there too.
 */
function primaryMockFor(query: string) {
  if (/^INC\d{7}$/.test(query)) return quickIncidentSearchMock;
  if (/^PRB\d{7}$/.test(query)) return quickProblemSearchMock;
  if (/^CHG\d{7}$/.test(query)) return quickChangeRequestSearchMock;
  if (/^CHAT\d+$/.test(query)) return quickConversationSearchMock;
  return quickCaseSearchMock;
}

async function openAndType(query: string) {
  renderQuickNav();
  fireEvent.click(screen.getByLabelText("Search or jump to (open quick nav)"));
  const input = await screen.findByLabelText("Quick nav search");
  fireEvent.change(input, { target: { value: query } });
  // Wait past the 180ms debounce for the search hooks to be called with the
  // settled query — waiting on whichever hook the query actually routes to
  // (see `primaryMockFor`), since an exact-match-shaped query suppresses the
  // other three down to an empty, disabled query.
  await waitFor(() =>
    expect(primaryMockFor(query)).toHaveBeenLastCalledWith(
      query,
      expect.objectContaining({ forceFreeText: false }),
    ),
  );
}

describe("QuickNav", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    quickCaseSearchMock.mockReturnValue(idleResult);
    quickIncidentSearchMock.mockReturnValue(idleResult);
    quickChangeRequestSearchMock.mockReturnValue(idleResult);
    quickProblemSearchMock.mockReturnValue(idleResult);
    quickConversationSearchMock.mockReturnValue(idleResult);
  });

  it("renders an 'Incidents' section for a live incident hit and links to the incident route", async () => {
    quickIncidentSearchMock.mockReturnValue({
      data: [
        {
          id: "inc-1",
          number: "INC0001234",
          subject: "Prod cluster down",
          state: "IN_PROGRESS",
          assigneeName: "Jane Doe",
        },
      ],
      isFetching: false,
    });

    await openAndType("cluster");

    expect(await screen.findByText("Incidents")).toBeInTheDocument();
    expect(screen.getByText("INC0001234")).toBeInTheDocument();
    expect(screen.getByText("Prod cluster down")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Prod cluster down"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/inc-1");
  });

  it("renders a 'Change Requests' section for a live CR hit and links to the change-request route", async () => {
    quickChangeRequestSearchMock.mockReturnValue({
      data: [
        {
          id: "cr-1",
          number: "CHG0005",
          subject: "Upgrade the API gateway",
          state: "assess",
          assigneeName: "John Smith",
        },
      ],
      isFetching: false,
    });

    await openAndType("upgrade");

    expect(await screen.findByText("Change Requests")).toBeInTheDocument();
    expect(screen.getByText("CHG0005")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Upgrade the API gateway"));
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/cr-1");
  });

  it("renders a 'Conversations' section for a live conversation hit and links to the conversation route", async () => {
    quickConversationSearchMock.mockReturnValue({
      data: [
        {
          id: "conv-1",
          number: "CHAT0000012345",
          initiatorName: "Jane Doe",
          initialMessage: "How do I reset my API key?",
        },
      ],
      isFetching: false,
    });

    await openAndType("reset");

    expect(await screen.findByText("Conversations")).toBeInTheDocument();
    expect(screen.getByText("CHAT0000012345")).toBeInTheDocument();
    expect(screen.getByText("Started by Jane Doe")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Started by Jane Doe"));
    expect(navigateMock).toHaveBeenCalledWith("/conversations/conv-1");
  });

  it("shows no entity sections beyond Pages while nothing has matched", async () => {
    await openAndType("zz");
    expect(screen.queryByText("Incidents")).not.toBeInTheDocument();
    expect(screen.queryByText("Change Requests")).not.toBeInTheDocument();
    expect(screen.queryByText("Problems")).not.toBeInTheDocument();
    expect(screen.queryByText("Conversations")).not.toBeInTheDocument();
    expect(screen.getByText("No matches.")).toBeInTheDocument();
  });

  it("opens pre-filled and searching from a `?q=` link", async () => {
    renderQuickNav(["/?q=CS0440883"]);

    const input = await screen.findByLabelText("Quick nav search");
    expect(input).toHaveValue("CS0440883");
    await waitFor(() =>
      expect(quickCaseSearchMock).toHaveBeenLastCalledWith(
        "CS0440883",
        expect.objectContaining({ forceFreeText: false }),
      ),
    );
  });

  it("auto-navigates to the single record a `?goto=` link resolves to", async () => {
    quickIncidentSearchMock.mockReturnValue({
      data: [
        {
          id: "inc-1",
          number: "INC0001234",
          subject: "Prod cluster down",
          state: "IN_PROGRESS",
        },
      ],
      isFetching: false,
    });

    renderQuickNav(["/?goto=INC0001234"]);

    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/inc-1"),
    );
  });

  it("leaves the palette open when `?goto=` matches nothing", async () => {
    renderQuickNav(["/?goto=NOPE0000"]);

    const input = await screen.findByLabelText("Quick nav search");
    await waitFor(() =>
      expect(quickCaseSearchMock).toHaveBeenLastCalledWith(
        "NOPE0000",
        expect.objectContaining({ forceFreeText: false }),
      ),
    );
    await waitFor(() => expect(screen.getByText("No matches.")).toBeInTheDocument());
    expect(input).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("shows an exact-match banner for a case-number-shaped query, with a widen affordance", async () => {
    quickCaseSearchMock.mockReturnValue(idleResult);

    await openAndType("CS0441174");

    expect(
      await screen.findByText(/Showing an exact match for case number/),
    ).toBeInTheDocument();
    const widenButton = screen.getByRole("button", { name: "Search in subject and description too" });

    fireEvent.click(widenButton);

    await waitFor(() =>
      expect(quickCaseSearchMock).toHaveBeenLastCalledWith(
        "CS0441174",
        expect.objectContaining({ forceFreeText: true }),
      ),
    );
    // Widening clears the banner — the search is no longer scoped.
    expect(
      screen.queryByText(/Showing an exact match for case number/),
    ).not.toBeInTheDocument();
  });

  it("shows an exact-match banner for a WSO2-case-id-shaped query", async () => {
    quickCaseSearchMock.mockReturnValue(idleResult);

    await openAndType("SOMEID-4");

    expect(
      await screen.findByText(/Showing an exact match for WSO2 case id/),
    ).toBeInTheDocument();
  });

  it("does not show the exact-match banner for a free-text query", async () => {
    quickCaseSearchMock.mockReturnValue(idleResult);

    await openAndType("printer jam");

    expect(screen.queryByText(/Showing an exact match/)).not.toBeInTheDocument();
  });

  it("shows an exact-match banner for an incident-number-shaped query, with a widen affordance", async () => {
    await openAndType("INC0090472");

    expect(
      await screen.findByText(/Showing an exact match for incident number/),
    ).toBeInTheDocument();
    const widenButton = screen.getByRole("button", { name: "Search in subject and description too" });

    fireEvent.click(widenButton);

    // Widening re-enables and searches ALL FOUR entity types as free text —
    // not just the incident search that was originally scoped.
    await waitFor(() =>
      expect(quickIncidentSearchMock).toHaveBeenLastCalledWith(
        "INC0090472",
        expect.objectContaining({ forceFreeText: true }),
      ),
    );
    expect(quickCaseSearchMock).toHaveBeenLastCalledWith(
      "INC0090472",
      expect.objectContaining({ forceFreeText: true }),
    );
    expect(quickProblemSearchMock).toHaveBeenLastCalledWith(
      "INC0090472",
      expect.objectContaining({ forceFreeText: true }),
    );
    expect(quickChangeRequestSearchMock).toHaveBeenLastCalledWith(
      "INC0090472",
      expect.objectContaining({ forceFreeText: true }),
    );
    expect(
      screen.queryByText(/Showing an exact match for incident number/),
    ).not.toBeInTheDocument();
  });

  it("shows an exact-match banner for a problem-number-shaped query", async () => {
    await openAndType("PRB0040192");

    expect(
      await screen.findByText(/Showing an exact match for problem number/),
    ).toBeInTheDocument();
  });

  it("shows an exact-match banner for a change-request-number-shaped query", async () => {
    await openAndType("CHG0038721");

    expect(
      await screen.findByText(/Showing an exact match for change request number/),
    ).toBeInTheDocument();
  });

  it("shows an exact-match banner for a conversation-number-shaped query", async () => {
    await openAndType("CHAT0000012345");

    expect(
      await screen.findByText(/Showing an exact match for conversation number/),
    ).toBeInTheDocument();
  });

  it.each([
    ["INC0090472", quickIncidentSearchMock, "incident"],
    ["PRB0040192", quickProblemSearchMock, "problem"],
    ["CHG0038721", quickChangeRequestSearchMock, "change request"],
    ["CS0441174", quickCaseSearchMock, "case"],
    ["CHAT0000012345", quickConversationSearchMock, "conversation"],
  ] as const)(
    "routes a %s-shaped query only to the %s search, suppressing the other four entirely",
    async (query, matchingMock, label) => {
      void label;
      await openAndType(query);

      // The matching type gets the real query, in exact-match mode.
      expect(matchingMock).toHaveBeenLastCalledWith(
        query,
        expect.objectContaining({ forceFreeText: false }),
      );

      // The other four are passed an empty query — not the typed text, not
      // even as a free-text search — so their own `enabled` gate (`q.length
      // >= MIN_LEN`) keeps them from firing at all.
      const allMocks = [
        quickCaseSearchMock,
        quickIncidentSearchMock,
        quickProblemSearchMock,
        quickChangeRequestSearchMock,
        quickConversationSearchMock,
      ];
      for (const mock of allMocks) {
        if (mock === matchingMock) continue;
        expect(mock).toHaveBeenLastCalledWith(
          "",
          expect.objectContaining({ forceFreeText: false }),
        );
      }
    },
  );

  it("runs all five searches as free text for a query that matches none of the exact-match patterns", async () => {
    await openAndType("printer jam");

    for (const mock of [
      quickCaseSearchMock,
      quickIncidentSearchMock,
      quickProblemSearchMock,
      quickChangeRequestSearchMock,
      quickConversationSearchMock,
    ]) {
      expect(mock).toHaveBeenLastCalledWith(
        "printer jam",
        expect.objectContaining({ forceFreeText: false }),
      );
    }
  });

  it("renders the exact-match banner after the result sections, not before them", async () => {
    quickIncidentSearchMock.mockReturnValue({
      data: [
        {
          id: "inc-1",
          number: "INC0090472",
          subject: "Prod cluster down",
          state: "IN_PROGRESS",
        },
      ],
      isFetching: false,
    });

    await openAndType("INC0090472");

    const banner = await screen.findByText(
      /Showing an exact match for incident number/,
    );
    const incidentsHeading = screen.getByText("Incidents");

    // DOM order, not just presence: the banner must come after the results,
    // not above them.
    expect(
      incidentsHeading.compareDocumentPosition(banner) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("shows a low-key 'didn't match a known pattern' hint while a free-text search is in flight, and hides the exact-match banner at the same time", async () => {
    quickCaseSearchMock.mockReturnValue({ data: undefined, isFetching: true });

    await openAndType("printer jam");

    expect(
      await screen.findByText(
        /Didn't match a known number pattern.*searching all fields/,
      ),
    ).toBeInTheDocument();
    // The two messages are mutually exclusive — a free-text query never
    // matches an exact-match shape, so the banner never renders alongside
    // the hint.
    expect(screen.queryByText(/Showing an exact match/)).not.toBeInTheDocument();
  });

  it("keeps showing the 'didn't match a known pattern' hint (settled wording) once the free-text search has settled with zero hits", async () => {
    // Regression: this hint used to be gated on isFetching and disappeared
    // the moment the search settled — so a query like "cs123" (doesn't
    // match the exact-match shape, free-text search finds nothing) landed
    // on a bare "No matches." with zero explanation of why it didn't get
    // the fast exact-match path. The hint must persist through a settled
    // zero-hit result, just with past-tense wording instead of the
    // in-flight "searching..." phrasing.
    quickCaseSearchMock.mockReturnValue(idleResult);

    await openAndType("printer jam");

    expect(
      await screen.findByText(/Didn't match a known number pattern.*searched all fields\./),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/searching all fields, this may take a moment/),
    ).not.toBeInTheDocument();
  });

  it("does not flash 'No matches.' for a scoped CHG query while the (only relevant) change-request search is still fetching", async () => {
    // A CHG-shaped query suppresses case/incident/problem entirely (their
    // hooks are passed "" and settle to idle immediately), so `casesLoading`
    // alone would already read `false` here — the regression this guards:
    // the empty state must keep waiting on the change-request search
    // specifically, not on the (irrelevant, already-idle) case search.
    quickChangeRequestSearchMock.mockReturnValue({
      data: undefined,
      isFetching: true,
    });

    await openAndType("CHG0038721");

    expect(screen.queryByText("No matches.")).not.toBeInTheDocument();
  });

  it("shows both 'No matches.' and the exact-match banner once a scoped CHG query genuinely settles with zero hits", async () => {
    quickChangeRequestSearchMock.mockReturnValue({ data: [], isFetching: false });

    await openAndType("CHG0038721");

    const emptyState = await screen.findByText("No matches.");
    const banner = await screen.findByText(
      /Showing an exact match for change request number/,
    );
    expect(emptyState).toBeInTheDocument();
    expect(banner).toBeInTheDocument();
    // The banner is a footnote below the empty state, not competing with it.
    expect(
      emptyState.compareDocumentPosition(banner) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    // And the widen affordance still works from this zero-hit state.
    fireEvent.click(screen.getByRole("button", { name: "Search in subject and description too" }));
    await waitFor(() =>
      expect(quickChangeRequestSearchMock).toHaveBeenLastCalledWith(
        "CHG0038721",
        expect.objectContaining({ forceFreeText: true }),
      ),
    );
  });
});
