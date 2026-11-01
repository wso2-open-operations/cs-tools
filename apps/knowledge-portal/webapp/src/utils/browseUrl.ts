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

export interface BrowseParams {
  knowledgeBaseId?: string;
  q?: string;
}

/** `/knowledge-bases[/:id]` URL with the given filters, every value percent-encoded. */
export function browseUrl({ knowledgeBaseId, q }: BrowseParams = {}): string {
  const path = knowledgeBaseId
    ? `/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}`
    : "/knowledge-bases";
  return q ? `${path}?q=${encodeURIComponent(q)}` : path;
}

/** `/knowledge-articles/:id` URL. */
export const articleUrl = (id: string): string =>
  `/knowledge-articles/${encodeURIComponent(id)}`;
