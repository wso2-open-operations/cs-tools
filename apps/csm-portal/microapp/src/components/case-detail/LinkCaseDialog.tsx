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

import { useMemo, useState } from "react";
import {
  alpha,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useQuery } from "@tanstack/react-query";
import { cases, QUICK_SEARCH_MIN_QUERY_LEN } from "@src/services/cases";
import type { CaseSummary } from "@src/types";
import { SeverityChip, StatusChip } from "@components/support/Chips";
import { useDebouncedValue } from "@utils/useDebouncedValue";

export type CaseLinkType = "parent" | "related";

interface LinkCaseDialogProps {
  /** The case being linked from — excluded from its own search results. */
  currentCaseId: string;
  /** True while a PATCH is in flight; disables the actions. */
  isLinking: boolean;
  onClose: () => void;
  /** Link the current case to `targetCaseId` as either its parent or a related case. */
  onLink: (targetCaseId: string, linkType: CaseLinkType) => void;
}

/**
 * Search-and-select a case to link the current one to — either as its **parent** (`PATCH
 * { parentId }`, the hierarchical major-case/child-case relationship) or as a **related case**
 * (`PATCH { relatedCaseId }`, a looser cross-link). Mirrors the webapp's LinkCaseDialog: picking a
 * search result doesn't link immediately — it shows a selected-case summary to confirm against
 * first, with a "Change" button back to search; the PATCH only fires from "Link".
 */
export function LinkCaseDialog({ currentCaseId, isLinking, onClose, onLink }: LinkCaseDialogProps) {
  const [linkType, setLinkType] = useState<CaseLinkType>("parent");
  const [input, setInput] = useState("");
  const [selected, setSelected] = useState<CaseSummary | null>(null);
  const debouncedInput = useDebouncedValue(input.trim(), 300);
  const { data, isFetching, isError } = useQuery(cases.quickSearch(debouncedInput));

  const candidates = useMemo(() => (data ?? []).filter((c) => c.id !== currentCaseId), [data, currentCaseId]);

  return (
    <Dialog
      open
      onClose={isLinking ? undefined : onClose}
      fullWidth
      maxWidth="xs"
      slotProps={{
        paper: {
          sx: {
            backgroundImage: "none",
            backgroundColor: "background.default",
            // Same explicit cap as LogTimeCardDialog — keeps the search results scrollable within
            // the dialog instead of letting the paper grow past the viewport. `dvh`, not `vh` — see
            // that file's comment for why.
            display: "flex",
            flexDirection: "column",
            maxHeight: "85dvh",
          },
        },
      }}
    >
      <DialogTitle>Link to another case</DialogTitle>
      <DialogContent dividers>
        <Stack gap={1.5}>
          <RadioGroup row value={linkType} onChange={(e) => setLinkType(e.target.value as CaseLinkType)}>
            <FormControlLabel value="parent" disabled={isLinking} control={<Radio size="small" />} label="As parent" />
            <FormControlLabel
              value="related"
              disabled={isLinking}
              control={<Radio size="small" />}
              label="As related case"
            />
          </RadioGroup>
          <Typography variant="caption" color="text.secondary">
            {linkType === "parent"
              ? "The hierarchical major-case/child-case relationship — this case can't close while it has open children linked this way."
              : "A looser cross-link; not subject to the child-case close restriction."}
          </Typography>

          {selected ? (
            <Stack
              direction="row"
              alignItems="flex-start"
              gap={1}
              sx={(theme) => ({
                px: 1.5,
                py: 1,
                borderRadius: 1,
                border: "1px solid",
                borderColor: "success.main",
                bgcolor: alpha(theme.palette.success.main, 0.1),
              })}
            >
              <Stack sx={{ minWidth: 0, flex: 1 }} gap={0.5}>
                <Typography
                  variant="body2"
                  noWrap
                  sx={{ fontWeight: 600 }}
                  title={`${selected.number} — ${selected.subject}`}
                >
                  {selected.number} — {selected.subject}
                </Typography>
                <Stack direction="row" alignItems="center" gap={0.75} flexWrap="wrap">
                  <StatusChip state={selected.state} />
                  {selected.severity && <SeverityChip severity={selected.severity} />}
                  {selected.assignedEngineer?.name && (
                    <Typography variant="caption" color="text.secondary" noWrap>
                      {selected.assignedEngineer.name}
                    </Typography>
                  )}
                </Stack>
              </Stack>
              <Button size="small" variant="text" disabled={isLinking} onClick={() => setSelected(null)}>
                Change
              </Button>
            </Stack>
          ) : (
            <Stack gap={0.75}>
              <TextField
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder="Search by case number or subject…"
                size="small"
                fullWidth
                autoFocus
                slotProps={{ input: { startAdornment: <Search size={16} style={{ marginRight: 8 }} /> } }}
              />
              {input.trim().length < QUICK_SEARCH_MIN_QUERY_LEN ? (
                <Typography variant="caption" color="text.secondary">
                  Type at least {QUICK_SEARCH_MIN_QUERY_LEN} characters to search
                </Typography>
              ) : isFetching ? (
                <Stack gap={0.5}>
                  {[0, 1, 2].map((i) => (
                    <Skeleton key={i} variant="rounded" height={44} />
                  ))}
                </Stack>
              ) : isError ? (
                <Typography variant="caption" color="error">
                  Could not search cases. Try again.
                </Typography>
              ) : candidates.length === 0 ? (
                <Typography variant="caption" color="text.secondary">
                  No matching cases.
                </Typography>
              ) : (
                <Stack>
                  {candidates.map((hit) => (
                    <Button
                      key={hit.id}
                      variant="text"
                      color="inherit"
                      disabled={isLinking}
                      onClick={() => setSelected(hit)}
                      sx={{ justifyContent: "flex-start", textTransform: "none", px: 1, py: 0.75, display: "flex" }}
                    >
                      <Stack sx={{ minWidth: 0, flex: 1, textAlign: "left" }} gap={0.25}>
                        <Typography variant="body2" noWrap>
                          {hit.number} — {hit.subject}
                        </Typography>
                        <Stack direction="row" gap={0.75}>
                          <StatusChip state={hit.state} />
                          {hit.severity && <SeverityChip severity={hit.severity} />}
                        </Stack>
                      </Stack>
                    </Button>
                  ))}
                </Stack>
              )}
            </Stack>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isLinking}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!selected || isLinking}
          onClick={() => selected && onLink(selected.id, linkType)}
        >
          {isLinking ? "Linking…" : "Link"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
