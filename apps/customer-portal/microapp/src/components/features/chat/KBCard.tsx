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

import { Box, Card, Stack, Typography, pxToRem, colors } from "@wso2/oxygen-ui";
import { BookOpen } from "@wso2/oxygen-ui-icons-react";

export function KBCard({ id, title }: { id: string; title: string }) {
  return (
    <Card component={Stack} direction="row" alignItems="center" gap={2} p={1} sx={{ bgcolor: "background.default" }}>
      <Box color={colors.blue[500]}>
        <BookOpen size={pxToRem(18)} />
      </Box>
      <Stack>
        <Typography variant="body2">{title}</Typography>
        <Typography variant="caption" color="text.secondary">
          {id}
        </Typography>
      </Stack>
    </Card>
  );
}
