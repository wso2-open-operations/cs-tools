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
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import Error403Page from "@components/error/Error403Page";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import AsyncUserIdSingleSelect from "@features/csm-cases/components/AsyncUserIdSingleSelect";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import type { BeCaseDeescalator } from "@api/backend/types";
import {
  useChangeCaseDeescalator,
  useGetCaseDeescalators,
} from "@features/csm-admin/api/useCaseDeescalators";

function formatGrantedOn(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleDateString();
}

/**
 * Settings > User management > Escalation access (admin only): who may
 * de-escalate any case, besides the leads of the case's own ABT team (who
 * always may, for their team's cases). An admin adds someone from the internal
 * staff directory and can take the right away again; "reassigning" is removing
 * one person and adding another. The backend enforces admin-only again.
 */
export default function CsmEscalationAccessPage(): JSX.Element {
  const { isAdmin } = usePortalAccess();
  const list = useGetCaseDeescalators(isAdmin);
  const add = useChangeCaseDeescalator("add");
  const remove = useChangeCaseDeescalator("remove");
  const [selectedUserId, setSelectedUserId] = useState("");
  const [removing, setRemoving] = useState<BeCaseDeescalator | null>(null);

  if (!isAdmin) {
    return <Error403Page />;
  }

  const rows = list.data ?? [];
  const alreadyNamed = rows.some((r) => r.userId === selectedUserId);

  const handleAdd = (): void => {
    if (!selectedUserId || alreadyNamed) return;
    add.mutate(selectedUserId, { onSuccess: () => setSelectedUserId("") });
  };

  const handleRemove = (): void => {
    if (!removing) return;
    remove.mutate(removing.userId, { onSuccess: () => setRemoving(null) });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box>
        <Typography variant="h6">Escalation access</Typography>
        <Typography variant="body2" color="text.secondary">
          People named here may de-escalate any case. The leads of a case&apos;s own ABT team
          can always de-escalate their team&apos;s cases, whatever is listed here. Anyone can
          escalate who holds the escalator, CS engineer or admin role.
        </Typography>
      </Box>

      <Card variant="outlined" sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle2">Add a person</Typography>
        {add.isError && <Alert severity="error">{add.error.message}</Alert>}
        <Box sx={{ display: "flex", gap: 1.5, alignItems: "flex-start", flexWrap: "wrap" }}>
          <Box sx={{ minWidth: 280, flex: "1 1 280px", maxWidth: 480 }}>
            <AsyncUserIdSingleSelect
              id="deescalator-user"
              label="Internal user"
              placeholder="Search internal users…"
              value={selectedUserId}
              onChange={setSelectedUserId}
              roleIds={INTERNAL_USER_ROLES}
              active
            />
          </Box>
          <Button
            variant="contained"
            onClick={handleAdd}
            disabled={!selectedUserId || alreadyNamed || add.isPending}
            sx={{ mt: 0.5 }}
          >
            {add.isPending ? "Adding…" : "Let them de-escalate"}
          </Button>
        </Box>
        {alreadyNamed && (
          <Typography variant="caption" color="text.secondary">
            That person can already de-escalate any case.
          </Typography>
        )}
      </Card>

      <Card variant="outlined">
        {list.isLoading ? (
          <Skeleton variant="rounded" height={120} />
        ) : list.isError ? (
          <Alert severity="error" sx={{ m: 2 }}>
            Could not load who may de-escalate.
          </Alert>
        ) : rows.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
            Nobody is named yet. Only the leads of each case&apos;s ABT team can de-escalate.
          </Typography>
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>Email</TableCell>
                  <TableCell>Added by</TableCell>
                  <TableCell>Added on</TableCell>
                  <TableCell align="right" />
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((r) => (
                  <TableRow key={r.userId}>
                    <TableCell>
                      {r.name || r.email}{" "}
                      {!r.active && (
                        <Tooltip title="No longer active internal staff, so this lets them do nothing. Remove it.">
                          <Chip size="small" label="Inactive" color="warning" variant="outlined" />
                        </Tooltip>
                      )}
                    </TableCell>
                    <TableCell>{r.email}</TableCell>
                    <TableCell>{r.grantedBy}</TableCell>
                    <TableCell>{formatGrantedOn(r.grantedOn)}</TableCell>
                    <TableCell align="right">
                      <Button size="small" color="error" onClick={() => setRemoving(r)}>
                        Remove
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>

      {removing && (
        <Dialog open onClose={remove.isPending ? undefined : () => setRemoving(null)} maxWidth="xs" fullWidth>
          <DialogTitle>Remove {removing.name || removing.email}?</DialogTitle>
          <DialogContent dividers>
            {remove.isError && <Alert severity="error" sx={{ mb: 1 }}>{remove.error.message}</Alert>}
            <Typography variant="body2">
              They will no longer be able to de-escalate cases, unless they are the lead of the
              case&apos;s ABT team.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button color="inherit" onClick={() => setRemoving(null)} disabled={remove.isPending}>
              Cancel
            </Button>
            <Button color="error" variant="contained" onClick={handleRemove} disabled={remove.isPending}>
              {remove.isPending ? "Removing…" : "Remove"}
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </Box>
  );
}
