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
import { type ReactNode, useState } from "react";

import { Box, Chip, Collapse, Stack, Typography } from "@wso2/oxygen-ui";
import { ListChevronsDownUp, ListChevronsUpDown } from "@wso2/oxygen-ui-icons-react";

import { CASE_TYPE_PLURAL_LABELS } from "@shared/constants";
import type { CaseType } from "@shared/types";

export function GroupAccordion({ type, count, children }: { type: CaseType; count?: number; children: ReactNode }) {
  const [open, setOpen] = useState(true);

  return (
    <Box
      sx={{
        bgcolor: "transparent",
        pb: 1,
        mx: -2,
        borderBottom: "2px solid",
        borderColor: "divider",
        display: count === 0 ? "none" : "block",
      }}
    >
      <Stack
        direction="row"
        onClick={() => setOpen(!open)}
        sx={{
          cursor: "pointer",
          mb: 1,
          justifyContent: "space-between",
          p: 1,
        }}
      >
        <Stack direction="row" alignItems="center" gap={1}>
          <Typography variant="body1" fontWeight="medium" sx={{ flex: 1 }}>
            {CASE_TYPE_PLURAL_LABELS[type]}
          </Typography>

          {count !== undefined && <Chip size="small" label={count} />}
        </Stack>

        <Stack direction="row" gap={1} alignItems="center">
          {open ? <ListChevronsDownUp size={19} /> : <ListChevronsUpDown size={19} />}
        </Stack>
      </Stack>

      <Collapse in={open}>
        <Stack gap={2} px={2} py={1}>
          {children}
        </Stack>
      </Collapse>
    </Box>
  );
}
