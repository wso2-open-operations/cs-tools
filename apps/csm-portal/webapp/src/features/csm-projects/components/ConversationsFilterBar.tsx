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
  Button,
  Checkbox,
  Divider,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import AsyncInitiatorMultiSelect from "@features/csm-projects/components/AsyncInitiatorMultiSelect";
import {
  ALL_CONVERSATION_STATES,
  CONVERSATION_STATE_LABEL,
  countActiveConversationFilters,
  type ConversationsFilters,
} from "@features/csm-projects/utils/conversationState";

interface ConversationsFilterBarProps {
  filters: ConversationsFilters;
  onChange: (next: ConversationsFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}

/**
 * Filter bar for a project's Conversations tab, scoped strictly to what
 * `POST /conversations/search` supports for a project-scoped search: free
 * text (`searchQuery`), state (`states`), "my conversations" (`createdByMe`),
 * an explicit exact-match chat number (`number`), and an explicit
 * initiator-email multi-select (`createdBy`). Deliberately does not offer a
 * date-range control — that field doesn't exist on this endpoint, unlike
 * `CasesFilterBar`. Visual/component patterns (search field, collapsible
 * filter grid, filters-toggle button) are copied from `CasesFilterBar` /
 * `IncidentsFilterBar` for a consistent feel across the app's filter bars.
 *
 * The top search box can also implicitly route a `CHAT`-number-shaped typed
 * string to `filters.number` (see `classifyConversationQuery`, used in
 * `useSearchConversations`) — the explicit Number field here is an
 * additional, precise control, not a replacement for that.
 */
export default function ConversationsFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: ConversationsFilterBarProps): JSX.Element {
  const activeCount = countActiveConversationFilters(filters);
  const hasActive = activeCount > 0;

  const stateOptions = useMemo(
    () => ALL_CONVERSATION_STATES.map((s) => ({ value: s, label: CONVERSATION_STATE_LABEL[s] })),
    [],
  );

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by conversation #, initiator, or message…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => onChange({ ...filters, search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        <Button
          variant="outlined"
          size="small"
          color="primary"
          onClick={onFiltersToggle}
          startIcon={<ListFilter size={16} />}
          endIcon={isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        >
          {hasActive ? `Filters (${activeCount})` : "Filters"}
        </Button>
        {hasActive && (
          <Button
            variant="text"
            size="small"
            color="primary"
            onClick={onReset}
            startIcon={<X size={16} />}
          >
            Clear filters
          </Button>
        )}
      </Box>

      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <TextField
                id="conversations-filter-number"
                fullWidth
                size="small"
                label="Number"
                placeholder="e.g. CHAT0000012345"
                value={filters.number}
                onChange={(e) => onChange({ ...filters, number: e.target.value })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="conversations-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                onChange={(next) => onChange({ ...filters, states: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <AsyncInitiatorMultiSelect
                values={filters.createdBy}
                onChange={(next) => onChange({ ...filters, createdBy: next })}
              />
            </Grid>
            <Grid
              size={{ xs: 12, sm: 6, md: 4 }}
              sx={{ display: "flex", alignItems: "center", height: 40 }}
            >
              <FormControlLabel
                control={
                  <Checkbox
                    id="conversations-filter-created-by-me"
                    size="small"
                    checked={filters.createdByMe}
                    onChange={(e) => onChange({ ...filters, createdByMe: e.target.checked })}
                  />
                }
                label="My conversations"
              />
            </Grid>
          </Grid>
        </>
      )}
    </Paper>
  );
}
