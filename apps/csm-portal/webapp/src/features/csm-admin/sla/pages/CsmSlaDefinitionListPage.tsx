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

import { Alert, Box, Button, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import { useSlaDefinitions } from "../api/useSlaDefinitions";
import type { SlaDefinitionStatus } from "../types";

const STATUS_COLOR: Record<SlaDefinitionStatus, "default" | "success" | "warning"> = {
  draft: "default",
  published: "success",
  retired: "warning",
};

/** Lists every SLA definition (R1-R21), newest first. Nothing here is
 *  tracking real work items until it is published (R9, R14). */
export default function CsmSlaDefinitionListPage(): JSX.Element {
  const definitions = useSlaDefinitions();
  const navigate = useNavigate();

  return (
    <Box>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 2 }}>
        <Typography variant="body2" color="text.secondary">
          {definitions.data ? `${definitions.data.total} definition(s)` : null}
        </Typography>
        <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => navigate("/admin/sla/definitions/new")}>
          New definition
        </Button>
      </Box>
      {definitions.isLoading ? <Skeleton variant="rectangular" height={300} /> : null}
      {definitions.isError ? <Alert severity="error">Failed to load SLA definitions.</Alert> : null}
      {definitions.data ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {definitions.data.definitions.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No SLA definitions yet.
            </Typography>
          ) : null}
          {definitions.data.definitions.map((d) => (
            <Box
              key={d.id}
              onClick={() => navigate(`/admin/sla/definitions/${d.id}`)}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 2,
                p: 1.5,
                border: "1px solid",
                borderColor: "divider",
                borderRadius: 1,
                cursor: "pointer",
                "&:hover": { backgroundColor: "action.hover" },
              }}
            >
              <Chip size="small" label={d.status} color={STATUS_COLOR[d.status]} />
              <Typography variant="body1" sx={{ flex: 1 }}>
                {d.name}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {d.workItemType} · {d.targetLabel}
              </Typography>
            </Box>
          ))}
        </Box>
      ) : null}
    </Box>
  );
}
