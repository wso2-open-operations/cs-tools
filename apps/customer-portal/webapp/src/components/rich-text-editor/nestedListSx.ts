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

/**
 * Styles for HTML saved by the rich text editor, for the views that show it.
 *
 * The editor stores a nested list inside a wrapper <li> that holds only the
 * sub-list. Without this the wrapper draws its own bullet/number, uses up a
 * number in ordered lists, and indents the sub-list twice. Matching on the
 * structure (not the editor's class) also covers HTML where the class was
 * stripped by sanitising.
 */
export const NESTED_LIST_SX = {
  "& li:has(> ul:only-child), & li:has(> ol:only-child)": {
    listStyleType: "none",
    counterIncrement: "list-item 0",
    "&::marker": { content: "none" },
    "& > ul, & > ol": { ml: 0, pl: 2.5 },
  },
};
