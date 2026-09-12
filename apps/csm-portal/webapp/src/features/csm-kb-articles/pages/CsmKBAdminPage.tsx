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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus, Trash2, Users } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useBackendApi } from "@api/backend/client";
import { useListKnowledgeBases } from "@features/csm-kb-articles/api/useListKnowledgeBases";
import { useCreateKnowledgeBase } from "@features/csm-kb-articles/api/useCreateKnowledgeBase";
import { useSearchProducts } from "@features/csm-projects/api/useSearchProducts";
import { useUpdateKnowledgeBaseName } from "@features/csm-kb-articles/api/useUpdateKnowledgeBaseName";
import { useSetKnowledgeBaseActive } from "@features/csm-kb-articles/api/useSetKnowledgeBaseActive";
import { useSearchKBManagers } from "@features/csm-kb-articles/api/useSearchKBManagers";
import { useCreateKBManager } from "@features/csm-kb-articles/api/useCreateKBManager";
import { useDeleteKBManager } from "@features/csm-kb-articles/api/useDeleteKBManager";
import { useUsersByIds } from "@features/csm-kb-articles/api/useUsersByIds";
import type { KnowledgeBase } from "@features/csm-kb-articles/types/csmKbArticles";

/**
 * NOTE (per the Sep 10 call): this tab is not yet role-gated to an actual
 * "Knowledge Admin" role -- Sajith needs to confirm whether the roles table
 * has been migrated from ServiceNow before that check can be added. Visible
 * to every user for now; flagged clearly, not a silent gap.
 */

function CreateKBDialog({ open, onClose }: { open: boolean; onClose: () => void }): JSX.Element {
  const [productId, setProductId] = useState("");
  const [name, setName] = useState("");
  const create = useCreateKnowledgeBase();
  const { data: products } = useSearchProducts();

  const handleCreate = (): void => {
    if (!name.trim()) return;
    create.mutate(
      { ...(productId && { productId }), name: name.trim() },
      {
        onSuccess: () => {
          setProductId("");
          setName("");
          onClose();
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Create knowledge base</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
        <FormControl size="small" fullWidth>
          <InputLabel id="kb-product-select-label">Product (optional)</InputLabel>
          <Select
            labelId="kb-product-select-label"
            label="Product (optional)"
            value={productId}
            onChange={(e) => setProductId(e.target.value)}
          >
            <MenuItem value="">
              <em>No product</em>
            </MenuItem>
            {(products ?? []).map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField label="Knowledge base name" value={name} onChange={(e) => setName(e.target.value)} fullWidth />
        {create.isError && (
          <Alert severity="error">
            {create.error instanceof Error ? create.error.message : "Failed to create knowledge base."}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={create.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={handleCreate} disabled={create.isPending}>
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * Combined rename + activate/deactivate dialog. Deactivating lives here
 * (not a one-click button on the row) specifically per Sajith's Sep 11
 * feedback: "that active/deactivate button is kind of a risky operation...
 * let's ask for a confirmation." A second, nested confirmation step is
 * required before the toggle actually fires.
 */
function EditKBDialog({ kb, onClose }: { kb: KnowledgeBase | null; onClose: () => void }): JSX.Element {
  const [name, setName] = useState(kb?.name ?? "");
  const [confirmingToggle, setConfirmingToggle] = useState(false);
  const update = useUpdateKnowledgeBaseName();
  const setActive = useSetKnowledgeBaseActive();

  if (kb && name === "") setName(kb.name);

  const handleSave = (): void => {
    if (!kb || !name.trim()) return;
    update.mutate({ id: kb.id, name: name.trim() }, { onSuccess: onClose });
  };

  const handleConfirmToggle = (): void => {
    if (!kb) return;
    setActive.mutate(
      { id: kb.id, isActive: !kb.isActive },
      { onSuccess: () => setConfirmingToggle(false) },
    );
  };

  const handleClose = (): void => {
    setConfirmingToggle(false);
    onClose();
  };

  return (
    <Dialog open={Boolean(kb)} onClose={handleClose} fullWidth maxWidth="sm">
      <DialogTitle>Edit "{kb?.name}"</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
        <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} fullWidth />
        {update.isError && (
          <Alert severity="error">
            {update.error instanceof Error ? update.error.message : "Failed to rename."}
          </Alert>
        )}

        <Box sx={{ borderTop: 1, borderColor: "divider", pt: 2, mt: 1 }}>
          {!confirmingToggle ? (
            <Button
              size="small"
              variant="outlined"
              color={kb?.isActive ? "error" : "success"}
              onClick={() => setConfirmingToggle(true)}
            >
              {kb?.isActive ? "Deactivate this knowledge base" : "Activate this knowledge base"}
            </Button>
          ) : (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Alert severity="warning">
                {kb?.isActive
                  ? "Engineers will no longer be able to create new articles under this knowledge base. Existing articles are unaffected. Are you sure?"
                  : "This will let engineers create new articles under this knowledge base again. Are you sure?"}
              </Alert>
              {setActive.isError && (
                <Alert severity="error">
                  {setActive.error instanceof Error ? setActive.error.message : "Failed to update status."}
                </Alert>
              )}
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button size="small" onClick={() => setConfirmingToggle(false)} disabled={setActive.isPending}>
                  Cancel
                </Button>
                <Button
                  size="small"
                  variant="contained"
                  color={kb?.isActive ? "error" : "success"}
                  onClick={handleConfirmToggle}
                  disabled={setActive.isPending}
                >
                  Yes, {kb?.isActive ? "deactivate" : "activate"}
                </Button>
              </Box>
            </Box>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={update.isPending}>
          Close
        </Button>
        <Button variant="contained" onClick={handleSave} disabled={update.isPending}>
          Save name
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function ApproversDialog({ kb, onClose }: { kb: KnowledgeBase | null; onClose: () => void }): JSX.Element {
  const api = useBackendApi();
  const { data: managers } = useSearchKBManagers(kb?.id);
  const userIds = useMemo(() => (managers?.managers ?? []).map((m) => m.userId), [managers]);
  const { data: nameById = new Map<string, string>() } = useUsersByIds(userIds);

  const [email, setEmail] = useState("");
  const [lookupError, setLookupError] = useState<string | null>(null);
  const createManager = useCreateKBManager();
  const deleteManager = useDeleteKBManager();

  const handleAdd = async (): Promise<void> => {
    if (!kb || !email.trim()) return;
    setLookupError(null);
    try {
      const res = await api.post<{ filters: { emails: string[] } }, { users: { id: string }[] }>(
        "/users/search",
        { filters: { emails: [email.trim()] } },
      );
      const user = res.users?.[0];
      if (!user) {
        setLookupError("No user found with that email.");
        return;
      }
      createManager.mutate(
        { knowledgeBaseId: kb.id, userId: user.id },
        { onSuccess: () => setEmail("") },
      );
    } catch {
      setLookupError("Failed to look up that email.");
    }
  };

  return (
    <Dialog open={Boolean(kb)} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Approvers for "{kb?.name}"</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {(managers?.managers ?? []).length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No approvers assigned yet.
            </Typography>
          ) : (
            (managers?.managers ?? []).map((m) => (
              <Box key={m.id} sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <Typography variant="body2">{nameById.get(m.userId) ?? m.userId}</Typography>
                <IconButton
                  size="small"
                  onClick={() => kb && deleteManager.mutate({ knowledgeBaseId: kb.id, userId: m.userId })}
                  disabled={deleteManager.isPending}
                  aria-label="Remove approver"
                >
                  <Trash2 size={14} />
                </IconButton>
              </Box>
            ))
          )}
        </Box>

        <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
          <TextField
            size="small"
            label="Add approver by email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            fullWidth
          />
          <Button variant="outlined" onClick={handleAdd} disabled={createManager.isPending}>
            Add
          </Button>
        </Box>
        {lookupError && <Alert severity="error">{lookupError}</Alert>}
        {createManager.isError && (
          <Alert severity="error">
            {createManager.error instanceof Error ? createManager.error.message : "Failed to add approver."}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}

export default function CsmKBAdminPage(): JSX.Element {
  const { data: kbList, isLoading } = useListKnowledgeBases();

  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<KnowledgeBase | null>(null);
  const [approversTarget, setApproversTarget] = useState<KnowledgeBase | null>(null);

  const kbs = kbList?.knowledgeBases ?? [];

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Typography variant="body2" color="text.secondary">
          Create, rename, deactivate knowledge bases, and manage their approvers.
        </Typography>
        <Button variant="contained" size="small" startIcon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>
          Create new
        </Button>
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Name</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3} align="center" sx={{ py: 3 }}>
                    Loading…
                  </TableCell>
                </TableRow>
              ) : kbs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} align="center" sx={{ py: 3 }}>
                    No knowledge bases yet.
                  </TableCell>
                </TableRow>
              ) : (
                kbs.map((kb) => (
                  <TableRow key={kb.id}>
                    <TableCell>{kb.name}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        label={kb.isActive ? "Active" : "Deactivated"}
                        color={kb.isActive ? "success" : "default"}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell align="right">
                      <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
                        <IconButton size="small" onClick={() => setApproversTarget(kb)} aria-label="Manage approvers">
                          <Users size={16} />
                        </IconButton>
                        <IconButton size="small" onClick={() => setEditTarget(kb)} aria-label="Edit">
                          <Pencil size={16} />
                        </IconButton>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>

      <CreateKBDialog open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditKBDialog kb={editTarget} onClose={() => setEditTarget(null)} />
      <ApproversDialog kb={approversTarget} onClose={() => setApproversTarget(null)} />
    </Box>
  );
}
