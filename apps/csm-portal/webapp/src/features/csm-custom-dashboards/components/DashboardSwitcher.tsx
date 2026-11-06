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

import { FormControl, InputAdornment, ListSubheader, MenuItem, Select, TextField, Typography } from "@wso2/oxygen-ui";
import { SearchIcon } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";

/** One entry of the switcher: a dashboard's id and name, and an optional second line. */
export interface DashboardSwitcherEntry {
  id: string;
  name: string;
  /** Shown under the name in the menu (e.g. "Shared by Jane Doe"), and searched too. */
  secondary?: string;
}

interface DashboardSwitcherProps {
  entries: DashboardSwitcherEntry[];
  selectedId: string;
  onSelect: (id: string) => void;
}

/**
 * A dashboard switcher with a search box at the top of its menu, as My
 * Dashboards has (the Usage Metrics project search pattern): typing filters
 * the entries by name and second line, case-insensitive, in the browser. The
 * box takes focus when the menu opens and is cleared when it closes; typed
 * keys stay in it (not the menu's jump-to-letter), except Escape, which
 * closes. Entries that do not match stay in the list, hidden and disabled, so
 * the selected one is always an option. The closed field shows the name only.
 */
export default function DashboardSwitcher({ entries, selectedId, onSelect }: DashboardSwitcherProps): JSX.Element {
  const [search, setSearch] = useState("");
  const query = search.trim().toLowerCase();
  const matches = (e: DashboardSwitcherEntry): boolean =>
    !query || e.name.toLowerCase().includes(query) || (e.secondary?.toLowerCase().includes(query) ?? false);
  const noMatch = query !== "" && !entries.some(matches);
  const selectedName = entries.find((e) => e.id === selectedId)?.name ?? "";

  return (
    <FormControl size="small" sx={{ minWidth: 200 }}>
      <Select
        value={selectedId}
        onClose={() => setSearch("")}
        MenuProps={{ autoFocus: false, slotProps: { paper: { sx: { maxHeight: 400 } } } }}
        onChange={(e) => onSelect(e.target.value)}
        renderValue={() => selectedName}
        // On the element screen readers announce: a plain aria-label lands on
        // Select's hidden input instead.
        SelectDisplayProps={{ "aria-label": "Select dashboard" }}
      >
        <ListSubheader sx={{ pt: 1, pb: 0.5, px: 1, bgcolor: "background.paper" }}>
          <TextField
            size="small"
            fullWidth
            autoFocus
            placeholder="Search dashboards"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== "Escape") e.stopPropagation();
            }}
            slotProps={{
              htmlInput: { "aria-label": "Search dashboards" },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
        </ListSubheader>
        {noMatch && (
          <MenuItem disabled>
            <Typography component="span" variant="inherit" color="text.secondary">
              No dashboards match &ldquo;{search.trim()}&rdquo;
            </Typography>
          </MenuItem>
        )}
        {entries.map((e) => (
          <MenuItem
            key={e.id}
            value={e.id}
            disabled={!matches(e)}
            sx={{ flexDirection: "column", alignItems: "flex-start", ...(matches(e) ? {} : { display: "none" }) }}
          >
            <Typography component="span" variant="inherit" noWrap>
              {e.name}
            </Typography>
            {e.secondary && (
              <Typography component="span" variant="caption" color="text.secondary" noWrap>
                {e.secondary}
              </Typography>
            )}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
