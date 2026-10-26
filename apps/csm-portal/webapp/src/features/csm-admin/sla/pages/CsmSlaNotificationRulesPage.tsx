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
  Checkbox,
  Chip,
  FormControlLabel,
  MenuItem,
  Select,
  Skeleton,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useSlaDefinitions } from "../api/useSlaDefinitions";
import {
  useCreateSlaNotificationRule,
  useSlaNotificationRules,
  useUpdateSlaNotificationRule,
} from "../api/useSlaNotificationRules";
import type { SlaNotificationChannel, SlaNotificationRule, UpsertSlaNotificationRuleRequest } from "../types";

const EMPTY: UpsertSlaNotificationRuleRequest = {
  channel: "email",
  recipients: { assignee: true, team: false },
  isActive: true,
};

function RuleRow({ rule }: { rule: SlaNotificationRule }): JSX.Element {
  const definitions = useSlaDefinitions();
  const update = useUpdateSlaNotificationRule();
  const definitionName = definitions.data?.definitions.find((d) => d.id === rule.definitionId)?.name;

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 2, p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
      <Switch
        checked={rule.isActive}
        disabled={update.isPending}
        onChange={(e) =>
          update.mutate({
            id: rule.id,
            body: {
              definitionId: rule.definitionId,
              thresholdPercentage: rule.thresholdPercentage,
              channel: rule.channel,
              recipients: rule.recipients,
              webhookUrl: rule.webhookUrl,
              isActive: e.target.checked,
            },
          })
        }
      />
      <Chip size="small" label={rule.channel} />
      <Typography variant="body2" sx={{ flex: 1 }}>
        {definitionName ?? "Every definition"}
        {rule.thresholdPercentage ? ` @ ${rule.thresholdPercentage}%` : " @ any threshold"}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {[
          rule.recipients.assignee ? "assignee" : null,
          rule.recipients.team ? "team" : null,
          ...(rule.recipients.users ?? []),
          ...(rule.recipients.groups ?? []),
        ]
          .filter(Boolean)
          .join(", ")}
      </Typography>
    </Box>
  );
}

/** Notification Rules tab (R46-R51): which threshold-crossing events
 *  trigger which action. Recipients are resolved fresh when the event is
 *  handled (R49) — this screen only configures the rule, never a frozen
 *  recipient list. */
export default function CsmSlaNotificationRulesPage(): JSX.Element {
  const rules = useSlaNotificationRules();
  const definitions = useSlaDefinitions();
  const create = useCreateSlaNotificationRule();
  const [form, setForm] = useState<UpsertSlaNotificationRuleRequest>(EMPTY);
  const [showForm, setShowForm] = useState(false);
  // Raw text of the two comma-separated fields, kept separately from
  // form.recipients.users/groups rather than derived from them on every
  // render: deriving the displayed text from the parsed array drops a
  // trailing comma or space the instant it's typed, making it impossible
  // to type a second email/group by hand. Parsed into the actual request
  // only on submit.
  const [usersText, setUsersText] = useState("");
  const [groupsText, setGroupsText] = useState("");

  const parseCommaList = (s: string): string[] => s.split(",").map((v) => v.trim()).filter(Boolean);

  const submit = (): void => {
    create.mutate(
      {
        ...form,
        recipients: { ...form.recipients, users: parseCommaList(usersText), groups: parseCommaList(groupsText) },
      },
      {
        onSuccess: () => {
          setForm(EMPTY);
          setUsersText("");
          setGroupsText("");
          setShowForm(false);
        },
      },
    );
  };

  return (
    <Box>
      {rules.isLoading ? <Skeleton variant="rectangular" height={200} /> : null}
      {rules.isError ? <Alert severity="error">Failed to load SLA notification rules.</Alert> : null}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, mb: 2 }}>
        {(rules.data ?? []).map((r) => (
          <RuleRow key={r.id} rule={r} />
        ))}
      </Box>

      {showForm ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, maxWidth: 600, border: "1px solid", borderColor: "divider", borderRadius: 1, p: 2 }}>
          <Select
            value={form.definitionId ?? ""}
            displayEmpty
            onChange={(e) => setForm({ ...form, definitionId: e.target.value || undefined })}
          >
            <MenuItem value="">Every definition</MenuItem>
            {(definitions.data?.definitions ?? []).map((d) => (
              <MenuItem key={d.id} value={d.id}>
                {d.name}
              </MenuItem>
            ))}
          </Select>
          <TextField
            label="Threshold percentage (blank = any)"
            type="number"
            value={form.thresholdPercentage ?? ""}
            onChange={(e) =>
              setForm({ ...form, thresholdPercentage: e.target.value ? Number(e.target.value) : undefined })
            }
          />
          <Select
            value={form.channel}
            onChange={(e) => setForm({ ...form, channel: e.target.value as SlaNotificationChannel })}
          >
            <MenuItem value="email">Email</MenuItem>
            <MenuItem value="webhook">Webhook</MenuItem>
          </Select>
          {form.channel === "webhook" ? (
            <TextField
              label="Webhook URL"
              value={form.webhookUrl ?? ""}
              onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
            />
          ) : null}
          <Box sx={{ display: "flex", gap: 2 }}>
            <FormControlLabel
              control={
                <Checkbox
                  checked={form.recipients.assignee}
                  onChange={(e) => setForm({ ...form, recipients: { ...form.recipients, assignee: e.target.checked } })}
                />
              }
              label="Assignee"
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={form.recipients.team}
                  onChange={(e) => setForm({ ...form, recipients: { ...form.recipients, team: e.target.checked } })}
                />
              }
              label="Team"
            />
          </Box>
          <TextField
            label="Named users (comma-separated emails)"
            value={usersText}
            onChange={(e) => setUsersText(e.target.value)}
          />
          <TextField
            label="Named groups (comma-separated)"
            value={groupsText}
            onChange={(e) => setGroupsText(e.target.value)}
          />
          {create.isError ? <Alert severity="error">{create.error.message}</Alert> : null}
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button variant="contained" disabled={create.isPending} onClick={submit}>
              Create rule
            </Button>
            <Button onClick={() => setShowForm(false)}>Cancel</Button>
          </Box>
        </Box>
      ) : (
        <Button startIcon={<Plus size={16} />} onClick={() => setShowForm(true)}>
          New notification rule
        </Button>
      )}
    </Box>
  );
}
