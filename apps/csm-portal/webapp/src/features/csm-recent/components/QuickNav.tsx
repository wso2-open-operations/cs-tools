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
  Box,
  ButtonBase,
  Form,
  InputBase,
  Modal,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";

import { navigableNavItems } from "@config/csmNavItems";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  useRecentViews,
  type RecentView,
} from "@features/csm-recent/hooks/useRecentViews";
import { kindIcon } from "@features/csm-recent/kindMeta";
import QuickNavCaseCard from "@features/csm-recent/components/QuickNavCaseCard";
import QuickNavResultSkeleton from "@features/csm-recent/components/QuickNavResultSkeleton";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import {
  QUICK_CASE_MIN_QUERY_LEN,
  useQuickCaseSearch,
  type QuickCaseHit,
} from "@features/csm-cases/api/useQuickCaseSearch";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useNavTransition } from "@hooks/useNavTransition";

type Section = "Cases" | "Pinned" | "Recents" | "Pages";

interface Result {
  key: string;
  icon: JSX.Element;
  label: string;
  sublabel?: string;
  href: string;
  section: Section;
  /** Present only for "Cases" results — renders as a rich card instead of a plain row. */
  caseHit?: QuickCaseHit;
}

const RECENT_LIMIT = 8;

const isMac =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);

export default function QuickNav(): JSX.Element | null {
  const { isSignedIn } = useAsgardeo();
  const navigate = useNavTransition();
  const recents = useRecentViews();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);

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

  // API-backed case lookup: a CS/WSO2 id (or any subject text) resolves to real
  // cases. Disabled until the query is long enough (see the hook).
  const caseSearch = useQuickCaseSearch(open ? debouncedQuery : "");
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

  const inputRef = useRef<HTMLInputElement>(null);

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
    // section once there's something to match against.
    const pages: Result[] = q
      ? navigableNavItems()
          .filter((i) => match(i.label))
          .map((i) => ({
            key: `page-${i.id}`,
            icon: <i.icon size={16} />,
            label: i.label,
            href: i.path,
            section: "Pages" as const,
          }))
      : [];

    return [...cases, ...pinned, ...recent, ...pages];
  }, [recents, trimmedQuery, caseHitsSettled, caseSearch.data]);

  // Clamp at render so a stale index from shrinking results never points past
  // the end (avoids a setState-in-effect cascade).
  const safeActive = results.length ? Math.min(active, results.length - 1) : 0;

  const close = () => {
    setOpen(false);
    setQuery("");
    setActive(0);
  };

  const choose = (r: Result | undefined) => {
    if (!r) return;
    close();
    navigate(r.href);
  };

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
          width: { xs: 40, sm: 340, md: 460, lg: 600 },
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
                placeholder="Search cases by id, or jump to pinned, recent, pages…"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setActive(0);
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
                casesLoading ? null : (
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
                        ? "Nothing pinned or recent yet. Start typing to search cases."
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
            </Box>
          </Box>
        </Paper>
      </Modal>
    </>
  );
}
