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
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSkillCategories, useSkillLevelTypes } from "@features/csm-skills/api/useSkillCatalogue";
import { useCheckSkillRequest, useCreateSkillRequest } from "@features/csm-skills/api/useSkillManagement";
import { CategorySelect } from "@features/csm-skills/components/CategorySelect";
import { defaultLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import { personName } from "@features/csm-skills/utils/skillRequests";
import type { SkillRef } from "@features/csm-skills/types";

interface RequestSkillDialogProps {
  /** Pre-fills the name (e.g. what was searched for and not found). */
  initialName?: string;
  /** Closed; `notice` is set when a request was sent. */
  onClose: (notice?: string) => void;
  /** The person chose an existing skill instead of requesting one. */
  onAddExisting: (skill: SkillRef) => void;
  /** Skills already on the profile, so a match can say so. */
  heldSkillIds: readonly string[];
}

/**
 * Request a skill the catalogue lacks. While the name is typed the backend
 * checks it: an existing skill is offered to add instead, a pending request
 * for the same name blocks a second one, and similar names are shown. The
 * level asked for is the one the skill is added at if the request is approved.
 */
export default function RequestSkillDialog({ initialName = "", onClose, onAddExisting, heldSkillIds }: RequestSkillDialogProps): JSX.Element {
  const categories = useSkillCategories();
  const levelTypes = useSkillLevelTypes();
  const create = useCreateSkillRequest();

  const [name, setName] = useState(initialName);
  const [categoryId, setCategoryId] = useState("");
  const [suggested, setSuggested] = useState("");
  const [description, setDescription] = useState("");
  const [levelId, setLevelId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const debounced = useDebouncedValue(name.trim(), 300);
  const check = useCheckSkillRequest(debounced);
  const checked = debounced.length >= 2 && check.data && !check.isPlaceholderData ? check.data : undefined;
  const exact = checked?.exactMatch ?? null;
  const pending = checked?.pendingRequest ?? null;
  const similar = checked?.similar ?? [];

  // Skills are filed only under active categories with no sub-categories.
  const leaves = useMemo(() => {
    const all = categories.data ?? [];
    const parents = new Set(all.map((c) => c.parentId).filter(Boolean));
    return all.filter((c) => c.active !== false && !parents.has(c.id));
  }, [categories.data]);
  const scale = defaultLevelType(levelTypes.data);
  const levels = sortedLevels(scale);

  const blocked = (exact?.active ?? false) || !!pending;
  const canSubmit = name.trim().length > 0 && !!levelId && !blocked && !create.isPending;

  const submit = (): void => {
    setError(null);
    create.mutate(
      {
        name: name.trim(),
        ...(categoryId ? { categoryId } : {}),
        ...(suggested.trim() && !categoryId ? { suggestedCategory: suggested.trim() } : {}),
        ...(description.trim() ? { description: description.trim() } : {}),
        levelId,
      },
      {
        onSuccess: (req) => onClose(`Your request for ${req.requestedName} was sent for review.`),
        onError: (e) => setError(e.message || "The request could not be sent."),
      },
    );
  };

  const held = (s: SkillRef): boolean => heldSkillIds.includes(s.id);

  return (
    <Dialog open onClose={create.isPending ? undefined : () => onClose()} maxWidth="sm" fullWidth aria-labelledby="request-skill-title">
      <DialogTitle id="request-skill-title">Request a skill</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          Ask for a skill that isn't in the catalogue yet. A reviewer adds it, and it's added to your skills at the
          level you choose here.
        </DialogContentText>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            size="small"
            label="Skill name"
            required
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            inputProps={{ maxLength: 255 }}
          />

          {exact?.active && (
            <Alert
              severity="info"
              action={
                held(exact) ? undefined : (
                  <Button size="small" onClick={() => onAddExisting(exact)}>
                    Add it
                  </Button>
                )
              }
            >
              <strong>{exact.name}</strong> is already in the catalogue
              {held(exact) ? " and on your profile." : ". Add it to your skills instead of requesting it."}
            </Alert>
          )}
          {exact && !exact.active && (
            <Alert severity="info">
              <strong>{exact.name}</strong> exists but is inactive. Your request asks a reviewer to make it available again.
            </Alert>
          )}
          {pending && (
            <Alert severity="warning">
              {personName(pending.requestedBy) || "Someone"} already requested <strong>{pending.requestedName}</strong>
              {" "}on {new Date(pending.createdOn).toLocaleDateString()}. It's waiting for review.
            </Alert>
          )}
          {similar.length > 0 && (
            <Box>
              <Typography variant="caption" color="text.secondary">
                Similar skills already in the catalogue{similar.some((s) => !held(s) && s.active) ? " (select one to add it)" : ""}:
              </Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75, mt: 0.5 }}>
                {similar.map((s) => (
                  <Chip
                    key={s.id}
                    size="small"
                    variant="outlined"
                    label={`${s.name}${held(s) ? " (on your profile)" : !s.active ? " (inactive)" : ""}`}
                    onClick={s.active && !held(s) ? () => onAddExisting(s) : undefined}
                  />
                ))}
              </Box>
            </Box>
          )}

          <CategorySelect
            id="request-skill-category"
            label="Category"
            categories={leaves}
            loading={categories.isLoading}
            value={categoryId}
            onChange={setCategoryId}
            helperText="Optional. Where the skill belongs."
          />
          {!categoryId && (
            <TextField
              size="small"
              label="Suggest a category"
              value={suggested}
              onChange={(e) => setSuggested(e.target.value)}
              helperText="Optional. If none of the categories fits."
              inputProps={{ maxLength: 255 }}
            />
          )}
          <TextField
            size="small"
            label="Description"
            multiline
            minRows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            helperText="Optional. What the skill covers, to help the reviewer."
          />
          <TextField
            select
            size="small"
            label="Your level"
            required
            value={levelId}
            onChange={(e) => setLevelId(e.target.value)}
            disabled={levels.length === 0}
            slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
            helperText={
              levelTypes.isLoading
                ? "Loading levels…"
                : levels.length === 0
                  ? "No default level scale is set up."
                  : "The skill is added to your profile at this level once approved."
            }
          >
            <option value="" disabled>
              Select a level
            </option>
            {levels.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </TextField>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()} disabled={create.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit} disabled={!canSubmit}>
          Send request
        </Button>
      </DialogActions>
    </Dialog>
  );
}
