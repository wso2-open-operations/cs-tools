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
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link } from "react-router";
import type { KnowledgeBase } from "@api/backend/types";
import type { KnowledgeBaseCounts } from "@api/useKb";
import { browseUrl } from "@utils/browseUrl";

interface KnowledgeBaseFilterProps {
  knowledgeBases: KnowledgeBase[];
  /** Empty string for "all". */
  selectedId: string;
  /** Carried over so switching knowledge base keeps the search. */
  searchQuery: string;
  counts: KnowledgeBaseCounts;
}

/**
 * Knowledge-base filter for the browse page: a sidebar list with counts on
 * wide screens, a horizontally scrolling chip row on narrow ones. Both are
 * real links, so each filter has a URL.
 */
export default function KnowledgeBaseFilter({
  knowledgeBases,
  selectedId,
  searchQuery,
  counts,
}: KnowledgeBaseFilterProps): JSX.Element {
  const options = [
    { id: "", title: "All knowledge bases", chipTitle: "All" },
    ...knowledgeBases.map((kb) => ({ id: kb.id, title: kb.title, chipTitle: kb.title })),
  ];
  const isSelected = (id: string): boolean => id === selectedId;

  return (
    <Box component="nav" aria-label="Knowledge bases" className="kb-no-print">
      <Box sx={{ display: { xs: "none", md: "block" } }}>
        <Typography
          variant="overline"
          component="h2"
          color="text.secondary"
          sx={{ display: "block", mb: 1, letterSpacing: 1 }}
        >
          Knowledge bases
        </Typography>
        <List dense disablePadding>
          {options.map((option) => (
            <ListItem key={option.id} disablePadding sx={{ mb: 0.25 }}>
              <ListItemButton
                component={Link}
                to={browseUrl({ knowledgeBaseId: option.id || undefined, q: searchQuery })}
                selected={isSelected(option.id)}
                aria-current={isSelected(option.id) ? "page" : undefined}
                sx={{ borderRadius: 1.5, py: 0.75 }}
              >
                <ListItemText
                  primary={option.title}
                  slotProps={{ primary: { sx: { fontWeight: isSelected(option.id) ? 700 : 500 } } }}
                />
                {option.id && counts[option.id] !== undefined && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ ml: 1.5, fontVariantNumeric: "tabular-nums" }}
                    aria-label={`${counts[option.id]} articles`}
                  >
                    {counts[option.id]}
                  </Typography>
                )}
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      </Box>

      <Stack
        direction="row"
        spacing={1}
        sx={{
          display: { xs: "flex", md: "none" },
          overflowX: "auto",
          pb: 1,
          mx: -2,
          px: 2,
          scrollbarWidth: "none",
          "&::-webkit-scrollbar": { display: "none" },
        }}
      >
        {options.map((option) => (
          <Chip
            key={option.id}
            component={Link}
            to={browseUrl({ knowledgeBaseId: option.id || undefined, q: searchQuery })}
            clickable
            label={option.chipTitle}
            variant="outlined"
            color={isSelected(option.id) ? "primary" : "default"}
            aria-current={isSelected(option.id) ? "page" : undefined}
            sx={{ flexShrink: 0, fontWeight: isSelected(option.id) ? 700 : 500 }}
          />
        ))}
      </Stack>
    </Box>
  );
}
