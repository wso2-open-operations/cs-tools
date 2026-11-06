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
  Link,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  useCreateStandardChangeProposal,
  useSearchStandardChangeCustomersForSelect,
  useSearchStandardChangeTemplates,
} from "@features/csm-operations/api/useStandardChanges";
import { errorText } from "@features/csm-operations/utils/changeRequests";
import {
  MAX_STANDARD_CHANGE_NAME,
  STANDARD_CHANGE_ONE_CUSTOMER_RULE,
} from "@features/csm-operations/utils/standardChanges";
import type { BeEntityRef, BeStandardChangeProposal } from "@api/backend/types";

/** How many of the customer's templates the dialog lists. */
const EXISTING_SHOWN = 8;

/**
 * "Propose a standard change": the name and the customer first, then the draft.
 * A draft is created only once both are given, so leaving here leaves nothing
 * behind (no empty "Untitled standard change" drafts). Once the customer is
 * chosen, their live templates are listed: a change one of them already covers
 * is a revision of it, proposed from the template's own page.
 */
export default function ProposeStandardChangeDialog({
  open,
  onClose,
  onCreated,
  onOpenTemplate,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (proposal: BeStandardChangeProposal) => void;
  onOpenTemplate: (id: string) => void;
}): JSX.Element {
  const [name, setName] = useState("");
  const [customer, setCustomer] = useState<BeEntityRef | null>(null);
  const create = useCreateStandardChangeProposal();
  const existing = useSearchStandardChangeTemplates(
    { accountId: customer?.id, pagination: { offset: 0, limit: EXISTING_SHOWN } },
    { enabled: open && !!customer },
  );
  // Not the previous customer's list while the new one loads (the hook keeps it as placeholder data).
  const current = !!customer && !existing.isPlaceholderData;
  const templates = current ? (existing.data?.templates ?? []) : [];
  const more = current && existing.data ? existing.data.total - templates.length : 0;
  const ready = !!name.trim() && !!customer;

  const reset = (): void => {
    setName("");
    setCustomer(null);
    create.reset();
  };

  const submit = (): void => {
    if (!ready) return;
    create.mutate({ kind: "new", name: name.trim(), accountId: customer!.id }, { onSuccess: onCreated });
  };

  return (
    <Dialog
      open={open}
      onClose={create.isPending ? undefined : onClose}
      aria-labelledby="std-propose-title"
      fullWidth
      maxWidth="sm"
      slotProps={{ transition: { onExited: reset } }}
    >
      <DialogTitle id="std-propose-title">Propose a standard change</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <DialogContentText>{STANDARD_CHANGE_ONE_CUSTOMER_RULE}</DialogContentText>
        {create.isError && <Alert severity="error">{errorText(create.error, "Could not start the proposal.")}</Alert>}
        <TextField
          label="Name"
          required
          size="small"
          autoFocus
          value={name}
          disabled={create.isPending}
          onChange={(e) => setName(e.target.value.slice(0, MAX_STANDARD_CHANGE_NAME))}
          helperText="The template's name in the catalogue. You can change it in the draft."
        />
        <AsyncEntitySelect<BeEntityRef>
          id="std-propose-customer"
          label="Customer"
          placeholder="Search customers…"
          required
          value={customer?.id ?? ""}
          knownLabel={customer?.name}
          onChange={(next, item) => setCustomer(next ? { id: next, name: item?.name ?? "" } : null)}
          disabled={create.isPending}
          useSearch={useSearchStandardChangeCustomersForSelect}
          getId={(a) => a.id}
          getLabel={(a) => a.name}
          helperText="The customer the template is for."
        />
        {customer && templates.length > 0 && (
          <Box component="section" aria-label={`${customer.name}'s templates`} sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography variant="subtitle2">{`${customer.name} already has these templates`}</Typography>
            <Typography variant="body2" color="text.secondary">
              If one of them covers this change, open it and propose a revision instead of a new template.
            </Typography>
            <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
              {templates.map((t) => (
                <li key={t.id}>
                  <Link component="button" variant="body2" onClick={() => onOpenTemplate(t.id)}>
                    {`${t.number} ${t.name}`}
                  </Link>
                  {t.currentVersion?.summary && (
                    <Typography variant="caption" color="text.secondary" sx={{ display: "block" }}>
                      {t.currentVersion.summary}
                    </Typography>
                  )}
                </li>
              ))}
            </Box>
            {more > 0 && (
              <Typography variant="caption" color="text.secondary">{`and ${more} more in the catalogue.`}</Typography>
            )}
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={create.isPending}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!ready} loading={create.isPending} onClick={submit}>
          Start the draft
        </Button>
      </DialogActions>
    </Dialog>
  );
}
