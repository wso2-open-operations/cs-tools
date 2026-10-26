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

import { Alert, Box, Chip, Skeleton, Switch, Typography } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useSlaFieldCatalog, useSetSlaFieldEnabled, useSlaWorkItemTypes } from "../api/useSlaFieldRegistry";

/**
 * Field Registry tab (SLA SPEC.md R5, R16): which work item types may carry
 * SLAs (developer-seeded, read-only here) and which of each type's fields
 * are available to the condition rule builder (the one thing an admin may
 * toggle).
 */
export default function CsmSlaFieldRegistryPage(): JSX.Element {
  const workItemTypes = useSlaWorkItemTypes();
  const [selectedType, setSelectedType] = useState<string | undefined>(undefined);
  const effectiveType = selectedType ?? workItemTypes.data?.[0]?.workItemType;
  const catalog = useSlaFieldCatalog(effectiveType);
  const setEnabled = useSetSlaFieldEnabled();

  if (workItemTypes.isLoading) return <Skeleton variant="rectangular" height={200} />;
  if (workItemTypes.isError) return <Alert severity="error">Failed to load SLA work item types.</Alert>;

  return (
    <Box>
      <Box sx={{ display: "flex", gap: 1, mb: 2 }}>
        {(workItemTypes.data ?? []).map((t) => (
          <Chip
            key={t.workItemType}
            label={t.workItemType}
            color={effectiveType === t.workItemType ? "primary" : "default"}
            onClick={() => setSelectedType(t.workItemType)}
          />
        ))}
      </Box>
      {catalog.isLoading ? <Skeleton variant="rectangular" height={300} /> : null}
      {catalog.isError ? <Alert severity="error">Failed to load the field catalog.</Alert> : null}
      {catalog.data ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {catalog.data.map((f) => (
            <Box key={f.id} sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Switch
                checked={f.enabled}
                disabled={setEnabled.isPending}
                onChange={(e) =>
                  setEnabled.mutate({ fieldCatalogEntryId: f.id, enabled: e.target.checked })
                }
              />
              <Typography variant="body2" sx={{ width: 220 }}>
                {f.displayLabel}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ width: 200 }}>
                {f.fieldKey} ({f.dataType})
              </Typography>
              {f.isStateField ? <Chip size="small" label="state field" /> : null}
            </Box>
          ))}
        </Box>
      ) : null}
    </Box>
  );
}
