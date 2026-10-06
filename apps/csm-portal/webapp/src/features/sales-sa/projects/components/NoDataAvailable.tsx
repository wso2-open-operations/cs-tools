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

import { Box, Paper, Typography } from "@wso2/oxygen-ui";
import { InboxIcon } from "@wso2/oxygen-ui-icons-react";

export default function NoDataAvailable({
  message = "No data available",
  description = "There are no items to display at the moment.",
}: {
  message?: string;
  description?: string;
}) {
  return (
    <Paper
      sx={{
        p: 4,
        textAlign: "center",
        backgroundColor: "background.default",
        border: "1px dashed",
        borderColor: "divider",
        minHeight: 200,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      <Box sx={{ color: "text.disabled", mb: 2 }}>
        <InboxIcon size={64} />
      </Box>
      <Typography variant="h6" sx={{ color: "text.secondary", mb: 1, fontWeight: 500 }}>
        {message}
      </Typography>
      <Box>
        <Typography variant="body2" sx={{ color: "text.disabled", maxWidth: 300 }}>
          {description}
        </Typography>
      </Box>
    </Paper>
  );
}
