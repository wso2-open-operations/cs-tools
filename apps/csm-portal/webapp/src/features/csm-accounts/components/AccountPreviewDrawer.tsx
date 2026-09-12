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

import { Box, Button, Chip, Divider, Drawer, IconButton, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import RelativeTime from "@components/RelativeTime";
import { resolveAccountTier, type Account } from "@features/csm-accounts/types/csmAccounts";

interface AccountPreviewDrawerProps {
  /** The account being previewed. `null` keeps the drawer mounted-but-closed,
   * so its close transition can play instead of unmounting mid-animation. */
  account: Account | null;
  onClose: () => void;
}

function Field({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" noWrap>
        {children}
      </Typography>
    </Box>
  );
}

/**
 * Read-only "quick look" for an account row — renders only fields already
 * present on the `/accounts/search` response (`Account`), the same data
 * already on screen in the widget's own table row, no extra fetch. Mirrors
 * `CasePreviewDrawer`'s shell.
 */
export default function AccountPreviewDrawer({
  account,
  onClose,
}: AccountPreviewDrawerProps): JSX.Element {
  const tier = account ? resolveAccountTier(account) : undefined;
  return (
    <Drawer
      anchor="right"
      open={!!account}
      onClose={onClose}
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 420 } } } }}
    >
      {account && (
        <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
          <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
              <Typography variant="h6" sx={{ wordBreak: "break-word" }}>
                {account.name}
              </Typography>
              <IconButton size="small" onClick={onClose} aria-label="Close preview">
                <X size={18} />
              </IconButton>
            </Box>
            {tier && (
              <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
                <Chip size="small" variant="outlined" label={tier} />
              </Box>
            )}
          </Box>

          <Divider />

          <Box sx={{ flex: 1, overflowY: "auto", p: 2.5 }}>
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <Field label="Region">{account.region || "—"}</Field>
              <Field label="Account manager">{account.accountManager?.name || "—"}</Field>
              <Field label="Technical owner">{account.technicalOwner?.name || "—"}</Field>
              <Field label="CRE team">{account.creTeam?.name || "—"}</Field>
              <Field label="Activation date">
                {account.activationDate ? <RelativeTime iso={account.activationDate} /> : "—"}
              </Field>
              <Field label="Updated">
                {account.updatedOn ? <RelativeTime iso={account.updatedOn} /> : "—"}
              </Field>
              <Field label="Agent enabled">{account.hasAgent ? "Yes" : "No"}</Field>
              <Field label="KB references enabled">{account.hasKbReferences ? "Yes" : "No"}</Field>
            </Box>
          </Box>

          <Divider />

          <Box sx={{ p: 2 }}>
            <Button
              component={RouterLink}
              to={`/customers/accounts/${account.id}`}
              variant="contained"
              fullWidth
              onClick={onClose}
              disabled={!account.id}
            >
              View full details
            </Button>
          </Box>
        </Box>
      )}
    </Drawer>
  );
}
