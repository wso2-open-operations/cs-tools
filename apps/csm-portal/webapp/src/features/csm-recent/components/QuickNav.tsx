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

import {
  Alert,
  Box,
  Button,
  ButtonBase,
  Form,
  InputBase,
  Modal,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import { MessageSquare, Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useSearchParams } from "react-router";
import { useAsgardeo } from "@asgardeo/react";

import { navNodeById } from "@config/csmNavItems";
import { navigableNavNodes } from "@config/featureFlags";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { usePermissions } from "@hooks/usePermissions";
import { isNavNodeAuthorized } from "@layouts/navFilter";
import {
  useRecentViews,
  type RecentView,
} from "@features/csm-recent/hooks/useRecentViews";
import { kindIcon } from "@features/csm-recent/kindMeta";
import QuickNavCaseCard from "@features/csm-recent/components/QuickNavCaseCard";
import QuickNavEntityCard from "@features/csm-recent/components/QuickNavEntityCard";
import QuickNavResultSkeleton from "@features/csm-recent/components/QuickNavResultSkeleton";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import {
  classifyQuickCaseQuery,
  QUICK_CASE_MIN_QUERY_LEN,
  useQuickCaseSearch,
  type QuickCaseHit,
} from "@features/csm-cases/api/useQuickCaseSearch";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useNavTransition } from "@hooks/useNavTransition";
import {
  classifyQuickIncidentQuery,
  QUICK_INCIDENT_MIN_QUERY_LEN,
  useQuickIncidentSearch,
} from "@features/csm-operations/api/useQuickIncidentSearch";
import {
  classifyQuickChangeRequestQuery,
  QUICK_CHANGE_REQUEST_MIN_QUERY_LEN,
  useQuickChangeRequestSearch,
} from "@features/csm-operations/api/useQuickChangeRequestSearch";
import {
  classifyQuickProblemQuery,
  QUICK_PROBLEM_MIN_QUERY_LEN,
  useQuickProblemSearch,
} from "@features/csm-operations/api/useQuickProblemSearch";
import {
  classifyQuickConversationQuery,
  QUICK_CONVERSATION_MIN_QUERY_LEN,
  useQuickConversationSearch,
} from "@features/csm-projects/api/useQuickConversationSearch";

type Section =
  | "Cases"
  | "Incidents"
  | "Change Requests"
  | "Problems"
  | "Conversations"
  | "Pinned"
  | "Recents"
  | "Pages";

/** Minimal shape `QuickNavEntityCard` needs, shared by incident/CR/problem hits. */
interface EntityCardHit {
  icon: JSX.Element;
  idLabel?: string | null;
  subject: string;
  state?: string | null;
  assigneeName?: string;
}

interface Result {
  key: string;
  icon: JSX.Element;
  label: string;
  sublabel?: string;
  href: string;
  section: Section;
  /** Present only for "Cases" results — renders as a rich card instead of a plain row. */
  caseHit?: QuickCaseHit;
  /** Present only for Incident/Change-request/Problem/Conversation results — see {@link EntityCardHit}. */
  entityHit?: EntityCardHit;
}

const RECENT_LIMIT = 8;

/**
 * Which entity kind's exact-match search actually ran for the current query
 * — drives the shared "showing an exact match" banner. A typed query only
 * ever matches one of these shapes (CS/INC/PRB/CHG are distinct prefixes,
 * all followed by exactly 7 digits; CHAT is its own distinct prefix, just
 * without a fixed digit count — see `classifyConversationQuery`), so at most
 * one is ever non-null.
 */
type ExactMatchKind =
  | "caseNumber"
  | "caseInternalId"
  | "incidentNumber"
  | "problemNumber"
  | "changeRequestNumber"
  | "conversationNumber"
  | null;

const EXACT_MATCH_LABELS: Record<Exclude<ExactMatchKind, null>, string> = {
  caseNumber: "case number",
  caseInternalId: "WSO2 case id",
  incidentNumber: "incident number",
  problemNumber: "problem number",
  changeRequestNumber: "change request number",
  conversationNumber: "conversation number",
};

const isMac =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);

export default function QuickNav(): JSX.Element | null {
  const { isSignedIn } = useAsgardeo();
  const navigate = useNavTransition();
  const recents = useRecentViews();
  const { roles: userRoles } = usePermissions();
  // Shrink the closed trigger (not the open palette) once something is
  // pinned, so PinnedTabs — which shares the header's flexible middle slot —
  // has room to actually show the pinned chips instead of getting squeezed.
  const hasPinned = recents.some((e) => e.pinned);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  // Set only via the "Search in subject and description too" affordance below — opts a
  // query that matched one of the case/incident/problem/change-request/
  // conversation exact-match number (or WSO2-id) patterns back into
  // free-text search for all five entity searches at once. Cleared whenever
  // the query text itself changes, so widening one search never silently
  // sticks for the next one typed. Shared across all five rather than one
  // flag per type: a typed query only ever matches one of the five
  // exact-match shapes (they have distinct prefixes — CS/INC/PRB/CHG/CHAT —
  // so widening always targets exactly the one search that was actually
  // scoped; passing it to the other four hooks is a no-op since they're
  // already on the free-text path).
  const [forceFreeText, setForceFreeText] = useState(false);

  // Shareable-link support: `?q=` opens the palette pre-filled with a search
  // (e.g. a link like `/?q=CS0440883` someone pastes to a colleague), and
  // `?goto=` additionally auto-jumps into the single matching record once
  // its search settles, instead of leaving the user to click it themselves.
  const [searchParams, setSearchParams] = useSearchParams();
  const [gotoTarget, setGotoTarget] = useState<string | null>(null);
  const consumedInitialParams = useRef(false);

  // Debounce the text fed to the case-search API so each keystroke doesn't fire
  // a request; the in-memory pinned/recent/page matching still reacts instantly
  // to `query`.
  const debouncedQuery = useDebouncedValue(query, 180);
  const trimmedQuery = query.trim();
  // Case hits lag the input by the debounce window, so `caseSearch.data` can
  // describe a previous query. Only surface (and allow navigating to) hits once
  // the query the API actually ran matches what's typed now — otherwise stale
  // results stay clickable during the debounce window or after the input shrinks.
  const caseHitsSettled = trimmedQuery === debouncedQuery.trim();

  // Whether the query the API actually ran (the debounced one) would be
  // routed as an exact-match filter rather than free-text search, for each
  // of the five searchable entity kinds — drives the shared "showing an
  // exact match" banner below. Computed off the debounced (not live) query
  // so the banner only reflects the search that actually ran, not what's
  // mid-typing.
  const caseSearchScope = classifyQuickCaseQuery(debouncedQuery.trim());
  const incidentSearchScope = classifyQuickIncidentQuery(debouncedQuery.trim());
  const problemSearchScope = classifyQuickProblemQuery(debouncedQuery.trim());
  const changeRequestSearchScope = classifyQuickChangeRequestQuery(
    debouncedQuery.trim(),
  );
  const conversationSearchScope = classifyQuickConversationQuery(
    debouncedQuery.trim(),
  );

  // This picks whichever of the five searches would run in exact-match mode
  // (see {@link ExactMatchKind}), to drive both a single shared banner
  // instead of one per entity kind (the palette is one search box, not five)
  // and — below — which of the five hooks actually get to search at all.
  // Computed off the classifications above (themselves off the debounced
  // query), so this doesn't depend on any of the hooks' own results.
  const exactMatchKind: ExactMatchKind =
    caseSearchScope === "number"
      ? "caseNumber"
      : caseSearchScope === "internalId"
        ? "caseInternalId"
        : incidentSearchScope === "number"
          ? "incidentNumber"
          : problemSearchScope === "number"
            ? "problemNumber"
            : changeRequestSearchScope === "number"
              ? "changeRequestNumber"
              : conversationSearchScope === "number"
                ? "conversationNumber"
                : null;

  // Whether each of the five searches should actually run for the current
  // debounced query. When the query matches exactly one entity kind's
  // exact-match shape (`exactMatchKind !== null`), only that one kind's hook
  // runs — the other four would otherwise burn a free-text request against
  // entity types the query obviously doesn't belong to (e.g. typing a CHG
  // number shouldn't also run a case/incident/problem/conversation search for
  // the literal string "CHG0038721"). `forceFreeText` (the "Search in
  // subject and description too" widen affordance) overrides this for all
  // five at once, since widening is meant to broaden the search across every
  // entity kind, not just the one that was originally scoped.
  const caseSearchShouldRun =
    forceFreeText ||
    exactMatchKind === null ||
    exactMatchKind === "caseNumber" ||
    exactMatchKind === "caseInternalId";
  const incidentSearchShouldRun =
    forceFreeText || exactMatchKind === null || exactMatchKind === "incidentNumber";
  const problemSearchShouldRun =
    forceFreeText || exactMatchKind === null || exactMatchKind === "problemNumber";
  const changeRequestSearchShouldRun =
    forceFreeText ||
    exactMatchKind === null ||
    exactMatchKind === "changeRequestNumber";
  const conversationSearchShouldRun =
    forceFreeText ||
    exactMatchKind === null ||
    exactMatchKind === "conversationNumber";

  // API-backed case lookup: a CS/WSO2 id (or any subject text) resolves to real
  // cases. Routed to an exact-match field filter when the debounced query
  // matches the case-number/WSO2-id shape (see `classifyQuickCaseQuery`),
  // unless the user asked to widen it via `forceFreeText`. Passed an empty
  // query (same as when the palette is closed) when `caseSearchShouldRun` is
  // false — that's the same `enabled: q.length >= MIN_LEN` gate the hook
  // already uses to skip a request while the palette is closed, reused here
  // to skip it while the query belongs to a different entity kind.
  const caseSearch = useQuickCaseSearch(
    open && caseSearchShouldRun ? debouncedQuery : "",
    { forceFreeText },
  );

  // Same debounced query string fans out to the other searchable entity
  // kinds — one shared debounce (above) rather than each hook debouncing its
  // own copy, so a keystroke costs at most one query-string change, not five.
  // Each is routed to its own exact-match number filter the same way cases
  // are (see `classifyQuickIncidentQuery`/`classifyQuickProblemQuery`/
  // `classifyQuickChangeRequestQuery`/`classifyQuickConversationQuery`), also
  // gated on the same shared `forceFreeText` widen flag and the same
  // should-run suppression as cases above. Incidents/CRs/problems/
  // conversations are comparatively rare hits, so — unlike Cases — these
  // don't get a dedicated skeleton: their sections simply appear once data
  // lands, same as Pinned/Recent/Pages.
  const incidentSearch = useQuickIncidentSearch(
    open && incidentSearchShouldRun ? debouncedQuery : "",
    { forceFreeText },
  );
  const changeRequestSearch = useQuickChangeRequestSearch(
    open && changeRequestSearchShouldRun ? debouncedQuery : "",
    { forceFreeText },
  );
  const problemSearch = useQuickProblemSearch(
    open && problemSearchShouldRun ? debouncedQuery : "",
    { forceFreeText },
  );
  // Global (unscoped) conversation search — no `projectIds` is passed (see
  // `useQuickConversationSearch`), unlike the project Work-items "Chats"
  // tab's own search, which is fixed to the surrounding project.
  const conversationSearch = useQuickConversationSearch(
    open && conversationSearchShouldRun ? debouncedQuery : "",
    { forceFreeText },
  );

  // Only shown once every search this query feeds has actually settled (not
  // while any is still fetching) and hasn't already been widened by the
  // user.
  const showExactMatchBanner =
    !forceFreeText &&
    exactMatchKind !== null &&
    caseHitsSettled &&
    !caseSearch.isFetching &&
    !incidentSearch.isFetching &&
    !changeRequestSearch.isFetching &&
    !problemSearch.isFetching &&
    !conversationSearch.isFetching;
  // True while a case search is in flight (or its result is for a stale
  // query) — drives the "Cases" section's skeleton independently of whether
  // Pinned/Recent/Pages already have matches to show.
  const casesLoading =
    trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN &&
    (caseSearch.isFetching || !caseHitsSettled);
  // Skeleton only while there's nothing to show yet — `isFetching` also
  // covers a background refetch of an already-settled, already-rendered
  // query (e.g. re-typing a query after the 15s staleTime), where
  // `caseSearch.data` still holds the previous results. Without this,
  // the skeleton block and the real "Cases" section would render together.
  const showCasesSkeleton = casesLoading && !caseSearch.data;
  // True while any of the five searches this query feeds is actually
  // in-flight — used below to show a quiet "didn't match a known number
  // pattern" hint only for the duration of the wait, not once results have
  // settled (settled-with-nothing already gets the "No matches." empty
  // state, and settled-with-hits speaks for itself). A suppressed hook
  // (passed an empty query per `*SearchShouldRun` above) settles to
  // `isFetching === false` almost immediately, so this naturally reflects
  // only whichever search(es) the current query actually feeds — no
  // separate "which hooks are enabled" bookkeeping needed here.
  const anySearchFetching =
    caseSearch.isFetching ||
    incidentSearch.isFetching ||
    changeRequestSearch.isFetching ||
    problemSearch.isFetching ||
    conversationSearch.isFetching;
  // Gates the "No matches." empty state below. `casesLoading` alone isn't
  // enough here: it only reflects the case search, so when the query is
  // scoped to (say) a CHG number — case search suppressed entirely per
  // `caseSearchShouldRun` — `caseSearch.isFetching` goes false almost
  // immediately (it isn't even running), and "No matches." would render
  // while the change-request search that's actually relevant is still in
  // flight. Reuses `anySearchFetching` (any of the five still fetching,
  // which for a suppressed hook is trivially false) alongside the shared
  // `caseHitsSettled` debounce check, so the empty state waits for BOTH
  // "the debounce settled" AND "nothing still fetching" before it can
  // render — same two-part wait `showExactMatchBanner` already does below.
  const resultsSettling =
    trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN &&
    (!caseHitsSettled || anySearchFetching);
  // Passive, low-key heads-up shown for genuinely free-text queries (none of
  // the five exact-match patterns matched) — mutually exclusive with
  // `showExactMatchBanner` by construction: this requires
  // `exactMatchKind === null`, that requires `exactMatchKind !== null`.
  // Deliberately NOT gated on `anySearchFetching`/`resultsSettling`: it
  // needs to stay visible through a settled zero-hit result too (e.g.
  // typing "cs123" — doesn't match the exact-match shape, free-text search
  // finds nothing), not just disappear the moment the search finishes,
  // which would leave "No matches." with no explanation of why a plain-
  // looking query didn't get the fast exact-match path.
  //
  // Gated on the free-text path's minimum query length (all five hooks'
  // MIN_QUERY_LEN happen to be the same value, 2 — using the case one as
  // the representative constant), not just `> 0`: below that length every
  // hook is disabled (`enabled: q.length >= MIN_LEN`) and no search runs at
  // all, so the hint's "searched all fields" framing would be describing a
  // request that was never made.
  const showNoPatternHint =
    !forceFreeText &&
    exactMatchKind === null &&
    trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN;

  const inputRef = useRef<HTMLInputElement>(null);

  // Consume `?q=`/`?goto=` once per page load, not on every render — a
  // `setSearchParams` below removes them from the URL, and re-reading them
  // after that (e.g. from a stale closure) would just no-op harmlessly, but
  // this guard also stops a re-mount from re-opening the palette if the user
  // has already closed it once this session.
  useEffect(() => {
    if (!isSignedIn || consumedInitialParams.current) return;
    const q = searchParams.get("q");
    const goto = searchParams.get("goto");
    if (!q && !goto) return;
    consumedInitialParams.current = true;
    /* eslint-disable react-hooks/set-state-in-effect -- syncs palette state to an external one-shot source (the URL's initial q/goto params) */
    setOpen(true);
    setQuery(goto || q || "");
    if (goto) setGotoTarget(goto.trim());
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [isSignedIn, searchParams]);

  // ⌘K / Ctrl+K toggles the palette — only while signed in, so we don't hijack
  // the browser shortcut on the sign-in screen (where the palette can't render).
  useEffect(() => {
    if (!isSignedIn) return;
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [isSignedIn]);

  // Focus the input once the palette has mounted. `autoFocus` alone can lose
  // a focus-trap race against the Modal claiming focus on open, leaving the
  // palette open but requiring a second click before typing works.
  useEffect(() => {
    if (!open) return;
    const id = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, [open]);

  const results: Result[] = useMemo(() => {
    const q = trimmedQuery.toLowerCase();
    const match = (...parts: (string | undefined)[]) =>
      !q || parts.some((p) => p?.toLowerCase().includes(q));

    // Live case hits go first — when someone types a case id, the matching case
    // is the thing they want, ahead of pinned/recent/pages. Only shown once the
    // debounced query the API ran matches the current input, so stale hits never
    // stay clickable mid-typing.
    const cases: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN
        ? (caseSearch.data ?? []).map((c) => {
            const idLabel = caseIdLabel(c);
            return {
              key: `case-${c.id}`,
              icon: kindIcon("case", 16),
              label: idLabel || c.subject,
              sublabel: idLabel ? c.subject : undefined,
              href: `/cases/${c.id}`,
              section: "Cases" as const,
              caseHit: c,
            };
          })
        : [];

    // Same "only once the debounce settled" gating as Cases above, so a
    // stale incident/CR/problem hit never stays clickable mid-typing either.
    const incidents: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_INCIDENT_MIN_QUERY_LEN
        ? (incidentSearch.data ?? []).map((i) => ({
            key: `incident-${i.id}`,
            icon: kindIcon("incident", 16),
            label: i.number || i.subject,
            sublabel: i.number ? i.subject : undefined,
            href: `/operations/incidents/${i.id}`,
            section: "Incidents" as const,
            entityHit: {
              icon: kindIcon("incident", 16),
              idLabel: i.number,
              subject: i.subject,
              state: i.state,
              assigneeName: i.assigneeName,
            },
          }))
        : [];

    const changeRequests: Result[] =
      caseHitsSettled &&
      trimmedQuery.length >= QUICK_CHANGE_REQUEST_MIN_QUERY_LEN
        ? (changeRequestSearch.data ?? []).map((cr) => ({
            key: `cr-${cr.id}`,
            icon: kindIcon("change_request", 16),
            label: cr.number || cr.subject,
            sublabel: cr.number ? cr.subject : undefined,
            href: `/operations/change-requests/${cr.id}`,
            section: "Change Requests" as const,
            entityHit: {
              icon: kindIcon("change_request", 16),
              idLabel: cr.number,
              subject: cr.subject,
              state: cr.state,
              assigneeName: cr.assigneeName,
            },
          }))
        : [];

    const problems: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_PROBLEM_MIN_QUERY_LEN
        ? (problemSearch.data ?? []).map((p) => ({
            key: `problem-${p.id}`,
            icon: kindIcon("problem", 16),
            label: p.number || p.subject,
            sublabel: p.number ? p.subject : undefined,
            href: `/operations/problems/${p.id}`,
            section: "Problems" as const,
            entityHit: {
              icon: kindIcon("problem", 16),
              idLabel: p.number,
              subject: p.subject,
              state: p.state,
              assigneeName: p.assigneeName,
            },
          }))
        : [];

    // Conversations render as a plain `EntityCardHit` row too, same as
    // Incidents/Change Requests/Problems above — no case-style rich card.
    // The "subject" line is whatever's most useful to identify the chat by:
    // who started it if known, otherwise a truncated initial message (the
    // data source doesn't always resolve an initiator to a name/email).
    const conversations: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_CONVERSATION_MIN_QUERY_LEN
        ? (conversationSearch.data ?? []).map((c) => {
            const truncatedMessage =
              c.initialMessage && c.initialMessage.length > 80
                ? `${c.initialMessage.slice(0, 80)}…`
                : c.initialMessage;
            const subject = c.initiatorName
              ? `Started by ${c.initiatorName}`
              : truncatedMessage || "(no initial message)";
            return {
              key: `conversation-${c.id}`,
              icon: <MessageSquare size={16} />,
              label: c.number || subject,
              sublabel: c.number ? subject : undefined,
              href: `/conversations/${c.id}`,
              section: "Conversations" as const,
              entityHit: {
                icon: <MessageSquare size={16} />,
                idLabel: c.number,
                subject,
              },
            };
          })
        : [];

    // A pinned/recent entry for a case carries a severity/status snapshot
    // from when it was last visited — render it as the same rich card a live
    // case search hit gets, instead of a plain icon+label row.
    const toCaseHit = (e: RecentView): QuickCaseHit | undefined =>
      e.kind === "case" && e.caseHit ? { id: e.id, ...e.caseHit } : undefined;

    const pinned: Result[] = recents
      .filter((e) => e.pinned)
      .filter((e) => match(e.title, e.subtitle))
      .map((e) => ({
        key: `pin-${e.kind}-${e.id}`,
        icon: kindIcon(e.kind, 16),
        label: e.title,
        sublabel: e.subtitle,
        href: e.href,
        section: "Pinned",
        caseHit: toCaseHit(e),
      }));

    const recent: Result[] = recents
      .filter((e) => !e.pinned)
      .filter((e) => match(e.title, e.subtitle))
      .slice(0, RECENT_LIMIT)
      .map((e) => ({
        key: `rec-${e.kind}-${e.id}`,
        icon: kindIcon(e.kind, 16),
        label: e.title,
        sublabel: e.subtitle,
        href: e.href,
        section: "Recents",
        caseHit: toCaseHit(e),
      }));

    // Pages are worth surfacing when someone types a page name to jump
    // straight there, but listing every sidebar page on the empty-query
    // default view just duplicates the sidebar itself — so only show this
    // section once there's something to match against. Second-level tabs are
    // offered too (matching on either the tab or its section name), so
    // "incidents" jumps straight into the tab rather than to Operations.
    const pages: Result[] = q
      ? navigableNavNodes()
          .filter((i) => {
            const originalNode = navNodeById(i.id);
            if (originalNode && !isNavNodeAuthorized(originalNode, userRoles)) {
              return false;
            }
            return match(i.label, i.sublabel);
          })
          .map((i) => ({
            key: `page-${i.id}`,
            icon: <i.icon size={16} />,
            label: i.label,
            sublabel: i.sublabel,
            href: i.href,
            section: "Pages" as const,
          }))
      : [];

    return [
      ...cases,
      ...incidents,
      ...changeRequests,
      ...problems,
      ...conversations,
      ...pinned,
      ...recent,
      ...pages,
    ];
  }, [
    recents,
    trimmedQuery,
    caseHitsSettled,
    caseSearch.data,
    incidentSearch.data,
    changeRequestSearch.data,
    problemSearch.data,
    conversationSearch.data,
    userRoles,
  ]);

  // Clamp at render so a stale index from shrinking results never points past
  // the end (avoids a setState-in-effect cascade).
  const safeActive = results.length ? Math.min(active, results.length - 1) : 0;

  // Strips `q`/`goto` from the URL if either is present. Called whenever the
  // palette stops being the reason `/` shouldn't redirect to `/dashboard`
  // yet (see `RootLanding` in App.tsx) — on manual close without picking a
  // result, and (separately, in the goto-resolution effect below) once a
  // `?goto=` link actually resolves to a single match and navigates away.
  const clearDeepLinkParams = () => {
    if (!searchParams.has("q") && !searchParams.has("goto")) return;
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete("q");
        next.delete("goto");
        return next;
      },
      { replace: true },
    );
  };

  const close = () => {
    setOpen(false);
    setQuery("");
    setActive(0);
    setForceFreeText(false);
    setGotoTarget(null);
    clearDeepLinkParams();
  };

  const choose = (r: Result | undefined) => {
    if (!r) return;
    close();
    navigate(r.href);
  };

  // `?goto=` resolution: once every search this query feeds has settled (not
  // just the case search — an incident/CR/problem/conversation number should
  // auto-jump too), look for an exact (case-insensitive) match on whichever
  // identifier a person would actually paste into a link — a display number
  // (`CS0440883`/`INC0012345`/`CHAT0000012345`/...), the internal WSO2 case
  // id, or a raw record id — across all five result sets combined. Exactly
  // one match navigates straight there; zero or multiple matches leave the
  // palette open on its normal search results so the user can pick, since a
  // forced jump would be wrong (or arbitrary) either way.
  useEffect(() => {
    if (!gotoTarget) return;
    const allSettled =
      caseHitsSettled &&
      !caseSearch.isFetching &&
      !incidentSearch.isFetching &&
      !changeRequestSearch.isFetching &&
      !problemSearch.isFetching &&
      !conversationSearch.isFetching;
    if (!allSettled) return;

    const target = gotoTarget.toLowerCase();
    const matchesTarget = (...ids: (string | null | undefined)[]) =>
      ids.some((id) => id?.toLowerCase() === target);

    const caseHref = (caseSearch.data ?? []).find((c) =>
      matchesTarget(c.id, c.caseNumber, c.wso2CaseId),
    );
    const incidentHref = (incidentSearch.data ?? []).find((i) =>
      matchesTarget(i.id, i.number),
    );
    const crHref = (changeRequestSearch.data ?? []).find((cr) =>
      matchesTarget(cr.id, cr.number),
    );
    const problemHref = (problemSearch.data ?? []).find((p) =>
      matchesTarget(p.id, p.number),
    );
    const conversationHref = (conversationSearch.data ?? []).find((c) =>
      matchesTarget(c.id, c.number),
    );

    const matches = [
      caseHref && `/cases/${caseHref.id}`,
      incidentHref && `/operations/incidents/${incidentHref.id}`,
      crHref && `/operations/change-requests/${crHref.id}`,
      problemHref && `/operations/problems/${problemHref.id}`,
      conversationHref && `/conversations/${conversationHref.id}`,
    ].filter((href): href is string => !!href);

    /* eslint-disable react-hooks/set-state-in-effect -- syncs palette state to the external goto/search resolution outcome, a one-shot action once the search settles */
    setGotoTarget(null);

    if (matches.length === 1) {
      // Only clear `q`/`goto` from the URL — and only here, on the single-
      // match path. Leaving them in place for the 0-or-multiple-match case
      // below is deliberate: RootLanding (App.tsx) reads their presence to
      // keep deferring its `/dashboard` redirect, so the background stays
      // blank behind the still-open palette instead of a dashboard loading
      // in underneath it. They get cleared once the user either picks a
      // result (route changes away from `/` entirely) or closes the palette
      // manually (see `close()`, which calls `clearDeepLinkParams`).
      close();
      navigate(matches[0]);
    }
    /* eslint-enable react-hooks/set-state-in-effect */
    // matches.length === 0 or > 1: leave the palette open on its normal
    // search results rather than guessing which one was meant.
  }, [
    gotoTarget,
    caseHitsSettled,
    caseSearch.isFetching,
    caseSearch.data,
    incidentSearch.isFetching,
    incidentSearch.data,
    changeRequestSearch.isFetching,
    changeRequestSearch.data,
    problemSearch.isFetching,
    problemSearch.data,
    conversationSearch.isFetching,
    conversationSearch.data,
    navigate,
    setSearchParams,
  ]);

  const onListKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive(results.length ? (safeActive + 1) % results.length : 0);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive(
        results.length ? (safeActive - 1 + results.length) % results.length : 0,
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(results[safeActive]);
    }
  };

  if (!isSignedIn) return null;

  const shortcut = isMac ? "⌘K" : "Ctrl K";

  return (
    <>
      <ButtonBase
        onClick={() => setOpen(true)}
        aria-label="Search or jump to (open quick nav)"
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          height: 36,
          width: hasPinned
            ? { xs: 40, sm: 260, md: 340, lg: 420 }
            : { xs: 40, sm: 340, md: 460, lg: 600 },
          px: { xs: 0, sm: 1.25 },
          justifyContent: { xs: "center", sm: "flex-start" },
          borderRadius: 1,
          border: 1,
          borderColor: "divider",
          color: "text.secondary",
          flexShrink: 0,
          "&:hover": { bgcolor: "action.hover" },
        }}
      >
        <Search size={16} />
        <Typography
          variant="body2"
          noWrap
          sx={{ flex: 1, textAlign: "left", display: { xs: "none", sm: "block" } }}
        >
          Search or jump to…
        </Typography>
        <Box
          component="span"
          sx={{
            display: { xs: "none", sm: "block" },
            fontSize: 11,
            px: 0.5,
            borderRadius: 0.5,
            border: 1,
            borderColor: "divider",
            color: "text.secondary",
          }}
        >
          {shortcut}
        </Box>
      </ButtonBase>

      {/*
        A `Dialog`'s paper is deliberately styled by the theme with a more
        opaque background + heavier blur, so a modal reads clearly over a
        dimmed page. Customer-portal's search dropdown isn't a Dialog at all
        — it's a plain `Paper` (oxygen-ui's `MuiPaper.styleOverrides.root`
        gives it the lighter, translucent "acrylic" background + a light
        blur + a divider border for free). Using `Modal` + `Paper` here
        — the same primitives, from the same "@wso2/oxygen-ui" import —
        gets the identical glassy look instead of fighting Dialog's styling.
      */}
      <Modal
        open={open}
        onClose={close}
        slotProps={{ backdrop: { sx: { backgroundColor: "transparent" } } }}
      >
        <Paper
          elevation={3}
          sx={{
            position: "fixed",
            top: "10vh",
            left: "50%",
            transform: "translateX(-50%)",
            width: { xs: "calc(100% - 32px)", sm: "calc(100% - 64px)" },
            maxWidth: 760,
            maxHeight: "65vh",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
            outline: "none",
          }}
        >
          <Box
            onKeyDown={onListKeyDown}
            sx={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                px: 2,
                py: 1.5,
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Search size={18} />
              <InputBase
                autoFocus
                inputRef={inputRef}
                fullWidth
                placeholder="Search cases, incidents, change requests, problems, conversations, or jump to pinned, recent, pages…"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setActive(0);
                  setForceFreeText(false);
                }}
                inputProps={{ "aria-label": "Quick nav search" }}
              />
            </Box>

            <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0, p: 2 }}>
              {showCasesSkeleton && (
                <Box sx={{ mb: results.length ? 2 : 0 }}>
                  <Typography
                    variant="subtitle2"
                    color="text.secondary"
                    sx={{ display: "block", pb: 0.75, fontWeight: 600 }}
                  >
                    Cases
                  </Typography>
                  <QuickNavResultSkeleton count={3} />
                </Box>
              )}
              {results.length === 0 ? (
                resultsSettling ? null : (
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      alignItems: "center",
                      py: 3,
                    }}
                  >
                    <SearchNoResultsIcon
                      style={{ width: 140, height: "auto", marginBottom: 12 }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      {trimmedQuery.length === 0
                        ? "Nothing pinned or recent yet. Start typing to search."
                        : "No matches."}
                    </Typography>
                  </Box>
                )
              ) : (
                results.map((r, i) => {
                  const newSection = i === 0 || results[i - 1].section !== r.section;
                  return (
                    <Box key={r.key} sx={{ mt: newSection && i !== 0 ? 2 : 0 }}>
                      {newSection && (
                        <Typography
                          variant="subtitle2"
                          color="text.secondary"
                          sx={{
                            display: "block",
                            pb: 0.75,
                            fontWeight: 600,
                          }}
                        >
                          {r.section}
                        </Typography>
                      )}
                      {r.caseHit ? (
                        <Box sx={{ pb: 1 }}>
                          <QuickNavCaseCard
                            hit={r.caseHit}
                            active={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                          />
                        </Box>
                      ) : r.entityHit ? (
                        <Box sx={{ pb: 1 }}>
                          <QuickNavEntityCard
                            icon={r.entityHit.icon}
                            idLabel={r.entityHit.idLabel}
                            subject={r.entityHit.subject}
                            state={r.entityHit.state}
                            assigneeName={r.entityHit.assigneeName}
                            active={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                          />
                        </Box>
                      ) : (
                        <Box sx={{ pb: 1 }}>
                          <Form.CardButton
                            selected={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                            sx={{
                              display: "flex",
                              flexDirection: "row",
                              alignItems: "center",
                              gap: 1.5,
                              p: 1.25,
                              width: "100%",
                              minWidth: 0,
                            }}
                          >
                            {r.icon}
                            <Box sx={{ flex: 1, minWidth: 0 }}>
                              <Typography variant="body2" noWrap>
                                {r.label}
                              </Typography>
                              {r.sublabel && (
                                <Typography
                                  variant="caption"
                                  color="text.secondary"
                                  noWrap
                                >
                                  {r.sublabel}
                                </Typography>
                              )}
                            </Box>
                          </Form.CardButton>
                        </Box>
                      )}
                    </Box>
                  );
                })
              )}
              {/*
                Both the exact-match banner and the no-pattern-match hint
                render below every result section, not above them — they're
                a footnote about how the search ran, not something that
                should push the actual results down the page. They're
                mutually exclusive by construction (`showExactMatchBanner`
                requires `exactMatchKind !== null`, `showNoPatternHint`
                requires it to be `null`), so at most one renders here.
              */}
              {showExactMatchBanner && exactMatchKind && (
                <Alert severity="info" sx={{ mt: results.length ? 2 : 0 }}>
                  {/* The widen button sits on its own row below the message,
                      not MUI's inline `action` slot — "Search in subject
                      and description too" is long enough that the inline
                      layout wrapped the message and button onto two
                      cramped, misaligned rows instead. */}
                  <Box>
                    Showing an exact match for {EXACT_MATCH_LABELS[exactMatchKind]}
                    &nbsp;"{debouncedQuery.trim()}".
                  </Box>
                  <Button
                    size="small"
                    sx={{ mt: 1 }}
                    onClick={() => setForceFreeText(true)}
                  >
                    Search in subject and description too
                  </Button>
                </Alert>
              )}
              {showNoPatternHint && (
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{
                    display: "block",
                    mt: results.length ? 2 : 0,
                    textAlign: "center",
                  }}
                >
                  Didn&apos;t match a known number pattern —{" "}
                  {anySearchFetching
                    ? "searching all fields, this may take a moment."
                    : "searched all fields."}
                </Typography>
              )}
            </Box>
          </Box>
        </Paper>
      </Modal>
    </>
  );
}
