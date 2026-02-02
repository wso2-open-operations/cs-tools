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

import { Box, Divider, Form, Typography, colors } from "@wso2/oxygen-ui";
import {
  Calendar,
  CircleAlert,
  MessageSquare,
} from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { formatProjectDate } from "@/utils/projectCard";


interface ProjectCardStatsProps {
  activeChats: number;
  date: string;
  openCases: number;
}

/**
 * Component to render the stats section (cases, chats, date) for the Project Card.
 *
 * @param {ProjectCardStatsProps} props - The props for the component.
 * @returns {JSX.Element} The rendered stats section.
 */
export default function ProjectCardStats({
  openCases,
  activeChats,
  date,
}: ProjectCardStatsProps): JSX.Element {
  return (
    <Form.CardContent sx={{ width: "100%", pb: 0 }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 1.5,
        }}
      >
        {/* Open Cases */}
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box
            display="flex"
            alignItems="center"
            gap={1}
            color="text.secondary"
          >
            <CircleAlert size={16} />
            <Typography variant="body2" color="inherit">
              Open Cases
            </Typography>
          </Box>
          <Typography variant="body2" color="primary">
            {openCases}
          </Typography>
        </Box>

        {/* Active Chats */}
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box
            display="flex"
            alignItems="center"
            gap={1}
            color="text.secondary"
          >
            <MessageSquare size={16} />
            <Typography variant="body2" color="inherit">
              Active Chats
            </Typography>
          </Box>
          <Typography variant="body2" color={colors.blue[500]}>
            {activeChats}
          </Typography>
        </Box>

        {/* Date */}
        <Box display="flex" alignItems="center" gap={1} color="text.secondary">
          <Calendar size={16} />
          <Typography variant="body2" color="inherit">
            {formatProjectDate(date)}
          </Typography>
        </Box>
        <Divider sx={{ width: "100%" }} />
      </Box>
    </Form.CardContent>
  );
}
