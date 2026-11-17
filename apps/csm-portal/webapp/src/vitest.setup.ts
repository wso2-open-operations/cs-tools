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

// Global vitest setup. The runtime config normally comes from public/config.js
// (gitignored, injected at deploy time), which vitest does not load. Modules
// such as apiConfig.ts read it at import time and throw when it is missing, so
// seed obviously-fake defaults here. Tests that need specific values override
// them (spread keeps whatever they set).
window.config = {
  ...window.config,
  CSM_PORTAL_BACKEND_BASE_URL: window.config?.CSM_PORTAL_BACKEND_BASE_URL ?? "http://localhost:0",
};
