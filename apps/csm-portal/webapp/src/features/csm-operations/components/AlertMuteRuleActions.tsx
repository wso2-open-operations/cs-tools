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
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
} from "@wso2/oxygen-ui";
import { CircleStop, Clock, MoreVertical, Pencil, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type MouseEvent, type ReactNode } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import type { AlertMuteRule, AlertMuteRuleState } from "@features/csm-operations/types/alertMuteRules";
import { useDeleteAlertMuteRule, useEndAlertMuteRule, useExtendAlertMuteRule } from "@features/csm-operations/api/useAlertMuteRules";
import ExtendAlertMuteRuleDialog from "@features/csm-operations/components/ExtendAlertMuteRuleDialog";
import { alertMuteRuleActions, type AlertMuteRuleAction } from "@features/csm-operations/utils/alertMuteRules";

const ACTION_META: Record<AlertMuteRuleAction, { label: string; icon: ReactNode }> = {
  end: { label: "End now", icon: <CircleStop size={16} /> },
  extend: { label: "Extend", icon: <Clock size={16} /> },
  edit: { label: "Edit", icon: <Pencil size={16} /> },
  delete: { label: "Delete", icon: <Trash2 size={16} /> },
};

interface AlertMuteRuleActionsProps {
  rule: AlertMuteRule;
  state: AlertMuteRuleState;
  /** "menu": a ⋮ button for a table row; "buttons": a row of buttons for the detail page. */
  variant: "menu" | "buttons";
  /** Where Edit returns to, and where Delete goes from the detail page. */
  returnTo: string;
  /** The returnTo page's own location.state, handed back by the form's Back. */
  parentState?: unknown;
  onDeleted?: () => void;
}

/** A rule's actions with their dialogs: End now and Delete confirm first, Extend picks the new end. */
export default function AlertMuteRuleActions({
  rule,
  state,
  variant,
  returnTo,
  parentState = null,
  onDeleted,
}: AlertMuteRuleActionsProps): JSX.Element | null {
  const navigate = useNavTransition();
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [open, setOpen] = useState<"end" | "extend" | "delete" | null>(null);
  const endRule = useEndAlertMuteRule();
  const extendRule = useExtendAlertMuteRule();
  const deleteRule = useDeleteAlertMuteRule();
  const actions = alertMuteRuleActions(state);
  if (actions.length === 0) return null;

  const run = (action: AlertMuteRuleAction): void => {
    setMenuAnchor(null);
    if (action === "edit") {
      navigate(`/operations/alert-mute-rules/${rule.id}/edit`, { state: { from: returnTo, parentState } });
      return;
    }
    endRule.reset();
    extendRule.reset();
    deleteRule.reset();
    setOpen(action);
  };
  const close = (): void => setOpen(null);
  const stop = (e: MouseEvent): void => e.stopPropagation();

  return (
    <Box onClick={stop} onKeyDown={(e) => e.stopPropagation()} sx={{ display: "inline-flex", gap: 1 }}>
      {variant === "menu" ? (
        <>
          <IconButton
            size="small"
            aria-label={`Actions for ${rule.name}`}
            aria-haspopup="menu"
            onClick={(e: MouseEvent<HTMLElement>) => setMenuAnchor(e.currentTarget)}
          >
            <MoreVertical size={16} />
          </IconButton>
          <Menu anchorEl={menuAnchor} open={!!menuAnchor} onClose={() => setMenuAnchor(null)}>
            {actions.map((a) => (
              <MenuItem key={a} onClick={() => run(a)} sx={a === "delete" ? { color: "error.main" } : undefined}>
                <ListItemIcon sx={a === "delete" ? { color: "error.main" } : undefined}>{ACTION_META[a].icon}</ListItemIcon>
                <ListItemText>{ACTION_META[a].label}</ListItemText>
              </MenuItem>
            ))}
          </Menu>
        </>
      ) : (
        actions.map((a) => (
          <Button
            key={a}
            size="small"
            variant={a === "end" ? "contained" : "outlined"}
            color={a === "delete" ? "error" : a === "end" ? "warning" : "primary"}
            startIcon={ACTION_META[a].icon}
            onClick={() => run(a)}
          >
            {ACTION_META[a].label}
          </Button>
        ))
      )}

      {open === "extend" && (
        <ExtendAlertMuteRuleDialog
          rule={rule}
          isSubmitting={extendRule.isPending}
          error={extendRule.error?.message ?? null}
          onClose={close}
          onSubmit={(endOn) => extendRule.mutate({ id: rule.id, endOn }, { onSuccess: close })}
        />
      )}

      {(open === "end" || open === "delete") && (
        <Dialog open onClose={close} maxWidth="xs" fullWidth aria-labelledby="alert-mute-rule-confirm-title">
          <DialogTitle id="alert-mute-rule-confirm-title">
            {open === "end" ? `End "${rule.name}" now?` : `Delete "${rule.name}"?`}
          </DialogTitle>
          <DialogContent>
            {(open === "end" ? endRule.error : deleteRule.error) && (
              <Alert severity="error" role="alert" sx={{ mb: 2 }}>
                {(open === "end" ? endRule.error : deleteRule.error)?.message}
              </Alert>
            )}
            <DialogContentText>
              {open === "end"
                ? "Matching alerts page again from now. Any held alert that is still firing becomes an incident."
                : "The rule hasn't started, so nothing has been muted by it. This can't be undone."}
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button color="inherit" onClick={close}>
              Cancel
            </Button>
            {open === "end" ? (
              <Button
                variant="contained"
                color="warning"
                loading={endRule.isPending}
                onClick={() => endRule.mutate(rule.id, { onSuccess: close })}
              >
                End now
              </Button>
            ) : (
              <Button
                variant="contained"
                color="error"
                loading={deleteRule.isPending}
                onClick={() =>
                  deleteRule.mutate(rule.id, {
                    onSuccess: () => {
                      close();
                      onDeleted?.();
                    },
                  })
                }
              >
                Delete
              </Button>
            )}
          </DialogActions>
        </Dialog>
      )}
    </Box>
  );
}
