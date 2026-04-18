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

import { Box, Form, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton loader for the ServiceRequestsList component.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function ServiceRequestsListSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2, 3, 4, 5].map((i) => (
        <Form.CardButton
          key={i}
          sx={{
            p: 3,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1,
          }}
        >
          <Form.CardHeader
            sx={{ p: 0 }}
            title={
              <Stack
                direction="row"
                spacing={1.5}
                alignItems="center"
                sx={{ mb: 1, flexWrap: "wrap" }}
              >
                <Skeleton variant="text" width={72} height={20} />
                <Skeleton variant="rounded" width={88} height={20} />
                <Skeleton variant="rounded" width={96} height={20} />
              </Stack>
            }
          />
          <Form.CardContent sx={{ p: 0 }}>
            <Skeleton variant="text" width="70%" height={32} sx={{ mb: 1 }} />
            <Skeleton variant="text" width="90%" height={20} />
            <Skeleton variant="text" width="85%" height={20} />
          </Form.CardContent>
          <Form.CardActions
            sx={{
              p: 0,
              justifyContent: "space-between",
              flexWrap: "wrap",
              gap: 2,
            }}
          >
            <Skeleton variant="text" width={120} height={16} />
            <Skeleton variant="text" width={100} height={16} />
          </Form.CardActions>
        </Form.CardButton>
      ))}
    </Box>
  );
}
