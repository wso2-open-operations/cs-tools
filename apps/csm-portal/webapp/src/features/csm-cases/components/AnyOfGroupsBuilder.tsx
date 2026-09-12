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
  Chip,
  Divider,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import AsyncAssignedUserIdMultiSelect from "@features/csm-cases/components/AsyncAssignedUserIdMultiSelect";
import {
  ANY_OF_FILTER_FIELDS,
  defaultAnyOfBranch,
  defaultAnyOfFilterRow,
  getAnyOfFilterFieldMeta,
  type AnyOfBranch,
  type AnyOfFilterField,
  type AnyOfFilterRow,
} from "@features/csm-cases/utils/anyOfFilters";

interface AnyOfGroupsBuilderProps {
  branches: AnyOfBranch[];
  onChange: (next: AnyOfBranch[]) => void;
}

/** Splits a comma-separated free-text entry into a trimmed, non-empty array. */
function splitCsv(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * "OR groups" (`filters.anyOf`) builder: a list of branches, each its own
 * bordered box of AND-ed field/value conditions (reusing the same row-editor
 * shape `AdvancedFiltersBuilder` uses, backed by the much narrower branch
 * field allowlist in `anyOfFilters.ts`), with the branches themselves OR'd
 * together and the whole result ANDed with every other active filter. See
 * `CasesFilters.anyOfBranches`'s own doc comment for the exact semantics.
 */
export default function AnyOfGroupsBuilder({
  branches,
  onChange,
}: AnyOfGroupsBuilderProps): JSX.Element {
  const updateBranch = (branchIndex: number, next: AnyOfBranch): void => {
    onChange(branches.map((b, i) => (i === branchIndex ? next : b)));
  };
  const removeBranch = (branchIndex: number): void => {
    onChange(branches.filter((_, i) => i !== branchIndex));
  };
  const addBranch = (): void => {
    onChange([...branches, defaultAnyOfBranch()]);
  };

  const updateRow = (branchIndex: number, rowIndex: number, next: AnyOfFilterRow): void => {
    const branch = branches[branchIndex];
    updateBranch(branchIndex, {
      filters: branch.filters.map((r, i) => (i === rowIndex ? next : r)),
    });
  };
  const removeRow = (branchIndex: number, rowIndex: number): void => {
    const branch = branches[branchIndex];
    updateBranch(branchIndex, { filters: branch.filters.filter((_, i) => i !== rowIndex) });
  };
  const addRow = (branchIndex: number): void => {
    const branch = branches[branchIndex];
    updateBranch(branchIndex, { filters: [...branch.filters, defaultAnyOfFilterRow()] });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="subtitle2" color="text.secondary">
        OR groups
      </Typography>
      <Typography variant="caption" color="text.secondary">
        Each group&apos;s own conditions are ANDed together; the groups themselves are OR&apos;d,
        then combined with every other active filter above.
      </Typography>
      {branches.map((branch, branchIndex) => (
        <Box key={`any-of-branch-${branchIndex}`}>
          {branchIndex > 0 && (
            <Box sx={{ display: "flex", justifyContent: "center", my: 0.5 }}>
              <Chip size="small" label="OR" color="default" />
            </Box>
          )}
          <Paper
            variant="outlined"
            sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 1 }}
          >
            <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <Typography variant="caption" color="text.secondary">
                Group {branchIndex + 1}
              </Typography>
              <IconButton
                size="small"
                aria-label={`Remove OR group ${branchIndex + 1}`}
                onClick={() => removeBranch(branchIndex)}
              >
                <Trash2 size={16} />
              </IconButton>
            </Box>
            {branch.filters.map((row, rowIndex) => {
              const fieldMeta = getAnyOfFilterFieldMeta(row.field);
              return (
                <Box key={`any-of-branch-${branchIndex}-row-${rowIndex}`}>
                  {rowIndex > 0 && (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ display: "block", mb: 0.5 }}
                    >
                      AND
                    </Typography>
                  )}
                  <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start", flexWrap: "wrap" }}>
                    <FormControl size="small" sx={{ minWidth: 180 }}>
                      <InputLabel id={`any-of-${branchIndex}-${rowIndex}-field-label`}>
                        Field
                      </InputLabel>
                      <Select
                        labelId={`any-of-${branchIndex}-${rowIndex}-field-label`}
                        label="Field"
                        value={row.field}
                        onChange={(e) =>
                          updateRow(branchIndex, rowIndex, {
                            field: e.target.value as AnyOfFilterField,
                            values: [],
                          })
                        }
                      >
                        {ANY_OF_FILTER_FIELDS.map((m) => (
                          <MenuItem key={m.field} value={m.field}>
                            {m.label}
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>

                    <Box sx={{ minWidth: 220, maxWidth: 320, flex: "1 1 220px" }}>
                      {fieldMeta?.valueKind === "multiSelect" && (
                        <MultiSelectField
                          id={`any-of-${branchIndex}-${rowIndex}-value`}
                          label="Value(s)"
                          values={row.values}
                          options={fieldMeta.options ?? []}
                          onChange={(next) =>
                            updateRow(branchIndex, rowIndex, { ...row, values: next })
                          }
                        />
                      )}
                      {fieldMeta?.valueKind === "multiText" && (
                        <TextField
                          size="small"
                          fullWidth
                          label="Value(s)"
                          placeholder={fieldMeta.placeholder ?? "Comma-separated values"}
                          value={row.values.join(", ")}
                          onChange={(e) =>
                            updateRow(branchIndex, rowIndex, {
                              ...row,
                              values: splitCsv(e.target.value),
                            })
                          }
                          helperText="Comma-separated"
                        />
                      )}
                      {fieldMeta?.valueKind === "asyncProject" && (
                        <AsyncProjectMultiSelect
                          id={`any-of-${branchIndex}-${rowIndex}-project`}
                          values={row.values}
                          onChange={(next) =>
                            updateRow(branchIndex, rowIndex, { ...row, values: next })
                          }
                        />
                      )}
                      {fieldMeta?.valueKind === "asyncAssignedUser" && (
                        <AsyncAssignedUserIdMultiSelect
                          values={row.values}
                          onChange={(next) =>
                            updateRow(branchIndex, rowIndex, { ...row, values: next })
                          }
                        />
                      )}
                    </Box>

                    <IconButton
                      size="small"
                      aria-label="Remove condition"
                      onClick={() => removeRow(branchIndex, rowIndex)}
                      sx={{ mt: 0.5 }}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Box>
                </Box>
              );
            })}
            <Box>
              <Button
                size="small"
                variant="outlined"
                startIcon={<Plus size={16} />}
                onClick={() => addRow(branchIndex)}
              >
                Add condition
              </Button>
            </Box>
          </Paper>
        </Box>
      ))}
      <Divider sx={{ display: branches.length > 0 ? "block" : "none" }} />
      <Box>
        <Button size="small" variant="outlined" startIcon={<Plus size={16} />} onClick={addBranch}>
          Add group
        </Button>
      </Box>
    </Box>
  );
}
