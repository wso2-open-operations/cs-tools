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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import { useMySkillRequests, useWithdrawSkillRequest } from "@features/csm-skills/api/useSkillManagement";
import LevelChip from "@features/csm-skills/components/LevelChip";
import {
  describeAutoAdd,
  personName,
  REQUEST_STATUS_COLOR,
  REQUEST_STATUS_LABEL,
} from "@features/csm-skills/utils/skillRequests";

interface MySkillRequestsProps {
  onNotice: (message: string) => void;
  onError: (message: string) => void;
}

/**
 * "My requests": the skills the caller asked for and where each stands. A
 * pending one can be withdrawn; a decided one shows who decided, the reason
 * given, and (when approved) what was added to the caller's skills. Shown only
 * once the caller has made a request.
 */
export default function MySkillRequests({ onNotice, onError }: MySkillRequestsProps): JSX.Element | null {
  const requests = useMySkillRequests();
  const withdraw = useWithdrawSkillRequest();
  const rows = requests.data?.requests ?? [];
  if (rows.length === 0) return null;

  return (
    <Box component="section" aria-labelledby="my-skill-requests-title" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography id="my-skill-requests-title" variant="subtitle1" component="h2">
        My skill requests
      </Typography>
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="My skill requests" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Skill</TableCell>
                <TableCell>Level asked</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Outcome</TableCell>
                <TableCell>Requested</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((r) => {
                const added = r.status === "APPROVED" ? describeAutoAdd(r) : null;
                return (
                  <TableRow key={r.id}>
                    <TableCell sx={{ maxWidth: 260 }}>
                      <Typography variant="body2">{r.resolvedSkill?.name ?? r.requestedName}</Typography>
                      {r.resolvedSkill && r.resolvedSkill.name !== r.requestedName && (
                        <Typography variant="caption" color="text.secondary">
                          Asked as “{r.requestedName}”
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <LevelChip level={r.requestedLevel} />
                    </TableCell>
                    <TableCell>
                      <Chip size="small" variant="outlined" color={REQUEST_STATUS_COLOR[r.status]} label={REQUEST_STATUS_LABEL[r.status]} />
                    </TableCell>
                    <TableCell sx={{ maxWidth: 360 }}>
                      {r.status === "PENDING" ? (
                        <Typography variant="caption" color="text.secondary">
                          Waiting for review
                        </Typography>
                      ) : r.status === "WITHDRAWN" ? (
                        <Typography variant="caption" color="text.secondary">
                          You withdrew it
                        </Typography>
                      ) : (
                        <Box>
                          {added && <Typography variant="body2">{added}</Typography>}
                          {r.reviewComment && (
                            <Typography variant="caption" color="text.secondary" component="div">
                              {r.status === "REJECTED" ? "Reason: " : "Note: "}
                              {r.reviewComment}
                            </Typography>
                          )}
                          {r.reviewedBy && (
                            <Typography variant="caption" color="text.secondary" component="div">
                              By {personName(r.reviewedBy)}
                            </Typography>
                          )}
                        </Box>
                      )}
                    </TableCell>
                    <TableCell>
                      <RelativeTime iso={r.createdOn} />
                    </TableCell>
                    <TableCell align="right">
                      {r.status === "PENDING" && (
                        <Button
                          size="small"
                          disabled={withdraw.isPending}
                          aria-label={`Withdraw the request for ${r.requestedName}`}
                          onClick={() =>
                            withdraw.mutate(r.id, {
                              onSuccess: () => onNotice(`Your request for ${r.requestedName} was withdrawn.`),
                              onError: (e) => onError(e.message || "The request could not be withdrawn."),
                            })
                          }
                        >
                          Withdraw
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Box>
  );
}
