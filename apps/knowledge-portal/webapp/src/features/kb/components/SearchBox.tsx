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
  IconButton,
  InputAdornment,
  LinearProgress,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Paper,
  Popper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { FileText, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useId, useRef, useState, type FormEvent, type JSX, type KeyboardEvent } from "react";
import { useNavigate } from "react-router";
import { MAX_SEARCH_QUERY_LENGTH, type KBArticle } from "@api/backend/types";
import { MIN_SUGGESTION_CHARS, useKnowledgeBases, useSearchSuggestions } from "@api/useKb";
import Highlight from "@components/Highlight";
import { opaquePaper } from "@config/colorUtils";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { articleUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";

/** Marks every search input, so the "/" shortcut can find the first one on the page. */
export const SEARCH_INPUT_ATTR = "data-kb-search";

const DEBOUNCE_MS = 250;

interface SearchBoxProps {
  /** Accessible name; there is no visible label. */
  label: string;
  placeholder: string;
  initialValue?: string;
  large?: boolean;
  onSearch: (query: string) => void;
  /** Show matching articles while typing. On by default. */
  suggestions?: boolean;
}

type Option = { kind: "article"; article: KBArticle } | { kind: "all" };

/**
 * Search form with type-ahead article suggestions. Submitting runs a full
 * search; picking a suggestion opens that article directly. Remount with a
 * new `key` to reset it to a new initial value.
 */
export default function SearchBox({
  label,
  placeholder,
  initialValue = "",
  large,
  onSearch,
  suggestions = true,
}: SearchBoxProps): JSX.Element {
  const [draft, setDraft] = useState(initialValue);
  const [focused, setFocused] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  // The form is the popper anchor; held in state (not a ref) so it can be
  // read during render.
  const [anchorEl, setAnchorEl] = useState<HTMLFormElement | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listId = useId();
  const navigate = useNavigate();

  const typed = normalizeSearchQuery(draft);
  const debounced = useDebouncedValue(typed, DEBOUNCE_MS);
  // Suggestions only for something the visitor typed here, not for the query
  // the page was opened with: those results are already on the page.
  const suggestActive =
    suggestions &&
    focused &&
    !dismissed &&
    typed.length >= MIN_SUGGESTION_CHARS &&
    typed !== normalizeSearchQuery(initialValue);
  const results = useSearchSuggestions(debounced, suggestActive);
  const { data: kbData } = useKnowledgeBases();
  const kbTitle = (id: string): string | undefined => kbData?.knowledgeBases.find((kb) => kb.id === id)?.title;

  // Options come only from results for the current draft. While the debounce
  // or the request is still catching up, the previous query's results (kept as
  // placeholder data) must not be selectable, or Enter could open an article
  // that matched an earlier draft.
  const resultsCurrent = typed === debounced && !results.isPlaceholderData;
  const articles =
    suggestActive && resultsCurrent && debounced.length >= MIN_SUGGESTION_CHARS ? (results.data?.articles ?? []) : [];
  const options: Option[] = articles.length > 0 ? [...articles.map((article) => ({ kind: "article" as const, article })), { kind: "all" }] : [];
  const settledEmpty = suggestActive && resultsCurrent && results.isSuccess && !results.isFetching && articles.length === 0;
  // Stay open (showing "Searching") while catching up, so the list does not flicker closed between keystrokes.
  const open = suggestActive && (options.length > 0 || results.isFetching || settledEmpty || !resultsCurrent);
  const optionId = (index: number): string => `${listId}-option-${index}`;

  const close = (): void => {
    setDismissed(true);
    setActiveIndex(-1);
  };
  const change = (value: string): void => {
    setDraft(value);
    setDismissed(false);
    setActiveIndex(-1);
  };
  const choose = (option: Option | undefined): void => {
    close();
    if (option?.kind === "article") {
      navigate(articleUrl(option.article.id));
      return;
    }
    onSearch(draft);
  };
  const submit = (event: FormEvent): void => {
    event.preventDefault();
    choose(open && activeIndex >= 0 ? options[activeIndex] : undefined);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>): void => {
    if (!open) {
      if (event.key === "Escape" && draft) change("");
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => (options.length === 0 ? -1 : (index + 1) % options.length));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => (options.length === 0 ? -1 : index <= 0 ? options.length - 1 : index - 1));
    } else if (event.key === "Escape") {
      event.preventDefault();
      close();
    }
  };

  return (
    <Box component="form" role="search" ref={setAnchorEl} onSubmit={submit} sx={{ width: "100%", position: "relative" }}>
      <TextField
        fullWidth
        type="search"
        size={large ? "medium" : "small"}
        value={draft}
        onChange={(event) => change(event.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={onKeyDown}
        placeholder={placeholder}
        inputRef={inputRef}
        slotProps={{
          htmlInput: {
            "aria-label": label,
            maxLength: MAX_SEARCH_QUERY_LENGTH,
            [SEARCH_INPUT_ATTR]: "",
            autoComplete: "off",
            enterKeyHint: "search",
            "aria-autocomplete": suggestions ? "list" : undefined,
            "aria-controls": open ? listId : undefined,
            "aria-activedescendant": open && activeIndex >= 0 ? optionId(activeIndex) : undefined,
          },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={large ? 20 : 18} aria-hidden />
              </InputAdornment>
            ),
            endAdornment: (
              <InputAdornment position="end">
                {draft ? (
                  <IconButton
                    size="small"
                    edge="end"
                    aria-label="Clear search"
                    onClick={() => {
                      change("");
                      inputRef.current?.focus();
                    }}
                  >
                    <X size={16} aria-hidden />
                  </IconButton>
                ) : null}
              </InputAdornment>
            ),
            sx: large
              ? { py: 0.5, fontSize: "1.05rem", borderRadius: 2.5, bgcolor: "background.paper", boxShadow: 2 }
              : { borderRadius: 2 },
          },
        }}
        sx={{ "& input[type=search]::-webkit-search-cancel-button": { display: "none" } }}
      />

      <Popper
        open={open}
        anchorEl={anchorEl}
        placement="bottom-start"
        modifiers={[{ name: "offset", options: { offset: [0, 6] } }]}
        sx={{ zIndex: (theme) => theme.zIndex.modal, width: anchorEl?.clientWidth }}
      >
        <Paper
          elevation={6}
          sx={(theme) => ({
            borderRadius: 2,
            overflow: "hidden",
            border: 1,
            borderColor: "divider",
            bgcolor: opaquePaper(theme),
            backdropFilter: "blur(16px)",
          })}
        >
          {results.isFetching && <LinearProgress sx={{ height: 2 }} aria-hidden />}
          {settledEmpty ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1.5 }} role="status">
              No article titles match. Press Enter to search anyway.
            </Typography>
          ) : options.length > 0 ? (
            <List id={listId} role="listbox" aria-label="Suggested articles" dense disablePadding sx={{ py: 0.5 }}>
              {options.map((option, index) => {
                const selected = index === activeIndex;
                const common = {
                  id: optionId(index),
                  role: "option",
                  "aria-selected": selected,
                  selected,
                  onMouseDown: (event: { preventDefault: () => void }) => event.preventDefault(),
                  onMouseEnter: () => setActiveIndex(index),
                  onClick: () => choose(option),
                  sx: { mx: 0.5, borderRadius: 1.5 },
                };
                return option.kind === "article" ? (
                  <ListItemButton key={option.article.id} {...common}>
                    <ListItemIcon sx={{ minWidth: 32, color: "text.secondary" }}>
                      <FileText size={16} aria-hidden />
                    </ListItemIcon>
                    <ListItemText
                      primary={<Highlight text={option.article.title} query={debounced} />}
                      secondary={kbTitle(option.article.knowledgeBaseId)}
                      slotProps={{ primary: { noWrap: true }, secondary: { noWrap: true } }}
                    />
                  </ListItemButton>
                ) : (
                  <ListItemButton key="all" {...common} sx={{ ...common.sx, borderTop: 1, borderColor: "divider", mt: 0.5, borderRadius: 0, mx: 0 }}>
                    <ListItemIcon sx={{ minWidth: 32, color: "text.secondary" }}>
                      <Search size={16} aria-hidden />
                    </ListItemIcon>
                    <ListItemText
                      primary={`See all results for “${typed}”`}
                      slotProps={{ primary: { noWrap: true, sx: { fontWeight: 600 } } }}
                    />
                  </ListItemButton>
                );
              })}
            </List>
          ) : (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1.5 }}>
              Searching…
            </Typography>
          )}
        </Paper>
      </Popper>
    </Box>
  );
}
