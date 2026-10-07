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

import { Autocomplete, Box, Button, Chip, TextField } from "@wso2/oxygen-ui";
import { Lock } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import type { ScopeOption } from "@features/csm-operations/api/useChangeRequestScopeLookups";
import type { ChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";

interface ChangeRequestScopeFieldsProps {
  scope: ChangeRequestScope;
  disabled?: boolean;
  /**
   * Disables the Customer Project picker alone (the deployments follow
   * `disabled`): the project is frozen once approval was requested, while the
   * deployments stay editable until Implement.
   */
  projectDisabled?: boolean;
  /** Prefix for element ids (`cr` on the create page, `cr-edit` in the dialog). */
  idPrefix: string;
  /** Whether the Customer Project can be cleared once picked (default true). */
  projectClearable?: boolean;
}

function selectedOptions(
  ids: string[],
  labels: Record<string, string>,
): ScopeOption[] {
  return ids.map((id) => ({ id, label: labels[id] ?? id }));
}

/**
 * The Customer Project / Deployments / Deployment products block, in the order
 * the ServiceNow change-request form lays them out. Deployments stays disabled
 * until a project is chosen; Deployment products is read-only (lock icon)
 * because it is derived. (A deployment already carries its environment role,
 * so there is no separate Environments field.)
 */
export default function ChangeRequestScopeFields({
  scope,
  disabled = false,
  projectDisabled = false,
  idPrefix,
  projectClearable = true,
}: ChangeRequestScopeFieldsProps): JSX.Element {
  const hasProject = !!scope.projectId;

  const deploymentValue = selectedOptions(scope.deploymentIds, scope.deploymentLabels);
  const productValue = selectedOptions(scope.deploymentProductIds, scope.deploymentProductLabels);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <AsyncProjectSelect
        id={`${idPrefix}-project`}
        label="Customer Project"
        value={scope.projectId}
        knownLabel={scope.projectLabel || undefined}
        onChange={scope.setProject}
        disabled={disabled || projectDisabled}
        disableClearable={!projectClearable}
      />

      <Autocomplete<ScopeOption, true>
        multiple
        fullWidth
        size="small"
        id={`${idPrefix}-deployments`}
        options={scope.deploymentOptions}
        value={deploymentValue}
        disabled={disabled || !hasProject}
        loading={scope.lookups.isLoading}
        disableCloseOnSelect
        getOptionLabel={(o) => o.label}
        isOptionEqualToValue={(o, v) => o.id === v.id}
        onChange={(_e, next) => scope.setDeployments(next.map((o) => o.id))}
        noOptionsText={
          scope.lookups.isError
            ? "Couldn't load deployments. Try again."
            : scope.lookups.isLoading
              ? "Loading deployments…"
              : "This project has no deployments"
        }
        renderInput={(params) => (
          <TextField
            {...params}
            label="Deployments"
            placeholder={deploymentValue.length ? undefined : "Select deployments…"}
            error={scope.lookups.isError}
            helperText={
              scope.lookups.isError ? (
                <>
                  Couldn&apos;t load deployments.{" "}
                  <Button size="small" variant="text" onClick={scope.lookups.refetch} sx={{ minWidth: 0, p: 0 }}>
                    Retry
                  </Button>
                </>
              ) : !hasProject ? (
                "Select a Customer Project first."
              ) : undefined
            }
          />
        )}
      />

      <TextField
        id={`${idPrefix}-deployment-products`}
        label="Deployment products"
        size="small"
        fullWidth
        value=""
        placeholder={productValue.length ? undefined : "—"}
        helperText="Derived from the selected deployments"
        slotProps={{
          inputLabel: { shrink: true },
          input: {
            readOnly: true,
            startAdornment: productValue.length ? (
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mr: 0.5 }}>
                {productValue.map((p) => (
                  <Chip key={p.id} size="small" variant="outlined" label={p.label} />
                ))}
              </Box>
            ) : undefined,
            endAdornment: <Lock size={16} aria-hidden style={{ opacity: 0.6 }} />,
          },
          htmlInput: { "aria-readonly": true },
        }}
      />
    </Box>
  );
}

interface ChangeRequestCustomerGroupFieldProps {
  scope: ChangeRequestScope;
  /** Prefix for element ids (`cr` on the create page, `cr-edit` in the dialog). */
  idPrefix: string;
}

/**
 * The change request's "Customer Group", READ-ONLY: the registered contacts of
 * the chosen Customer Project, listed as chips (lock icon, like Deployment
 * products). There is nothing to pick — the group is derived by the backend
 * from the project, so it can never name another customer's people — and
 * nothing is sent back; it follows the project the moment that changes.
 */
export function ChangeRequestCustomerGroupField({
  scope,
  idPrefix,
}: ChangeRequestCustomerGroupFieldProps): JSX.Element {
  const hasProject = !!scope.projectId;
  const contacts = hasProject ? scope.customerContacts : [];
  const helper = !hasProject
    ? "Select a Customer Project first."
    : scope.lookups.isError
      ? "Couldn't load the project's contacts."
      : !scope.customerContactsReady
        ? "Loading the project's registered contacts…"
        : contacts.length === 0
          ? "No registered contacts on this project. Derived from the customer project's registered contacts."
          : "Derived from the customer project's registered contacts";

  return (
    <TextField
      id={`${idPrefix}-customer-group`}
      label="Customer Group"
      size="small"
      fullWidth
      value=""
      placeholder={contacts.length ? undefined : "—"}
      helperText={helper}
      slotProps={{
        inputLabel: { shrink: true },
        input: {
          readOnly: true,
          startAdornment: contacts.length ? (
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mr: 0.5 }}>
              {contacts.map((c) => (
                <Chip
                  key={c.id}
                  size="small"
                  variant="outlined"
                  label={c.name}
                  title={c.email || undefined}
                />
              ))}
            </Box>
          ) : undefined,
          endAdornment: <Lock size={16} aria-hidden style={{ opacity: 0.6 }} />,
        },
        htmlInput: { "aria-readonly": true },
      }}
    />
  );
}
