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
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, type JSX, type ReactNode } from "react";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { PLAN_RICH_TEXT_SX } from "@features/csm-operations/utils/planRichText";
import {
  changeRequestCategoryLabel,
  changeRequestImpactLabel,
} from "@features/csm-operations/utils/changeRequests";
import {
  changeRequestRiskLabel,
  highlightStandardChangeHtml,
  splitPlaceholders,
  standardChangeParameterTypeLabel,
} from "@features/csm-operations/utils/standardChanges";
import type { BeStandardChangeContent, BeStandardChangeParameter } from "@api/backend/types";

/**
 * How a placeholder reads inside a template's text: an open one (`{{key}}`, or a
 * preview with no value yet) in monospace on a tinted background, a filled-in
 * one tinted only. Relative `action.*` tokens, so it stands out in both schemes.
 */
const PLACEHOLDER_MARK_SX = {
  "& mark": {
    bgcolor: "action.selected",
    color: "text.primary",
    borderRadius: 0.5,
    px: 0.25,
    fontFamily: "monospace",
  },
  "& mark[data-filled]": { fontFamily: "inherit", bgcolor: "action.hover", textDecoration: "underline dotted" },
} as const;

/** What a preview fills the placeholders with; without it they are shown as placeholders. */
export interface StandardChangeFill {
  parameters: readonly BeStandardChangeParameter[];
  values: Readonly<Record<string, string>>;
}

/**
 * A plain text field (subject, rollback duration) with its placeholders
 * highlighted, or filled in from `fill` (whitespace collapsed, as the server
 * renders a plain field).
 */
export function PlaceholderText({ text, fill }: { text: string; fill?: StandardChangeFill }): JSX.Element {
  const declared = new Set((fill?.parameters ?? []).map((p) => p.key));
  return (
    <Box component="span" sx={PLACEHOLDER_MARK_SX}>
      {splitPlaceholders(text).map((s, i) => {
        if ("text" in s) return <span key={i}>{s.text}</span>;
        const value = fill && declared.has(s.key) ? (fill.values[s.key] ?? "").trim().split(/\s+/).filter(Boolean).join(" ") : "";
        return value ? (
          <mark key={i} data-param={s.key} data-filled="true">
            {value}
          </mark>
        ) : (
          <mark key={i} data-param={s.key}>
            {s.raw}
          </mark>
        );
      })}
    </Box>
  );
}

function RichSection({ title, html, fill }: { title: string; html: string | null; fill?: StandardChangeFill }): JSX.Element | null {
  const rendered = useMemo(
    () => (html && !isBlankHtml(html) ? highlightStandardChangeHtml(html, sanitizeRichTextHtml, fill) : null),
    [html, fill],
  );
  if (!rendered) return null;
  return (
    <Box component="section" aria-label={title} sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="subtitle2">{title}</Typography>
      <Box
        sx={[PLAN_RICH_TEXT_SX, PLACEHOLDER_MARK_SX]}
        // Sanitized in highlightStandardChangeHtml before any placeholder is replaced.
        dangerouslySetInnerHTML={{ __html: rendered }}
      />
    </Box>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Typography variant="body2" component="div">
        {children}
      </Typography>
    </Box>
  );
}

const yesNo = (v: boolean): string => (v ? "Yes" : "No");

/**
 * What a standard change template (a version, or a proposal's draft of one)
 * specifies, read-only: the subject, the classification and customer settings,
 * and the rich-text plans, with every `{{key}}` placeholder highlighted. With
 * `fill` it previews the change request raised with those values.
 */
export default function StandardChangeContentView({
  content,
  fill,
}: {
  content: BeStandardChangeContent;
  fill?: StandardChangeFill;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Field label="Subject">
        {content.subject ? <PlaceholderText text={content.subject} fill={fill} /> : "—"}
      </Field>
      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", md: "repeat(4, minmax(0, 1fr))" },
        }}
      >
        <Field label="Category">{changeRequestCategoryLabel(content.category)}</Field>
        <Field label="Impact">{content.impact ? changeRequestImpactLabel(content.impact) : "—"}</Field>
        <Field label="Risk">{changeRequestRiskLabel(content.risk)}</Field>
        <Field label="Rollback duration">
          {content.rollbackDurationText ? <PlaceholderText text={content.rollbackDurationText} fill={fill} /> : "—"}
        </Field>
        <Field label="Implementation plan visible to customers">{yesNo(content.isPlanningVisibleToCustomers)}</Field>
        <Field label="Customer approval">{yesNo(content.customerApprovalRequired)}</Field>
        <Field label="Customer review">{yesNo(content.customerReviewRequired)}</Field>
      </Box>
      <RichSection title="Description" html={content.description} fill={fill} />
      <RichSection title="Justification" html={content.justification} fill={fill} />
      <RichSection title="Implementation plan" html={content.implementationPlan} fill={fill} />
      <RichSection title="Risk and impact analysis" html={content.riskImpactAnalysis} fill={fill} />
      <RichSection title="Backout plan" html={content.backoutPlan} fill={fill} />
      <RichSection title="Test plan" html={content.testPlan} fill={fill} />
    </Box>
  );
}

/** A template's declared parameters ("change specifics"), read-only. */
export function StandardChangeParametersTable({
  parameters,
}: {
  parameters: readonly BeStandardChangeParameter[];
}): JSX.Element {
  if (parameters.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No parameters: every change request raised from it is the same.
      </Typography>
    );
  }
  return (
    <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
      <TableContainer>
        <Table size="small" aria-label="Parameters" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
          <TableHead>
            <TableRow sx={{ bgcolor: "action.hover" }}>
              <TableCell>Placeholder</TableCell>
              <TableCell>Label</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Required</TableCell>
              <TableCell>Help text / options</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {parameters.map((p) => (
              <TableRow key={p.key}>
                <TableCell sx={{ fontFamily: "monospace", whiteSpace: "nowrap" }}>{`{{${p.key}}}`}</TableCell>
                <TableCell sx={{ maxWidth: 260, wordBreak: "break-word" }}>{p.label}</TableCell>
                <TableCell sx={{ whiteSpace: "nowrap" }}>{standardChangeParameterTypeLabel(p.type)}</TableCell>
                <TableCell>{p.required ? "Yes" : "No"}</TableCell>
                <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>
                  {p.helpText && <Typography variant="body2">{p.helpText}</Typography>}
                  {p.type === "select" && (p.options?.length ?? 0) > 0 && (
                    <Box sx={{ display: "flex", gap: 0.5, flexWrap: "wrap", mt: p.helpText ? 0.5 : 0 }}>
                      {p.options!.map((o) => (
                        <Chip key={o} size="small" variant="outlined" label={o} />
                      ))}
                    </Box>
                  )}
                  {!p.helpText && p.type !== "select" && "—"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
