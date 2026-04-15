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

import { Box, Card, Typography } from "@wso2/oxygen-ui";
import { type JSX, type ReactNode } from "react";

export interface GenericSubCountCardProps {
  label: string;
  value: string | number | ReactNode;
  icon: ReactNode;
  color?: string;
  footerContent?: ReactNode;
}

/**
 * Generic stat card with label, value, icon, and optional footer.
 * @returns {JSX.Element}
 */
const GenericSubCountCard = ({
  label,
  value,
  icon,
  color = "primary.main",
  footerContent,
}: GenericSubCountCardProps): JSX.Element => {
  return (
    <Card
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
        p: 2.5,
        height: "100%",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 2 }}>
        <Box sx={{ color }}>{icon}</Box>
        <Box sx={{ flex: 1 }}>
          <Typography variant="h4">{value}</Typography>
          <Typography variant="body2" color="text.secondary">
            {label}
          </Typography>
        </Box>
      </Box>
      {footerContent && (
        <Box sx={{ borderTop: 1, borderColor: "divider", pt: 2 }}>
          {footerContent}
        </Box>
      )}
    </Card>
  );
};

export default GenericSubCountCard;
