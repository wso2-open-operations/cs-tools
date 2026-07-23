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

const MAX_TAG_LABEL_LEN = 100;
const TAG_SEARCH_DEBOUNCE_MS = 300;

interface AddTagDialogProps {
  /** Tags already on the case, so a duplicate label can't be submitted twice. */
  existingLabels: string[];
  isSaving: boolean;
  onClose: () => void;
  onSave: (label: string) => void;
}

/**
 * Add a tag to a case (`POST /cases/{id}/tags`). Tags are genuinely free-text
 * on the backing data source (SN's generic label mechanism, e.g. `micro-gw`,
 * `ws-policy`) — this is a search-and-select-or-create combobox, not a picker
 * constrained to a closed enum: it searches existing labels via
 * `GET /tags/search` as the user types (debounced), and still lets them type a
 * label with no match to create a genuinely new tag. ServiceNow-source only;
 * the caller surfaces a rejection on another source.
 */
export default function AddTagDialog({
  existingLabels,
  isSaving,
  onClose,
  onSave,
}: AddTagDialogProps): JSX.Element {
  const [label, setLabel] = useState("");
  const debouncedLabel = useDebouncedValue(label, TAG_SEARCH_DEBOUNCE_MS);
  const query = debouncedLabel.trim();
  const { data: matches, isFetching, isError } = useSearchTags(query, true);

  const trimmed = label.trim();
  const isDuplicate = existingLabels.some(
    (l) => l.toLowerCase() === trimmed.toLowerCase(),
  );
  const canSubmit = trimmed.length > 0 && !isDuplicate;

  // Suggestions from the search, minus labels already on this case (those
  // would just be rejected as a duplicate anyway).
  const options = useMemo(() => {
    const existing = new Set(existingLabels.map((l) => l.toLowerCase()));
    return (matches ?? [])
      .map((t) => t.label)
      .filter((l) => !existing.has(l.toLowerCase()));
  }, [matches, existingLabels]);

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    onSave(trimmed);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add tag</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          <Autocomplete<string, false, false, true>
            freeSolo
            size="small"
            fullWidth
            options={options}
            inputValue={label}
            loading={isFetching}
            disabled={isSaving}
            onInputChange={(_event, value, reason) => {
              if (reason === "input" || reason === "reset" || reason === "clear") {
                setLabel(value.slice(0, MAX_TAG_LABEL_LEN));
              }
            }}
            noOptionsText={
              isError
                ? "Couldn't search existing tags — you can still type a new one."
                : query
                  ? "No matching tags — press Enter to create it."
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
                onKeyDown={(e) => {
                  // `params.inputProps.onKeyDown` is the built-in listbox
                  // navigation handler (arrow keys / Enter-to-select) that
                  // Autocomplete wires onto the underlying <input>. MUI types
                  // this TextField's `onKeyDown` against the root div and
                  // `inputProps.onKeyDown` against the input element, but
                  // they fire for the same DOM keydown here — cast rather
                  // than drop the call, so listbox keyboard nav still works.
                  params.inputProps.onKeyDown?.(
                    e as unknown as React.KeyboardEvent<HTMLInputElement>,
                  );
                  if (e.key === "Enter" && !e.defaultPrevented) {
                    e.preventDefault();
                    handleSubmit();
                  }
                }}
              />
            )}
          />
          <Typography variant="caption" color="text.secondary">
            Search existing tags or type a new label — tags are free-text,
            there's no fixed list.
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
