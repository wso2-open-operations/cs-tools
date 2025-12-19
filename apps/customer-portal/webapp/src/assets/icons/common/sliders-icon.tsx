// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import React from "react";
import type { IconProps } from "@/types/icon.types";
import { BaseIcon } from "../base-icon";

export const SlidersIcon: React.FC<IconProps> = (props) => (
  <BaseIcon {...props}>
    <path d="M10 5H3"></path>
    <path d="M12 19H3"></path>
    <path d="M14 3v4"></path>
    <path d="M16 17v4"></path>
    <path d="M21 12h-9"></path>
    <path d="M21 19h-5"></path>
    <path d="M21 5h-7"></path>
    <path d="M8 10v4"></path>
    <path d="M8 12H3"></path>
  </BaseIcon>
);
