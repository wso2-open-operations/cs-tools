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
import { Card, CardActionArea, Stack, Typography, type CardProps } from "@wso2/oxygen-ui";

interface WidgetBoxProps extends Omit<CardProps, "variant"> {
  title?: string;
  children: ReactNode;
}

export function WidgetBox({ title, children, ...props }: WidgetBoxProps) {
  return (
    <Card sx={{ height: "100%", bgcolor: "background.paper" }} {...props}>
      <CardActionArea disabled={!props.onClick} sx={{ p: 1.2 }}>
        <Stack gap={0.5}>
          {title && (
            <Stack direction="row" justifyContent="space-between" alignItems="center">
              <Typography variant="body1" fontWeight="medium" color="text.primary">
                {title}
              </Typography>
            </Stack>
          )}
          {children}
        </Stack>
      </CardActionArea>
    </Card>
  );
}
