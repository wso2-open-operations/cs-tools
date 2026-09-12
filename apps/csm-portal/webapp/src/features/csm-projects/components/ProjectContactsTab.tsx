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
  Chip,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import UserRefLink from "@components/UserRefLink";
import { useSearchProjectContacts } from "@features/csm-projects/api/useSearchProjectContacts";
import type { BeProjectContact } from "@api/backend/types";

const COLUMN_COUNT = 5;

type ChipColor = "success" | "warning" | "error" | "default";

// Mirrors the customer portal's own registration-status colour convention
// (`getUserStatusColor` in `project-details/utils/projectDetails.ts`) so the
// two surfaces agree on what "registered" vs "invited" looks like, without
// importing across the customer-portal/CSM-portal boundary.
function registrationChipColor(state: string | undefined): ChipColor {
  switch ((state ?? "").toLowerCase()) {
    case "registered":
      return "success";
    case "invited":
      return "warning";
    default:
      return "default";
  }
}

interface AccessStatus {
  label: string;
  color: ChipColor;
  reason?: string;
}

// Prefers the explicit access-status fields the backend computes per row
// (customerContactPresent/grantsCaseAccess). Falls back to the old
// id-presence heuristic when a backend predates those fields
// (`grantsCaseAccess` absent), so a backend deploy that hasn't caught up yet
// degrades to the previous behaviour instead of mislabeling every row.
function deriveAccessStatus(c: BeProjectContact): AccessStatus {
  if (c.grantsCaseAccess !== undefined) {
    if (c.grantsCaseAccess) {
      return { label: "Has access", color: "success" };
    }
    // Two distinct faults behind one verdict, and which one it is changes
    // what you do about it: no contact record means the invite never
    // completed, so chase the onboarding; a linked record whose own address
    // differs means the row names the wrong address, so fix the row. When
    // `customerContactPresent` is absent (a backend that has one field but
    // not the other), the missing-record reading is the safer default — it is
    // both the commoner fault and the one that doesn't accuse a healthy row
    // of naming the wrong person.
    return {
      label: "No access",
      color: "error",
      reason: c.customerContactPresent
        ? "Invited under a different address than its linked contact's own — this project, and every case on it, is invisible to both of them."
        : "No linked contact record — the invite never completed, so this person can't see this project's cases.",
    };
  }
  if (!c.id) {
    return {
      label: "Orphaned",
      color: "error",
      reason: "No linked contact record — this person can't see this project's cases.",
    };
  }
  return { label: "Has access", color: "success" };
}

interface ProjectContactsTabProps {
  projectId: string;
}

/**
 * Lists a project's contacts (`POST /projects/{id}/contacts/search`), with an
 * Access column showing whether each row would actually grant its person
 * visibility into this project's cases — not just whether they're listed.
 * Every row with a linked contact record is click-through to that person's
 * profile via `UserRefLink` (so nullable-id resolution and the plain-text
 * fallback come for free); a row that would fail the customer portal's own
 * access rule renders unlinked-or-flagged with an inline reason, since
 * that's precisely the case a support engineer needs to notice without
 * hovering — a row can fail access two distinct ways (no linked contact
 * record at all, or a linked contact whose email doesn't match what this row
 * was invited under), and the reason line says which.
 *
 * A real project can return dozens of failed-invite rows. Those have no
 * `name` (it is only ever known from the contact record) but do carry the
 * address they were invited under as `email`, so they are identifiable — and
 * the explanatory line is always rendered inline, not tucked behind a hover
 * tooltip, so scanning the table surfaces every "can't see their cases" row
 * without hovering each one.
 */
export default function ProjectContactsTab({
  projectId,
}: ProjectContactsTabProps): JSX.Element {
  const { data, isLoading, isError, error } = useSearchProjectContacts(projectId);
  const contacts = data ?? [];

  return (
    <Paper variant="outlined">
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Roles</TableCell>
              <TableCell>Registration</TableCell>
              <TableCell>Access</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  {Array.from({ length: COLUMN_COUNT }).map((__, c) => (
                    <TableCell key={c}>
                      <Skeleton variant="rounded" width="70%" height={18} />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center">
                  <QueryErrorState
                    message={error instanceof Error && error.message.trim() ? error.message : "Failed to load project contacts."}
                    error={error}
                  />
                </TableCell>
              </TableRow>
            ) : contacts.length === 0 ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No contacts found for this project.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              contacts.map((c, i) => {
                // No natural identifier at all when both are empty — say so
                // plainly instead of showing a bare "—" that reads as a data
                // glitch rather than the operationally meaningful fact that
                // this row has no linked contact record.
                const name = c.name || c.email || "No linked contact record";
                const access = deriveAccessStatus(c);
                return (
                  // Contacts have no stable identifier when unlinked (no `id`);
                  // email is the closest thing to a natural key, and index
                  // disambiguates the rare case of a duplicate/blank email.
                  <TableRow key={c.id ?? `${c.email ?? "unknown"}-${i}`} hover>
                    <TableCell sx={{ maxWidth: 280 }}>
                      <UserRefLink name={name} email={c.email} userId={c.id ?? null} />
                    </TableCell>
                    <TableCell sx={{ wordBreak: "break-all" }}>{c.email || "—"}</TableCell>
                    <TableCell>
                      {c.roles && c.roles.length > 0
                        ? c.roles.map((r) => (
                            <Chip
                              key={r}
                              size="small"
                              label={r}
                              variant="outlined"
                              sx={{ mr: 0.5, mb: 0.5 }}
                            />
                          ))
                        : "—"}
                    </TableCell>
                    <TableCell>
                      {c.registrationState ? (
                        <Chip
                          size="small"
                          label={c.registrationState}
                          color={registrationChipColor(c.registrationState)}
                          variant="outlined"
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip size="small" label={access.label} color={access.color} variant="outlined" />
                      {access.reason && (
                        <Typography
                          variant="caption"
                          color="error.main"
                          component="div"
                          sx={{ mt: 0.25 }}
                        >
                          {access.reason}
                        </Typography>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Paper>
  );
}
