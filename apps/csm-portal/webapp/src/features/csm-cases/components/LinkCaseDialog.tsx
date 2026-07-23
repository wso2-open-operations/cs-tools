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
  FormControlLabel,
  InputAdornment,
  Radio,
  RadioGroup,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  QUICK_CASE_MIN_QUERY_LEN,
  useQuickCaseSearch,
  type QuickCaseHit,
} from "@features/csm-cases/api/useQuickCaseSearch";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";

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
 * Search-and-select a case to link the current one to — either as its
 * **parent** (`PATCH { parentId }`, the hierarchical major-case/child-case
 * relationship, e.g. linking a service-request case back to the case or
 * incident that spawned it) or as a **related case** (`PATCH
 * { relatedCaseId }`, a looser cross-link not subject to the child-case
 * close restriction). Search reuses the same `POST /cases/search` lookup as
 * the quick-nav palette. Modeled on {@link AssignEngineerDialog}'s
 * search-and-pick pattern, but over cases instead of users. ServiceNow-source
 * only; the caller surfaces a rejection on another source.
 */
export default function LinkCaseDialog({
  currentCaseId,
  isLinking,
  onClose,
  onLink,
}: LinkCaseDialogProps): JSX.Element {
  const [linkType, setLinkType] = useState<CaseLinkType>("parent");
  const [input, setInput] = useState("");
  const search = useDebouncedValue(input.trim(), 300);
  const { data, isFetching, isError } = useQuickCaseSearch(search);

  const candidates = useMemo(
    () => (data ?? []).filter((c) => c.id !== currentCaseId),
    [data, currentCaseId],
  );

  const renderHit = (hit: QuickCaseHit): JSX.Element => (
    <Button
      key={hit.id}
      variant="text"
      color="inherit"
      disabled={isLinking}
      onClick={() => onLink(hit.id, linkType)}
      sx={{
        justifyContent: "flex-start",
        textTransform: "none",
        px: 1,
        py: 0.75,
        gap: 1.25,
        display: "flex",
      }}
    >
      <Box sx={{ minWidth: 0, textAlign: "left", flex: 1 }}>
        <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
          {hit.caseNumber ?? hit.wso2CaseId ?? hit.id} — {hit.subject}
        </Typography>
        <Box sx={{ display: "flex", gap: 0.75, mt: 0.25 }}>
          <SeverityChip severity={hit.severity} />
          <StateChip state={hit.state} />
        </Box>
      </Box>
    </Button>
  );

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Link to another case</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <RadioGroup
            row
            value={linkType}
            onChange={(e) => setLinkType(e.target.value as CaseLinkType)}
          >
            <FormControlLabel
              value="parent"
              disabled={isLinking}
              control={<Radio size="small" />}
              label="As parent"
            />
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

          <TextField
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Search by case number or subject…"
            size="small"
            fullWidth
            autoFocus
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            }}
          />

          <Box sx={{ minHeight: 160 }}>
            {search.length < QUICK_CASE_MIN_QUERY_LEN ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                Type at least {QUICK_CASE_MIN_QUERY_LEN} characters to search.
              </Typography>
            ) : isFetching ? (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, pt: 0.75 }}>
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} variant="rounded" height={44} />
                ))}
              </Box>
            ) : isError ? (
              <Typography variant="body2" color="error" sx={{ py: 2 }}>
                Could not search cases. Try again.
              </Typography>
            ) : candidates.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                No matching cases.
              </Typography>
            ) : (
              <Box sx={{ display: "flex", flexDirection: "column" }}>
                {candidates.map(renderHit)}
              </Box>
            )}
          </Box>

          <Typography variant="caption" color="text.secondary">
            Linking applies to ServiceNow-managed cases.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isLinking}>
          Cancel
        </Button>
      </DialogActions>
    </Dialog>
  );
}
