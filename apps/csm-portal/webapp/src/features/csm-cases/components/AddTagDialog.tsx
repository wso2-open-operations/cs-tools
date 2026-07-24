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
  Autocomplete,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

const TAG_SEARCH_DEBOUNCE_MS = 300;

interface AddTagDialogProps {
  /** Tags already on the case, so a duplicate label can't be submitted twice. */
  existingLabels: string[];
  isSaving: boolean;
  onClose: () => void;
  onSave: (label: string) => void;
}

/**
 * Add a tag to a case (`POST /cases/{id}/tags`). Tags are a curated,
 * pre-existing vocabulary, not free text — this is search-and-SELECT only:
 * it searches existing labels via `GET /tags/search` as the user types
 * (debounced), but there is no "create a new tag" fallback for typed text
 * with no match. ServiceNow-source only; the caller surfaces a rejection on
 * another source.
 */
export default function AddTagDialog({
  existingLabels,
  isSaving,
  onClose,
  onSave,
}: AddTagDialogProps): JSX.Element {
  const [inputValue, setInputValue] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const debouncedInput = useDebouncedValue(inputValue, TAG_SEARCH_DEBOUNCE_MS);
  const query = debouncedInput.trim();
  const { data: matches, isFetching, isError } = useSearchTags(query, true);

  const isDuplicate =
    selected !== null &&
    existingLabels.some((l) => l.toLowerCase() === selected.toLowerCase());
  const canSubmit = selected !== null && !isDuplicate;

  // Suggestions from the search, minus labels already on this case (those
  // would just be rejected as a duplicate anyway).
  const options = useMemo(() => {
    const existing = new Set(existingLabels.map((l) => l.toLowerCase()));
    return (matches ?? [])
      .map((t) => t.label)
      .filter((l) => !existing.has(l.toLowerCase()));
  }, [matches, existingLabels]);

  const handleSubmit = (): void => {
    if (!canSubmit || selected === null) return;
    onSave(selected);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add tag</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <Autocomplete<string, false, false, false>
            size="small"
            fullWidth
            options={options}
            value={selected}
            inputValue={inputValue}
            loading={isFetching}
            disabled={isSaving}
            onChange={(_event, value) => setSelected(value)}
            onInputChange={(_event, value, reason) => {
              if (reason === "input" || reason === "reset" || reason === "clear") {
                setInputValue(value);
                if (reason !== "reset") setSelected(null);
              }
            }}
            noOptionsText={
              isError
                ? "Couldn't search existing tags — try again."
                : query
                  ? "No matching tags."
                  : "Type to search existing tags…"
            }
            renderInput={(params) => (
              <TextField
                {...params}
                label="Tag"
                placeholder="e.g. micro-gw, ws-policy"
                autoFocus
                error={isDuplicate}
                helperText={
                  isDuplicate ? "This case already has that tag." : undefined
                }
              />
            )}
          />
          <Typography variant="caption" color="text.secondary">
            Search and select an existing tag — tags are a fixed vocabulary,
            not free text.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSubmit}
        >
          Add tag
        </Button>
      </DialogActions>
    </Dialog>
  );
}
