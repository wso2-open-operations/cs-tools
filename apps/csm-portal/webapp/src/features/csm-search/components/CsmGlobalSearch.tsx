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
  Chip,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Building,
  FolderOpen,
  Headset,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent,
} from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  MIN_QUERY_LENGTH,
  useGetCsmSearch,
} from "@features/csm-search/api/useGetCsmSearch";
import type { CsmSearchHit } from "@features/csm-search/types/csmSearch";

const SEARCH_DEBOUNCE_MS = 200;
const DROPDOWN_WIDTH = 480;

function severityChipColor(
  severity: string,
): "error" | "warning" | "info" | "default" {
  if (severity === "S0" || severity === "S1") return "error";
  if (severity === "S2") return "warning";
  if (severity === "S3") return "info";
  return "default";
}

function tierChipColor(
  tier: string,
): "primary" | "warning" | "default" {
  if (tier === "Platinum") return "primary";
  if (tier === "Gold") return "warning";
  return "default";
}

function HitRow({
  hit,
  active,
  onMouseEnter,
  onClick,
}: {
  hit: CsmSearchHit;
  active: boolean;
  onMouseEnter: () => void;
  onClick: () => void;
}): JSX.Element {
  const Icon =
    hit.kind === "case" ? Headset : hit.kind === "project" ? FolderOpen : Building;
  return (
    <Box
      role="option"
      aria-selected={active}
      onMouseEnter={onMouseEnter}
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClick}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        px: 1.5,
        py: 1,
        cursor: "pointer",
        bgcolor: active ? "action.hover" : "transparent",
        "&:hover": { bgcolor: "action.hover" },
      }}
    >
      <Icon size={16} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body2" noWrap>
          {hit.title}
        </Typography>
        {hit.subtitle && (
          <Typography variant="caption" color="text.secondary" noWrap>
            {hit.subtitle}
          </Typography>
        )}
      </Box>
      {hit.badge && (
        <Chip
          size="small"
          variant="outlined"
          label={hit.badge}
          color={
            hit.kind === "case"
              ? severityChipColor(hit.badge)
              : hit.kind === "project" || hit.kind === "account"
                ? tierChipColor(hit.badge)
                : "default"
          }
        />
      )}
    </Box>
  );
}

function GroupHeader({ label, count }: { label: string; count: number }): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        px: 1.5,
        py: 0.5,
        bgcolor: "background.default",
      }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
        {label}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {count}
      </Typography>
    </Box>
  );
}

/**
 * Global search box shown in the top bar.
 *
 * Searches across cases (by case-id, WSO2 case number, subject), projects
 * (by id/key, name) and accounts (by name). Results render in a portalled
 * dropdown anchored to the input, grouped by entity type. Click → navigate
 * to the corresponding detail page.
 */
export default function CsmGlobalSearch(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const debounced = useDebouncedValue(input, SEARCH_DEBOUNCE_MS);
  const anchorRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [anchorRect, setAnchorRect] = useState<DOMRect | null>(null);

  const { data, isFetching } = useGetCsmSearch(debounced);

  const flatHits = useMemo<CsmSearchHit[]>(() => {
    if (!data) return [];
    return [...data.cases, ...data.projects, ...data.accounts];
  }, [data]);

  const queryTooShort = debounced.trim().length < MIN_QUERY_LENGTH;
  const showDropdown = open && input.trim().length > 0;

  const updateAnchorRect = useCallback(() => {
    if (anchorRef.current) {
      setAnchorRect(anchorRef.current.getBoundingClientRect());
    }
  }, []);

  useLayoutEffect(() => {
    updateAnchorRect();
  }, [showDropdown, updateAnchorRect]);

  useEffect(() => {
    if (!showDropdown) return;
    const handle = () => updateAnchorRect();
    window.addEventListener("resize", handle);
    window.addEventListener("scroll", handle, true);
    return () => {
      window.removeEventListener("resize", handle);
      window.removeEventListener("scroll", handle, true);
    };
  }, [showDropdown, updateAnchorRect]);

  // Reset highlight when results change.
  useEffect(() => {
    setActiveIndex(0);
  }, [debounced]);

  const navigateToHit = useCallback(
    (hit: CsmSearchHit) => {
      navigate(hit.href);
      setOpen(false);
      setInput("");
    },
    [navigate],
  );

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!showDropdown || flatHits.length === 0) {
      if (e.key === "Escape") setOpen(false);
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => (i + 1) % flatHits.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => (i - 1 + flatHits.length) % flatHits.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const hit = flatHits[activeIndex];
      if (hit) navigateToHit(hit);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  let runningIndex = 0;
  const groups: Array<{ label: string; hits: CsmSearchHit[] }> = [];
  if (data) {
    if (data.cases.length) groups.push({ label: "Cases", hits: data.cases });
    if (data.projects.length)
      groups.push({ label: "Projects", hits: data.projects });
    if (data.accounts.length)
      groups.push({ label: "Accounts", hits: data.accounts });
  }
  const totalHits = flatHits.length;

  const dropdown = showDropdown && anchorRect && (
    <Paper
      elevation={6}
      sx={{
        position: "fixed",
        top: anchorRect.bottom + 6,
        left: anchorRect.left,
        width: Math.max(anchorRect.width, DROPDOWN_WIDTH),
        maxHeight: 480,
        overflowY: "auto",
        zIndex: theme.zIndex.modal + 1,
        border: 1,
        borderColor: "divider",
      }}
      role="listbox"
    >
      {queryTooShort && (
        <Box sx={{ px: 1.5, py: 1.5 }}>
          <Typography variant="caption" color="text.secondary">
            Type at least {MIN_QUERY_LENGTH} characters to search.
          </Typography>
        </Box>
      )}
      {!queryTooShort && isFetching && totalHits === 0 && (
        <Box sx={{ px: 1.5, py: 1.5 }}>
          <Typography variant="caption" color="text.secondary">
            Searching…
          </Typography>
        </Box>
      )}
      {!queryTooShort && !isFetching && totalHits === 0 && (
        <Box sx={{ px: 1.5, py: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            No matches for &ldquo;{debounced}&rdquo;.
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Searches case id, WSO2 case number, project id/name, account name, case subject.
          </Typography>
        </Box>
      )}
      {!queryTooShort &&
        groups.map((group, gi) => (
          <Box key={group.label}>
            {gi > 0 && (
              <Box sx={{ borderTop: 1, borderColor: "divider" }} />
            )}
            <GroupHeader label={group.label} count={group.hits.length} />
            {group.hits.map((hit) => {
              const idx = runningIndex++;
              return (
                <HitRow
                  key={hit.id}
                  hit={hit}
                  active={idx === activeIndex}
                  onMouseEnter={() => setActiveIndex(idx)}
                  onClick={() => navigateToHit(hit)}
                />
              );
            })}
          </Box>
        ))}
    </Paper>
  );

  return (
    <Box
      ref={anchorRef}
      onKeyDown={handleKeyDown}
      sx={{ ml: 2, width: { xs: 220, sm: 320, md: DROPDOWN_WIDTH }, maxWidth: "100%" }}
    >
      <TextField
        size="small"
        fullWidth
        inputRef={inputRef}
        value={input}
        onChange={(e) => {
          setInput(e.target.value);
          if (!open) setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => {
          // Defer close so click handlers on the dropdown can fire first.
          window.setTimeout(() => setOpen(false), 120);
        }}
        placeholder="Search cases, projects, accounts…"
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
            endAdornment: input ? (
              <InputAdornment position="end">
                <IconButton
                  size="small"
                  aria-label="Clear search"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    setInput("");
                    inputRef.current?.focus();
                  }}
                >
                  <X size={14} />
                </IconButton>
              </InputAdornment>
            ) : undefined,
          },
        }}
      />
      {dropdown && createPortal(dropdown, document.body)}
    </Box>
  );
}
