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

import { Alert, Box, Button, IconButton, TextField, Typography, alpha } from "@wso2/oxygen-ui";
import { SquarePen, Trash2 } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import type { BeProductUpdate } from "@api/backend/types";

/** Which action currently owns the in-flight PATCH, for the footer button label. */
export type UpdateHistorySaveAction = "add" | "edit" | "delete";

/**
 * State the panel lifts up to {@link EditDeployedProductDialog} so its footer
 * "Add update" button can trigger (and reflect) the panel's own add-form
 * submit — mirrors customer-portal's `onFormStateChange` contract.
 */
export interface UpdateHistoryFormState {
  canAdd: boolean;
  isSaving: boolean;
  saveAction: UpdateHistorySaveAction | null;
  handleAdd: () => void;
}

interface Feedback {
  message: string;
  severity: "success" | "error";
}

interface UpdateHistoryPanelProps {
  updates: BeProductUpdate[];
  /**
   * Persist the whole resulting array immediately (add/edit/delete each call
   * this independently — there is no per-entry endpoint, see
   * {@link BeDeployedProductDetailUpdatePayload}). Rejects on failure so the
   * panel can show an inline error and keep the in-flight form open.
   */
  onSaveUpdates: (updates: BeProductUpdate[]) => Promise<void>;
  /** Lifts the add-form's submit affordance up to the dialog's footer button. */
  onFormStateChange: (state: UpdateHistoryFormState | null) => void;
}

const EMPTY_FORM = { updateLevel: "", date: "", details: "" };

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

/**
 * Update-history tab of {@link EditDeployedProductDialog}: a vertical timeline
 * of `BeProductUpdate` entries with add/edit/delete, each saving immediately
 * via {@link onSaveUpdates} — independent of the dialog's Details tab. Only
 * the latest (highest `updateLevel`) entry is editable in place; delete has
 * no confirm step, matching customer-portal's `UpdateHistoryTab`.
 *
 * csm-portal has no recommended-update-levels lookup (unlike customer-portal),
 * so update level is a plain number field here rather than a dropdown.
 */
export default function UpdateHistoryPanel({
  updates,
  onSaveUpdates,
  onFormStateChange,
}: UpdateHistoryPanelProps): JSX.Element {
  const [form, setForm] = useState(EMPTY_FORM);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editForm, setEditForm] = useState(EMPTY_FORM);
  const [saveInFlight, setSaveInFlight] = useState<UpdateHistorySaveAction | null>(null);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const isSaving = saveInFlight !== null;

  const sortedUpdates = useMemo(
    () =>
      updates
        .map((u, originalIndex) => ({ u, originalIndex }))
        .sort((a, b) => b.u.updateLevel - a.u.updateLevel),
    [updates],
  );

  const currentUpdateLevel = sortedUpdates.length > 0 ? sortedUpdates[0].u.updateLevel : null;
  const latestOriginalIndex = sortedUpdates.length > 0 ? sortedUpdates[0].originalIndex : null;

  const formLevelNum = form.updateLevel.trim() === "" ? null : Number(form.updateLevel);
  const formLevelError =
    form.updateLevel.trim() !== "" &&
    (!Number.isInteger(formLevelNum) || (formLevelNum as number) < 0);
  const isFormValid = form.updateLevel.trim() !== "" && !formLevelError && form.date.trim() !== "";

  const handleAddUpdate = useCallback(async () => {
    if (!isFormValid) return;
    const newUpdate: BeProductUpdate = {
      updateLevel: formLevelNum as number,
      date: form.date,
      details: form.details.trim() ? form.details.trim() : undefined,
    };
    setSaveInFlight("add");
    try {
      await onSaveUpdates([...updates, newUpdate]);
      setForm(EMPTY_FORM);
      setFeedback({ message: "Update history entry added.", severity: "success" });
    } catch (err) {
      setFeedback({ message: errorMessage(err, "Could not add the update."), severity: "error" });
    } finally {
      setSaveInFlight(null);
    }
  }, [isFormValid, formLevelNum, form.date, form.details, updates, onSaveUpdates]);

  // Keep a stable ref so the lifted `handleAdd` always calls the latest closure.
  const handleAddUpdateRef = useRef(handleAddUpdate);
  handleAddUpdateRef.current = handleAddUpdate;

  useEffect(() => {
    onFormStateChange({
      canAdd: isFormValid && !isSaving,
      isSaving,
      saveAction: saveInFlight,
      handleAdd: () => void handleAddUpdateRef.current(),
    });
  }, [isFormValid, isSaving, saveInFlight, onFormStateChange]);

  // Clear the lifted state on unmount only (tab switch away / dialog close).
  useEffect(() => {
    return () => onFormStateChange(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const startEdit = (originalIndex: number): void => {
    const u = updates[originalIndex];
    setEditingIndex(originalIndex);
    setEditForm({ updateLevel: String(u.updateLevel), date: u.date, details: u.details ?? "" });
  };

  const cancelEdit = (): void => {
    setEditingIndex(null);
    setEditForm(EMPTY_FORM);
  };

  const editLevelNum = editForm.updateLevel.trim() === "" ? null : Number(editForm.updateLevel);
  const editLevelError =
    editForm.updateLevel.trim() !== "" &&
    (!Number.isInteger(editLevelNum) || (editLevelNum as number) < 0);
  const isEditFormValid =
    editForm.updateLevel.trim() !== "" && !editLevelError && editForm.date.trim() !== "";

  const handleSaveEdit = useCallback(async () => {
    if (editingIndex === null || !isEditFormValid) return;
    const edited: BeProductUpdate = {
      updateLevel: editLevelNum as number,
      date: editForm.date,
      details: editForm.details.trim() ? editForm.details.trim() : undefined,
    };
    const next = updates.map((u, i) => (i === editingIndex ? edited : u));
    setSaveInFlight("edit");
    try {
      await onSaveUpdates(next);
      setEditingIndex(null);
      setEditForm(EMPTY_FORM);
      setFeedback({ message: "Update history entry saved.", severity: "success" });
    } catch (err) {
      setFeedback({ message: errorMessage(err, "Could not save the update."), severity: "error" });
    } finally {
      setSaveInFlight(null);
    }
  }, [editingIndex, isEditFormValid, editLevelNum, editForm.date, editForm.details, updates, onSaveUpdates]);

  const handleDelete = useCallback(
    async (index: number) => {
      const next = updates.filter((_, i) => i !== index);
      setSaveInFlight("delete");
      try {
        await onSaveUpdates(next);
        if (editingIndex === index) cancelEdit();
        setFeedback({ message: "Update history entry deleted.", severity: "success" });
      } catch (err) {
        setFeedback({
          message: errorMessage(err, "Could not delete the update."),
          severity: "error",
        });
      } finally {
        setSaveInFlight(null);
      }
    },
    [updates, onSaveUpdates, editingIndex],
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
      {feedback && (
        <Alert severity={feedback.severity} onClose={() => setFeedback(null)}>
          {feedback.message}
        </Alert>
      )}

      {currentUpdateLevel !== null && (
        <Box
          sx={{
            bgcolor: (theme) => alpha(theme.palette.primary.main, 0.08),
            border: 1,
            borderRadius: 1,
            borderColor: (theme) => alpha(theme.palette.primary.main, 0.3),
            p: 1.5,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography variant="body2" sx={{ fontWeight: 500 }}>
            Current Update Level:
          </Typography>
          <Typography variant="subtitle1" sx={{ fontWeight: 600, color: "primary.main" }}>
            U{currentUpdateLevel}
          </Typography>
        </Box>
      )}

      {sortedUpdates.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No update history recorded.
        </Typography>
      ) : (
        <Box sx={{ position: "relative" }}>
          <Box
            sx={{
              position: "absolute",
              left: 13,
              top: 0,
              bottom: 0,
              width: "2px",
              bgcolor: "divider",
            }}
          />
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {sortedUpdates.map(({ u, originalIndex }) => {
              const isLatest = originalIndex === latestOriginalIndex;
              const isEditingThis = editingIndex === originalIndex;
              return (
                <Box key={originalIndex} sx={{ position: "relative", display: "flex", gap: 2 }}>
                  <Box sx={{ position: "relative", zIndex: 1, flexShrink: 0 }}>
                    <Box
                      sx={{
                        width: 28,
                        height: 28,
                        bgcolor: "primary.main",
                        border: 4,
                        borderColor: "background.paper",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        borderRadius: "50%",
                      }}
                    >
                      <Box sx={{ width: 8, height: 8, bgcolor: "background.paper", borderRadius: "50%" }} />
                    </Box>
                  </Box>
                  <Box
                    sx={{
                      flex: 1,
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 1,
                      p: 1.5,
                    }}
                  >
                    {isEditingThis ? (
                      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                        <Box sx={{ display: "flex", gap: 1.5, flexWrap: "wrap" }}>
                          <TextField
                            label="Update level"
                            value={editForm.updateLevel}
                            onChange={(e) =>
                              setEditForm((prev) => ({ ...prev, updateLevel: e.target.value }))
                            }
                            size="small"
                            type="number"
                            slotProps={{ htmlInput: { min: 0, step: 1 } }}
                            error={editLevelError}
                            helperText={editLevelError ? "Must be a non-negative integer." : " "}
                            disabled={isSaving}
                            sx={{ flex: 1, minWidth: 120 }}
                          />
                          <TextField
                            label="Date"
                            value={editForm.date}
                            onChange={(e) => setEditForm((prev) => ({ ...prev, date: e.target.value }))}
                            size="small"
                            type="date"
                            slotProps={{ inputLabel: { shrink: true } }}
                            disabled={isSaving}
                            sx={{ flex: 1, minWidth: 160 }}
                          />
                        </Box>
                        <TextField
                          label="Details"
                          value={editForm.details}
                          onChange={(e) => setEditForm((prev) => ({ ...prev, details: e.target.value }))}
                          size="small"
                          fullWidth
                          multiline
                          minRows={2}
                          disabled={isSaving}
                        />
                        <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
                          <Button size="small" onClick={cancelEdit} disabled={isSaving}>
                            Cancel
                          </Button>
                          <Button
                            size="small"
                            variant="contained"
                            onClick={() => void handleSaveEdit()}
                            disabled={!isEditFormValid || isSaving}
                          >
                            {saveInFlight === "edit" ? "Saving Update..." : "Save"}
                          </Button>
                        </Box>
                      </Box>
                    ) : (
                      <>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "flex-start",
                            justifyContent: "space-between",
                          }}
                        >
                          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                            <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                              U{u.updateLevel}
                            </Typography>
                            {isLatest && (
                              <IconButton
                                size="small"
                                aria-label={`Edit update level ${u.updateLevel}`}
                                onClick={() => startEdit(originalIndex)}
                                disabled={isSaving}
                              >
                                <SquarePen size={14} />
                              </IconButton>
                            )}
                          </Box>
                          <IconButton
                            size="small"
                            aria-label={`Delete update level ${u.updateLevel}`}
                            onClick={() => void handleDelete(originalIndex)}
                            disabled={isSaving}
                          >
                            <Trash2 size={14} />
                          </IconButton>
                        </Box>
                        <Typography variant="body2" color="text.secondary">
                          Date: {u.date}
                        </Typography>
                        {u.details && (
                          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                            {u.details}
                          </Typography>
                        )}
                      </>
                    )}
                  </Box>
                </Box>
              );
            })}
          </Box>
        </Box>
      )}

      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 1.5,
          borderTop: 1,
          borderColor: "divider",
          pt: 2,
        }}
      >
        <Typography variant="caption" color="text.secondary">
          Add an update
        </Typography>
        <Box sx={{ display: "flex", gap: 1.5, flexWrap: "wrap" }}>
          <TextField
            label="Update level"
            value={form.updateLevel}
            onChange={(e) => setForm((prev) => ({ ...prev, updateLevel: e.target.value }))}
            size="small"
            type="number"
            slotProps={{ htmlInput: { min: 0, step: 1 } }}
            error={formLevelError}
            helperText={formLevelError ? "Must be a non-negative integer." : " "}
            disabled={isSaving}
            sx={{ flex: 1, minWidth: 120 }}
          />
          <TextField
            label="Date"
            value={form.date}
            onChange={(e) => setForm((prev) => ({ ...prev, date: e.target.value }))}
            size="small"
            type="date"
            slotProps={{ inputLabel: { shrink: true } }}
            disabled={isSaving}
            sx={{ flex: 1, minWidth: 160 }}
          />
        </Box>
        <TextField
          label="Details"
          value={form.details}
          onChange={(e: ChangeEvent<HTMLInputElement>) =>
            setForm((prev) => ({ ...prev, details: e.target.value }))
          }
          size="small"
          fullWidth
          multiline
          minRows={2}
          disabled={isSaving}
        />
      </Box>
    </Box>
  );
}
