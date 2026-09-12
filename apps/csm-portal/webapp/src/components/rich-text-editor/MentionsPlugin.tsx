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

import { useCallback, useMemo, useState, type JSX } from "react";
import { createPortal } from "react-dom";
import {
  LexicalTypeaheadMenuPlugin,
  MenuOption,
  useBasicTypeaheadTriggerMatch,
} from "@lexical/react/LexicalTypeaheadMenuPlugin";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import type { TextNode } from "lexical";
import {
  CircularProgress,
  ListItemText,
  MenuItem,
  MenuList,
  Paper,
} from "@wso2/oxygen-ui";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useUserDirectorySearch } from "@features/csm-cases/api/useUserSearch";
import { $createMentionNode } from "@components/rich-text-editor/MentionNode";

const MENTION_TRIGGER = "@";
/** Cap the dropdown to a manageable list; mirrors `POST /users/search`'s page limit. */
const MAX_MENTION_RESULTS = 10;
const MENTION_QUERY_DEBOUNCE_MS = 300;

class MentionTypeaheadOption extends MenuOption {
  userId: string;
  name: string;
  email: string;

  constructor(userId: string, name: string, email: string) {
    super(userId);
    this.userId = userId;
    this.name = name;
    this.email = email;
  }
}

/**
 * Typing `@` opens a debounced user-search typeahead (`POST /users/search`,
 * scoped client-side to `userType === "internal"` — only staff can be
 * mentioned, never a customer/partner contact or the `system` account).
 * Selecting a result inserts a {@link MentionNode} chip. Registered in
 * `Editor.tsx` alongside `ImagesPlugin`.
 */
export default function MentionsPlugin(): JSX.Element | null {
  const [editor] = useLexicalComposerContext();
  const [queryString, setQueryString] = useState<string | null>(null);
  const debouncedQuery = useDebouncedValue(
    queryString ?? "",
    MENTION_QUERY_DEBOUNCE_MS,
  );
  const trimmedQuery = debouncedQuery.trim();

  const checkForMentionMatch = useBasicTypeaheadTriggerMatch(MENTION_TRIGGER, {
    minLength: 0,
  });

  // Only search once the menu is actually open (queryString !== null); an
  // empty query still lists the first page of staff so the menu isn't blank
  // right after typing "@".
  const { data, isFetching } = useUserDirectorySearch(
    trimmedQuery,
    queryString !== null,
    MAX_MENTION_RESULTS,
  );

  const options = useMemo(() => {
    const users = (data ?? []).filter((u) => u.userType === "internal");
    return users
      .slice(0, MAX_MENTION_RESULTS)
      .map((u) => new MentionTypeaheadOption(u.id, u.name || u.email, u.email));
  }, [data]);

  const onSelectOption = useCallback(
    (
      selectedOption: MentionTypeaheadOption,
      nodeToReplace: TextNode | null,
      closeMenu: () => void,
    ) => {
      editor.update(() => {
        const mentionNode = $createMentionNode(
          selectedOption.name,
          selectedOption.userId,
        );
        if (nodeToReplace) {
          nodeToReplace.replace(mentionNode);
        }
        mentionNode.selectNext();
      });
      closeMenu();
    },
    [editor],
  );

  return (
    <LexicalTypeaheadMenuPlugin<MentionTypeaheadOption>
      onQueryChange={setQueryString}
      onSelectOption={onSelectOption}
      triggerFn={checkForMentionMatch}
      options={options}
      menuRenderFn={(
        anchorElementRef,
        { selectedIndex, selectOptionAndCleanUp, setHighlightedIndex },
      ) => {
        if (!anchorElementRef.current) return null;
        if (options.length === 0 && !isFetching) return null;

        return createPortal(
          <Paper
            elevation={4}
            sx={{
              minWidth: 220,
              maxWidth: 320,
              maxHeight: 260,
              overflowY: "auto",
              mt: 0.5,
              zIndex: (theme) => theme.zIndex.modal + 1,
            }}
          >
            <MenuList dense>
              {options.length === 0 ? (
                <MenuItem disabled>
                  <CircularProgress size={14} sx={{ mr: 1 }} />
                  <ListItemText primary="Searching…" />
                </MenuItem>
              ) : (
                options.map((option, index) => (
                  <MenuItem
                    key={option.key}
                    selected={selectedIndex === index}
                    onMouseEnter={() => setHighlightedIndex(index)}
                    onClick={() => {
                      setHighlightedIndex(index);
                      selectOptionAndCleanUp(option);
                    }}
                  >
                    <ListItemText
                      primary={option.name}
                      secondary={option.email}
                      slotProps={{
                        primary: { style: { fontSize: 13 } },
                        secondary: { style: { fontSize: 11 } },
                      }}
                    />
                  </MenuItem>
                ))
              )}
            </MenuList>
          </Paper>,
          anchorElementRef.current,
        );
      }}
    />
  );
}
