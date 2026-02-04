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

import { sloganListItems } from "@/constants/loginScreenConstants";
import { Stack, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";

export default function LoginSlogan(): JSX.Element {
  return (
    <>
      <Typography variant="h3" sx={{ fontWeight: "bold", mb: 1 }}>
        Welcome to the WSO2 Customer Portal
      </Typography>
      <Typography variant="body1" sx={{ color: "text.secondary", mb: 4 }}>
        Manage your projects, track support cases, and connect with our experts
        in one place.
      </Typography>

      <Stack sx={{ gap: 3 }}>
        {sloganListItems.map((item) => (
          <Stack
            key={item.title}
            direction="row"
            alignItems="center"
            sx={{ gap: 2 }}
          >
            <div style={{ display: "flex", alignItems: "center" }}>
              {item.icon}
            </div>
            <Typography sx={{ fontWeight: "medium", fontSize: "1.1rem" }}>
              {item.title}
            </Typography>
          </Stack>
        ))}
      </Stack>
    </>
  );
}
