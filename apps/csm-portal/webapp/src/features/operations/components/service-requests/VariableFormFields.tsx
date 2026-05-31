// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  Box,
  Button,
  FormControl,
  Grid,
  IconButton,
  MenuItem,
  Paper,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Upload, X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, type JSX } from "react";
import type { CatalogItemVariable } from "@features/operations/types/serviceRequests";
import {
  isAttachmentField,
  isFileCopyPathField,
  isDescriptionField,
  isDateTimeField,
} from "@features/operations/utils/serviceRequestValidation";
import { computeMinScheduleDatetimeLocalForTimeZone } from "@features/support/utils/support";
import { resolveDisplayTimeZone } from "@utils/dateTime";
import Editor from "@components/rich-text-editor/Editor";

export interface VariableFormFieldsProps {
  variables: CatalogItemVariable[] | undefined;
  isLoading: boolean;
  values: Record<string, string>;
  onChange: (variableId: string, value: string) => void;
  /** Selected service request type label (e.g. "Configuration Changes"). */
  selectedRequestTypeLabel?: string;
  /** Auto-populated context: Project, Deployment, Product (from form above). */
  contextValues?: {
    projectDisplay: string;
    deploymentDisplay: string;
    productDisplay: string;
  };
  attachments?: Array<{ id: string; file: File }>;
  onAttachmentClick?: () => void;
  onAttachmentRemove?: (index: number) => void;
  onAttachmentAdd?: (file: File, variableLabel?: string) => void;
  userTimeZone?: string;
}

const VARIABLE_TYPE_SINGLE_LINE = "Single Line Text";
const VARIABLE_TYPE_MULTI_LINE = "Multi Line Text";
const VARIABLE_TYPE_SELECT = "Select Box";
const VARIABLE_TYPE_CHECKBOX = "Checkbox";
const VARIABLE_TYPE_RADIO = "Radio Buttons";

/** Parse display label (strip leading asterisk). Hot fix: all typable fields are mandatory. */
function parseRequiredLabel(questionText: string): { label: string } {
  const raw = (questionText ?? "").trim();
  const label = raw.replace(/^\s*\*?\s*/, "").trim() || raw;
  return { label };
}

function isTitleField(questionText: string): boolean {
  return (
    questionText.replace(/^\s*\*?\s*/, "").trim().toLowerCase() === "title"
  );
}

/** Hot fix: all typable (user-editable) fields are mandatory due to API hasMandatory inconsistency. */
const TYPABLE_FIELDS_ALL_REQUIRED = true;

const CONTEXT_FIELD_PATTERNS: Array<{
  pattern: RegExp;
  getValue: (ctx: {
    projectDisplay: string;
    deploymentDisplay: string;
    productDisplay: string;
  }) => string;
}> = [
  { pattern: /^project$/i, getValue: (c) => c.projectDisplay },
  { pattern: /^deployments?$/i, getValue: (c) => c.deploymentDisplay },
  { pattern: /^product$/i, getValue: (c) => c.productDisplay },
  { pattern: /^wso2\s*product$/i, getValue: (c) => c.productDisplay },
  { pattern: /^environment$/i, getValue: (c) => c.deploymentDisplay },
];

/** Context fields to hide from UI (Project, Deployment, Product, Environment already selected above). */
const CONTEXT_FIELDS_HIDDEN_FROM_DISPLAY = [
  /^project$/i,
  /^deployments?$/i,
  /^product$/i,
  /^wso2\s*product$/i,
  /^environment$/i,
];

/** Fields hidden from customers but still sent in the payload (internal/system use). */
const HIDDEN_FIELD_PATTERNS: RegExp[] = [
  /^case\s*type$/i,
  /^service\s*request\s*category$/i,
  /^classification$/i,
  /^class\s*fication$/i,
  /^srns$/i,
  /^state$/i,
  /^assignment\s*group$/i,
  /^assigned\s*to$/i,
  /^priority$/i,
  /^impact$/i,
];

function isContextField(
  questionText: string,
  contextValues?: VariableFormFieldsProps["contextValues"],
): boolean {
  if (!contextValues) return false;
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return CONTEXT_FIELD_PATTERNS.some(({ pattern }) => pattern.test(normalized));
}

function getContextValue(
  questionText: string,
  contextValues: {
    projectDisplay: string;
    deploymentDisplay: string;
    productDisplay: string;
  },
): string {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  const match = CONTEXT_FIELD_PATTERNS.find(({ pattern }) =>
    pattern.test(normalized),
  );
  return match?.getValue(contextValues) ?? "";
}

function isHiddenField(questionText: string): boolean {
  const normalized = questionText?.trim().toLowerCase() ?? "";
  return HIDDEN_FIELD_PATTERNS.some((p) => p.test(normalized));
}

function FieldLabel({
  questionText,
  isRequired = TYPABLE_FIELDS_ALL_REQUIRED,
}: {
  questionText: string;
  isRequired?: boolean;
}): JSX.Element {
  const { label } = parseRequiredLabel(questionText);
  return (
    <Typography
      variant="caption"
      component="span"
      sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
    >
      {label}
      {isRequired && (
        <Box
          component="span"
          sx={{ color: "error.main", fontWeight: 600, marginLeft: "2px" }}
          aria-hidden
        >
          *
        </Box>
      )}
    </Typography>
  );
}

/** Deduplicate variables by questionText - keep first occurrence. */
function deduplicateVariables(
  variables: CatalogItemVariable[],
): CatalogItemVariable[] {
  const seen = new Set<string>();
  return variables.filter((v) => {
    const key = (v.questionText ?? "").trim().toLowerCase();
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

/**
 * Renders dynamic form fields based on catalog item variable types.
 * Adopts Create Case styling: Paper, Grid, outlined TextFields.
 *
 * @param {VariableFormFieldsProps} props - Variables schema, values, and context.
 * @returns {JSX.Element} The variable form fields.
 */
export default function VariableFormFields({
  variables,
  isLoading,
  values,
  onChange,
  selectedRequestTypeLabel,
  contextValues,
  attachments = [],
  onAttachmentClick,
  onAttachmentRemove,
  onAttachmentAdd,
  userTimeZone,
}: VariableFormFieldsProps): JSX.Element {
  const effectiveTimeZone = userTimeZone ?? resolveDisplayTimeZone();
  const minDatetime = computeMinScheduleDatetimeLocalForTimeZone(0, effectiveTimeZone);
  const sortedVariables = useMemo(
    () => (variables ? [...variables].sort((a, b) => a.order - b.order) : []),
    [variables],
  );
  const deduplicatedForDisplay = deduplicateVariables(sortedVariables);

  const allContextFields = useMemo(
    () =>
      contextValues
        ? sortedVariables.filter((v) =>
            isContextField(v.questionText, contextValues),
          )
        : [],
    [contextValues, sortedVariables],
  );
  const contextFieldsForDisplay = deduplicateVariables(allContextFields).filter(
    (v) =>
      !CONTEXT_FIELDS_HIDDEN_FROM_DISPLAY.some((p) =>
        p.test((v.questionText ?? "").trim().toLowerCase()),
      ),
  );
  const userFields = deduplicatedForDisplay.filter(
    (v) =>
      !isContextField(v.questionText, contextValues) &&
      !isHiddenField(v.questionText ?? ""),
  );

  useEffect(() => {
    if (!contextValues || !allContextFields.length) return;
    allContextFields.forEach((v) => {
      const val = getContextValue(v.questionText, contextValues);
      if (val && (values[v.id] ?? "") !== val) {
        onChange(v.id, val);
      }
    });
  }, [contextValues, allContextFields, onChange, values]);

  if (isLoading) {
    return (
      <Paper sx={{ p: 3 }}>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            mb: 3,
          }}
        >
          <Typography variant="h6">Request Details</Typography>
        </Box>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Skeleton variant="rounded" height={56} sx={{ maxWidth: "100%" }} />
          <Skeleton variant="rounded" height={56} sx={{ maxWidth: "100%" }} />
          <Skeleton variant="rounded" height={80} sx={{ maxWidth: "100%" }} />
        </Box>
      </Paper>
    );
  }

  if (!sortedVariables.length) {
    return (
      <Paper sx={{ p: 3 }}>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            mb: 3,
          }}
        >
          <Typography variant="h6">Request Details</Typography>
        </Box>
        <Typography variant="body2" color="text.secondary">
          No additional fields required for this request type.
        </Typography>
      </Paper>
    );
  }

  const renderField = (variable: CatalogItemVariable) => {
    const value = values[variable.id] ?? "";
    const type = (variable.type ?? VARIABLE_TYPE_SINGLE_LINE).trim();
    const isContext = contextValues
      ? isContextField(variable.questionText, contextValues)
      : false;
    const displayValue = isContext
      ? getContextValue(variable.questionText!, contextValues!)
      : value;
    const isTitle = isTitleField(variable.questionText ?? "");
    const titleLength = displayValue.trim().length;
    const isTitleTooLong = isTitle && titleLength > 160;

    if (
      type === VARIABLE_TYPE_SELECT ||
      type === VARIABLE_TYPE_RADIO ||
      type === VARIABLE_TYPE_CHECKBOX
    ) {
      return (
        <Grid key={variable.id} size={{ xs: 12 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel questionText={variable.questionText ?? ""} />
          </Box>
          <FormControl fullWidth size="small">
            <Select
              value={value}
              onChange={(e) => onChange(variable.id, e.target.value as string)}
              displayEmpty
              disabled={isContext}
              renderValue={(v) => v || "Select..."}
            >
              <MenuItem value="">
                <em>Select...</em>
              </MenuItem>
              <MenuItem value="Yes">Yes</MenuItem>
              <MenuItem value="No">No</MenuItem>
            </Select>
          </FormControl>
        </Grid>
      );
    }

    if (isDescriptionField(variable.questionText ?? "")) {
      return (
        <Grid key={variable.id} size={{ xs: 12 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel questionText={variable.questionText ?? ""} />
          </Box>
          <Editor
            value={value}
            onChange={(html) => onChange(variable.id, html)}
            disabled={isContext}
            placeholder=""
            minHeight={150}
            maxHeight="300px"
            showToolbar
            toolbarVariant="full"
            onAttachmentClick={onAttachmentClick}
            attachments={attachments.map((a) => a.file)}
            onAttachmentRemove={onAttachmentRemove}
          />
        </Grid>
      );
    }

    if (
      (type === VARIABLE_TYPE_MULTI_LINE ||
        (variable.type ?? "").toLowerCase().includes("multi")) &&
      !isDescriptionField(variable.questionText ?? "")
    ) {
      return (
        <Grid key={variable.id} size={{ xs: 12 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel questionText={variable.questionText ?? ""} />
          </Box>
          <TextField
            fullWidth
            multiline
            size="small"
            rows={4}
            value={displayValue}
            onChange={(e) => onChange(variable.id, e.target.value)}
            disabled={isContext}
          />
        </Grid>
      );
    }

    if (isFileCopyPathField(variable)) {
      return (
        <Grid key={variable.id} size={{ xs: 12 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel
              questionText={variable.questionText ?? ""}
              isRequired={false}
            />
          </Box>
          <TextField
            fullWidth
            size="small"
            value={displayValue}
            onChange={(e) => onChange(variable.id, e.target.value)}
            disabled={isContext}
          />
        </Grid>
      );
    }

    if (isAttachmentField(variable)) {
      return (
        <Grid key={variable.id} size={{ xs: 12 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel
              questionText={variable.questionText ?? ""}
              isRequired={false}
            />
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {onAttachmentAdd ? (
              <Button
                variant="outlined"
                size="small"
                startIcon={<Upload size={16} />}
                component="label"
                sx={{ alignSelf: "flex-start" }}
              >
                <input
                  type="file"
                  hidden
                  multiple
                  onChange={(e) => {
                    const files = e.target.files;
                    if (files?.length) {
                      for (let i = 0; i < files.length; i++) {
                        const f = files[i];
                        if (f)
                          onAttachmentAdd(
                            f,
                            variable.questionText ?? undefined,
                          );
                      }
                      e.target.value = "";
                    }
                  }}
                />
                Choose file(s)
              </Button>
            ) : (
              <Button
                variant="outlined"
                size="small"
                startIcon={<Upload size={16} />}
                onClick={onAttachmentClick}
                sx={{ alignSelf: "flex-start" }}
              >
                Add attachment
              </Button>
            )}
            {attachments.length > 0 && (
              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 0.5,
                  mt: 1,
                }}
              >
                {attachments.map((item, idx) => (
                  <Box
                    key={item.id}
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                      py: 0.5,
                      px: 1,
                      bgcolor: "action.hover",
                    }}
                  >
                    <Typography variant="body2" sx={{ flex: 1 }} noWrap>
                      {item.file.name}
                    </Typography>
                    {onAttachmentRemove && (
                      <IconButton
                        size="small"
                        onClick={() => onAttachmentRemove(idx)}
                        aria-label={`Remove ${item.file.name}`}
                      >
                        <X size={14} />
                      </IconButton>
                    )}
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        </Grid>
      );
    }

    if (isDateTimeField(variable)) {
      return (
        <Grid key={variable.id} size={{ xs: 12, sm: 6 }}>
          <Box sx={{ mb: 1 }}>
            <FieldLabel questionText={variable.questionText ?? ""} />
          </Box>
          <TextField
            fullWidth
            size="small"
            type="datetime-local"
            value={displayValue}
            onChange={(e) => onChange(variable.id, e.target.value)}
            disabled={isContext}
            slotProps={{ inputLabel: { shrink: true } }}
            inputProps={{ min: minDatetime }}
            helperText={`Timezone: ${effectiveTimeZone}`}
          />
        </Grid>
      );
    }

    return (
      <Grid key={variable.id} size={{ xs: 12 }}>
        <Box sx={{ mb: 1 }}>
          <FieldLabel questionText={variable.questionText ?? ""} />
        </Box>
        <TextField
          fullWidth
          size="small"
          value={displayValue}
          onChange={(e) => onChange(variable.id, e.target.value)}
          disabled={isContext}
          error={isTitleTooLong}
          helperText={isTitleTooLong ? "Title must be 160 characters or fewer." : undefined}
        />
        {isTitle && (
          <Typography
            variant="caption"
            color={isTitleTooLong ? "error.main" : "text.secondary"}
            align="right"
            sx={{ mt: 0.5, display: "block" }}
          >
            {titleLength}/160
          </Typography>
        )}
      </Grid>
    );
  };

  return (
    <Paper sx={{ p: 3 }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "flex-start",
          mb: 3,
          gap: 0.5,
        }}
      >
        <Typography variant="h6">Request Details</Typography>
        {selectedRequestTypeLabel && (
          <Typography variant="body2" color="text.secondary">
            Service request type: {selectedRequestTypeLabel}
          </Typography>
        )}
      </Box>

      {contextFieldsForDisplay.length > 0 && contextValues && (
        <Box sx={{ mb: 3 }}>
          <Typography
            variant="subtitle2"
            color="text.secondary"
            sx={{ mb: 1.5 }}
          >
            Context (from selection above)
          </Typography>
          <Grid container spacing={2}>
            {contextFieldsForDisplay.map((v) => (
              <Grid key={v.id} size={{ xs: 12, sm: 6, md: 4 }}>
                <Box sx={{ mb: 0.5 }}>
                  <Typography variant="caption" color="text.secondary">
                    {v.questionText}
                  </Typography>
                </Box>
                <TextField
                  fullWidth
                  size="small"
                  value={getContextValue(v.questionText, contextValues)}
                  disabled
                />
              </Grid>
            ))}
          </Grid>
        </Box>
      )}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Grid container spacing={2}>
          {userFields.map(renderField)}
        </Grid>
      </Box>
    </Paper>
  );
}
