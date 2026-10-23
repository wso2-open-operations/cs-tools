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

import { Box, Button, IconButton, MenuItem, Select, TextField, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { SlaFieldCatalogEntry } from "../types";
import {
  emptyLeaf,
  isAllGroup,
  isLeaf,
  isNotGroup,
  OP_LABELS,
  OPS_WITH_LIST_VALUE,
  OPS_WITH_VALUE,
  type PredicateNode,
  type PredicateOp,
} from "../utils/condition";

const ALL_OPS: PredicateOp[] = [
  "eq",
  "neq",
  "in",
  "not_in",
  "is_null",
  "is_not_null",
  "changed",
  "changed_to",
  "changed_from",
];

interface Props {
  /** `null` means "not set" (R19-R21: a condition that is not set never
   *  fires). `undefined` is treated the same as `null`. */
  value: PredicateNode | null | undefined;
  onChange: (value: PredicateNode | null) => void;
  /** The enabled field catalog for the definition's own work item type
   *  (R5) — the only fields a condition may reference. */
  fields: SlaFieldCatalogEntry[];
  /** Whether an empty condition is allowed here (pause/stop/cancel) or must
   *  always have a value (start — R18 is the only required condition). */
  required?: boolean;
}

function fieldValues(fields: SlaFieldCatalogEntry[], fieldKey: string): string[] | undefined {
  return fields.find((f) => f.fieldKey === fieldKey)?.enumValues;
}

function LeafEditor({
  leaf,
  fields,
  onChange,
}: {
  leaf: { field: string; op: PredicateOp; value?: unknown };
  fields: SlaFieldCatalogEntry[];
  onChange: (next: PredicateNode) => void;
}): JSX.Element {
  const enumValues = fieldValues(fields, leaf.field);
  const needsValue = OPS_WITH_VALUE.has(leaf.op);
  const isListValue = OPS_WITH_LIST_VALUE.has(leaf.op);

  const valueAsArray: string[] = Array.isArray(leaf.value) ? (leaf.value as string[]) : [];
  const valueAsString = typeof leaf.value === "string" ? leaf.value : "";

  // Raw text of the free-form comma-separated list field, kept in local
  // state while the user is typing. The committed value (leaf.value, what
  // is actually saved) is derived from this on every keystroke, but the
  // *displayed* text is this draft, not a re-join of the parsed array --
  // deriving the displayed value from the parsed array drops a trailing
  // comma or space the instant it's typed (split+trim+filter(Boolean)
  // removes the empty segment it would produce), making it impossible to
  // type a second value by hand. null means "not currently editing this
  // field" -- falls back to the committed value, e.g. right after the
  // field/op selection changes it from elsewhere.
  const [listDraft, setListDraft] = useState<string | null>(null);

  return (
    <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
      <Select
        size="small"
        value={leaf.field}
        onChange={(e) => {
          // Changing the field never changes leaf.op, so a value shape the
          // retained operator needs (an array, for "is any of"/"is none
          // of") must be preserved here the same way the operator-change
          // handler already does it -- resetting to a bare "" regardless
          // of op left a list operator pointing at a string value, shown
          // as an empty selection but saved as the wrong shape.
          setListDraft(null);
          const field = String(e.target.value);
          onChange(
            !OPS_WITH_VALUE.has(leaf.op)
              ? { field, op: leaf.op }
              : { field, op: leaf.op, value: OPS_WITH_LIST_VALUE.has(leaf.op) ? [] : "" },
          );
        }}
        sx={{ minWidth: 180 }}
      >
        {fields.map((f) => (
          <MenuItem key={f.fieldKey} value={f.fieldKey}>
            {f.displayLabel}
          </MenuItem>
        ))}
      </Select>
      <Select
        size="small"
        value={leaf.op}
        onChange={(e) => {
          // The new operator may take a differently-shaped value (or none)
          // than the old one -- switching from "is any of" (an array) to
          // "is" (a single value) must not leave the array in place, and
          // switching to an operator that takes no value at all (is_null,
          // is_not_null, changed) must not leave a stale one a predicate
          // evaluator would never read but which still reaches storage.
          const op = e.target.value as PredicateOp;
          const value = !OPS_WITH_VALUE.has(op) ? undefined : OPS_WITH_LIST_VALUE.has(op) ? [] : "";
          setListDraft(null);
          onChange(value === undefined ? { field: leaf.field, op } : { field: leaf.field, op, value });
        }}
        sx={{ minWidth: 160 }}
      >
        {ALL_OPS.map((op) => (
          <MenuItem key={op} value={op}>
            {OP_LABELS[op]}
          </MenuItem>
        ))}
      </Select>
      {needsValue && isListValue && enumValues ? (
        <Select
          size="small"
          multiple
          value={valueAsArray}
          onChange={(e) => onChange({ ...leaf, value: e.target.value })}
          sx={{ minWidth: 200 }}
          renderValue={(selected) => (selected as string[]).join(", ")}
        >
          {enumValues.map((v) => (
            <MenuItem key={v} value={v}>
              {v}
            </MenuItem>
          ))}
        </Select>
      ) : null}
      {needsValue && !isListValue && enumValues ? (
        <Select
          size="small"
          value={valueAsString}
          onChange={(e) => onChange({ ...leaf, value: e.target.value })}
          sx={{ minWidth: 160 }}
        >
          {enumValues.map((v) => (
            <MenuItem key={v} value={v}>
              {v}
            </MenuItem>
          ))}
        </Select>
      ) : null}
      {needsValue && !enumValues ? (
        <TextField
          size="small"
          placeholder={isListValue ? "comma-separated values" : "value"}
          value={isListValue ? (listDraft ?? valueAsArray.join(", ")) : valueAsString}
          onChange={(e) => {
            if (!isListValue) {
              onChange({ ...leaf, value: e.target.value });
              return;
            }
            setListDraft(e.target.value);
            onChange({ ...leaf, value: e.target.value.split(",").map((v) => v.trim()).filter(Boolean) });
          }}
          onBlur={() => setListDraft(null)}
        />
      ) : null}
    </Box>
  );
}

/**
 * A recursive `all`/`any`/`not` predicate-tree builder (SLA SPEC.md R4).
 * Nothing in this webapp already does this generically — the closest
 * existing pieces (Dashboard Builder's `WidgetFilterConditionEditor`, Case
 * search's `AnyOfGroupsBuilder`) are each flat or two-level only and
 * hardcoded to a different backend contract, so this is built fresh,
 * reusing their row-based visual style (field/operator/value in a row,
 * "Add condition"/"Add group" buttons) rather than their state shape.
 */
export default function PredicateConditionEditor({ value, onChange, fields, required }: Props): JSX.Element {
  const node = value ?? null;

  if (node === null) {
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          {required ? "No condition set — this condition is required." : "Not set — never fires."}
        </Typography>
        <Button
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => onChange(fields.length ? emptyLeaf(fields[0].fieldKey) : emptyLeaf(""))}
        >
          Add condition
        </Button>
      </Box>
    );
  }

  if (isLeaf(node)) {
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <LeafEditor leaf={node} fields={fields} onChange={onChange} />
        <Box sx={{ display: "flex", gap: 0.5 }}>
          <Tooltip title="Group with another condition (AND)">
            <Button size="small" onClick={() => onChange({ all: [node, emptyLeaf(fields[0]?.fieldKey ?? "")] })}>
              AND
            </Button>
          </Tooltip>
          <Tooltip title="Group with another condition (OR)">
            <Button size="small" onClick={() => onChange({ any: [node, emptyLeaf(fields[0]?.fieldKey ?? "")] })}>
              OR
            </Button>
          </Tooltip>
          <Tooltip title="Negate">
            <Button size="small" onClick={() => onChange({ not: node })}>
              NOT
            </Button>
          </Tooltip>
          <Tooltip title="Remove condition">
            <IconButton size="small" onClick={() => onChange(null)}>
              <Trash2 size={16} />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
    );
  }

  if (isNotGroup(node)) {
    return (
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1 }}>
        <Typography variant="overline" sx={{ pt: 1 }}>
          NOT
        </Typography>
        <Box sx={{ flex: 1, border: "1px solid", borderColor: "divider", borderRadius: 1, p: 1 }}>
          <PredicateConditionEditor
            value={node.not}
            onChange={(child) => (child === null ? onChange(null) : onChange({ not: child }))}
            fields={fields}
          />
        </Box>
        <IconButton size="small" onClick={() => onChange(node.not)}>
          <Trash2 size={16} />
        </IconButton>
      </Box>
    );
  }

  const kind = isAllGroup(node) ? "all" : "any";
  const children = isAllGroup(node) ? node.all : (node as { any: PredicateNode[] }).any;

  const setChildren = (next: PredicateNode[]): void => {
    if (next.length === 0) {
      onChange(null);
      return;
    }
    if (next.length === 1) {
      onChange(next[0]);
      return;
    }
    onChange(kind === "all" ? { all: next } : { any: next });
  };

  return (
    <Box sx={{ border: "1px solid", borderColor: "divider", borderRadius: 1, p: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
        <Typography variant="overline">{kind === "all" ? "ALL of (AND)" : "ANY of (OR)"}</Typography>
      </Box>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {children.map((child, idx) => (
          <Box key={idx} sx={{ display: "flex", alignItems: "flex-start", gap: 1 }}>
            <Box sx={{ flex: 1 }}>
              <PredicateConditionEditor
                value={child}
                onChange={(next) => {
                  const updated = [...children];
                  if (next === null) {
                    updated.splice(idx, 1);
                  } else {
                    updated[idx] = next;
                  }
                  setChildren(updated);
                }}
                fields={fields}
              />
            </Box>
          </Box>
        ))}
      </Box>
      <Box sx={{ display: "flex", gap: 1, mt: 1 }}>
        <Button
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setChildren([...children, emptyLeaf(fields[0]?.fieldKey ?? "")])}
        >
          Add condition
        </Button>
        <Button
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() =>
            setChildren([...children, { all: [emptyLeaf(fields[0]?.fieldKey ?? "")] }])
          }
        >
          Add nested group
        </Button>
      </Box>
    </Box>
  );
}
