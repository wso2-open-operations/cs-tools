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
  useCreateCsmUser,
  useGetCsmGroups,
  useGetCsmRoles,
} from "@features/csm-admin/api/useCsmAdmin";
import type { CsmUser } from "@features/csm-admin/types/csmAdmin";

interface CreateUserDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated?: (user: CsmUser) => void;
}

const EMAIL_RX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function CreateUserDialog({
  open,
  onClose,
  onCreated,
}: CreateUserDialogProps): JSX.Element {
  const create = useCreateCsmUser();
  const { data: roles } = useGetCsmRoles();
  const { data: groups } = useGetCsmGroups();

  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [roleIds, setRoleIds] = useState<string[]>([]);
  const [groupIds, setGroupIds] = useState<string[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Reset state every time the dialog opens.
  useEffect(() => {
    if (open) {
      setName("");
      setEmail("");
      setRoleIds([]);
      setGroupIds([]);
      setSubmitError(null);
    }
  }, [open]);

  const roleOptions: AssignmentOption[] = useMemo(
    () =>
      (roles ?? []).map((r) => ({
        id: r.id,
        label: r.name,
        description: r.description,
      })),
    [roles],
  );
  const groupOptions: AssignmentOption[] = useMemo(
    () =>
      (groups ?? []).map((g) => ({
        id: g.id,
        label: g.name,
        description: g.description,
      })),
    [groups],
  );

  const trimmedName = name.trim();
  const trimmedEmail = email.trim();
  const emailValid = EMAIL_RX.test(trimmedEmail);
  const formValid = trimmedName.length > 0 && emailValid;

  const handleSubmit = () => {
    if (!formValid || create.isPending) return;
    setSubmitError(null);
    create.mutate(
      {
        name: trimmedName,
        email: trimmedEmail,
        roleIds,
        groupIds,
      },
      {
        onSuccess: (user) => {
          onCreated?.(user);
          onClose();
        },
        onError: (err) => {
          setSubmitError(err.message);
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={create.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Invite user</DialogTitle>
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
            label="Email"
            type="email"
            size="small"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={create.isPending}
            error={email.length > 0 && !emailValid}
            helperText={
              email.length > 0 && !emailValid
                ? "Enter a valid email address."
                : "Status starts as Invited until the user signs in."
            }
            required
          />
          <Box>
            <Typography variant="caption" color="text.secondary">
              Initial roles (optional)
            </Typography>
            <AssignmentEditor
              value={roleIds}
              options={roleOptions}
              onChange={setRoleIds}
              emptyLabel="No roles. User will have no permissions until you assign some."
              itemNoun="roles"
              disabled={create.isPending}
            />
          </Box>
          <Box>
            <Typography variant="caption" color="text.secondary">
              Initial groups (optional)
            </Typography>
            <AssignmentEditor
              value={groupIds}
              options={groupOptions}
              onChange={setGroupIds}
              emptyLabel="Not a member of any group."
              itemNoun="groups"
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
          {create.isPending ? "Creating…" : "Invite user"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
