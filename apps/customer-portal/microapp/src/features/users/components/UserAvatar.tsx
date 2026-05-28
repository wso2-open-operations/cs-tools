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
import { getAvatarInitials } from "@root/src/shared/utils";
import { Avatar } from "@wso2/oxygen-ui";

export function UserAvatar({ size = "medium", children }: { size?: "medium" | "large"; children: string }) {
  return (
    <Avatar
      sx={(theme) => ({
        height: size === "large" ? 52 : 36,
        width: size === "large" ? 52 : 36,
        bgcolor: "primary.main",
        fontSize: size === "large" ? theme.typography.h3 : theme.typography.h5,
        fontWeight: "medium",
      })}
    >
      {getAvatarInitials(children)}
    </Avatar>
  );
}
