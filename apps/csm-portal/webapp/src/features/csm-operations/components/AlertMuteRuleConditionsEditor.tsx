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
  Autocomplete,
  Box,
  Button,
  FormControl,
  FormHelperText,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useAlertFieldValues } from "@features/csm-operations/api/useAlertMuteRules";
import type { AlertMuteRuleCondition, AlertMuteRuleField } from "@features/csm-operations/types/alertMuteRules";
import {
  ALERT_MUTE_RULE_ENVIRONMENTS,
  ALERT_MUTE_RULE_FIELDS,
  ALERT_MUTE_RULE_VALUE_SUGGESTIONS,
  MAX_ALERT_MUTE_RULE_CONDITION_VALUE,
  REQUIRED_ALERT_MUTE_RULE_FIELDS,
} from "@features/csm-operations/utils/alertMuteRules";

interface ValueFieldProps {
  field: AlertMuteRuleField;
  value: string;
  error?: string;
  onChange: (value: string) => void;
  /** Shown as the field's label; the key/value rows use "Value". */
  label?: string;
  required?: boolean;
  sx?: object;
}

/**
 * Free text with suggestions: the values real alerts carry in this field (from
 * the alerting service), then the usual ones for the field. A rule matches the
 * alert's own text, so a CSM service name would not match anything.
 */
function ValueField({ field, value, error, onChange, label, required = true, sx }: ValueFieldProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const query = useDebouncedValue(value, 300);
  const seen = useAlertFieldValues(field, query, open);
  const options = [...(seen.data ?? [])];
  const have = new Set(options.map((o) => o.toLowerCase()));
  for (const s of ALERT_MUTE_RULE_VALUE_SUGGESTIONS[field] ?? []) {
    if (!have.has(s.toLowerCase())) options.push(s);
  }

  return (
    <Autocomplete
      freeSolo
      size="small"
      options={options}
      inputValue={value}
      onInputChange={(_, next) => onChange(next)}
      open={open && options.length > 0}
      onOpen={() => setOpen(true)}
      onClose={() => setOpen(false)}
      sx={sx ?? { flex: "1 1 260px" }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label ?? "Value"}
          required={required}
          error={!!error}
          helperText={error}
          slotProps={{
            htmlInput: {
              ...params.inputProps,
              maxLength: MAX_ALERT_MUTE_RULE_CONDITION_VALUE,
              ...(label ? {} : { "aria-label": `${field} value` }),
            },
          }}
        />
      )}
    />
  );
}

interface AlertMuteRuleConditionsEditorProps {
  conditions: AlertMuteRuleCondition[];
  onChange: (next: AlertMuteRuleCondition[]) => void;
  /** The rule-level error, e.g. a required field is missing. */
  error?: string;
  rowErrors?: Record<number, string>;
}

/**
 * "What to mute": a Service field and an Environment list, which every rule
 * has, then optional key/value rows (like CloudWatch's tag editor) for the
 * alert's other fields, each usable once. An alert is muted only when every
 * condition matches. Service suggests the services alerts actually report.
 */
export default function AlertMuteRuleConditionsEditor({
  conditions,
  onChange,
  error,
  rowErrors,
}: AlertMuteRuleConditionsEditorProps): JSX.Element {
  const used = new Set(conditions.map((c) => c.field));
  const unused = ALERT_MUTE_RULE_FIELDS.filter((f) => !used.has(f.value));
  const indexOf = (field: AlertMuteRuleField): number => conditions.findIndex((c) => c.field === field);

  const update = (index: number, change: Partial<AlertMuteRuleCondition>): void =>
    onChange(conditions.map((c, i) => (i === index ? { ...c, ...change } : c)));
  const setRequired = (field: AlertMuteRuleField, value: string): void => {
    const i = indexOf(field);
    onChange(i >= 0 ? conditions.map((c, j) => (j === i ? { ...c, value } : c)) : [...conditions, { field, value }]);
  };

  const serviceIndex = indexOf("service");
  const environmentIndex = indexOf("environment");
  const service = serviceIndex >= 0 ? conditions[serviceIndex].value : "";
  const environment = environmentIndex >= 0 ? conditions[environmentIndex].value : "";
  const environmentOptions = ALERT_MUTE_RULE_ENVIRONMENTS.some((o) => o.value === environment) || !environment
    ? ALERT_MUTE_RULE_ENVIRONMENTS
    : [...ALERT_MUTE_RULE_ENVIRONMENTS, { value: environment, label: environment }];
  const extras = conditions
    .map((condition, index) => ({ condition, index }))
    .filter(({ condition }) => !REQUIRED_ALERT_MUTE_RULE_FIELDS.includes(condition.field));

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <ValueField
          field="service"
          label="Service"
          value={service}
          onChange={(next) => setRequired("service", next)}
          error={serviceIndex >= 0 ? rowErrors?.[serviceIndex] : undefined}
          sx={{ flex: "1 1 320px" }}
        />
        <FormControl
          size="small"
          required
          error={environmentIndex >= 0 && !!rowErrors?.[environmentIndex]}
          sx={{ flex: "1 1 220px" }}
        >
          <InputLabel id="alert-mute-rule-environment-label">Environment</InputLabel>
          <Select
            labelId="alert-mute-rule-environment-label"
            label="Environment"
            value={environment}
            onChange={(e) => setRequired("environment", String(e.target.value))}
          >
            {environmentOptions.map((o) => (
              <MenuItem key={o.value} value={o.value}>
                {o.label}
              </MenuItem>
            ))}
          </Select>
          {environmentIndex >= 0 && rowErrors?.[environmentIndex] && (
            <FormHelperText>{rowErrors[environmentIndex]}</FormHelperText>
          )}
        </FormControl>
      </Box>

      {extras.map(({ condition, index }) => (
        <Box key={`${condition.field}-${index}`} sx={{ display: "flex", gap: 1.5, alignItems: "flex-start", flexWrap: "wrap" }}>
          <Select
            size="small"
            value={condition.field}
            onChange={(e) => update(index, { field: e.target.value as AlertMuteRuleField })}
            inputProps={{ "aria-label": `Condition ${index + 1} field` }}
            sx={{ flex: "0 1 220px", minWidth: 180 }}
          >
            {ALERT_MUTE_RULE_FIELDS.filter((f) => f.value === condition.field || !used.has(f.value)).map((f) => (
              <MenuItem key={f.value} value={f.value}>
                {f.label}
              </MenuItem>
            ))}
          </Select>
          <ValueField
            field={condition.field}
            value={condition.value}
            error={rowErrors?.[index]}
            onChange={(value) => update(index, { value })}
          />
          <IconButton
            aria-label={`Remove ${condition.field} condition`}
            onClick={() => onChange(conditions.filter((_, i) => i !== index))}
            sx={{ mt: 0.25 }}
          >
            <Trash2 size={16} />
          </IconButton>
        </Box>
      ))}
      {error && (
        <Typography variant="caption" color="error" role="alert">
          {error}
        </Typography>
      )}
      <Box>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Plus size={16} />}
          disabled={unused.length === 0}
          onClick={() => onChange([...conditions, { field: unused[0].value, value: "" }])}
        >
          Add condition
        </Button>
      </Box>
    </Box>
  );
}
