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

import { Box, Card, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import { tierColor, tierLabel } from "@features/csm-cases/utils/caseTier";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import SemanticChip from "@components/SemanticChip";

interface CaseMetaBandProps {
  detail: CsmCaseDetail;
  collapsed: boolean;
  onToggleCollapsed: () => void;
}

function Cell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 0.25,
        minWidth: 0,
      }}
    >
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function LinkText({
  to,
  children,
}: {
  to: string;
  children: ReactNode;
}): JSX.Element {
  return (
    // Real anchor (RouterLink) so account/project links are cmd/middle-clickable
    // and copyable, with plain left-click staying in-app.
    <Typography
      component={RouterLink}
      to={to}
      variant="body2"
      noWrap
      sx={(t) => ({
        // Block (not the default inline anchor) so `noWrap` actually clips with
        // an ellipsis inside the min-width:0 grid cell instead of overflowing
        // into the next column on a long project/account name.
        display: "block",
        cursor: "pointer",
        textDecoration: "none",
        // Brand orange (`primary.main`) on a light surface fails WCAG AA
        // (~2.5:1), and the darker `primary.dark` shade fails on the dark
        // surface (~4.0:1). Pick per colour scheme. `palette.mode` is unreliable
        // here: the app drives theming through MUI CssVars (`useColorScheme`),
        // where `palette.mode` stays pinned to the default scheme, so a
        // `mode === "dark"` check always resolved to the light branch. Scope the
        // dark colour with `applyStyles("dark", …)` instead — the same mechanism
        // the a11y button overrides use.
        color: t.palette.primary.dark,
        ...t.applyStyles("dark", {
          color: t.palette.primary.main,
        }),
        "&:hover": { textDecoration: "underline" },
        "&:focus-visible": {
          outline: "2px solid",
          outlineColor: "primary.main",
          outlineOffset: 2,
          borderRadius: 0.5,
        },
      })}
    >
      {children}
    </Typography>
  );
}

/**
 * Always-visible facts band shown directly under the case header. Engineers
 * see assignee, project type, deployment/product, who created the case, and
 * which account/project it belongs to without diving into a tab. The chevron in
 * the top-right collapses the band body into a slim header strip.
 */
export default function CaseMetaBand({
  detail: c,
  collapsed,
  onToggleCollapsed,
}: CaseMetaBandProps): JSX.Element {
  const product = c.productContext;
  const tier = c.customerContext.tier;
  // Project type is encoded as the second " - "-delimited segment of the
  // project name (e.g. "Acme - Managed Cloud" → "Managed Cloud"). Temporary
  // until the backend exposes it as a first-class field.
  const projectType = c.projectName?.split(" - ")[1]?.trim() || "—";
  // One-line digest shown when the band is collapsed, so collapsing doesn't
  // hide every triage fact — account, tier, and who owns the case stay visible.
  const collapsedSummary = [c.customer, tierLabel(tier), c.assignee]
    .filter(Boolean)
    .join(" · ");

  return (
    <Card
      variant="outlined"
      sx={{
        backgroundColor: "background.default",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          px: 2,
          py: collapsed ? 0.75 : 1,
        }}
      >
        {collapsed ? (
          <Typography
            variant="body2"
            color="text.secondary"
            noWrap
            sx={{ minWidth: 0, mr: 1 }}
          >
            {collapsedSummary}
          </Typography>
        ) : (
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ textTransform: "uppercase", letterSpacing: 0.4, fontWeight: 600 }}
          >
            Overview
          </Typography>
        )}
        <Tooltip title={collapsed ? "Show details" : "Hide details"}>
          <IconButton
            size="small"
            onClick={onToggleCollapsed}
            aria-label={collapsed ? "Show case details" : "Hide case details"}
          >
            {collapsed ? <ChevronDown size={18} /> : <ChevronUp size={18} />}
          </IconButton>
        </Tooltip>
      </Box>
      {!collapsed && (
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr 1fr",
              sm: "repeat(3, minmax(0, 1fr))",
              md: "repeat(4, minmax(0, 1fr))",
            },
            gap: 2,
            px: 2,
            pb: 2,
            pt: 0.5,
            borderTop: 1,
            borderColor: "divider",
          }}
        >
          <Cell label="Tier">
            <Box sx={{ minWidth: 0 }}>
              <SemanticChip role={tierColor(tier)} label={tierLabel(tier)} />
            </Box>
          </Cell>
          <Cell label="Assignee">
            <Typography variant="body2" noWrap>
              {c.assigneeIsMe ? (
                <strong>{c.assignee}</strong>
              ) : (
                c.assignee
              )}
            </Typography>
          </Cell>
          <Cell label="Created by">
            <Typography variant="body2" noWrap>
              {c.createdBy ?? c.customerContext.primaryContact ?? "—"}
            </Typography>
          </Cell>
          <Cell label="Account">
            {c.accountId ? (
              <LinkText to={`/accounts/${c.accountId}`}>
                {c.customer}
              </LinkText>
            ) : (
              <Typography variant="body2" noWrap>
                {c.customer}
              </Typography>
            )}
          </Cell>
          <Cell label="Project">
            {c.projectId ? (
              <LinkText to={`/projects/${c.projectId}`}>
                {c.projectName}
              </LinkText>
            ) : (
              <Typography variant="body2" noWrap>
                {c.projectName}
              </Typography>
            )}
          </Cell>
          <Cell label="Project type">
            <Typography variant="body2" noWrap>
              {projectType}
            </Typography>
          </Cell>
          <Cell label="Deployment">
            <Typography variant="body2" noWrap>
              {product.deployment ?? "—"}
            </Typography>
          </Cell>
          <Cell label="Product">
            <Typography variant="body2" noWrap>
              {product.product ?? "—"}
            </Typography>
          </Cell>
        </Box>
      )}
    </Card>
  );
}
