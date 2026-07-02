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

import { useMemo, useState, type JSX } from "react";
import {
  Avatar,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { initialsOf } from "@utils/userClaims";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { MOCK_APPROVERS } from "@features/csm-timecards/constants/timeCardConstants";
import type { ApproverDelegation } from "@features/csm-timecards/types/timeCards";

interface DelegateInput {
  delegateId: string;
  delegateName: string;
  from: string;
  to: string;
}

interface DelegateApprovalsDialogProps {
  current: ApproverDelegation | null;
  isSaving: boolean;
  onClose: () => void;
  /** Save a delegation, or pass `null` to clear it. */
  onSave: (input: DelegateInput | null) => void;
}

function isoToday(): string {
  return new Date().toISOString().slice(0, 10);
}
function isoInDays(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return d.toISOString().slice(0, 10);
}

/**
 * Lets an approver delegate their approvals to another approver for a date
 * range (no per-resource delegation, no re-delegation — per the spec). While a
 * delegation is active, the approver's own accept/reject actions are suspended.
 */
export default function DelegateApprovalsDialog({
  current,
  isSaving,
  onClose,
  onSave,
}: DelegateApprovalsDialogProps): JSX.Element {
  const [delegate, setDelegate] = useState<{ id: string; name: string } | null>(
    current ? { id: current.delegateId, name: current.delegateName } : null,
  );
  const [input, setInput] = useState("");
  const [from, setFrom] = useState(current?.from ?? isoToday());
  const [to, setTo] = useState(current?.to ?? isoInDays(14));

  const search = useDebouncedValue(input.trim(), 300);
  const { data } = useSearchUsers({
    ...(search.length > 0 && { searchQuery: search }),
    pagination: { limit: 6, offset: 0 },
  });
  const candidates = useMemo(() => {
    const live = (data?.users ?? [])
      .filter((u) => u.userType === "internal" && !!u.email)
      .map((u) => ({
        id: u.id,
        name: u.name.trim() || u.userName,
      }));
    if (live.length > 0) return live;
    const q = input.trim().toLowerCase();
    return MOCK_APPROVERS.filter((a) => !q || a.name.toLowerCase().includes(q));
  }, [data, input]);

  const canSave = !!delegate && !!from && !!to && from <= to;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Delegate approvals</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            Hand your time-card approvals to another approver while you're away.
            Your own accept/reject is paused for the period.
          </Typography>

          {delegate ? (
            <Chip
              label={delegate.name}
              onDelete={() => setDelegate(null)}
              deleteIcon={<X size={14} />}
              sx={{ alignSelf: "flex-start" }}
            />
          ) : (
            <>
              <TextField
                size="small"
                label="Delegate to"
                placeholder="Search approvers…"
                value={input}
                onChange={(e) => setInput(e.target.value)}
              />
              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 1,
                  maxHeight: 160,
                  overflowY: "auto",
                }}
              >
                {candidates.length === 0 ? (
                  <Typography variant="caption" color="text.secondary" sx={{ p: 1 }}>
                    No matching approvers.
                  </Typography>
                ) : (
                  candidates.map((u) => (
                    <Button
                      key={u.id}
                      variant="text"
                      color="inherit"
                      onClick={() => {
                        setDelegate(u);
                        setInput("");
                      }}
                      sx={{
                        justifyContent: "flex-start",
                        textTransform: "none",
                        px: 1,
                        py: 0.5,
                        gap: 1,
                      }}
                    >
                      <Avatar sx={{ width: 24, height: 24, fontSize: "0.7rem" }}>
                        {initialsOf(u.name)}
                      </Avatar>
                      <Typography variant="body2">{u.name}</Typography>
                    </Button>
                  ))
                )}
              </Box>
            </>
          )}

          <Box sx={{ display: "flex", gap: 1.5 }}>
            <TextField
              type="date"
              label="From"
              size="small"
              fullWidth
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              slotProps={{ inputLabel: { shrink: true } }}
            />
            <TextField
              type="date"
              label="To"
              size="small"
              fullWidth
              value={to}
              onChange={(e) => setTo(e.target.value)}
              slotProps={{ inputLabel: { shrink: true } }}
            />
          </Box>
        </Box>
      </DialogContent>
      <DialogActions sx={{ justifyContent: "space-between" }}>
        <Button
          color="error"
          disabled={!current || isSaving}
          onClick={() => onSave(null)}
        >
          Remove delegation
        </Button>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button color="inherit" onClick={onClose} disabled={isSaving}>
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={!canSave || isSaving}
            onClick={() =>
              delegate &&
              onSave({
                delegateId: delegate.id,
                delegateName: delegate.name,
                from,
                to,
              })
            }
          >
            Delegate
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}
