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
  IconButton,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { FolderPlus, Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { useSkillCategories } from "@features/csm-skills/api/useSkillCatalogue";
import { useCreateSkillCategory, useUpdateSkillCategory } from "@features/csm-skills/api/useSkillManagement";
import { buildCategoryTree, flattenCategoryTree } from "@features/csm-skills/utils/categoryTree";
import type { SkillCategory } from "@features/csm-skills/types";

type Editing = { kind: "new"; parent: SkillCategory | null } | { kind: "edit"; category: SkillCategory };

/**
 * Administration › Categories: the category tree with each category's skill
 * count. A catalogue manager adds top-level and sub-categories, renames and
 * describes them, and deactivates or reactivates them. The backend refuses a
 * sub-category under a category with skills filed directly under it, and a
 * deactivation while active sub-categories or skills remain; its message is
 * shown as is.
 */
export default function SkillCategoriesAdmin(): JSX.Element {
  const categories = useSkillCategories();
  const update = useUpdateSkillCategory();
  const flat = useMemo(() => flattenCategoryTree(buildCategoryTree(categories.data ?? [])), [categories.data]);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const toggle = (c: SkillCategory): void => {
    setError(null);
    setNotice(null);
    const active = c.active === false;
    update.mutate(
      { id: c.id, body: { active } },
      {
        onSuccess: () => setNotice(`${c.name} was ${active ? "activated" : "deactivated"}.`),
        onError: (e) => setError(e.message || "The category could not be changed."),
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Skills are filed under categories with no sub-categories.
        </Typography>
        <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setEditing({ kind: "new", parent: null })}>
          New category
        </Button>
      </Box>

      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {notice && (
        <Alert severity="success" role="status" onClose={() => setNotice(null)}>
          {notice}
        </Alert>
      )}

      {categories.isLoading ? (
        <Skeleton variant="rounded" height={200} aria-label="Loading categories" />
      ) : categories.isError ? (
        <QueryErrorState message={categories.error.message || "Failed to load categories."} error={categories.error} onRetry={() => void categories.refetch()} />
      ) : flat.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No categories yet.
        </Typography>
      ) : (
        <Box component="ul" aria-label="Skill categories" sx={{ listStyle: "none", m: 0, p: 0, border: 1, borderColor: "divider", borderRadius: 1 }}>
          {flat.map(({ category: c, depth }) => {
            const inactive = c.active === false;
            const isLeaf = !flat.some((o) => o.category.parentId === c.id);
            return (
              <Box
                key={c.id}
                component="li"
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  py: 1,
                  pr: 1,
                  pl: `${12 + depth * 24}px`,
                  borderBottom: 1,
                  borderColor: "divider",
                  "&:last-of-type": { borderBottom: 0 },
                  opacity: inactive ? 0.6 : 1,
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                    <Typography variant="body2" sx={{ fontWeight: depth === 0 ? 600 : 400 }}>
                      {c.name}
                    </Typography>
                    {isLeaf && (
                      <Chip size="small" variant="outlined" label={`${c.skillCount ?? 0} ${c.skillCount === 1 ? "skill" : "skills"}`} />
                    )}
                    {inactive && <Chip size="small" variant="outlined" label="Inactive" />}
                  </Box>
                  {c.description && (
                    <Typography variant="caption" color="text.secondary">
                      {c.description}
                    </Typography>
                  )}
                </Box>
                {!inactive && (
                  <Tooltip title={isLeaf && (c.skillCount ?? 0) > 0 ? "Move its skills first: skills are only filed under the lowest categories" : "Add sub-category"}>
                    <span>
                      <IconButton
                        size="small"
                        aria-label={`Add a sub-category under ${c.name}`}
                        disabled={isLeaf && (c.skillCount ?? 0) > 0}
                        onClick={() => setEditing({ kind: "new", parent: c })}
                      >
                        <FolderPlus size={16} />
                      </IconButton>
                    </span>
                  </Tooltip>
                )}
                <Tooltip title="Edit">
                  <IconButton size="small" aria-label={`Edit ${c.name}`} onClick={() => setEditing({ kind: "edit", category: c })}>
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                <Button size="small" disabled={update.isPending} onClick={() => toggle(c)} aria-label={`${inactive ? "Activate" : "Deactivate"} ${c.name}`}>
                  {inactive ? "Activate" : "Deactivate"}
                </Button>
              </Box>
            );
          })}
        </Box>
      )}

      {editing && (
        <CategoryFormDialog
          editing={editing}
          onClose={(saved) => {
            setEditing(null);
            if (saved) {
              setError(null);
              setNotice(saved);
            }
          }}
        />
      )}
    </Box>
  );
}

function CategoryFormDialog({ editing, onClose }: { editing: Editing; onClose: (saved?: string) => void }): JSX.Element {
  const create = useCreateSkillCategory();
  const update = useUpdateSkillCategory();
  const current = editing.kind === "edit" ? editing.category : null;
  const [name, setName] = useState(current?.name ?? "");
  const [description, setDescription] = useState(current?.description ?? "");
  const [error, setError] = useState<string | null>(null);
  const busy = create.isPending || update.isPending;
  const title =
    editing.kind === "edit"
      ? `Edit ${editing.category.name}`
      : editing.parent
        ? `New sub-category of ${editing.parent.name}`
        : "New category";

  const submit = (): void => {
    setError(null);
    const fail = (e: Error): void => setError(e.message || "The category could not be saved.");
    if (current) {
      const body: { name?: string; description?: string } = {};
      if (name.trim() !== current.name) body.name = name.trim();
      if (description !== (current.description ?? "")) body.description = description;
      if (Object.keys(body).length === 0) {
        onClose();
        return;
      }
      update.mutate({ id: current.id, body }, { onSuccess: () => onClose(`${name.trim()} was updated.`), onError: fail });
    } else {
      create.mutate(
        {
          name: name.trim(),
          ...(editing.kind === "new" && editing.parent ? { parentId: editing.parent.id } : {}),
          ...(description.trim() ? { description: description.trim() } : {}),
        },
        { onSuccess: () => onClose(`${name.trim()} was created.`), onError: fail },
      );
    }
  };

  return (
    <Dialog open onClose={busy ? undefined : () => onClose()} maxWidth="sm" fullWidth aria-labelledby="category-form-title">
      <DialogTitle id="category-form-title">{title}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          {editing.kind === "new" && editing.parent?.isLowestCategory && (
            <DialogContentText>
              {editing.parent.name} will no longer be a place to file skills: they go under its sub-categories.
            </DialogContentText>
          )}
          <TextField
            size="small"
            label="Name"
            required
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            helperText="Unique among its siblings. Cannot contain “/”."
            inputProps={{ maxLength: 255 }}
          />
          <TextField
            size="small"
            label="Description"
            multiline
            minRows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => onClose()} disabled={busy}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit} disabled={busy || !name.trim() || name.includes("/")}>
          {current ? "Save" : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
