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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useMemo, useState, type JSX } from "react";
import AssignmentEditor, {
  type AssignmentOption,
} from "@features/csm-admin/components/AssignmentEditor";
import {
  useCreateCsmRole,
  useGetCsmPermissions,
} from "@features/csm-admin/api/useCsmAdmin";
import type { CsmRole } from "@features/csm-admin/types/csmAdmin";

interface CreateRoleDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (role: CsmRole) => void;
}

const CATEGORY_LABEL: Record<string, string> = {
  cases: "Cases",
  projects: "Projects",
  accounts: "Accounts",
  engagements: "Engagements",
  updates: "Updates",
  security: "Security",
  time_cards: "Time cards",
  admin: "Administration",
};

export default function CreateRoleDialog({
  open,
  onClose,
  onCreated,
}: CreateRoleDialogProps): JSX.Element {
  const create = useCreateCsmRole();
  const { data: permissions } = useGetCsmPermissions();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [permissionIds, setPermissionIds] = useState<string[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setPermissionIds([]);
      setSubmitError(null);
    }
  }, [open]);

  const options: AssignmentOption[] = useMemo(
    () =>
      (permissions ?? []).map((p) => ({
        id: p.id,
        label: `${CATEGORY_LABEL[p.category] ?? p.category} · ${p.name}`,
        description: p.description,
      })),
    [permissions],
  );

  const trimmedName = name.trim();
  const formValid = trimmedName.length > 0;

  const handleSubmit = () => {
    if (!formValid || create.isPending) return;
    setSubmitError(null);
    create.mutate(
      {
        name: trimmedName,
        description: description.trim(),
        permissionIds,
      },
      {
        onSuccess: (role) => {
          onCreated?.(role);
          onClose();
        },
        onError: (err) => setSubmitError(err.message),
      },
    );
  };

  return (
    <Dialog open={open} onClose={create.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Create role</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField
            label="Name"
            size="small"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={create.isPending}
            autoFocus
            required
          />
          <TextField
            label="Description"
            size="small"
            multiline
            minRows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={create.isPending}
          />
          <Box>
            <Typography variant="caption" color="text.secondary">
              Permissions (optional)
            </Typography>
            <AssignmentEditor
              value={permissionIds}
              options={options}
              onChange={setPermissionIds}
              emptyLabel="No permissions granted yet."
              itemNoun="permissions"
              disabled={create.isPending}
            />
          </Box>
          {submitError && (
            <Typography variant="caption" color="error">
              {submitError}
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={create.isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleSubmit}
          disabled={!formValid || create.isPending}
        >
          {create.isPending ? "Creating…" : "Create role"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
