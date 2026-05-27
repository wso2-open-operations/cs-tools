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
import type { ReactNode } from "react";

import { Button, Card, Divider, pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";

import { SUPPORT_TAB_VIEW_CONFIG } from "@shared/constants";
import { useNavigation } from "@shared/hooks";
import type { CaseType } from "@shared/types";

export function ItemListWrapper({ type, children }: { type: CaseType; children: ReactNode }) {
  const theme = useTheme();
  const { toAll } = useNavigation();

  return (
    <Card component={Stack} p={2} mt={2} gap={0.5}>
      <Stack direction="row" justifyContent="space-between" alignItems="center" pb={1} width="100%">
        <Stack mr={2} minWidth={0}>
          <Typography variant="h6">{SUPPORT_TAB_VIEW_CONFIG[type].title}</Typography>

          <Typography variant="subtitle2" color="text.secondary" noWrap>
            {SUPPORT_TAB_VIEW_CONFIG[type].subtitle}
          </Typography>
        </Stack>
        <Button variant="text" sx={{ flexShrink: 0 }} onClick={() => toAll(type)}>
          <Stack direction="row" gap={1}>
            <Typography variant="body1" color="primary">
              View All
            </Typography>
            <ChevronRight size={pxToRem(18)} color={theme.palette.primary.main} />
          </Stack>
        </Button>
      </Stack>
      <Divider />
      <Stack gap={2} pt={2}>
        {children}
      </Stack>
    </Card>
  );
}
