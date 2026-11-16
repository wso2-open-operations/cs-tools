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
  Divider,
  FormControlLabel,
  InputAdornment,
  Modal,
  Paper,
  Radio,
  RadioGroup,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
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
  /**
   * Why the current ticket can't have a related case, when it can't. Only a case
   * keeps one (the backend refuses `relatedCaseId` on a service request,
   * engagement, security report or announcement), so for those the option is
   * shown disabled with this reason. Undefined: both options are offered.
   */
  relatedUnavailableReason?: string;
}

/**
 * Search-and-select a case to link the current one to — either as its
 * **parent** (`PATCH { parentId }`, the hierarchical major-case/child-case
 * relationship, e.g. linking a service-request case back to the case or
 * incident that spawned it) or as a **related case** (`PATCH
 * { relatedCaseId }`, a looser cross-link not subject to the child-case
 * close restriction). Search reuses the same `POST /cases/search` lookup as
 * the quick-nav palette. Picking a search result doesn't link immediately —
 * it shows a selected-case summary panel (case number/subject, severity,
 * state, assignee) to confirm against first, the same "search → pick →
 * review before committing" shape {@link ProjectSelectionField} uses for
 * picking a project on case creation; a "Change" button clears the
 * selection back to search, and the actual PATCH only fires from the
 * dialog's "Link" action. A plain `Modal` + `Paper` rather than `Dialog` —
 * same reasoning as {@link ProjectSelectionField}'s confirm panel: `Dialog`'s
 * paper renders on the theme's more opaque `background.paper`, while bare
 * `Paper` gets the lighter, translucent "acrylic" look for free. ServiceNow-
 * source only; the caller surfaces a rejection on another source.
 */
export default function LinkCaseDialog({
  currentCaseId,
  isLinking,
  onClose,
  onLink,
  relatedUnavailableReason,
}: LinkCaseDialogProps): JSX.Element {
  const [linkType, setLinkType] = useState<CaseLinkType>("parent");
  const [input, setInput] = useState("");
  const [selected, setSelected] = useState<QuickCaseHit | null>(null);
  const search = useDebouncedValue(input.trim(), 300);
  const { data, isFetching, isError } = useQuickCaseSearch(search);
  // MUI Button doesn't forward `autoFocus` to its underlying element, so once
  // the search list unmounts in favour of the confirm panel, focus would
  // otherwise drop to the document body. Move it to "Change" explicitly.
  const changeButtonRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (selected) changeButtonRef.current?.focus();
  }, [selected]);

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
      onClick={() => setSelected(hit)}
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
    <Modal
      open
      onClose={onClose}
      slotProps={{ backdrop: { sx: { backgroundColor: "transparent" } } }}
    >
      <Paper
        elevation={3}
        role="dialog"
        aria-modal="true"
        aria-labelledby="link-case-dialog-title"
        sx={{
          position: "fixed",
          top: "10vh",
          left: "50%",
          transform: "translateX(-50%)",
          width: { xs: "calc(100% - 32px)", sm: 440 },
          maxWidth: 440,
          maxHeight: "80vh",
          display: "flex",
          flexDirection: "column",
          outline: "none",
          border: "1px solid",
          borderColor: "divider",
        }}
      >
        <Typography id="link-case-dialog-title" variant="h6" sx={{ p: 3, pb: 2 }}>
          Link to another case
        </Typography>
        <Divider />
        <Box sx={{ p: 3, overflowY: "auto", display: "flex", flexDirection: "column", gap: 1.5 }}>
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
              disabled={isLinking || !!relatedUnavailableReason}
              control={
                <Radio
                  size="small"
                  inputProps={
                    relatedUnavailableReason
                      ? { "aria-describedby": "link-case-related-unavailable" }
                      : undefined
                  }
                />
              }
              label="As related case"
            />
          </RadioGroup>
          <Typography variant="caption" color="text.secondary">
            {linkType === "parent"
              ? "The hierarchical major-case/child-case relationship — this case can't close while it has open children linked this way."
              : "A looser cross-link; not subject to the child-case close restriction."}
          </Typography>
          {relatedUnavailableReason && (
            <Typography
              id="link-case-related-unavailable"
              variant="caption"
              color="text.secondary"
            >
              {relatedUnavailableReason}
            </Typography>
          )}

          {selected ? (
            // Confirm-before-committing panel — same shape as
            // ProjectSelectionField's "selected project" summary: a
            // success-tinted panel with the pick's key facts and a "Change"
            // button back to search, rather than linking on row-click.
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1,
                px: 1.5,
                py: 1,
                borderRadius: 1,
                border: "1px solid",
                borderColor: "success.main",
                bgcolor: "success.50",
              }}
            >
              <Box sx={{ display: "flex", color: "success.main", flexShrink: 0, mt: 0.25 }}>
                <CheckCircle size={16} aria-hidden />
              </Box>
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography variant="body2" noWrap sx={{ fontWeight: 600, lineHeight: 1.3 }}>
                  {selected.caseNumber ?? selected.wso2CaseId ?? selected.id} — {selected.subject}
                </Typography>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, mt: 0.5 }}>
                  <SeverityChip severity={selected.severity} />
                  <StateChip state={selected.state} />
                  {selected.assigneeName && (
                    <Typography variant="caption" color="text.secondary" noWrap>
                      {selected.assigneeName}
                    </Typography>
                  )}
                </Box>
              </Box>
              <Button
                ref={changeButtonRef}
                size="small"
                variant="text"
                disabled={isLinking}
                onClick={() => setSelected(null)}
                sx={{ minWidth: 0, px: 1, flexShrink: 0 }}
              >
                Change
              </Button>
            </Box>
          ) : (
            // Own tight gap (not the outer content Box's 1.5-unit gap) between
            // the search box and whatever's below it — a Fragment here would
            // let the outer gap apply between TextField and this area too,
            // compounding with any padding on the hint text below.
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
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

              <Box>
                {search.length < QUICK_CASE_MIN_QUERY_LEN ? (
                  <Typography variant="caption" color="text.secondary">
                    Type at least {QUICK_CASE_MIN_QUERY_LEN} characters to search
                  </Typography>
                ) : isFetching ? (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, minHeight: 160 }}>
                    {[0, 1, 2].map((i) => (
                      <Skeleton key={i} variant="rounded" height={44} />
                    ))}
                  </Box>
                ) : isError ? (
                  <Typography variant="caption" color="error">
                    Could not search cases. Try again.
                  </Typography>
                ) : candidates.length === 0 ? (
                  <Typography variant="caption" color="text.secondary">
                    No matching cases.
                  </Typography>
                ) : (
                  <Box sx={{ display: "flex", flexDirection: "column", minHeight: 160 }}>
                    {candidates.map(renderHit)}
                  </Box>
                )}
              </Box>
            </Box>
          )}

          <Typography variant="caption" color="text.secondary">
            Linking applies to ServiceNow-managed cases.
          </Typography>
        </Box>
        <Divider />
        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, p: 2 }}>
          <Button variant="outlined" color="inherit" onClick={onClose} disabled={isLinking}>
            Cancel
          </Button>
          <Button
            variant="contained"
            disabled={!selected || isLinking}
            onClick={() => selected && onLink(selected.id, linkType)}
          >
            {isLinking ? "Linking…" : "Link"}
          </Button>
        </Box>
      </Paper>
    </Modal>
  );
}
