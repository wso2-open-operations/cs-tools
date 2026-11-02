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

import { Alert, Box, Link, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeChangeRequestStandardChange } from "@api/backend/types";
import { standardChangeCustomerMismatchWarning } from "@features/csm-operations/utils/standardChanges";

/**
 * On a change request raised from a standard change template: which template and
 * version it follows (a link to the template) and the change specifics it was
 * raised with, by label, and the template's customer (the account of the change
 * request's project). What the template specifies is frozen on the change
 * request; the parameters cannot be edited either.
 *
 * When the project is no longer the template's customer's (`customerMismatch`:
 * the project sync, the source of truth for projects, moved it to another
 * account after the change was raised) the banner turns into a warning that
 * says so: the backend refuses Request Approval of such a change (400
 * `standard_change_template_other_customer`).
 */
export default function StandardChangeTemplateBanner({
  standardChange,
  onOpenTemplate,
}: {
  standardChange: BeChangeRequestStandardChange;
  onOpenTemplate: () => void;
}): JSX.Element {
  const parameters = standardChange.parameters ?? [];
  const mismatch = standardChange.customerMismatch === true;
  return (
    <Alert severity={mismatch ? "warning" : "info"} role="region" aria-label="Standard change template">
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        Raised from standard change template{" "}
        <Link component="button" variant="body2" onClick={onOpenTemplate} sx={{ fontWeight: 600, verticalAlign: "baseline" }}>
          {standardChange.templateNumber}
          {standardChange.template.name ? ` ${standardChange.template.name}` : ""}
        </Link>{" "}
        (version {standardChange.version})
      </Typography>
      {standardChange.customer?.name && (
        <Typography variant="body2">
          {`Customer: ${standardChange.customer.name}. The template is used only for this customer's projects.`}
        </Typography>
      )}
      {mismatch && (
        <Typography variant="body2" sx={{ fontWeight: 600 }} data-testid="standard-change-customer-mismatch">
          {standardChangeCustomerMismatchWarning(standardChange.customer?.name)}
        </Typography>
      )}
      <Typography variant="body2" color="text.secondary">
        What the template specifies cannot be changed on this change request.
      </Typography>
      {parameters.length > 0 && (
        <Box
          component="dl"
          sx={{
            display: "grid",
            gridTemplateColumns: "max-content minmax(0, 1fr)",
            columnGap: 2,
            rowGap: 0.5,
            m: 0,
            mt: 1,
          }}
        >
          {parameters.map((p) => (
            <Box key={p.key} sx={{ display: "contents" }}>
              <Typography component="dt" variant="body2" color="text.secondary">
                {p.label}
              </Typography>
              <Typography component="dd" variant="body2" sx={{ m: 0, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
                {p.value?.trim() ? p.value : "—"}
              </Typography>
            </Box>
          ))}
        </Box>
      )}
    </Alert>
  );
}
