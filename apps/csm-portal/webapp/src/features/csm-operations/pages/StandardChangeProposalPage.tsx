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
import { ArrowLeft, Ban, Braces, Check, CopyPlus, Save, Send, Trash2, X, XCircle } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX, type ReactNode } from "react";
import { useLocation } from "react-router";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import QueryErrorState from "@components/QueryErrorState";
import StandardChangeBreadcrumbs from "@features/csm-operations/components/StandardChangeBreadcrumbs";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  useCancelStandardChangeProposal,
  useCreateStandardChangeProposal,
  useDecideStandardChangeProposal,
  useDeleteStandardChangeProposal,
  useGetStandardChangeProposal,
  useSearchClosedChangeRequestsForSelect,
  useSearchStandardChangeCustomersForSelect,
  useSubmitStandardChangeProposal,
  useUpdateStandardChangeProposal,
  useWithdrawStandardChangeProposal,
} from "@features/csm-operations/api/useStandardChanges";
import ExpandableRichTextField from "@features/csm-operations/components/ExpandableRichTextField";
import StandardChangeContentView, {
  Field,
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
  CHANGE_REQUEST_PRIORITY_OPTIONS,
  CHANGE_REQUEST_ROLLBACK_DURATION_MAX,
  errorText,
} from "@features/csm-operations/utils/changeRequests";
import {
  appendPlaceholderToHtml,
  customerAccountPath,
  hasParameterIssues,
  insertPlaceholderAt,
  isStandardChangeProposalFinal,
  isStandardChangeProposalInReview,
  MAX_STANDARD_CHANGE_NAME,
  MAX_STANDARD_CHANGE_SUMMARY,
  normalizeStandardChangeParameters,
  retirementChangeRequestsInNewText,
  STANDARD_CHANGE_APPROVAL_RULE,
  STANDARD_CHANGE_CAB_APPROVALS,
  STANDARD_CHANGE_CAB_STAGE,
  STANDARD_CHANGE_LEGACY_AUTHORITY_STAGE,
  STANDARD_CHANGE_HIGH_RISK_REASON,
  STANDARD_CHANGE_RISK_OPTIONS,
  STANDARD_CHANGE_PROPOSALS_PATH,
  standardChangeProposalCustomerEditable,
  standardChangeProposalCustomerFixedReason,
  standardChangeProposalGaps,
  standardChangeProposalKindLabel,
  standardChangeProposalPath,
  standardChangeProposalStateColor,
  standardChangeProposalStateLabel,
  standardChangeRecentEntry,
  standardChangeTemplatePath,
  undeclaredPlaceholders,
  unusedParameters,
  validateStandardChangeParameters,
} from "@features/csm-operations/utils/standardChanges";
import {
  DEPLOYMENT_TYPES,
  deploymentTypeLabel,
  deploymentTypesText,
} from "@features/csm-projects/utils/deployments";
import type {
  BeChangeRequestCategory,
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestRisk,
  BeChangeRequestSearchView,
  BeDeploymentType,
  BeEntityRef,
  BeStandardChangeContent,
  BeStandardChangeEvidence,
  BeStandardChangeParameter,
  BeStandardChangeProposal,
  BeStandardChangeProposalApprover,
  BeStandardChangeProposalStage,
  BeUpdateStandardChangeProposalPayload,
} from "@api/backend/types";

function formatDateTime(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

function personLabel(p?: { name?: string | null; email?: string | null } | null): string {
  return p?.name || p?.email || "—";
}

/** The proposal's rich-text fields, in the order the form shows them. */
const RICH_FIELDS = [
  {
    field: "description",
    label: "Description",
    helper: "Required. The description of every change request raised from this template; placeholders are filled in.",
  },
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
  /** The customer (account) the proposal is for; "" while a new draft names none. */
  accountId: string;
  /** Its name, for the picker before a search has run. */
  customerName: string;
  rationale: string;
  summary: string;
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
  /** "" leaves the priority to whoever raises the change. */
  priority: BeChangeRequestPriority | "";
  /** Empty: any deployment. */
  deploymentTypes: BeDeploymentType[];
  isPlanningVisibleToCustomers: boolean;
  customerApprovalRequired: boolean;
  customerReviewRequired: boolean;
  parameters: BeStandardChangeParameter[];
  evidence: Array<Pick<BeStandardChangeEvidence, "id" | "number" | "subject" | "closedOn">>;
}

function formFromProposal(p: BeStandardChangeProposal): ProposalForm {
  return {
    name: p.name ?? "",
    accountId: p.customer?.id ?? "",
    customerName: p.customer?.name ?? "",
    rationale: p.rationale ?? "",
    summary: p.summary ?? "",
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
    priority: p.priority ?? "",
    deploymentTypes: p.deploymentTypes ?? [],
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
 * its name, rationale and evidence: the template it retires is what it is. The
 * customer is sent only where it can be named (`customerEditable`: a new
 * proposal that did not start from a change request) and once one is chosen;
 * elsewhere it is fixed and left out.
 */
function buildProposalPatch(kind: string, form: ProposalForm, customerEditable: boolean): BeUpdateStandardChangeProposalPayload {
  const patch: BeUpdateStandardChangeProposalPayload = {
    name: form.name.trim(),
    rationale: richOut(form.rationale),
    evidenceChangeRequestIds: form.evidence.map((e) => e.id),
  };
  if (customerEditable && form.accountId) patch.accountId = form.accountId;
  if (kind === "retire") return patch;
  patch.summary = form.summary.trim();
  patch.subject = form.subject.trim();
  for (const { field } of RICH_FIELDS) patch[field] = richOut(form[field]);
  // Always sent: an empty value clears the stored one, so "-- Select --" saves as shown.
  patch.category = form.category;
  patch.impact = form.impact;
  patch.risk = form.risk;
  patch.rollbackDurationText = form.rollbackDurationText.trim();
  patch.priority = form.priority;
  patch.deploymentTypes = form.deploymentTypes;
  patch.isPlanningVisibleToCustomers = form.isPlanningVisibleToCustomers;
  patch.customerApprovalRequired = form.customerApprovalRequired;
  patch.customerReviewRequired = form.customerReviewRequired;
  patch.parameters = normalizeStandardChangeParameters(form.parameters);
  return patch;
}

/** The form as template content, for the live preview and the placeholder checks. */
function formContent(form: ProposalForm): BeStandardChangeContent {
  return {
    summary: form.summary,
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
    priority: form.priority || null,
    deploymentTypes: form.deploymentTypes.length > 0 ? form.deploymentTypes : null,
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
  customerId,
}: {
  evidence: Array<ProposalForm["evidence"][number] & Pick<Partial<BeStandardChangeEvidence>, "customer">>;
  onOpen: (id: string) => void;
  onRemove?: (id: string) => void;
  disabled?: boolean;
  /**
   * The proposal's customer: a change request whose project now belongs to
   * another customer (or to none) is marked, since the CAB cannot approve
   * the proposal while it cites one.
   */
  customerId?: string | null;
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
                <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>
                  {e.subject || "—"}
                  {customerId && e.customer !== undefined && e.customer?.id?.toLowerCase() !== customerId.toLowerCase() && (
                    <Typography variant="caption" color="warning.main" component="div">
                      {e.customer?.name
                        ? `Now a change request of another customer (${e.customer.name})`
                        : "Now a change request of no customer"}
                    </Typography>
                  )}
                </TableCell>
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
  onDelete,
}: {
  proposal: BeStandardChangeProposal;
  onOpenChangeRequest: (id: string) => void;
  onWithdraw: () => void;
  onDelete: () => void;
}): JSX.Element {
  const [form, setForm] = useState<ProposalForm>(() => formFromProposal(proposal));
  // Bumped to remount a rich-text editor whose value was changed from outside
  // (a placeholder inserted): the editor reads its value only when it mounts.
  const [editorRev, setEditorRev] = useState<Record<string, number>>({});
  const [sampleValues, setSampleValues] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<{ severity: "success" | "error" | "info"; text: string } | null>(null);
  const subjectRef = useRef<HTMLInputElement>(null);
  const rollbackRef = useRef<HTMLInputElement>(null);
  const update = useUpdateStandardChangeProposal();
  const submit = useSubmitStandardChangeProposal();
  const isRetire = proposal.kind === "retire";
  const busy = update.isPending || submit.isPending;
  const customerEditable = standardChangeProposalCustomerEditable(proposal);
  const customerFixedReason = standardChangeProposalCustomerFixedReason(proposal);

  // A standard change is one customer's, and its evidence must be that
  // customer's closed change requests: naming another customer drops the
  // evidence already cited, and says so.
  const changeCustomer = (next: string, item?: BeEntityRef): void => {
    if (next === form.accountId) return;
    const dropped = form.evidence.length;
    setForm((f) => ({ ...f, accountId: next, customerName: item?.name ?? "", evidence: [] }));
    setNotice(
      dropped > 0
        ? {
            severity: "info",
            text: `The evidence was removed (${dropped} change request${dropped === 1 ? "" : "s"}): it must be closed change requests of the proposal's customer. Add ${item?.name ?? "the new customer"}'s.`,
          }
        : null,
    );
  };

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
      { id: proposal.id, patch: buildProposalPatch(proposal.kind, form, customerEditable) },
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
      await update.mutateAsync({ id: proposal.id, patch: buildProposalPatch(proposal.kind, form, customerEditable) });
    } catch (err) {
      setNotice({ severity: "error", text: errorText(err, "Could not save the proposal.") });
      return;
    }
    try {
      await submit.mutateAsync(proposal.id);
    } catch (err) {
      setNotice({ severity: "error", text: `Saved, but not submitted: ${errorText(err, "the submission was refused.")}` });
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
        {/* A retirement is named after its template and renames nothing, so its name is not edited (the server refuses another). */}
        <TextField
          label="Name"
          required={!isRetire}
          size="small"
          value={form.name}
          disabled={busy || isRetire}
          onChange={(e) => set("name", e.target.value.slice(0, MAX_STANDARD_CHANGE_NAME))}
          helperText={isRetire ? "The template being retired." : "The template's name in the catalogue."}
        />
        {!isRetire && (
          <TextField
            label="Summary"
            required
            size="small"
            multiline
            minRows={2}
            value={form.summary}
            disabled={busy}
            onChange={(e) => set("summary", e.target.value.slice(0, MAX_STANDARD_CHANGE_SUMMARY))}
            helperText="What the template is for, when to use it and what it does not cover. Shown in the catalogue and when a template is picked; never copied onto a change request, so no placeholders."
          />
        )}
        {customerEditable ? (
          <AsyncEntitySelect<BeEntityRef>
            id="std-proposal-customer"
            label="Customer"
            placeholder="Search customers…"
            required
            value={form.accountId}
            knownLabel={form.customerName || undefined}
            onChange={changeCustomer}
            disabled={busy}
            hideClearButton
            useSearch={useSearchStandardChangeCustomersForSelect}
            getId={(a) => a.id}
            getLabel={(a) => a.name}
            helperText="Required. The customer the template is for: it is used only for their projects, and its evidence must be their closed change requests."
          />
        ) : (
          <TextField
            label="Customer"
            size="small"
            value={form.customerName || "—"}
            helperText={customerFixedReason ?? undefined}
            slotProps={{ input: { readOnly: true }, htmlInput: { "aria-readonly": true } }}
          />
        )}
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
              onChange={(e) => set("subject", e.target.value.slice(0, MAX_STANDARD_CHANGE_NAME))}
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
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1fr) minmax(0, 2fr)" },
              alignItems: "start",
            }}
          >
            <FormControl size="small" fullWidth disabled={busy}>
              <InputLabel id="std-proposal-priority-label" shrink>
                Priority
              </InputLabel>
              <Select
                labelId="std-proposal-priority-label"
                label="Priority"
                value={form.priority}
                displayEmpty
                onChange={(e) => set("priority", e.target.value as BeChangeRequestPriority | "")}
              >
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    Chosen per change
                  </Typography>
                </MenuItem>
                {CHANGE_REQUEST_PRIORITY_OPTIONS.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
              <FormHelperText>
                {form.priority
                  ? "Every change raised from the template has this priority."
                  : "Left to whoever raises the change."}
              </FormHelperText>
            </FormControl>
            <FormControl size="small" fullWidth disabled={busy}>
              <InputLabel id="std-proposal-deployment-types-label" shrink>
                Deployment types
              </InputLabel>
              <Select<BeDeploymentType[]>
                labelId="std-proposal-deployment-types-label"
                label="Deployment types"
                multiple
                displayEmpty
                value={form.deploymentTypes}
                onChange={(e) => {
                  const v = e.target.value;
                  set("deploymentTypes", (typeof v === "string" ? v.split(",") : v) as BeDeploymentType[]);
                }}
                renderValue={(selected) =>
                  selected.length === 0 ? (
                    <Typography component="span" color="text.secondary">
                      Any deployment
                    </Typography>
                  ) : (
                    selected.map((t) => deploymentTypeLabel(t)).join(", ")
                  )
                }
              >
                {DEPLOYMENT_TYPES.map((t) => (
                  <MenuItem key={t} value={t}>
                    <Checkbox size="small" checked={form.deploymentTypes.includes(t)} sx={{ p: 0, mr: 1 }} />
                    {deploymentTypeLabel(t)}
                  </MenuItem>
                ))}
              </Select>
              <FormHelperText>
                {form.deploymentTypes.length > 0
                  ? `A change raised from the template needs at least one deployment, all ${deploymentTypesText(form.deploymentTypes)}.`
                  : "The deployments are chosen when the change is raised, of any type, or none."}
              </FormHelperText>
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
          Closed change requests of the proposal&apos;s customer that show this change has been done safely before. The
          CAB judges them.
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
            placeholder={form.customerName ? `Search ${form.customerName}'s closed change requests…` : "Search closed change requests…"}
            value=""
            onChange={(next, item) => {
              if (!next || form.evidence.some((e) => e.id === next)) return;
              set("evidence", [
                ...form.evidence,
                { id: next, number: item?.number ?? next, subject: item?.subject ?? "", closedOn: null },
              ]);
            }}
            disabled={busy || !form.accountId}
            helperText={form.accountId ? undefined : "Name the customer first: the evidence must be that customer's closed change requests."}
            useSearch={useSearchClosedChangeRequestsForSelect}
            searchExtra={form.accountId || undefined}
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
              Ready to submit: two CAB members other than you must authorise it.
              Submitting saves first.
            </Typography>
          ) : (
            <Typography variant="body2" color="text.secondary">
              {[
                gaps.length ? `Before it can be submitted it still needs ${gaps.join(", ")}.` : "",
                undeclared.length ? "Every placeholder needs a parameter." : "",
                highRisk ? "The risk cannot be high." : "",
              ]
                .filter(Boolean)
                .join(" ")}
            </Typography>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          {/* A draft never submitted is deleted, not withdrawn: it has been before nobody. */}
          {proposal.canDelete ? (
            <Button variant="outlined" color="error" size="small" startIcon={<Trash2 size={16} />} disabled={busy} onClick={onDelete}>
              Delete draft
            </Button>
          ) : (
            proposal.canWithdraw && (
              <Button variant="outlined" color="error" size="small" startIcon={<Ban size={16} />} disabled={busy} onClick={onWithdraw}>
                Withdraw
              </Button>
            )
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
              Submit for approval
            </Button>
          )}
        </Box>
      </Card>
    </Box>
  );
}

/**
 * A dialog that asks for a text before its action: kept open while the action is
 * pending, cleared once closed, and the action held back while a required text
 * is blank.
 */
function ReasonDialog({
  open,
  titleId,
  title,
  text,
  label,
  required,
  helperText,
  confirmLabel,
  confirmColor,
  pending,
  error,
  onClose,
  onConfirm,
}: {
  open: boolean;
  titleId: string;
  title: ReactNode;
  text: ReactNode;
  label: string;
  required: boolean;
  helperText?: string;
  confirmLabel: string;
  confirmColor: "error" | "success";
  pending: boolean;
  error: string | null;
  onClose: () => void;
  onConfirm: (text: string) => void;
}): JSX.Element {
  const [value, setValue] = useState("");
  return (
    <Dialog
      open={open}
      onClose={pending ? undefined : onClose}
      aria-labelledby={titleId}
      fullWidth
      maxWidth="sm"
      slotProps={{ transition: { onExited: () => setValue("") } }}
    >
      <DialogTitle id={titleId}>{title}</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <DialogContentText>{text}</DialogContentText>
        {error && <Alert severity="error">{error}</Alert>}
        <TextField
          label={label}
          multiline
          minRows={3}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          required={required}
          disabled={pending}
          helperText={helperText}
        />
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={pending}>
          Go back
        </Button>
        <Button
          variant="contained"
          color={confirmColor}
          disabled={pending || (required && !value.trim())}
          loading={pending}
          onClick={() => onConfirm(value.trim())}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** An approver's Approve / Reject on the stage the proposal waits on; a rejection needs a comment. */
function DecisionDialog({
  decision,
  stage,
  pending,
  error,
  onClose,
  onDecide,
}: {
  decision: "approved" | "rejected" | null;
  /** The stage the caller decides on. */
  stage: BeStandardChangeProposalStage | null;
  pending: boolean;
  error: string | null;
  onClose: () => void;
  onDecide: (comment: string) => void;
}): JSX.Element {
  const reject = decision === "rejected";
  // From the page as loaded: another member may have authorised since, so it
  // never promises that nothing will be applied.
  const required = stage?.requiredApprovals ?? STANDARD_CHANGE_CAB_APPROVALS;
  const remaining = Math.max(required - (stage?.approvals ?? 0), 1);
  const approveText =
    remaining > 1
      ? `${required} different CAB members must approve; the template change is applied by the last of them (yours, if another member has approved since you opened this page).`
      : `Yours is the last approval needed: authorising applies the template change.`;
  return (
    <ReasonDialog
      open={decision !== null}
      titleId="std-decision-title"
      title={reject ? "Reject the proposal" : "Authorise the proposal"}
      text={
        reject
          ? "A rejection ends the proposal: the catalogue is left as it is and the proposal is kept as the record, with your comment."
          : approveText
      }
      label={reject ? "Comment (required)" : "Comment (optional)"}
      required={reject}
      helperText={reject ? "Say why, so the proposer can act on it." : undefined}
      confirmLabel={reject ? "Reject" : "Authorise"}
      confirmColor={reject ? "error" : "success"}
      pending={pending}
      error={error}
      onClose={onClose}
      onConfirm={onDecide}
    />
  );
}

/** A CAB member ends someone else's open proposal; the reason is required. */
function CancelDialog({
  open,
  number,
  pending,
  error,
  onClose,
  onCancel,
}: {
  open: boolean;
  number: string;
  pending: boolean;
  error: string | null;
  onClose: () => void;
  onCancel: (reason: string) => void;
}): JSX.Element {
  return (
    <ReasonDialog
      open={open}
      titleId="std-cancel-title"
      title={`Cancel ${number}?`}
      text={
        <>
          For a proposal its proposer has abandoned or can no longer take forward. It is kept as the record with your
          reason; approvals still waiting are cancelled, and a revision or retirement no longer holds its template.
        </>
      }
      label="Reason (required)"
      required
      helperText="Shown to the proposer and on the proposal's record."
      confirmLabel="Cancel the proposal"
      confirmColor="error"
      pending={pending}
      error={error}
      onClose={onClose}
      onConfirm={onCancel}
    />
  );
}

/** A yes / no question before a destructive action, answered with "Go back" or the action. */
function ConfirmDialog({
  open,
  titleId,
  title,
  text,
  confirmLabel,
  pending,
  onClose,
  onConfirm,
}: {
  open: boolean;
  titleId: string;
  title: string;
  text: string;
  confirmLabel: string;
  pending?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}): JSX.Element {
  return (
    <Dialog open={open} onClose={onClose} aria-labelledby={titleId}>
      <DialogTitle id={titleId}>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{text}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>
          Go back
        </Button>
        <Button variant="contained" color="error" loading={pending} onClick={onConfirm}>
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * One approval stage, named by its own label: its progress and its approvers. A
 * proposal has one stage now, the CAB's ("CAB Approval"); one decided before may
 * show the Standard Change Authority's instead, and an earlier one-approval CAB
 * stage too, numbered in order.
 */
function StageApprovers({
  stage,
  index,
  latest,
  approvers,
  proposalState,
}: {
  stage: BeStandardChangeProposalStage | null;
  /** Its place among several stages; null when it is the only one. */
  index: number | null;
  /** Whether it is the proposal's latest stage, the only one that can still be open. */
  latest: boolean;
  approvers: BeStandardChangeProposalApprover[];
  proposalState: string;
}): JSX.Element {
  const label = stage?.label || STANDARD_CHANGE_CAB_STAGE;
  const status = !stage
    ? null
    : stage.decision === "APPROVED"
      ? { text: "Approved", color: "success" as const }
      : stage.decision === "REJECTED"
        ? { text: "Rejected", color: "error" as const }
        : !latest || !isStandardChangeProposalInReview(proposalState)
          ? // Ended before the stage was settled (or a later stage replaced it,
            // as the CAB stage of a proposal returned to draft): it waits for nobody now.
            {
              text:
                latest && proposalState === "cancelled"
                  ? "Cancelled"
                  : latest && proposalState === "withdrawn"
                    ? "Withdrawn"
                    : "Not decided",
              color: "default" as const,
            }
          : {
            text:
              stage.requiredApprovals > 1
                ? `${stage.approvals} of ${stage.requiredApprovals} approvals`
                : "Waiting for a decision",
            color: "info" as const,
          };
  return (
    <Box component="section" aria-label={`${label} approvers`} sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
        <Typography variant="subtitle2">{index === null ? label : `${index + 1}. ${label}`}</Typography>
        {status && <Chip size="small" variant="outlined" color={status.color} label={status.text} />}
      </Box>
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label={`${label} approvers`} sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
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
    </Box>
  );
}

/**
 * Who settled a proposal: the CAB (its two-approval "CAB Approval" stage), the
 * Standard Change Authority on one decided before 2026-11-10, or, on one decided
 * before that under the oldest flow, the stage named by its own label (its
 * one-approval "CAB Approval" stage, told apart by the approvals it needed).
 */
function settledBy(verb: "Approved" | "Rejected", stage: BeStandardChangeProposalStage | undefined): string {
  if (!stage || (stage.label === STANDARD_CHANGE_CAB_STAGE && stage.requiredApprovals >= STANDARD_CHANGE_CAB_APPROVALS)) {
    return `${verb} by the CAB`;
  }
  if (stage.label === STANDARD_CHANGE_LEGACY_AUTHORITY_STAGE) return `${verb} by the ${STANDARD_CHANGE_LEGACY_AUTHORITY_STAGE}`;
  return `${verb} at the ${stage.label} stage`;
}

/** How a final proposal ended, for the decision banner. */
function decisionHeadline(proposal: BeStandardChangeProposal): string {
  const stages = proposal.stages ?? [];
  switch (proposal.state) {
    case "approved":
      // The last stage approved is the one that applied it.
      return settledBy("Approved", stages.filter((s) => s.decision === "APPROVED").pop());
    case "rejected":
      return settledBy("Rejected", stages.find((s) => s.decision === "REJECTED"));
    case "cancelled":
      // By a member of the group its latest stage asked (the CAB since 2026-11-10).
      return stages[stages.length - 1]?.label === STANDARD_CHANGE_LEGACY_AUTHORITY_STAGE
        ? `Cancelled by the ${STANDARD_CHANGE_LEGACY_AUTHORITY_STAGE}`
        : "Cancelled by a CAB member";
    default:
      return "Withdrawn by the proposer";
  }
}

/**
 * One standard change proposal (`/operations/standard-change-proposals/:id`): a
 * request to the CAB to add a template to the catalogue,
 * revise one or retire one. Its proposer edits a draft here (`canEdit`): the
 * content, generalised with `{{key}}` placeholders, the parameters they need, a
 * live preview and the evidence, then submits it. Everybody else (and the
 * proposer once it is submitted) reads it; a CAB member decides on their
 * own row (`canDecide`), and the second approval applies it. A
 * rejected or withdrawn new-template proposal stays as the record and can be
 * copied into a new one.
 */
export default function StandardChangeProposalPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const location = useLocation();
  const { canWrite } = usePortalAccess();
  const backTarget = (location.state as { from?: string } | undefined)?.from ?? STANDARD_CHANGE_PROPOSALS_PATH;
  const here = `${location.pathname}${location.search}`;
  const { data: proposal, isLoading, isError, error, refetch } = useGetStandardChangeProposal(id);
  // Recent and "Pin this page" name the page after the proposal, as a change
  // request's or an incident's own page is named after it.
  const recordView = useRecordRecentView();
  useEffect(() => {
    if (proposal?.id) recordView(standardChangeRecentEntry("proposal", proposal));
  }, [proposal, recordView]);
  const withdraw = useWithdrawStandardChangeProposal();
  const decide = useDecideStandardChangeProposal();
  const copy = useCreateStandardChangeProposal();
  const cancel = useCancelStandardChangeProposal();
  const remove = useDeleteStandardChangeProposal();
  const [confirmWithdraw, setConfirmWithdraw] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [cancelling, setCancelling] = useState(false);
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
  const Trail = <StandardChangeBreadcrumbs show="proposals" current={proposal?.number} />;

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        {Trail}
        <Skeleton variant="rounded" height={120} />
        <Skeleton variant="rounded" height={320} />
      </Box>
    );
  }
  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        {Trail}
        <QueryErrorState message={errorText(error, "Failed to load the proposal.")} error={error} onRetry={() => void refetch()} />
      </Box>
    );
  }
  if (!proposal) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        {Trail}
        <Typography variant="h5">Proposal not found</Typography>
      </Box>
    );
  }

  const editing = proposal.canEdit && proposal.state === "draft";
  const approvers = proposal.approvers ?? [];
  const myRow = approvers.find((a) => a.canDecide);
  const stages = proposal.stages ?? [];
  const myStage = stages.find((st) => st.id === myRow?.stageId) ?? stages[stages.length - 1] ?? null;
  // One table per stage, oldest first (an older decided proposal may hold a
  // stage before its last). An answer without stages is one table.
  const stageGroups =
    stages.length > 0
      ? stages.map((st) => ({ stage: st, rows: approvers.filter((a) => a.stageId === st.id) }))
      : [{ stage: null, rows: approvers }];
  const canCopy =
    canWrite &&
    proposal.kind === "new" &&
    (proposal.state === "rejected" || proposal.state === "withdrawn" || proposal.state === "cancelled");

  const runWithdraw = (): void => {
    setConfirmWithdraw(false);
    setPageError(null);
    withdraw.mutate(proposal.id, { onError: (err) => setPageError(errorText(err, "Could not withdraw the proposal.")) });
  };

  const runDelete = (): void => {
    setConfirmDelete(false);
    setPageError(null);
    remove.mutate(proposal.id, {
      onSuccess: () => navigate(backTarget),
      onError: (err) => setPageError(errorText(err, "Could not delete the draft.")),
    });
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
      {Trail}

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
            {proposal.submittedOn ? `; submitted ${formatDateTime(proposal.submittedOn)}` : ""}
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Typography variant="body2">
              Customer:{" "}
              {proposal.customer ? (
                <Link
                  component="button"
                  variant="body2"
                  onClick={() => navigate(customerAccountPath(proposal.customer!.id), { state: { from: here } })}
                >
                  {proposal.customer.name}
                </Link>
              ) : (
                <Typography component="span" variant="body2" color="text.secondary">
                  not named yet
                </Typography>
              )}
            </Typography>
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
          {proposal.canCancel && (
            <Button variant="outlined" color="error" size="small" startIcon={<XCircle size={16} />} onClick={() => setCancelling(true)}>
              Cancel proposal
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

      {proposal.kind === "retire" && retirementChangeRequestsInNewText(proposal.changeRequestsInNewTotal) && (
        <Alert severity="warning" role="region" aria-label="Change requests still New">
          <Typography variant="body2">{retirementChangeRequestsInNewText(proposal.changeRequestsInNewTotal)}</Typography>
          <Typography variant="body2">
            {(proposal.changeRequestsInNew ?? []).map((c, i) => (
              <span key={c.id}>
                {i > 0 && ", "}
                <Link component="button" variant="body2" onClick={() => openChangeRequest(c.id)} sx={{ verticalAlign: "baseline" }}>
                  {c.name || "change request"}
                </Link>
              </span>
            ))}
            {(proposal.changeRequestsInNewTotal ?? 0) > (proposal.changeRequestsInNew ?? []).length &&
              ` and ${(proposal.changeRequestsInNewTotal ?? 0) - (proposal.changeRequestsInNew ?? []).length} more`}
          </Typography>
        </Alert>
      )}

      {isStandardChangeProposalFinal(proposal.state) && (
        <Alert
          severity={proposal.state === "approved" ? "success" : proposal.state === "rejected" ? "error" : "info"}
          role="region"
          aria-label="Decision"
        >
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {decisionHeadline(proposal)}
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
          onDelete={() => setConfirmDelete(true)}
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
              <Field label="Summary">
                <Box sx={{ whiteSpace: "pre-wrap" }}>{proposal.summary?.trim() || "—"}</Box>
              </Field>
              <StandardChangeContentView content={proposal} />
              <Typography variant="subtitle2">Parameters</Typography>
              <StandardChangeParametersTable parameters={proposal.parameters ?? []} />
            </Section>
          )}
          <Section title="Evidence">
            <EvidenceTable evidence={proposal.evidence ?? []} onOpen={openChangeRequest} customerId={proposal.customer?.id} />
          </Section>
        </>
      )}

      {approvers.length > 0 && (
        <Section
          title="Approval"
          action={
            myRow ? (
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button size="small" variant="contained" color="success" startIcon={<Check size={16} />} onClick={() => setDecision("approved")}>
                  Authorise
                </Button>
                <Button size="small" variant="outlined" color="error" startIcon={<X size={16} />} onClick={() => setDecision("rejected")}>
                  Reject
                </Button>
              </Box>
            ) : undefined
          }
        >
          {/* The rule holds for a proposal still to be decided, not one decided under an older one. */}
          {(proposal.state === "draft" || isStandardChangeProposalInReview(proposal.state)) && (
            <Typography variant="body2" color="text.secondary">
              {STANDARD_CHANGE_APPROVAL_RULE}
            </Typography>
          )}
          {stageGroups.map((g, i) => (
            <StageApprovers
              key={g.stage?.id ?? "all"}
              stage={g.stage}
              index={stageGroups.length > 1 ? i : null}
              latest={i === stageGroups.length - 1}
              approvers={g.rows}
              proposalState={proposal.state}
            />
          ))}
        </Section>
      )}

      <DecisionDialog
        decision={decision}
        stage={myStage}
        pending={decide.isPending}
        error={decide.isError ? errorText(decide.error, "Could not record the decision.") : null}
        onClose={() => {
          setDecision(null);
          decide.reset();
        }}
        onDecide={(comment) =>
          decide.mutate(
            {
              id: proposal.id,
              decision: decision!,
              ...(comment ? { comment } : {}),
              // The stage this page shows: the decision is refused, not recorded
              // elsewhere, if that stage was settled or closed meanwhile.
              ...(myRow?.stageId ? { stageId: myRow.stageId } : {}),
            },
            { onSuccess: () => setDecision(null) },
          )
        }
      />

      <CancelDialog
        open={cancelling}
        number={proposal.number}
        pending={cancel.isPending}
        error={cancel.isError ? errorText(cancel.error, "Could not cancel the proposal.") : null}
        onClose={() => {
          setCancelling(false);
          cancel.reset();
        }}
        onCancel={(reason) => cancel.mutate({ id: proposal.id, reason }, { onSuccess: () => setCancelling(false) })}
      />

      <ConfirmDialog
        open={confirmDelete}
        titleId="std-delete-title"
        title={`Delete ${proposal.number}?`}
        text="The draft has never been submitted, so it is deleted with everything in it. This cannot be undone."
        confirmLabel="Delete draft"
        pending={remove.isPending}
        onClose={() => setConfirmDelete(false)}
        onConfirm={runDelete}
      />

      <ConfirmDialog
        open={confirmWithdraw}
        titleId="std-withdraw-title"
        title={`Withdraw ${proposal.number}?`}
        text="The proposal is kept as the record but can no longer be edited or approved; any approvals still waiting are cancelled."
        confirmLabel="Withdraw"
        onClose={() => setConfirmWithdraw(false)}
        onConfirm={runWithdraw}
      />
    </Box>
  );
}
