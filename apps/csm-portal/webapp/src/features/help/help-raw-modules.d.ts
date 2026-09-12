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

// `vite/client`'s own ambient types cover `*.svg`/`*.png`/etc. as default-export
// asset URLs, but not Vite's `?raw` suffix (a source-as-string import) — needed
// here to pull each help topic's Markdown file in as plain text at build time.
// Scoped to this feature folder's own declaration file rather than added
// globally, since it's the only place in the app that currently needs it.
declare module "*.md?raw" {
  const content: string;
  export default content;
}
