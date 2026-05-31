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
  useCreateCsmGroup,
  useGetCsmUsers,
} from "@features/csm-admin/api/useCsmAdmin";
import type { CsmGroup } from "@features/csm-admin/types/csmAdmin";

interface CreateGroupDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (group: CsmGroup) => void;
}

export default function CreateGroupDialog({
  open,
  onClose,
  onCreated,
}: CreateGroupDialogProps): JSX.Element {
  const create = useCreateCsmGroup();
  const { data: users } = useGetCsmUsers();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [memberIds, setMemberIds] = useState<string[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setMemberIds([]);
      setSubmitError(null);
    }
  }, [open]);

  const userOptions: AssignmentOption[] = useMemo(
    () =>
      (users ?? []).map((u) => ({
        id: u.id,
        label: u.name,
        description: u.email,
      })),
    [users],
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
        memberIds,
      },
      {
        onSuccess: (group) => {
          onCreated?.(group);
          onClose();
        },
        onError: (err) => setSubmitError(err.message),
      },
    );
  };

  return (
    <Dialog open={open} onClose={create.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Create group</DialogTitle>
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
              Initial members (optional)
            </Typography>
            <AssignmentEditor
              value={memberIds}
              options={userOptions}
              onChange={setMemberIds}
              emptyLabel="No members yet."
              itemNoun="users"
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
          {create.isPending ? "Creating…" : "Create group"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
