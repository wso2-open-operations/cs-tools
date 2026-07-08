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
  Dialog,
  InputBase,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";

import { navigableNavItems } from "@config/csmNavItems";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useRecentViews } from "@features/csm-recent/hooks/useRecentViews";
import { kindIcon } from "@features/csm-recent/kindMeta";
import {
  QUICK_CASE_MIN_QUERY_LEN,
  useQuickCaseSearch,
} from "@features/csm-cases/api/useQuickCaseSearch";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useNavTransition } from "@hooks/useNavTransition";

type Section = "Cases" | "Pinned" | "Recent" | "Pages";

interface Result {
  key: string;
  icon: JSX.Element;
  label: string;
  sublabel?: string;
  href: string;
  section: Section;
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
            };
          })
        : [];

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
        section: "Recent",
      }));

    const pages: Result[] = navigableNavItems()
      .filter((i) => match(i.label))
      .map((i) => ({
        key: `page-${i.id}`,
        icon: <i.icon size={16} />,
        label: i.label,
        href: i.path,
        section: "Pages",
      }));

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
          width: { xs: 40, sm: 200, md: 260 },
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

      <Dialog
        open={open}
        onClose={close}
        fullWidth
        maxWidth="sm"
        sx={{ "& .MuiDialog-container": { alignItems: "flex-start" } }}
        slotProps={{ paper: { sx: { mt: "12vh", overflow: "hidden" } } }}
      >
        <Box
          onKeyDown={onListKeyDown}
          sx={{ display: "flex", flexDirection: "column", maxHeight: "60vh" }}
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

          <Box sx={{ overflowY: "auto" }}>
            {results.length === 0 ? (
              <Box sx={{ px: 2, py: 3, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  {trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN &&
                  (caseSearch.isFetching || !caseHitsSettled)
                    ? "Searching cases…"
                    : "No matches."}
                </Typography>
              </Box>
            ) : (
              results.map((r, i) => {
                const newSection = i === 0 || results[i - 1].section !== r.section;
                return (
                  <Box key={r.key}>
                    {newSection && (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{
                          display: "block",
                          px: 2,
                          pt: 1,
                          pb: 0.5,
                          fontWeight: 600,
                        }}
                      >
                        {r.section}
                      </Typography>
                    )}
                    <Box
                      role="button"
                      tabIndex={-1}
                      onMouseEnter={() => setActive(i)}
                      onClick={() => choose(r)}
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1.5,
                        px: 2,
                        py: 1,
                        cursor: "pointer",
                        bgcolor: i === safeActive ? "action.selected" : undefined,
                      }}
                    >
                      {r.icon}
                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Typography variant="body2" noWrap>
                          {r.label}
                        </Typography>
                        {r.sublabel && (
                          <Typography variant="caption" color="text.secondary" noWrap>
                            {r.sublabel}
                          </Typography>
                        )}
                      </Box>
                    </Box>
                  </Box>
                );
              })
            )}
          </Box>
        </Box>
      </Dialog>
    </>
  );
}
