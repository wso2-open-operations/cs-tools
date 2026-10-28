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
  Checkbox,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormHelperText,
  IconButton,
  InputLabel,
  Link,
  Menu,
  MenuItem,
  Select,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Ban, Braces, Check, CopyPlus, Save, Send, Trash2, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useRef, useState, type JSX, type ReactNode } from "react";
import { useLocation } from "react-router";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  useCreateStandardChangeProposal,
  useDecideStandardChangeProposal,
  useGetStandardChangeProposal,
  useSearchClosedChangeRequestsForSelect,
  useSubmitStandardChangeProposal,
  useUpdateStandardChangeProposal,
  useWithdrawStandardChangeProposal,
} from "@features/csm-operations/api/useStandardChanges";
import ExpandableRichTextField from "@features/csm-operations/components/ExpandableRichTextField";
import StandardChangeContentView, {
  StandardChangeParametersTable,
} from "@features/csm-operations/components/StandardChangeContentView";
import StandardChangeParameterInputs from "@features/csm-operations/components/StandardChangeParameterInputs";
import StandardChangeParametersEditor from "@features/csm-operations/components/StandardChangeParametersEditor";
import { PLAN_RICH_TEXT_SX } from "@features/csm-operations/utils/planRichText";
import {
  approvalStatusColor,
  approvalStatusLabel,
  CHANGE_REQUEST_CATEGORY_OPTIONS,
  CHANGE_REQUEST_IMPACT_OPTIONS,
  CHANGE_REQUEST_ROLLBACK_DURATION_MAX,
} from "@features/csm-operations/utils/changeRequests";
import {
  appendPlaceholderToHtml,
  hasParameterIssues,
  insertPlaceholderAt,
  isStandardChangeProposalFinal,
  normalizeStandardChangeParameters,
  STANDARD_CHANGE_HIGH_RISK_REASON,
  STANDARD_CHANGE_RISK_OPTIONS,
  STANDARD_CHANGES_TAB_PATH,
  standardChangeProposalGaps,
  standardChangeProposalKindLabel,
  standardChangeProposalPath,
  standardChangeProposalStateColor,
  standardChangeProposalStateLabel,
  standardChangeTemplatePath,
  undeclaredPlaceholders,
  unusedParameters,
  validateStandardChangeParameters,
} from "@features/csm-operations/utils/standardChanges";
import type {
  BeChangeRequestCategory,
  BeChangeRequestImpact,
  BeChangeRequestRisk,
  BeChangeRequestSearchView,
  BeStandardChangeContent,
  BeStandardChangeEvidence,
  BeStandardChangeParameter,
  BeStandardChangeProposal,
  BeUpdateStandardChangeProposalPayload,
} from "@api/backend/types";

/** The proposal's subject is the template's name on the change requests raised from it (work_item.subject). */
const NAME_MAX = 255;

function formatDateTime(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

function personLabel(p?: { name?: string | null; email?: string | null } | null): string {
  return p?.name || p?.email || "—";
}

/** The proposal's rich-text fields, in the order the form shows them. */
const RICH_FIELDS = [
  { field: "description", label: "Description", helper: "What the change does." },
  { field: "justification", label: "Justification", helper: "Why the change is made." },
  { field: "implementationPlan", label: "Implementation plan", helper: "Required. The steps, generalised: mark what varies as {{key}}." },
  { field: "riskImpactAnalysis", label: "Risk and impact analysis", helper: "What could go wrong, and what is affected if it does." },
  { field: "backoutPlan", label: "Backout plan", helper: "Required. How the change is backed out." },
  { field: "testPlan", label: "Test plan", helper: "Required. How the change is verified." },
] as const;

type RichField = (typeof RICH_FIELDS)[number]["field"];

/** The editable copy of a draft. */
interface ProposalForm {
  name: string;
  rationale: string;
  subject: string;
  description: string;
  justification: string;
  implementationPlan: string;
  riskImpactAnalysis: string;
  backoutPlan: string;
  testPlan: string;
  category: BeChangeRequestCategory | "";
  impact: BeChangeRequestImpact | "";
  risk: BeChangeRequestRisk | "";
  rollbackDurationText: string;
  isPlanningVisibleToCustomers: boolean;
  customerApprovalRequired: boolean;
  customerReviewRequired: boolean;
  parameters: BeStandardChangeParameter[];
  evidence: Array<Pick<BeStandardChangeEvidence, "id" | "number" | "subject" | "closedOn">>;
}

function formFromProposal(p: BeStandardChangeProposal): ProposalForm {
  return {
    name: p.name ?? "",
    rationale: p.rationale ?? "",
    subject: p.subject ?? "",
    description: p.description ?? "",
    justification: p.justification ?? "",
    implementationPlan: p.implementationPlan ?? "",
    riskImpactAnalysis: p.riskImpactAnalysis ?? "",
    backoutPlan: p.backoutPlan ?? "",
    testPlan: p.testPlan ?? "",
    category: p.category ?? "",
    impact: p.impact ?? "",
    risk: p.risk ?? "",
    rollbackDurationText: p.rollbackDurationText ?? "",
    isPlanningVisibleToCustomers: p.isPlanningVisibleToCustomers,
    customerApprovalRequired: p.customerApprovalRequired,
    customerReviewRequired: p.customerReviewRequired,
    parameters: (p.parameters ?? []).map((x) => ({ ...x, helpText: x.helpText ?? "", options: x.options ?? undefined })),
    evidence: (p.evidence ?? []).map((e) => ({ id: e.id, number: e.number, subject: e.subject, closedOn: e.closedOn })),
  };
}

const richOut = (html: string): string => (isBlankHtml(html) ? "" : html);

/**
 * The PATCH for a draft: every field the form holds (absent = unchanged on the
 * server, so a field the form never set is left out). A retirement only carries
 * its name, rationale and evidence: the template it retires is what it is.
 */
function buildProposalPatch(kind: string, form: ProposalForm): BeUpdateStandardChangeProposalPayload {
  const patch: BeUpdateStandardChangeProposalPayload = {
    name: form.name.trim(),
    rationale: richOut(form.rationale),
    evidenceChangeRequestIds: form.evidence.map((e) => e.id),
  };
  if (kind === "retire") return patch;
  patch.subject = form.subject.trim();
  for (const { field } of RICH_FIELDS) patch[field] = richOut(form[field]);
  if (form.category) patch.category = form.category;
  if (form.impact) patch.impact = form.impact;
  if (form.risk) patch.risk = form.risk;
  patch.rollbackDurationText = form.rollbackDurationText.trim();
  patch.isPlanningVisibleToCustomers = form.isPlanningVisibleToCustomers;
  patch.customerApprovalRequired = form.customerApprovalRequired;
  patch.customerReviewRequired = form.customerReviewRequired;
  patch.parameters = normalizeStandardChangeParameters(form.parameters);
  return patch;
}

/** The form as template content, for the live preview and the placeholder checks. */
function formContent(form: ProposalForm): BeStandardChangeContent {
  return {
    subject: form.subject,
    description: form.description,
    justification: form.justification,
    implementationPlan: form.implementationPlan,
    riskImpactAnalysis: form.riskImpactAnalysis,
    backoutPlan: form.backoutPlan,
    testPlan: form.testPlan,
    category: form.category || null,
    impact: form.impact || null,
    risk: form.risk || null,
    rollbackDurationText: form.rollbackDurationText,
    isPlanningVisibleToCustomers: form.isPlanningVisibleToCustomers,
    customerApprovalRequired: form.customerApprovalRequired,
    customerReviewRequired: form.customerReviewRequired,
    parameters: normalizeStandardChangeParameters(form.parameters),
  };
}

/**
 * "Insert {{…}}": a menu of the declared parameters; picking one puts its
 * placeholder into the field it sits under (at the caret of a one-line field,
 * at the end of a rich-text one).
 */
function InsertPlaceholderButton({
  fieldLabel,
  parameters,
  onInsert,
  disabled,
}: {
  fieldLabel: string;
  parameters: readonly BeStandardChangeParameter[];
  onInsert: (key: string) => void;
  disabled?: boolean;
}): JSX.Element {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const usable = parameters.filter((p) => /^[a-z][a-z0-9_]{0,49}$/.test(p.key.trim()));
  return (
    <>
      <Button
        size="small"
        variant="text"
        startIcon={<Braces size={14} />}
        disabled={disabled || usable.length === 0}
        aria-label={`Insert a placeholder into ${fieldLabel}`}
        aria-haspopup="menu"
        onClick={(e) => setAnchor(e.currentTarget)}
        // Keep the one-line field's caret where it is.
        onMouseDown={(e) => e.preventDefault()}
      >
        Insert {"{{…}}"}
      </Button>
      <Menu anchorEl={anchor} open={!!anchor} onClose={() => setAnchor(null)}>
        {usable.map((p) => (
          <MenuItem
            key={p.key}
            onClick={() => {
              setAnchor(null);
              onInsert(p.key.trim());
            }}
          >
            <Box component="span" sx={{ fontFamily: "monospace", mr: 1 }}>{`{{${p.key.trim()}}}`}</Box>
            <Typography component="span" variant="body2" color="text.secondary">
              {p.label}
            </Typography>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}

function Section({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }): JSX.Element {
  return (
    <Card component="section" aria-label={title} sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
          {title}
        </Typography>
        {action}
      </Box>
      {children}
    </Card>
  );
}

function EvidenceTable({
  evidence,
  onOpen,
  onRemove,
  disabled,
}: {
  evidence: ProposalForm["evidence"];
  onOpen: (id: string) => void;
  onRemove?: (id: string) => void;
  disabled?: boolean;
}): JSX.Element {
  if (evidence.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No evidence cited.
      </Typography>
    );
  }
  return (
    <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
      <TableContainer>
        <Table size="small" aria-label="Evidence" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
          <TableHead>
            <TableRow sx={{ bgcolor: "action.hover" }}>
              <TableCell sx={{ width: 130 }}>Number</TableCell>
              <TableCell>Subject</TableCell>
              <TableCell sx={{ width: 170 }}>Closed</TableCell>
              {onRemove && <TableCell sx={{ width: 60 }} />}
            </TableRow>
          </TableHead>
          <TableBody>
            {evidence.map((e) => (
              <TableRow key={e.id}>
                <TableCell>
                  <Link component="button" variant="body2" onClick={() => onOpen(e.id)}>
                    {e.number || e.id}
                  </Link>
                </TableCell>
                <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>{e.subject || "—"}</TableCell>
                <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDateTime(e.closedOn)}</TableCell>
                {onRemove && (
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      aria-label={`Remove ${e.number || e.id} from the evidence`}
                      disabled={disabled}
                      onClick={() => onRemove(e.id)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}

/** The proposer's editor for a draft. */
function ProposalEditor({
  proposal,
  onOpenChangeRequest,
  onWithdraw,
}: {
  proposal: BeStandardChangeProposal;
  onOpenChangeRequest: (id: string) => void;
  onWithdraw: () => void;
}): JSX.Element {
  const [form, setForm] = useState<ProposalForm>(() => formFromProposal(proposal));
  // Bumped to remount a rich-text editor whose value was changed from outside
  // (a placeholder inserted): the editor reads its value only when it mounts.
  const [editorRev, setEditorRev] = useState<Record<string, number>>({});
  const [sampleValues, setSampleValues] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<{ severity: "success" | "error"; text: string } | null>(null);
  const subjectRef = useRef<HTMLInputElement>(null);
  const rollbackRef = useRef<HTMLInputElement>(null);
  const update = useUpdateStandardChangeProposal();
  const submit = useSubmitStandardChangeProposal();
  const isRetire = proposal.kind === "retire";
  const busy = update.isPending || submit.isPending;

  const set = <K extends keyof ProposalForm>(key: K, value: ProposalForm[K]): void =>
    setForm((f) => ({ ...f, [key]: value }));

  const issues = useMemo(() => validateStandardChangeParameters(form.parameters), [form.parameters]);
  const paramsInvalid = !isRetire && hasParameterIssues(issues);
  const content = useMemo(() => formContent(form), [form]);
  const undeclared = isRetire ? [] : undeclaredPlaceholders(content, form.parameters);
  const unused = isRetire ? [] : unusedParameters(content, form.parameters);
  const gaps = standardChangeProposalGaps(proposal.kind, form);
  const highRisk = !isRetire && form.risk === "high";
  const canSubmitNow = gaps.length === 0 && undeclared.length === 0 && !highRisk && !paramsInvalid;

  const insertRich = (field: RichField, key: string): void => {
    setForm((f) => ({ ...f, [field]: appendPlaceholderToHtml(f[field], key, isBlankHtml(f[field])) }));
    setEditorRev((r) => ({ ...r, [field]: (r[field] ?? 0) + 1 }));
  };
  const insertPlain = (field: "subject" | "rollbackDurationText", key: string): void => {
    const el = field === "subject" ? subjectRef.current : rollbackRef.current;
    setForm((f) => ({ ...f, [field]: insertPlaceholderAt(f[field], key, el?.selectionStart) }));
  };

  const save = (): void => {
    setNotice(null);
    update.mutate(
      { id: proposal.id, patch: buildProposalPatch(proposal.kind, form) },
      {
        onSuccess: () => setNotice({ severity: "success", text: "Saved." }),
        onError: (err) => setNotice({ severity: "error", text: errorText(err, "Could not save the proposal.") }),
      },
    );
  };

  // Submit saves first, so what the CAB sees is what is on screen.
  const saveAndSubmit = async (): Promise<void> => {
    setNotice(null);
    try {
      await update.mutateAsync({ id: proposal.id, patch: buildProposalPatch(proposal.kind, form) });
    } catch (err) {
      setNotice({ severity: "error", text: errorText(err, "Could not save the proposal.") });
      return;
    }
    try {
      await submit.mutateAsync(proposal.id);
    } catch (err) {
      setNotice({ severity: "error", text: `Saved, but not sent to the CAB: ${errorText(err, "the submission was refused.")}` });
    }
  };

  const richField = (field: RichField, label: string, helper: string): JSX.Element => (
    <Box key={field} sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      <ExpandableRichTextField
        key={`${field}-${editorRev[field] ?? 0}`}
        id={`std-proposal-${field}`}
        label={label}
        value={form[field]}
        onChange={(html) => set(field, html)}
        disabled={busy}
        helperText={helper}
      />
      <Box>
        <InsertPlaceholderButton
          fieldLabel={label}
          parameters={form.parameters}
          onInsert={(key) => insertRich(field, key)}
          disabled={busy}
        />
      </Box>
    </Box>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {notice && (
        <Alert severity={notice.severity} onClose={() => setNotice(null)} role={notice.severity === "error" ? "alert" : "status"}>
          {notice.text}
        </Alert>
      )}

      <Section title="Proposal">
        <TextField
          label="Name"
          required
          size="small"
          value={form.name}
          disabled={busy}
          onChange={(e) => set("name", e.target.value.slice(0, NAME_MAX))}
          helperText={isRetire ? "The template being retired." : "The template's name in the catalogue."}
        />
        <ExpandableRichTextField
          id="std-proposal-rationale"
          label="Rationale"
          value={form.rationale}
          onChange={(html) => set("rationale", html)}
          disabled={busy}
          helperText={
            isRetire
              ? "Required. Why the template should be retired."
              : "Required. Why this change is low risk and repeatable enough to be pre-authorised."
          }
        />
      </Section>

      {!isRetire && (
        <Section title="Parameters (change specifics)">
          <Typography variant="body2" color="text.secondary">
            Generalise the text below and mark each value that differs from one change to the next as a {"{{key}}"}
            placeholder. Every placeholder needs a parameter here; it is what is asked when a change request is
            raised from the template.
          </Typography>
          <StandardChangeParametersEditor
            parameters={form.parameters}
            onChange={(next) => set("parameters", next)}
            issues={issues}
            disabled={busy}
          />
          {undeclared.length > 0 && (
            <Alert severity="warning" role="status">
              The text refers to placeholders no parameter declares: {undeclared.map((k) => `{{${k}}}`).join(", ")}. Add
              them as parameters or remove them from the text.
            </Alert>
          )}
          {unused.length > 0 && (
            <Alert severity="info" role="status">
              Declared but not used in the text: {unused.map((k) => `{{${k}}}`).join(", ")}.
            </Alert>
          )}
        </Section>
      )}

      {!isRetire && (
        <Section title="What the template specifies">
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <TextField
              label="Subject"
              required
              size="small"
              fullWidth
              value={form.subject}
              disabled={busy}
              inputRef={subjectRef}
              onChange={(e) => set("subject", e.target.value.slice(0, NAME_MAX))}
              helperText="The change request's subject (plain text). Placeholders are filled in."
            />
            <Box>
              <InsertPlaceholderButton
                fieldLabel="Subject"
                parameters={form.parameters}
                onInsert={(key) => insertPlain("subject", key)}
                disabled={busy}
              />
            </Box>
          </Box>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" },
              alignItems: "start",
            }}
          >
            <FormControl size="small" fullWidth disabled={busy}>
              <InputLabel id="std-proposal-category-label" shrink>
                Category
              </InputLabel>
              <Select
                labelId="std-proposal-category-label"
                label="Category"
                value={form.category}
                displayEmpty
                onChange={(e) => set("category", e.target.value as BeChangeRequestCategory | "")}
              >
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
                {CHANGE_REQUEST_CATEGORY_OPTIONS.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl size="small" fullWidth disabled={busy} required>
              <InputLabel id="std-proposal-impact-label" shrink>
                Impact
              </InputLabel>
              <Select
                labelId="std-proposal-impact-label"
                label="Impact"
                value={form.impact}
                displayEmpty
                onChange={(e) => set("impact", e.target.value as BeChangeRequestImpact | "")}
              >
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
                {CHANGE_REQUEST_IMPACT_OPTIONS.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl size="small" fullWidth disabled={busy} required error={highRisk}>
              <InputLabel id="std-proposal-risk-label" shrink>
                Risk
              </InputLabel>
              <Select
                labelId="std-proposal-risk-label"
                label="Risk"
                value={form.risk}
                displayEmpty
                onChange={(e) => set("risk", e.target.value as BeChangeRequestRisk | "")}
              >
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
                {STANDARD_CHANGE_RISK_OPTIONS.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
                {/* Only to show a value copied from a high-risk change; it cannot be chosen. */}
                {highRisk && (
                  <MenuItem value="high" disabled>
                    High
                  </MenuItem>
                )}
              </Select>
              <FormHelperText>{highRisk ? STANDARD_CHANGE_HIGH_RISK_REASON : "High is not allowed for a standard change."}</FormHelperText>
            </FormControl>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <TextField
              label="Rollback duration"
              size="small"
              fullWidth
              value={form.rollbackDurationText}
              disabled={busy}
              inputRef={rollbackRef}
              onChange={(e) => set("rollbackDurationText", e.target.value.slice(0, CHANGE_REQUEST_ROLLBACK_DURATION_MAX))}
              placeholder="e.g. 30 mins"
            />
            <Box>
              <InsertPlaceholderButton
                fieldLabel="Rollback duration"
                parameters={form.parameters}
                onInsert={(key) => insertPlain("rollbackDurationText", key)}
                disabled={busy}
              />
            </Box>
          </Box>
          <Box sx={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
            <FormControlLabel
              disabled={busy}
              control={
                <Checkbox
                  size="small"
                  checked={form.isPlanningVisibleToCustomers}
                  onChange={(e) => set("isPlanningVisibleToCustomers", e.target.checked)}
                />
              }
              label="Implementation plan visible to customers"
            />
            <FormControlLabel
              disabled={busy}
              control={
                <Checkbox
                  size="small"
                  checked={form.customerApprovalRequired}
                  onChange={(e) => set("customerApprovalRequired", e.target.checked)}
                />
              }
              label="Customer approval"
            />
            <FormControlLabel
              disabled={busy}
              control={
                <Checkbox
                  size="small"
                  checked={form.customerReviewRequired}
                  onChange={(e) => set("customerReviewRequired", e.target.checked)}
                />
              }
              label="Customer review"
            />
          </Box>
          {RICH_FIELDS.map((f) => richField(f.field, f.label, f.helper))}
        </Section>
      )}

      {!isRetire && (
        <Section title="Preview">
          <Typography variant="body2" color="text.secondary">
            How a change request raised from this template reads. Try sample values: they are not saved.
          </Typography>
          <StandardChangeParameterInputs
            parameters={content.parameters ?? []}
            values={sampleValues}
            onChange={(key, value) => setSampleValues((v) => ({ ...v, [key]: value }))}
            idPrefix="std-sample"
            disabled={busy}
          />
          <Box role="region" aria-label="Preview of a change request raised from this template">
            <StandardChangeContentView content={content} fill={{ parameters: content.parameters ?? [], values: sampleValues }} />
          </Box>
        </Section>
      )}

      <Section title="Evidence">
        <Typography variant="body2" color="text.secondary">
          Closed change requests that show this change has been done safely before. The CAB judges them.
        </Typography>
        <EvidenceTable
          evidence={form.evidence}
          onOpen={onOpenChangeRequest}
          onRemove={(id) => set("evidence", form.evidence.filter((e) => e.id !== id))}
          disabled={busy}
        />
        <Box sx={{ maxWidth: 520 }}>
          <AsyncEntitySelect<BeChangeRequestSearchView>
            id="std-proposal-evidence"
            label="Add a closed change request"
            placeholder="Search closed change requests…"
            value=""
            onChange={(next, item) => {
              if (!next || form.evidence.some((e) => e.id === next)) return;
              set("evidence", [
                ...form.evidence,
                { id: next, number: item?.number ?? next, subject: item?.subject ?? "", closedOn: null },
              ]);
            }}
            disabled={busy}
            useSearch={useSearchClosedChangeRequestsForSelect}
            getId={(cr) => cr.id}
            getLabel={(cr) => [cr.number, cr.subject].filter(Boolean).join(" — ") || cr.id}
            excludeIds={form.evidence.map((e) => e.id)}
          />
        </Box>
      </Section>

      <Card sx={{ p: 2, display: "flex", gap: 1.5, alignItems: "center", flexWrap: "wrap", justifyContent: "space-between" }}>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          {paramsInvalid ? (
            <Typography variant="body2" color="error">
              Fix the parameters above before saving.
            </Typography>
          ) : canSubmitNow ? (
            <Typography variant="body2" color="text.secondary">
              Ready to send to the CAB. Submitting saves first.
            </Typography>
          ) : (
            <Typography variant="body2" color="text.secondary">
              {[
                gaps.length ? `Before it can be sent to the CAB it still needs ${gaps.join(", ")}.` : "",
                undeclared.length ? "Every placeholder needs a parameter." : "",
                highRisk ? "The risk cannot be high." : "",
              ]
                .filter(Boolean)
                .join(" ")}
            </Typography>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          {proposal.canWithdraw && (
            <Button variant="outlined" color="error" size="small" startIcon={<Ban size={16} />} disabled={busy} onClick={onWithdraw}>
              Withdraw
            </Button>
          )}
          <Button
            variant="outlined"
            size="small"
            startIcon={<Save size={16} />}
            disabled={busy || paramsInvalid}
            loading={update.isPending && !submit.isPending}
            onClick={save}
          >
            Save
          </Button>
          {proposal.canSubmit && (
            <Button
              variant="contained"
              size="small"
              startIcon={<Send size={16} />}
              disabled={busy || !canSubmitNow}
              loading={submit.isPending}
              onClick={() => void saveAndSubmit()}
            >
              Submit to CAB
            </Button>
          )}
        </Box>
      </Card>
    </Box>
  );
}

/** A CAB member's Approve / Reject; a rejection needs a comment. */
function DecisionDialog({
  decision,
  pending,
  error,
  onClose,
  onDecide,
}: {
  decision: "approved" | "rejected" | null;
  pending: boolean;
  error: string | null;
  onClose: () => void;
  onDecide: (comment: string) => void;
}): JSX.Element {
  const [comment, setComment] = useState("");
  const reject = decision === "rejected";
  const commentMissing = reject && !comment.trim();
  return (
    <Dialog
      open={decision !== null}
      onClose={pending ? undefined : onClose}
      aria-labelledby="std-decision-title"
      fullWidth
      maxWidth="sm"
      slotProps={{ transition: { onExited: () => setComment("") } }}
    >
      <DialogTitle id="std-decision-title">{reject ? "Reject the proposal" : "Approve the proposal"}</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <DialogContentText>
          {reject
            ? "The catalogue is left as it is and the proposal is kept as the record, with your comment."
            : "The first decision settles the proposal: approving it writes the template change for the CAB."}
        </DialogContentText>
        {error && <Alert severity="error">{error}</Alert>}
        <TextField
          label={reject ? "Comment (required)" : "Comment (optional)"}
          multiline
          minRows={3}
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          required={reject}
          disabled={pending}
          helperText={reject ? "Say why, so the proposer can act on it." : undefined}
        />
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={pending}>
          Go back
        </Button>
        <Button
          variant="contained"
          color={reject ? "error" : "success"}
          disabled={pending || commentMissing}
          loading={pending}
          onClick={() => onDecide(comment.trim())}
        >
          {reject ? "Reject" : "Approve"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * One standard change proposal (`/operations/standard-change-proposals/:id`): a
 * request to the CAB to add a template to the catalogue, revise one or retire one.
 * Its proposer edits a draft here (`canEdit`): the content, generalised with
 * `{{key}}` placeholders, the parameters they need, a live preview and the
 * evidence, then sends it to the CAB. Everybody else (and the proposer once it is
 * submitted) reads it; a CAB member decides on their own row (`canDecide`). A
 * rejected or withdrawn new-template proposal stays as the record and can be
 * copied into a new one.
 */
export default function StandardChangeProposalPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const location = useLocation();
  const { canWrite } = usePortalAccess();
  const backTarget = (location.state as { from?: string } | undefined)?.from ?? `${STANDARD_CHANGES_TAB_PATH}?view=proposals`;
  const here = `${location.pathname}${location.search}`;
  const { data: proposal, isLoading, isError, error, refetch } = useGetStandardChangeProposal(id);
  const withdraw = useWithdrawStandardChangeProposal();
  const decide = useDecideStandardChangeProposal();
  const copy = useCreateStandardChangeProposal();
  const [confirmWithdraw, setConfirmWithdraw] = useState(false);
  const [decision, setDecision] = useState<"approved" | "rejected" | null>(null);
  const [pageError, setPageError] = useState<string | null>(null);

  const openChangeRequest = (crId: string): void =>
    navigate(`/operations/change-requests/${encodeURIComponent(crId)}`, { state: { from: here } });

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate(backTarget)}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Skeleton variant="rounded" height={120} />
        <Skeleton variant="rounded" height={320} />
      </Box>
    );
  }
  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <QueryErrorState message={errorText(error, "Failed to load the proposal.")} error={error} onRetry={() => void refetch()} />
      </Box>
    );
  }
  if (!proposal) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Proposal not found</Typography>
      </Box>
    );
  }

  const editing = proposal.canEdit && proposal.state === "draft";
  const approvers = proposal.approvers ?? [];
  const myRow = approvers.find((a) => a.canDecide);
  const canCopy = canWrite && proposal.kind === "new" && (proposal.state === "rejected" || proposal.state === "withdrawn");

  const runWithdraw = (): void => {
    setConfirmWithdraw(false);
    setPageError(null);
    withdraw.mutate(proposal.id, { onError: (err) => setPageError(errorText(err, "Could not withdraw the proposal.")) });
  };

  const runCopy = (): void => {
    setPageError(null);
    copy.mutate(
      { kind: "new", copyFromProposalId: proposal.id },
      {
        onSuccess: (p) => navigate(standardChangeProposalPath(p.id), { state: { from: here } }),
        onError: (err) => setPageError(errorText(err, "Could not copy the proposal.")),
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start", justifyContent: "space-between", flexWrap: "wrap" }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0, flex: 1 }}>
          <Typography variant="h6" sx={{ fontFamily: "monospace", fontWeight: 700 }}>
            {proposal.number}
          </Typography>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
            <Chip
              size="small"
              color={standardChangeProposalStateColor(proposal.state)}
              label={standardChangeProposalStateLabel(proposal.state)}
            />
            <Chip size="small" variant="outlined" label={standardChangeProposalKindLabel(proposal.kind)} />
          </Box>
          <Typography variant="h5">{proposal.name || "Untitled proposal"}</Typography>
          <Typography variant="body2" color="text.secondary">
            {`Proposed by ${personLabel(proposal.proposedBy)} on ${formatDateTime(proposal.createdOn)}`}
            {proposal.submittedOn ? `; sent to the CAB ${formatDateTime(proposal.submittedOn)}` : ""}
          </Typography>
          {(proposal.template || proposal.sourceChangeRequest) && (
            <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
              {proposal.template && (
                <Typography variant="body2">
                  Template:{" "}
                  <Link component="button" variant="body2" onClick={() => navigate(standardChangeTemplatePath(proposal.template!.id), { state: { from: here } })}>
                    {proposal.template.name}
                  </Link>
                  {proposal.baseVersion != null && ` (from version ${proposal.baseVersion})`}
                </Typography>
              )}
              {proposal.sourceChangeRequest && (
                <Typography variant="body2">
                  Started from:{" "}
                  <Link component="button" variant="body2" onClick={() => openChangeRequest(proposal.sourceChangeRequest!.id)}>
                    {proposal.sourceChangeRequest.name}
                  </Link>
                </Typography>
              )}
            </Box>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          {!editing && proposal.canWithdraw && (
            <Button
              variant="outlined"
              color="error"
              size="small"
              startIcon={<Ban size={16} />}
              disabled={withdraw.isPending}
              onClick={() => setConfirmWithdraw(true)}
            >
              Withdraw
            </Button>
          )}
          {canCopy && (
            <Button variant="outlined" size="small" startIcon={<CopyPlus size={16} />} loading={copy.isPending} onClick={runCopy}>
              Copy into a new proposal
            </Button>
          )}
        </Box>
      </Box>

      {pageError && (
        <Alert severity="error" onClose={() => setPageError(null)}>
          {pageError}
        </Alert>
      )}

      {isStandardChangeProposalFinal(proposal.state) && (
        <Alert
          severity={proposal.state === "approved" ? "success" : proposal.state === "rejected" ? "error" : "info"}
          role="region"
          aria-label="Decision"
        >
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {proposal.state === "approved"
              ? "Approved by the CAB"
              : proposal.state === "rejected"
                ? "Rejected by the CAB"
                : "Withdrawn by the proposer"}
            {proposal.decidedOn ? ` on ${formatDateTime(proposal.decidedOn)}` : ""}
            {proposal.decidedBy ? ` (${personLabel(proposal.decidedBy)})` : ""}
          </Typography>
          {proposal.decisionComment && <Typography variant="body2">{proposal.decisionComment}</Typography>}
          {proposal.state === "approved" && proposal.template && (
            <Typography variant="body2">
              {proposal.kind === "retire" ? "Retired " : "Resulting template: "}
              <Link component="button" variant="body2" onClick={() => navigate(standardChangeTemplatePath(proposal.template!.id), { state: { from: here } })}>
                {proposal.template.name}
              </Link>
              {proposal.resultingVersion != null && `, version ${proposal.resultingVersion}`}
            </Typography>
          )}
          {proposal.state === "rejected" && (
            <Typography variant="caption" sx={{ display: "block", mt: 0.5 }}>
              The catalogue was not changed. This proposal is kept as the record.
            </Typography>
          )}
        </Alert>
      )}

      {editing ? (
        <ProposalEditor
          key={proposal.id}
          proposal={proposal}
          onOpenChangeRequest={openChangeRequest}
          onWithdraw={() => setConfirmWithdraw(true)}
        />
      ) : (
        <>
          <Section title="Rationale">
            {proposal.rationale && !isBlankHtml(proposal.rationale) ? (
              <Box sx={PLAN_RICH_TEXT_SX} dangerouslySetInnerHTML={{ __html: sanitizeRichTextHtml(proposal.rationale) }} />
            ) : (
              <Typography variant="body2" color="text.secondary">
                —
              </Typography>
            )}
          </Section>
          {proposal.kind !== "retire" && (
            <Section title="What the template specifies">
              <StandardChangeContentView content={proposal} />
              <Typography variant="subtitle2">Parameters</Typography>
              <StandardChangeParametersTable parameters={proposal.parameters ?? []} />
            </Section>
          )}
          <Section title="Evidence">
            <EvidenceTable evidence={proposal.evidence ?? []} onOpen={openChangeRequest} />
          </Section>
        </>
      )}

      {approvers.length > 0 && (
        <Section
          title="CAB approval"
          action={
            myRow ? (
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button size="small" variant="contained" color="success" startIcon={<Check size={16} />} onClick={() => setDecision("approved")}>
                  Approve
                </Button>
                <Button size="small" variant="outlined" color="error" startIcon={<X size={16} />} onClick={() => setDecision("rejected")}>
                  Reject
                </Button>
              </Box>
            ) : undefined
          }
        >
          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
            <TableContainer>
              <Table size="small" aria-label="CAB approvers" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
                <TableHead>
                  <TableRow sx={{ bgcolor: "action.hover" }}>
                    <TableCell>Approver</TableCell>
                    <TableCell sx={{ width: 130 }}>State</TableCell>
                    <TableCell>Comment</TableCell>
                    <TableCell sx={{ width: 170 }}>Updated</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {approvers.map((a) => (
                    <TableRow key={a.id} selected={a.canDecide}>
                      <TableCell sx={{ maxWidth: 260, wordBreak: "break-word" }}>
                        {personLabel(a.approver)}
                        {a.canDecide && (
                          <Typography component="span" variant="caption" color="text.secondary">
                            {" "}
                            (you)
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Chip size="small" variant="outlined" color={approvalStatusColor(a.state)} label={approvalStatusLabel(a.state)} />
                      </TableCell>
                      <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>{a.comments || "—"}</TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDateTime(a.updatedOn)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        </Section>
      )}

      <DecisionDialog
        decision={decision}
        pending={decide.isPending}
        error={decide.isError ? errorText(decide.error, "Could not record the decision.") : null}
        onClose={() => {
          setDecision(null);
          decide.reset();
        }}
        onDecide={(comment) =>
          decide.mutate(
            { id: proposal.id, decision: decision!, ...(comment ? { comment } : {}) },
            { onSuccess: () => setDecision(null) },
          )
        }
      />

      <Dialog open={confirmWithdraw} onClose={() => setConfirmWithdraw(false)} aria-labelledby="std-withdraw-title">
        <DialogTitle id="std-withdraw-title">Withdraw {proposal.number}?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            The proposal is kept as the record but can no longer be edited or sent to the CAB; any approvals still
            waiting are cancelled.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setConfirmWithdraw(false)}>
            Go back
          </Button>
          <Button variant="contained" color="error" onClick={runWithdraw}>
            Withdraw
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
