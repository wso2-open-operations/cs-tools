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
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import { useAddMySkill } from "@features/csm-skills/api/useMySkills";
import {
  useSkillCategories,
  useSkillLevelTypes,
  useSkillPickerSearch,
} from "@features/csm-skills/api/useSkillCatalogue";
import { CategorySelect } from "@features/csm-skills/components/CategorySelect";
import { findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import type { Skill } from "@features/csm-skills/types";

interface AddSkillDialogProps {
  onClose: () => void;
  /** Skills already on the profile; left out of the picker. */
  heldSkillIds: readonly string[];
}

const getSkillId = (s: Skill): string => s.id;
const getSkillLabel = (s: Skill): string => s.name;

/**
 * Add a skill to the caller's profile: optionally narrow by category (tree),
 * pick a skill (type-ahead, searched by the backend), then a level from that
 * skill's own scale. The backend still validates all of it; its message is
 * shown if it refuses.
 */
export default function AddSkillDialog({ onClose, heldSkillIds }: AddSkillDialogProps): JSX.Element {
  const categories = useSkillCategories();
  const levelTypes = useSkillLevelTypes();
  const add = useAddMySkill();

  const [categoryId, setCategoryId] = useState("");
  const [skill, setSkill] = useState<Skill | null>(null);
  const [levelId, setLevelId] = useState("");
  const [error, setError] = useState<string | null>(null);

  const scale = findLevelType(levelTypes.data, skill?.levelTypeId);
  const levels = sortedLevels(scale);
  const noScale = !!skill && !skill.levelTypeId;

  const pickCategory = (id: string): void => {
    setCategoryId(id);
    setSkill(null);
    setLevelId("");
  };
  const pickSkill = (id: string, item?: Skill): void => {
    setSkill(id ? (item ?? null) : null);
    setLevelId("");
  };

  const submit = (): void => {
    if (!skill || !levelId) return;
    setError(null);
    add.mutate(
      { skillId: skill.id, levelId },
      { onSuccess: onClose, onError: (e) => setError(e.message || "The skill could not be added.") },
    );
  };

  return (
    <Dialog open onClose={add.isPending ? undefined : onClose} maxWidth="sm" fullWidth aria-labelledby="add-skill-title">
      <DialogTitle id="add-skill-title">Add a skill</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          {categories.isError && (
            <Alert severity="warning">Categories could not be loaded. You can still search for a skill by name.</Alert>
          )}
          <CategorySelect
            id="add-skill-category"
            label="Category"
            categories={categories.data}
            loading={categories.isLoading}
            value={categoryId}
            onChange={pickCategory}
            helperText="Optional. Narrows the skills below to this category."
          />
          <AsyncEntitySelect<Skill>
            id="add-skill-skill"
            label="Skill"
            placeholder="Search skills…"
            value={skill?.id ?? ""}
            onChange={pickSkill}
            useSearch={useSkillPickerSearch}
            searchExtra={categoryId || undefined}
            getId={getSkillId}
            getLabel={getSkillLabel}
            excludeIds={heldSkillIds}
            required
          />
          <TextField
            select
            size="small"
            label="Level"
            required
            value={levelId}
            onChange={(e) => setLevelId(e.target.value)}
            disabled={!skill || noScale || levels.length === 0 || add.isPending}
            slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
            helperText={
              !skill
                ? "Choose a skill first."
                : noScale
                  ? "This skill has no level scale, so it can't be added."
                  : levelTypes.isLoading
                    ? "Loading levels…"
                    : levelTypes.isError
                      ? "Levels could not be loaded."
                      : scale
                        ? `Scale: ${scale.name}`
                        : undefined
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
        <Button onClick={onClose} disabled={add.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit} disabled={!skill || !levelId || add.isPending}>
          Add skill
        </Button>
      </DialogActions>
    </Dialog>
  );
}
