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

import { useEffect, useState } from "react";
import {
  AdapterDateFns,
  Button,
  Chip,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { format, parse } from "date-fns";
import {
  CHANGE_REQUEST_FILTERABLE_STATES,
  CHANGE_REQUEST_IMPACTS,
  CHANGE_REQUEST_IMPACT_LABELS,
  CHANGE_REQUEST_STATE_LABELS,
} from "./config";
import { EMPTY_CR_FILTERS, type ChangeRequestFilters } from "./changeRequestFilters";

const { LocalizationProvider, DatePicker } = DatePickers;
const ISO_DATE = "yyyy-MM-dd";

function fromIsoDate(iso: string): Date | null {
  if (!iso) return null;
  const d = parse(iso, ISO_DATE, new Date());
  return Number.isNaN(d.getTime()) ? null : d;
}

function toIsoDate(date: Date | null): string {
  return date && !Number.isNaN(date.getTime()) ? format(date, ISO_DATE) : "";
}

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

interface ChangeRequestsFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: ChangeRequestFilters;
  onApply: (filters: ChangeRequestFilters) => void;
}

// Mirrors the support FiltersSheet's bottom-sheet pattern (src/components/support/FiltersSheet.tsx).
export function ChangeRequestsFiltersSheet({ open, onClose, filters, onApply }: ChangeRequestsFiltersSheetProps) {
  const [draft, setDraft] = useState<ChangeRequestFilters>(filters);

  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open
  }, [open]);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        Filters
        <IconButton size="small" aria-label="Close filters" onClick={onClose}>
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <Divider />

      <DialogContent>
        <Stack gap={2.5}>
          <Stack gap={1}>
            <Typography variant="subtitle2">State</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {CHANGE_REQUEST_FILTERABLE_STATES.map((state) => {
                const isSelected = draft.states.includes(state);
                return (
                  <Chip
                    key={state}
                    label={CHANGE_REQUEST_STATE_LABELS[state]}
                    size="small"
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => setDraft({ ...draft, states: toggle(draft.states, state) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2">Impact</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {CHANGE_REQUEST_IMPACTS.map((impact) => {
                const isSelected = draft.impacts.includes(impact);
                return (
                  <Chip
                    key={impact}
                    label={CHANGE_REQUEST_IMPACT_LABELS[impact]}
                    size="small"
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => setDraft({ ...draft, impacts: toggle(draft.impacts, impact) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2">Closed date range</Typography>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <Stack direction="row" gap={1}>
                <DatePicker
                  label="From"
                  value={fromIsoDate(draft.closedStartDate)}
                  onChange={(date: Date | null) => setDraft({ ...draft, closedStartDate: toIsoDate(date) })}
                  maxDate={fromIsoDate(draft.closedEndDate) ?? undefined}
                  slotProps={{ textField: { size: "small", fullWidth: true }, field: { clearable: true } }}
                />
                <DatePicker
                  label="To"
                  value={fromIsoDate(draft.closedEndDate)}
                  onChange={(date: Date | null) => setDraft({ ...draft, closedEndDate: toIsoDate(date) })}
                  minDate={fromIsoDate(draft.closedStartDate) ?? undefined}
                  slotProps={{ textField: { size: "small", fullWidth: true }, field: { clearable: true } }}
                />
              </Stack>
            </LocalizationProvider>
          </Stack>
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            setDraft(EMPTY_CR_FILTERS);
            onApply(EMPTY_CR_FILTERS);
            onClose();
          }}
        >
          Clear all
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            onApply(draft);
            onClose();
          }}
        >
          Apply
        </Button>
      </DialogActions>
    </Dialog>
  );
}
