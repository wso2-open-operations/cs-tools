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
  Card,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import AdminTabs from "@features/csm-admin/components/AdminTabs";
import type { JSX } from "react";

/** Service Request catalog — items the customer can pick from when raising an SR. */
interface CatalogRow {
  id: string;
  name: string;
  category: "Service" | "Change" | "Information" | "Security";
  appliesTo: string;
  approvalRequired: boolean;
  active: boolean;
}

const ITEMS: CatalogRow[] = [
  { id: "cat.sr.cert_renewal", name: "Certificate renewal", category: "Service", appliesTo: "Managed cloud (APIM/IS)", approvalRequired: false, active: true },
  { id: "cat.sr.scale_up", name: "Infrastructure scale-up", category: "Change", appliesTo: "Managed cloud (Choreo)", approvalRequired: true, active: true },
  { id: "cat.sr.config_change", name: "Configuration change", category: "Change", appliesTo: "Managed cloud", approvalRequired: true, active: true },
  { id: "cat.sr.restart", name: "Service restart", category: "Service", appliesTo: "Managed cloud", approvalRequired: false, active: true },
  { id: "cat.sr.upgrade", name: "Product version upgrade", category: "Change", appliesTo: "Managed cloud", approvalRequired: true, active: true },
  { id: "cat.sr.security_patch", name: "Security patch application", category: "Security", appliesTo: "Managed cloud", approvalRequired: true, active: true },
  { id: "cat.sr.log_request", name: "Log / heap dump pull", category: "Information", appliesTo: "All subscriptions", approvalRequired: false, active: true },
  { id: "cat.sr.invoice", name: "Invoice clarification", category: "Information", appliesTo: "All subscriptions", approvalRequired: false, active: true },
  { id: "cat.sr.proxy_case", name: "On-behalf-of case creation", category: "Service", appliesTo: "All subscriptions", approvalRequired: false, active: true },
  { id: "cat.sr.legacy_eol", name: "Legacy EOL extension request", category: "Change", appliesTo: "EOL products", approvalRequired: true, active: false },
];

function catColor(c: CatalogRow["category"]): "primary" | "info" | "success" | "warning" {
  switch (c) {
    case "Service": return "primary";
    case "Change": return "warning";
    case "Information": return "info";
    case "Security": return "success";
  }
}

export default function CsmAdminCatalogPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Item</TableCell>
                <TableCell>Category</TableCell>
                <TableCell>Applies to</TableCell>
                <TableCell>Approval</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {ITEMS.map((c) => (
                <TableRow key={c.id} hover>
                  <TableCell>{c.name}</TableCell>
                  <TableCell>
                    <Chip size="small" label={c.category} color={catColor(c.category)} variant="outlined" />
                  </TableCell>
                  <TableCell>{c.appliesTo}</TableCell>
                  <TableCell>
                    {c.approvalRequired ? (
                      <Chip size="small" label="Required" color="warning" variant="outlined" />
                    ) : (
                      <Chip size="small" label="None" color="success" variant="outlined" />
                    )}
                  </TableCell>
                  <TableCell>
                    {c.active ? (
                      <Chip size="small" label="Active" color="success" variant="outlined" />
                    ) : (
                      <Chip size="small" label="Inactive" color="default" variant="outlined" />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Catalog items appear in the customer portal SR picker. Per-item variables (e.g. new instance size) wire up with the catalog backend.
      </Typography>
    </Box>
  );
}
