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

import { Box, Chip, Collapse, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type { BeWidgetResourceType } from "@api/backend/types";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import { groupCasesByAccount } from "@features/csm-dashboard/utils/groupCasesByAccount";

interface AccountGroupedCasesListProps {
  items: Record<string, unknown>[];
  isLoading: boolean;
  isError: boolean;
  resourceType: BeWidgetResourceType;
  /** Set when the fetch stopped short of the search's own total. */
  truncated?: boolean;
  total?: number;
}

/**
 * One row per account (name + how many of the widget's cases it has, most
 * first), each expanding to that account's cases. The expanded table is the
 * widget's own list renderer (`WIDGET_LIST_RENDERERS`, i.e. the Cases tab's
 * `CasesList` for a case widget), so columns, links and the preview drawer
 * behave exactly as in the flat preview.
 */
export default function AccountGroupedCasesList({
  items,
  isLoading,
  isError,
  resourceType,
  truncated = false,
  total,
}: AccountGroupedCasesListProps): JSX.Element {
  const groups = useMemo(() => groupCasesByAccount(items), [items]);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const ListRenderer = WIDGET_LIST_RENDERERS[resourceType];

  if (isError) {
    return (
      <Typography variant="body2" color="text.secondary">
        Could not load this widget.
      </Typography>
    );
  }
  if (isLoading) {
    return (
      <Box aria-label="Loading accounts" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} variant="rounded" height={44} />
        ))}
      </Box>
    );
  }
  if (groups.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No accounts match this widget.
      </Typography>
    );
  }

  const toggle = (key: string): void =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {truncated && (
        <Typography variant="caption" color="warning.main" role="status">
          Showing the first {items.length} of {total} cases; accounts and counts below may be incomplete.
        </Typography>
      )}
      {groups.map((group) => {
        const key = group.accountId || `name:${group.accountName}`;
        const isOpen = expanded.has(key);
        const panelId = `account-cases-${key.replace(/[^a-zA-Z0-9_-]/g, "_")}`;
        const caseCount = group.items.length;
        return (
          <Box key={key} sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
            <Box
              component="button"
              type="button"
              onClick={() => toggle(key)}
              aria-expanded={isOpen}
              aria-controls={panelId}
              aria-label={`${group.accountName}, ${caseCount} ${caseCount === 1 ? "case" : "cases"}`}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
                width: "100%",
                px: 1.5,
                py: 1,
                border: 0,
                bgcolor: "transparent",
                color: "text.primary",
                font: "inherit",
                textAlign: "left",
                cursor: "pointer",
                "&:hover": { bgcolor: "action.hover" },
              }}
            >
              <ChevronDown
                size={16}
                aria-hidden
                style={{ transform: isOpen ? "none" : "rotate(-90deg)", transition: "transform 0.15s" }}
              />
              <Typography variant="subtitle2" sx={{ flex: 1 }} noWrap>
                {group.accountName}
              </Typography>
              <Chip size="small" label={`${caseCount} ${caseCount === 1 ? "case" : "cases"}`} />
            </Box>
            <Collapse in={isOpen} timeout="auto" unmountOnExit>
              <Box id={panelId} sx={{ px: 1, pb: 1 }}>
                <ListRenderer items={group.items} isLoading={false} resourceType={resourceType} />
              </Box>
            </Collapse>
          </Box>
        );
      })}
    </Box>
  );
}
