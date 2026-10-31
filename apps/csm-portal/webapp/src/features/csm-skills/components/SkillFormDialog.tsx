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
  FormControlLabel,
  Switch,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  useCreateSkill,
  useSkillCategories,
  useSkillLevelTypes,
  useUpdateSkill,
} from "@features/csm-skills/api/useSkillCatalogue";
import { CategoryMultiSelect } from "@features/csm-skills/components/CategorySelect";
import type { Skill, UpdateSkillRequest } from "@features/csm-skills/types";

interface SkillFormDialogProps {
  /** The skill being edited; omit to create a new one. */
  skill?: Skill;
  onClose: (saved?: string) => void;
}

const sameSet = (a: readonly string[], b: readonly string[]): boolean =>
  a.length === b.length && a.every((x) => b.includes(x));

/**
 * Admin-only create/edit form for a catalogue skill. On edit only the fields
 * that changed are sent. The backend enforces who may do this and rules such as
 * "the level scale can't change while people hold levels"; its message is shown
 * as is.
 */
export default function SkillFormDialog({ skill, onClose }: SkillFormDialogProps): JSX.Element {
  const editing = !!skill;
  const categories = useSkillCategories();
  const levelTypes = useSkillLevelTypes();
  const create = useCreateSkill();
  const update = useUpdateSkill();

  const initialCategoryIds = skill?.categories.map((c) => c.id) ?? [];
  const [name, setName] = useState(skill?.name ?? "");
  const [description, setDescription] = useState(skill?.description ?? "");
  const [keywords, setKeywords] = useState(skill?.keywords ?? "");
  const [levelTypeId, setLevelTypeId] = useState(skill?.levelTypeId ?? "");
  const [categoryIds, setCategoryIds] = useState<string[]>(initialCategoryIds);
  const [active, setActive] = useState(skill?.active ?? true);
  const [error, setError] = useState<string | null>(null);

  const busy = create.isPending || update.isPending;

  const patch: UpdateSkillRequest = {};
  if (skill) {
    if (name.trim() !== skill.name) patch.name = name.trim();
    if (description !== (skill.description ?? "")) patch.description = description;
    if (keywords !== (skill.keywords ?? "")) patch.keywords = keywords;
    if (levelTypeId && levelTypeId !== (skill.levelTypeId ?? "")) patch.levelTypeId = levelTypeId;
    if (!sameSet(categoryIds, initialCategoryIds)) patch.categoryIds = categoryIds;
    if (active !== skill.active) patch.active = active;
  }
  const changed = Object.keys(patch).length > 0;
  const valid = name.trim().length > 0 && (editing || !!levelTypeId);

  const submit = (): void => {
    setError(null);
    const fail = (e: Error): void => setError(e.message || "The skill could not be saved.");
    if (skill) {
      update.mutate({ id: skill.id, body: patch }, { onSuccess: () => onClose(`${name.trim()} was updated.`), onError: fail });
    } else {
      create.mutate(
        {
          name: name.trim(),
          levelTypeId,
          active,
          ...(description.trim() ? { description } : {}),
          ...(keywords.trim() ? { keywords } : {}),
          ...(categoryIds.length ? { categoryIds } : {}),
        },
        { onSuccess: () => onClose(`${name.trim()} was created.`), onError: fail },
      );
    }
  };

  return (
    <Dialog open onClose={busy ? undefined : () => onClose()} maxWidth="sm" fullWidth aria-labelledby="skill-form-title">
      <DialogTitle id="skill-form-title">{editing ? `Edit ${skill.name}` : "New skill"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            label="Name"
            size="small"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            slotProps={{ htmlInput: { maxLength: 255 } }}
          />
          <TextField
            label="Description"
            size="small"
            multiline
            minRows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            slotProps={{ htmlInput: { maxLength: 4000 } }}
          />
          <TextField
            label="Keywords"
            size="small"
            value={keywords}
            onChange={(e) => setKeywords(e.target.value)}
            helperText="Extra words the skill can be found by."
            slotProps={{ htmlInput: { maxLength: 500 } }}
          />
          <TextField
            select
            label="Level scale"
            size="small"
            required
            value={levelTypeId}
            onChange={(e) => setLevelTypeId(e.target.value)}
            slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
            helperText={
              levelTypes.isError
                ? "Level scales could not be loaded."
                : (levelTypes.data?.find((t) => t.id === levelTypeId)?.levels.map((l) => l.name).join(", ") ??
                  "The levels people can hold in this skill.")
            }
          >
            <option value="" disabled>
              Select a scale
            </option>
            {(levelTypes.data ?? []).map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </TextField>
          <CategoryMultiSelect
            id="skill-form-categories"
            label="Categories"
            categories={categories.data}
            loading={categories.isLoading}
            values={categoryIds}
            onChange={setCategoryIds}
          />
          <FormControlLabel
            control={<Switch checked={active} onChange={(e) => setActive(e.target.checked)} />}
            label="Active"
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()} disabled={busy}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit} disabled={busy || !valid || (editing && !changed)}>
          {editing ? "Save changes" : "Create skill"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
