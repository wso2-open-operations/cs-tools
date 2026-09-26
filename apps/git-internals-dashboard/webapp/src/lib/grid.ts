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

// Shared grid templates so issue-list headers and rows stay aligned.
export type IssueRowVariant = "full" | "compact";
// full    = Issue | Project | Opened by | Pri | Status | SLA state | SLA Elapsed % | Created | Updated (/issues)
// compact = Issue | Project | Pri | Status | Budget | Age                                              (attention set)
export const gridTemplate = (variant: IssueRowVariant): string =>
  variant === "full"
    ? "minmax(0,1fr) 150px 190px 48px 160px 120px 150px 96px 96px"
    : "minmax(0,1fr) 158px 56px 168px 150px 56px";
