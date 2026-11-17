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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useCatalogWrite } from "@features/csm-products/api/useCatalogWrite";
import type { CatalogReleaseLine } from "@features/csm-products/types/productCatalog";
import {
  buildReleaseLinePayload,
  releaseLineToForm,
  validateReleaseLineForm,
  type ReleaseLineFormValues,
} from "@features/csm-products/utils/catalogChange";

interface ReleaseLineDialogProps {
  /** The line to edit; null creates one. */
  line: CatalogReleaseLine | null;
  onClose: () => void;
  /** Called after a change was applied, with a message for the success banner. */
  onSaved: (message: string) => void;
}

/**
 * Create or edit a release line; saving applies the change at once. Retiring
 * an existing line is here too, behind an in-dialog confirmation step.
 * Retired lines are not listed, so there is no Restore.
 */
export default function ReleaseLineDialog({ line, onClose, onSaved }: ReleaseLineDialogProps): JSX.Element {
  const [form, setForm] = useState<ReleaseLineFormValues>(() => releaseLineToForm(line));
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmingRetire, setConfirmingRetire] = useState(false);
  const save = useCatalogWrite();
  const pending = save.isPending;
  const errors = validateReleaseLineForm(form);

  const handleSave = async (): Promise<void> => {
    setSubmitted(true);
    if (Object.keys(errors).length > 0) return;
    setError(null);
    const payload = buildReleaseLinePayload(line, form);
    try {
      if (!line) {
        await save.mutateAsync({ kind: "createReleaseLine", payload });
        onSaved(`Added release line ${form.name.trim()}.`);
      } else if (Object.keys(payload).length === 0) {
        setError("Nothing has changed.");
      } else {
        await save.mutateAsync({ kind: "updateReleaseLine", id: line.id, payload });
        onSaved(`Saved release line ${form.name.trim()}.`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save the change.");
    }
  };

  const handleRetire = async (): Promise<void> => {
    if (!line) return;
    setError(null);
    try {
      await save.mutateAsync({ kind: "retireReleaseLine", id: line.id });
      onSaved(`Retired release line ${line.name}.`);
    } catch (err) {
      setConfirmingRetire(false);
      setError(err instanceof Error ? err.message : "Failed to save the change.");
    }
  };

  const field = (key: keyof ReleaseLineFormValues) => ({
    value: form[key],
    onChange: (event: { target: { value: string } }) => setForm({ ...form, [key]: event.target.value }),
    error: submitted && Boolean(errors[key]),
    helperText: submitted ? errors[key] : undefined,
    size: "small" as const,
    fullWidth: true,
  });

  return (
    <Dialog open onClose={pending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{line ? `Edit ${line.name}` : "New release line"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField label="Name" required {...field("name")} />
          <TextField label="Start date" type="date" {...field("startDate")} slotProps={{ inputLabel: { shrink: true } }} />
          <TextField label="Status" placeholder="e.g. Active" {...field("status")} />
          {error && <Alert severity="error">{error}</Alert>}
        </Box>
      </DialogContent>
      {confirmingRetire && line ? (
        <DialogActions sx={{ flexWrap: "wrap", gap: 1 }}>
          <Typography variant="body2" sx={{ mr: "auto" }}>
            Retire this release line?
          </Typography>
          <Button onClick={() => setConfirmingRetire(false)} disabled={pending}>
            Back
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={() => void handleRetire()}
            disabled={pending}
          >
            Retire
          </Button>
        </DialogActions>
      ) : (
        <DialogActions>
          {line && (
            <Button
              color="error"
              onClick={() => {
                setError(null);
                setConfirmingRetire(true);
              }}
              disabled={pending}
              sx={{ mr: "auto" }}
            >
              Retire this release line
            </Button>
          )}
          <Button onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button variant="contained" onClick={() => void handleSave()} disabled={pending}>
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogActions>
      )}
    </Dialog>
  );
}
