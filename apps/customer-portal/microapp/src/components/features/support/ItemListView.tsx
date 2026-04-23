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

import { Link } from "react-router-dom";
import { Stack, Typography, Divider, Button, useTheme, pxToRem } from "@wso2/oxygen-ui";
import { ChevronRight } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface ItemListViewProps {
  title: string;
  viewAllPath: string;
  children: ReactNode;
}

export function ItemListView({ title, viewAllPath, children }: ItemListViewProps) {
  const theme = useTheme();

  return (
    <>
      <Stack direction="row" justifyContent="space-between" alignItems="center" pb={1}>
        <Typography variant="h6">{title}</Typography>
        <Button variant="text" component={Link} to={viewAllPath} sx={{ textTransform: "initial" }}>
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
    </>
  );
}
