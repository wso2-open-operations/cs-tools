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
  Chip,
  IconButton,
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
import { Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useGetReleaseLines } from "@features/csm-products/api/useGetReleaseLines";
import ReleaseLineDialog from "@features/csm-products/components/ReleaseLineDialog";
import type { CatalogReleaseLine } from "@features/csm-products/types/productCatalog";
import { EMPTY_VALUE, formatCatalogDate } from "@features/csm-products/utils/catalogFormat";

const SKELETON_ROWS = 5;

/**
 * Release lines tab: every live release line with its start date, status and
 * how many versions belong to it; retired lines are not listed. Catalogue
 * managers create and edit lines, and retire one from its edit dialog; a save
 * applies at once.
 */
export default function ReleaseLinesPage(): JSX.Element {
  const canManage = usePortalAccess().canManageProductCatalog;
  const [editing, setEditing] = useState<{ line: CatalogReleaseLine | null } | null>(null);
  const { showSuccess } = useSuccessBanner();
  const lines = useGetReleaseLines();
  const columnCount = canManage ? 5 : 4;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {canManage && (
        <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
          <Button variant="contained" size="small" startIcon={<Plus size={16} />} onClick={() => setEditing({ line: null })}>
            New release line
          </Button>
        </Box>
      )}

      {lines.isError ? (
        <QueryErrorState message="Could not load release lines." error={lines.error} onRetry={() => void lines.refetch()} />
      ) : (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
          <TableContainer>
            <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
              <TableHead>
                <TableRow sx={{ bgcolor: "action.hover" }}>
                  <TableCell>Name</TableCell>
                  <TableCell>Start date</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Versions</TableCell>
                  {canManage && <TableCell align="right">Actions</TableCell>}
                </TableRow>
              </TableHead>
              <TableBody>
                {lines.isLoading &&
                  Array.from({ length: SKELETON_ROWS }).map((_, i) => (
                    <TableRow key={`s-${i}`}>
                      {Array.from({ length: columnCount }).map((__, c) => (
                        <TableCell key={c}>
                          <Skeleton />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                {!lines.isLoading && (lines.data ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={columnCount}>
                      <Typography variant="body2" color="text.secondary">
                        No release lines.
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
                {(lines.data ?? []).map((line) => (
                    <TableRow key={line.id} hover>
                      <TableCell sx={{ maxWidth: 320 }}>{line.name}</TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{formatCatalogDate(line.startDate)}</TableCell>
                      <TableCell>
                        {line.status ? <Chip size="small" variant="outlined" label={line.status} /> : EMPTY_VALUE}
                      </TableCell>
                      <TableCell align="right">{line.versionCount}</TableCell>
                      {canManage && (
                        <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                          <Tooltip title="Edit">
                            <IconButton size="small" aria-label={`Edit ${line.name}`} onClick={() => setEditing({ line })}>
                              <Pencil size={16} />
                            </IconButton>
                          </Tooltip>
                        </TableCell>
                      )}
                    </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}

      {editing && (
        <ReleaseLineDialog
          line={editing.line}
          onClose={() => setEditing(null)}
          onSaved={(message) => {
            setEditing(null);
            showSuccess(message);
          }}
        />
      )}

    </Box>
  );
}
